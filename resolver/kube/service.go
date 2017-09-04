package kube

import (
	"context"
	"errors"

	"github.com/sirupsen/logrus"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubernetes/pkg/kubelet/types"
)

var (
	ErrContainerNotRecognized = errors.New("container not recognized")
	ErrInvalidPodUID          = errors.New("invalid pod UID")
	ErrInvalidContainerID     = errors.New("invalid container ID")

	log = logrus.StandardLogger().WithField("package", "resolver/kube")
)

type RequiredProps interface {
	DockerID() string
	DockerLabels() map[string]string
}

type Service interface {
	Lookup(context.Context, RequiredProps) (Props, error)
	Shutdown()
	Done() <-chan struct{}
}

func NewService(ctx context.Context) (Service, error) {

	client, err := defaultKubeClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(ctx)
	return &service{
		client: client,
		cancel: cancel,
		ctx:    ctx,
	}, nil

}

type service struct {
	client kubernetes.Interface
	cancel context.CancelFunc
	ctx    context.Context
}

func (s *service) Shutdown() {
	s.cancel()
}

func (s *service) Done() <-chan struct{} {
	return s.ctx.Done()
}

func (s *service) Lookup(ctx context.Context, dprops RequiredProps) (Props, error) {
	log := log.WithField("docker-id", dprops.DockerID())

	log.Debug("resolving pod...")

	// todo: use given ctx for kube api call
	pod, cs, err := podFromDockerLabels(s.client, dprops.DockerLabels())
	if err != nil {
		log.Debug("no pod found")
		return nil, err
	}

	log = log.
		WithField("kube-ns", pod.Namespace).
		WithField("kube-pod", pod.Name).
		WithField("kube-container", cs.Name)

	if cs.ContainerID != dprops.DockerID() {
		log.WithField("kube-container", cs.ContainerID).Warn("container ID mismatch")
		return nil, ErrInvalidContainerID
	}

	log.Debug("pod found for container")

	return newProps(pod, cs), nil
}

func podFromDockerLabels(client kubernetes.Interface, labels map[string]string) (*v1.Pod, *v1.ContainerStatus, error) {

	containerName := types.GetContainerName(labels)
	if containerName == "" {
		return nil, nil, ErrContainerNotRecognized
	}

	podName := types.GetPodName(labels)
	if podName == "" {
		return nil, nil, ErrContainerNotRecognized
	}

	podUID := types.GetPodUID(labels)
	if podUID == "" {
		return nil, nil, ErrContainerNotRecognized
	}

	podNamespace := types.GetPodNamespace(labels)
	if podNamespace == "" {
		return nil, nil, ErrContainerNotRecognized
	}

	pod, err := client.CoreV1().Pods(podNamespace).Get(podName, metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}

	if string(pod.UID) != podUID {
		return nil, nil, ErrInvalidPodUID
	}

	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Name == containerName {
			return pod, &cs, nil
		}
	}

	return nil, nil, ErrContainerNotRecognized

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

	return clientcmd.DefaultClientConfig.ClientConfig()
}
