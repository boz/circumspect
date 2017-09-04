package docker

import (
	"context"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

const (
	defaultPeriod  = 10 * time.Second
	defaultTimeout = 5 * time.Second
)

// Lister periodically fetches the complete list
// of running containers and sends them to the `Containers()` channel.
type Lister interface {
	Containers() <-chan []types.Container
	Shutdown()
	Done() <-chan struct{}
}

func NewLister(ctx context.Context, client *client.Client) Lister {

	ctx, cancel := context.WithCancel(ctx)

	lister := &lister{
		client: client,
		period: defaultPeriod,
		outch:  make(chan []types.Container),
		donech: make(chan struct{}),
		cancel: cancel,
		ctx:    ctx,
	}

	go lister.run()

	return lister
}

type lister struct {
	client *client.Client
	period time.Duration

	outch chan []types.Container

	err    error
	donech chan struct{}
	cancel context.CancelFunc
	ctx    context.Context
}

func (l *lister) Containers() <-chan []types.Container {
	return l.outch
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

	var ticker *time.Timer
	var tickch <-chan time.Time

	var containers []types.Container
	var outch chan []types.Container

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

			containers = filterContainers(runner.Result().([]types.Container))

			if len(containers) > 0 {
				outch = l.outch
			} else {
				outch = nil
			}

			runner = nil
			runnerch = nil

			ticker = time.NewTimer(l.period)
			tickch = ticker.C

		case <-tickch:
			tickch = nil

			runner = newListRunner(l.ctx, l.client)
			runnerch = runner.Done()

		case outch <- containers:

			containers = nil
			outch = nil

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

func filterContainers(containers []types.Container) []types.Container {
	var filtered []types.Container

	for _, container := range containers {
		if acceptContainer(container) {
			filtered = append(filtered, container)
		}
	}

	return filtered
}

func acceptContainer(container types.Container) bool {
	return container.State == "running"
}
