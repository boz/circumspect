package docker

import (
	"context"

	"github.com/docker/docker/client"
)

type RequiredProps interface {
	Pid() int32
}

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

		containers:  make(map[string]Container),
		containerch: make(chan Container),

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

	donech chan struct{}
	cancel context.CancelFunc
	ctx    context.Context
}

func (s *service) Lookup(ctx context.Context, pprops RequiredProps) (Props, error) {
	return s.registry.Lookup(ctx, int(pprops.Pid()))
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
		case events := <-s.lister.Events():
			s.handleListEvents(events)
		case event := <-s.watcher.Events():
			s.handleWatchEvent(event)
		}
	}

	s.cancel()

	for len(s.containers) > 0 {
		c := <-s.containerch
		delete(s.containers, c.ID())
	}

	<-s.lister.Done()
	<-s.watcher.Done()
	<-s.registry.Done()
}

func (s *service) handleListEvents(events []ListEvent) {
	for _, event := range events {
		switch event.Type {
		case EventTypeCreate, EventTypeUpdate:
			s.refreshContainer(event.ID)
		case EventTypeDelete:
			s.purgeContainer(event.ID)
		}
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

	c := NewContainer(s.ctx, s.client, s.registry, id)
	s.containers[c.ID()] = c

	go func() {
		<-c.Done()
		s.containerch <- c
	}()
}

func (s *service) purgeContainer(id string) {
	if c, ok := s.containers[id]; ok {
		c.Shutdown()
	}
}
