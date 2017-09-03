package docker

import (
	"context"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

const (
	defaultPeriod  = 5 * time.Second
	defaultTimeout = 5 * time.Second
)

type Lister interface {
	Events() <-chan []ListEvent
	Shutdown()
	Done() <-chan struct{}
}

func NewLister(ctx context.Context, client *client.Client) Lister {

	ctx, cancel := context.WithCancel(ctx)

	lister := &lister{
		client:  client,
		current: make(map[string]types.Container),
		period:  defaultPeriod,
		eventch: make(chan []ListEvent),
		donech:  make(chan struct{}),
		cancel:  cancel,
		ctx:     ctx,
	}

	go lister.run()

	return lister
}

type lister struct {
	client *client.Client

	current map[string]types.Container

	period time.Duration

	eventch chan []ListEvent
	donech  chan struct{}

	err    error
	cancel context.CancelFunc
	ctx    context.Context
}

func (l *lister) Events() <-chan []ListEvent {
	return l.eventch
}

func (l *lister) Shutdown() {
	l.cancel()
	<-l.donech
}

func (l *lister) Done() <-chan struct{} {
	return l.donech
}

func (l *lister) run() {
	defer close(l.donech)
	defer l.cancel()

	current := l.current

	var ticker *time.Timer
	var tickch <-chan time.Time

	var events []ListEvent
	var eventch chan []ListEvent

	runner := newListRunner(l.ctx, l.client)
	runnerch := runner.Done()

loop:

	for {

		select {

		case <-l.ctx.Done():
			break loop

		case <-runnerch:

			if err := runner.Err(); err != nil {
				l.err = err
				break loop
			}

			current, events = updateCache(l.current, runner.Result().([]types.Container))

			if len(events) > 0 {
				eventch = l.eventch
			} else {
				eventch = nil
			}

			runner = nil
			runnerch = nil

			ticker = time.NewTimer(l.period)
			tickch = ticker.C

		case <-tickch:
			tickch = nil

			runner = newListRunner(l.ctx, l.client)
			runnerch = runner.Done()

		case eventch <- events:
			events = nil
			eventch = nil
			l.current = current

		}

	}

	if ticker != nil {
		ticker.Stop()
	}

	if runner != nil {
		<-runner.Done()
		l.err = runner.Err()
	}

}

func newListRunner(ctx context.Context, client *client.Client) Runner {
	return NewRunner(ctx, func(ctx context.Context) (interface{}, error) {
		options := types.ContainerListOptions{
			All: true,
		}
		return client.ContainerList(ctx, options)
	})
}

func updateCache(prev map[string]types.Container, containers []types.Container) (map[string]types.Container, []ListEvent) {
	current := make(map[string]types.Container)

	var events []ListEvent

	for _, c := range containers {
		if !acceptContainer(c) {
			continue
		}

		current[c.ID] = c

		if _, ok := prev[c.ID]; !ok {
			events = append(events, NewListEvent(EventTypeCreate, c))
		}
	}

	for id, c := range prev {
		if _, ok := current[id]; !ok {
			events = append(events, NewListEvent(EventTypeDelete, c))
		}
	}

	return current, events
}

type ListEvent struct {
	Type      EventType
	ID        string
	Container types.Container
}

func NewListEvent(t EventType, c types.Container) ListEvent {
	return ListEvent{t, c.ID, c}
}

func acceptContainer(c types.Container) bool {
	return c.State == "running"
}
