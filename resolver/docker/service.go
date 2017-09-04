package docker

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/sirupsen/logrus"
)

var log = logrus.StandardLogger().WithField("package", "resolver/docker")

type RequiredProps interface {
	Pid() int
}

// Searvice maintains a set of active containers
// and a registry for searching for containers by PID.
// It determinines active/dead containers from two
// sources: Lister and Watcher.
//
// Watcher listens to docker events and reports new, dying containers.
//
// Lister periodically scans all active containers.  It is needed for startup
// and make sure any missed "container died" events don't cause memory/container leaks.
type Service interface {
	Lookup(context.Context, RequiredProps) (Props, error)
	Shutdown()
	Done() <-chan struct{}
}

func NewService(ctx context.Context) (Service, error) {
	log := log.WithField("component", "service")

	// todo: configurable
	client, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}

	// todo: ensure server is on same machine+os
	ping, err := client.Ping(ctx)
	if err != nil {
		client.Close()
		return nil, err
	}

	log.Debugf("connected to docker %#v", ping)

	// todo: configurable
	filter := filters.NewArgs()
	filter.Add("status", "running")

	ctx, cancel := context.WithCancel(ctx)

	lister := NewLister(ctx, client, filter)
	watcher := NewWatcher(ctx, client, filter)
	registry := NewRegistry(ctx)

	svc := &service{
		client:   client,
		lister:   lister,
		watcher:  watcher,
		registry: registry,

		containers:      make(map[string]Container),
		containerch:     make(chan Container),
		staleContainers: make(map[string]Container),

		log:    log,
		cancel: cancel,
		ctx:    ctx,
		donech: make(chan struct{}),
	}

	go svc.run()

	return svc, nil
}

type service struct {
	client   *client.Client
	lister   Lister
	watcher  Watcher
	registry Registry

	containers  map[string]Container
	containerch chan Container

	// Containers that have been missing from one lister.Containers() delivery
	staleContainers map[string]Container

	log    logrus.FieldLogger
	donech chan struct{}
	cancel context.CancelFunc
	ctx    context.Context
}

func (s *service) Lookup(ctx context.Context, pprops RequiredProps) (Props, error) {
	return s.registry.Lookup(ctx, pprops.Pid())
}

func (s *service) Shutdown() {
	s.cancel()
	<-s.donech
}

func (s *service) Done() <-chan struct{} {
	return s.donech
}

func (s *service) run() {
	defer close(s.donech)
	defer s.log.Debug("done")

loop:
	for {
		select {

		case <-s.ctx.Done():
			s.log.Debug("context cancelled")
			break loop

		case <-s.lister.Done():
			s.log.Debug("early lister copletion")
			break loop

		case <-s.watcher.Done():
			s.log.Debug("early watcher copletion")
			break loop

		case <-s.registry.Done():
			s.log.Debug("early registry copletion")
			break loop

		case c := <-s.containerch:
			s.log.WithField("docker-id", c.ID()).
				Debug("container complete")

			delete(s.containers, c.ID())
			delete(s.staleContainers, c.ID())

		case containers := <-s.lister.Containers():
			s.handleContainerList(containers)

		case event := <-s.watcher.Events():
			s.handleWatchEvent(event)
		}
	}

	s.cancel()

	s.log.Debugf("draining %v containers", len(s.containers))

	// drain containers
	for len(s.containers) > 0 {

		c := <-s.containerch

		s.log.WithField("docker-id", c.ID()).
			Debug("container drained")

		delete(s.containers, c.ID())
		delete(s.staleContainers, c.ID())
	}

	<-s.lister.Done()
	<-s.watcher.Done()
	<-s.registry.Done()
}

// handleContainerList updates the current set of running containers.
// A new Container will be created for all new container IDs given.
// If a currently running container is not in the new list, it will
// be marked as "stale".
// If a "stale" container is not in the list, it will be shut down.
func (s *service) handleContainerList(containers []types.Container) {
	s.log.WithField("active", len(s.containers)).
		WithField("stale", len(s.staleContainers)).
		Debugf("updating with %v containers", len(containers))

	newset := make(map[string]bool)

	for _, c := range containers {

		newset[c.ID] = true

		// no longer stale
		delete(s.staleContainers, c.ID)

		// already created
		if _, ok := s.containers[c.ID]; ok {
			continue
		}

		// new container
		s.createContainer(c.ID)

	}

	// handle containers not in new list
	for id, c := range s.containers {

		if newset[id] {
			continue
		}

		// already stale once. purge.
		if _, ok := s.staleContainers[id]; ok {
			s.log.WithField("docker-id", id).Debug("shutting down stale container")
			c.Shutdown()
			continue
		}

		// queue up to be purged on the next list
		s.staleContainers[id] = c
	}
}

func (s *service) handleWatchEvent(event WatchEvent) {
	s.log.WithField("docker-id", event.ID).Debugf("watcher event received: %v")
	switch event.Type {
	case EventTypeCreate, EventTypeUpdate:
		s.refreshContainer(event.ID)
	case EventTypeDelete:
		s.purgeContainer(event.ID)
	}
}

func (s *service) refreshContainer(id string) {
	if c, ok := s.containers[id]; ok {
		c.Refresh()
		return
	}
	s.createContainer(id)
}

func (s *service) purgeContainer(id string) {
	if c, ok := s.containers[id]; ok {
		c.Shutdown()
	}
}

func (s *service) createContainer(id string) {
	log := s.log.WithField("docker-id", id)
	log.Debug("creating container")

	c := NewContainer(s.ctx, s.client, s.registry, id)
	s.containers[c.ID()] = c

	go func() {
		<-c.Done()
		s.containerch <- c
	}()
}
