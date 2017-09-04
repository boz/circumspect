package docker

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

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

	client, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}

	_, err = client.Ping(ctx)
	if err != nil {
		client.Close()
		return nil, err
	}

	ctx, cancel := context.WithCancel(ctx)

	lister := NewLister(ctx, client)
	watcher := NewWatcher(ctx, client)
	registry := NewRegistry(ctx)

	svc := &service{
		client:   client,
		lister:   lister,
		watcher:  watcher,
		registry: registry,

		containers:      make(map[string]Container),
		containerch:     make(chan Container),
		staleContainers: make(map[string]Container),

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

loop:
	for {
		select {

		case <-s.ctx.Done():
			break loop

		case <-s.lister.Done():
			break loop

		case <-s.watcher.Done():
			break loop

		case <-s.registry.Done():
			break loop

		case c := <-s.containerch:

			delete(s.containers, c.ID())
			delete(s.staleContainers, c.ID())

		case containers := <-s.lister.Containers():
			s.handleContainerList(containers)

		case event := <-s.watcher.Events():
			s.handleWatchEvent(event)

		}
	}

	s.cancel()

	// drain containers
	for len(s.containers) > 0 {
		c := <-s.containerch
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
			c.Shutdown()
			continue
		}

		// queue up to be purged on the next list
		s.staleContainers[id] = c
	}

}

func (s *service) handleWatchEvent(event WatchEvent) {
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

	c := NewContainer(s.ctx, s.client, s.registry, id)
	s.containers[c.ID()] = c

	go func() {
		<-c.Done()
		s.containerch <- c
	}()
}
