package kube

import (
	"context"
	"errors"
	"time"

	"github.com/sirupsen/logrus"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubernetes/pkg/kubelet/types"
)

var (
	ErrContainerNotRecognized = errors.New("container not recognized")
	ErrInvalidPodUID          = errors.New("invalid pod UID")
	ErrInvalidContainerID     = errors.New("invalid container ID")
	ErrInvalidObject          = errors.New("invalid kube object type detected")
	ErrNotFound               = errors.New("not found")

	pkglog = logrus.StandardLogger().WithField("package", "resolver/kube")
)

const (
	// todo: configureable
	informerSyncDuration = time.Minute
	kubeNamespace        = "default"
	queryTimeout         = time.Second

	containerIdPrefix = "docker://"
)

type RequiredProps interface {
	DockerID() string
	DockerLabels() map[string]string
}

type Service interface {
	Lookup(context.Context, RequiredProps) (Props, error)
	Shutdown()

	// todo: Ready() <- chan struct{}

	Done() <-chan struct{}
}

func NewService(ctx context.Context) (Service, error) {
	ctx, cancel := context.WithCancel(ctx)

	client, err := defaultKubeClient()
	if err != nil {
		return nil, err
	}

	// ping kube
	list, err := client.CoreV1().Pods(kubeNamespace).List(metav1.ListOptions{})
	if err != nil {
		pkglog.WithError(err).Error("can't connect to kubernetes")
		return nil, err
	}

	pkglog.WithField("kube-pods", len(list.Items)).Debug("connected to kube")

	s := &service{
		client:    client,
		requestch: make(chan *lookupRequest),
		reqdonech: make(chan *lookupRequest),
		requests:  make(map[string][]*lookupRequest),
		recheckch: make(chan *v1.Pod),
		donech:    make(chan struct{}),
		log:       pkglog,
		cancel:    cancel,
		ctx:       ctx,
	}

	s.store, s.controller = cache.NewInformer(
		s.makeListWatch(),
		&v1.Pod{},
		informerSyncDuration,
		s.makeEventHandler(),
	)

	go s.run()

	return s, nil

}

type service struct {
	client     kubernetes.Interface
	store      cache.Store
	controller cache.Controller
	requestch  chan *lookupRequest
	reqdonech  chan *lookupRequest
	requests   map[string][]*lookupRequest
	recheckch  chan *v1.Pod
	donech     chan struct{}
	log        logrus.FieldLogger
	cancel     context.CancelFunc
	ctx        context.Context
}

type lookupRequest struct {
	ch     chan<- Props
	donech <-chan struct{}
	qp     queryParams
}

type queryParams struct {
	namespace     string
	podName       string
	containerName string
	podUID        string
	containerID   string
}

func (qp queryParams) key() string {
	return qp.namespace + "/" + qp.podName
}

func (s *service) Shutdown() {
	s.cancel()
	<-s.donech
}

func (s *service) Done() <-chan struct{} {
	return s.donech
}

func (s *service) Lookup(ctx context.Context, dprops RequiredProps) (Props, error) {
	log := s.log.WithField("docker-id", dprops.DockerID())

	log.Debug("resolving pod...")

	// extract kube properties from docker labels
	qp, err := queryParamsFromProps(dprops)
	if err != nil {
		log.WithError(err).Info("invalid docker properties")
		return nil, err
	}

	log = log.WithField("lookup-key", qp.key())

	// look for pod
	obj, found, err := s.store.GetByKey(qp.key())

	if err != nil {
		log.WithError(err).Error("store lookup")
		return nil, err
	}

	if found {

		// find kube properties for container
		props, found, err := s.matchQuery(qp, obj)
		if err != nil {
			return nil, err
		}
		if found {
			return props, nil
		}
	}

	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	ch := make(chan Props, 1)

	lookupRequest := &lookupRequest{ch, ctx.Done(), qp}

	// send request
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.ctx.Done():
		return nil, s.ctx.Err()
	case s.requestch <- lookupRequest:
	}

	// wait for response
	select {
	case <-ctx.Done():
		return nil, ErrNotFound
	case <-s.ctx.Done():
		return nil, s.ctx.Err()
	case props, ok := <-ch:
		if !ok {
			return nil, ErrNotFound
		}
		return props, nil
	}
}

func (s *service) run() {
	defer close(s.donech)
	defer s.cancel()

	log := s.log.WithField("method", "run")
	defer log.Debug("done")

	cdonech := make(chan struct{})

	go func() {
		defer close(cdonech)
		s.controller.Run(s.ctx.Done())
	}()

loop:
	for {
		select {
		case <-s.ctx.Done():
			break loop
		case req := <-s.requestch:
			s.handleRequest(req)
		case req := <-s.reqdonech:
			s.handleRequestDone(req)
		case pod := <-s.recheckch:
			s.handleRecheck(pod)
		}
	}

	log.Debugf("draining %v pod request", len(s.requests))

	for len(s.requests) > 0 {
		s.handleRequestDone(<-s.reqdonech)
	}

	<-cdonech
}

func (s *service) handleRequest(req *lookupRequest) {
	s.requests[req.qp.key()] = append(s.requests[req.qp.key()], req)
	go func() {
		<-req.donech
		s.reqdonech <- req
	}()
}

func (s *service) handleRequestDone(req *lookupRequest) {
	log := s.log.WithField("request-key", req.qp.key())

	requests, ok := s.requests[req.qp.key()]
	if !ok {
		return
	}

	log.Debug("removing request")

	for idx, item := range requests {
		if item == req {
			requests = append(requests[:idx], requests[idx+1:]...)
		}
	}

	log.Debugf("%v requests for pod remaining", len(requests))

	if len(requests) == 0 {
		delete(s.requests, req.qp.key())
		return
	}

	s.requests[req.qp.key()] = requests
}

func (s *service) handleRecheck(pod *v1.Pod) {
	key := pod.Namespace + "/" + pod.Name

	log := s.log.WithField("lookup-key", key)

	requests := s.requests[key]

	log.Debugf("rechecking %v requests", len(requests))

	for _, req := range requests {
		props, found, err := s.matchQuery(req.qp, pod)
		if err != nil {
			continue
		}
		if found {
			req.ch <- props
		}
	}

}

func (s *service) matchQuery(qp queryParams, obj interface{}) (Props, bool, error) {
	log := s.log.WithField("method", "matchQuery").
		WithField("lookup-key", qp.key())

	pod, ok := obj.(*v1.Pod)
	if !ok {
		return nil, false, ErrInvalidObject
	}

	if string(pod.UID) != qp.podUID {
		log.Warn("mismatched pod uid")
		return nil, false, ErrInvalidPodUID
	}

	// todo: what pod states are valid to continue?
	//       it might not be necessary to wait for running.

	// todo: check init containers
	for _, cs := range pod.Status.ContainerStatuses {

		// todo: check container state
		if cs.Name == qp.containerName {

			switch {

			case cs.ContainerID == "":
				// container not populated in kube yet.
				log.Debug("empty container id")
				return nil, false, nil

			case cs.ContainerID != containerIdPrefix+qp.containerID:

				log.
					WithField("kube.container-id", cs.ContainerID).
					WithField("docker.container-id", qp.containerID).
					Warn("mismatched container id")
				return nil, false, ErrInvalidContainerID

			default:

				log.
					WithField("kube-ns", pod.Namespace).
					WithField("kube-pod", pod.Name).
					WithField("kube-container", cs.Name).
					WithField("docker-container", cs.ContainerID).
					Debug("container found")

				return newProps(pod, &cs), true, nil

			}
		}
	}

	log.Debug("container not found")

	return nil, false, nil

}

func (s *service) makeListWatch() *cache.ListWatch {

	// todo: only listen for pods on this node.

	return &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return s.client.CoreV1().Pods(kubeNamespace).List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return s.client.CoreV1().Pods(kubeNamespace).Watch(options)
		},
	}
}

func (s *service) makeEventHandler() cache.ResourceEventHandler {
	return &cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			s.signalRecheck(obj)
		},
		UpdateFunc: func(_ interface{}, obj interface{}) {
			s.signalRecheck(obj)
		},
		DeleteFunc: func(obj interface{}) {
			// todo: delete requests for this object
		},
	}
}

func (s *service) signalRecheck(obj interface{}) {
	log := s.log.WithField("method", "signalRecheck")

	pod, ok := obj.(*v1.Pod)

	if !ok {
		log.Warnf("unknown type: %#v", pod)
		return
	}

	select {
	case <-s.ctx.Done():
	case s.recheckch <- pod:
	}
}

func queryParamsFromProps(dprops RequiredProps) (queryParams, error) {
	labels := dprops.DockerLabels()
	qp := queryParams{}

	if qp.namespace = types.GetPodNamespace(labels); qp.namespace == "" {
		return qp, ErrContainerNotRecognized
	}

	if qp.podName = types.GetPodName(labels); qp.podName == "" {
		return qp, ErrContainerNotRecognized
	}

	if qp.containerName = types.GetContainerName(labels); qp.containerName == "" {
		return qp, ErrContainerNotRecognized
	}

	if qp.podUID = types.GetPodUID(labels); qp.podUID == "" {
		return qp, ErrContainerNotRecognized
	}

	if qp.containerID = dprops.DockerID(); qp.containerID == "" {
		return qp, ErrContainerNotRecognized
	}

	return qp, nil
}

func defaultKubeClient() (kubernetes.Interface, error) {
	config, err := defaultKubeConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(config)
}

func defaultKubeConfig() (*rest.Config, error) {
	config, err := rest.InClusterConfig()
	if err == nil {
		return config, nil
	}

	// todo: add -f kubeconfig
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	).ClientConfig()
}
