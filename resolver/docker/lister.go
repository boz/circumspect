package docker

import (
	"context"
	"time"

	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/filters"
	"github.com/sirupsen/logrus"
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

func NewLister(ctx context.Context, client *client.Client, filter filters.Args) Lister {
	log := pkglog.WithField("component", "lister")

	ctx, cancel := context.WithCancel(ctx)

	lister := &lister{
		client: client,
		filter: filter,
		period: defaultPeriod,
		outch:  make(chan []types.Container),
		donech: make(chan struct{}),
		log:    log,
		cancel: cancel,
		ctx:    ctx,
	}

	go lister.run()

	return lister
}

type lister struct {
	client *client.Client
	filter filters.Args
	period time.Duration

	outch chan []types.Container

	err    error
	donech chan struct{}
	log    logrus.FieldLogger
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
	defer l.log.Debug("done")

	var ticker *time.Timer
	var tickch <-chan time.Time

	var containers []types.Container
	var outch chan []types.Container

	runner := newListRunner(l.ctx, l.client, l.filter)
	runnerch := runner.Done()

loop:

	for {

		select {

		case <-l.ctx.Done():
			break loop

		case <-runnerch:
			if err := runner.Err(); err != nil {
				l.log.WithError(err).Error("runner failed")
				l.err = err
				break loop
			}

			containers = filterContainers(runner.Result().([]types.Container))

			l.log.Debugf("list complete: %v containers found", len(containers))

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

			l.log.Debug("starting runner")
			runner = newListRunner(l.ctx, l.client, l.filter)
			runnerch = runner.Done()

		case outch <- containers:
			l.log.Debugf("%v containers delivered", len(containers))

			containers = nil
			outch = nil

		}

	}

	if ticker != nil {
		ticker.Stop()
	}

	if runner != nil {
		l.log.Debug("draining runner")
		<-runner.Done()
		l.err = runner.Err()
	}
}

func newListRunner(ctx context.Context, client *client.Client, filter filters.Args) Runner {
	return NewRunner(ctx, func(ctx context.Context) (interface{}, error) {
		options := types.ContainerListOptions{
			Filter: filter,
			All:    true,
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
