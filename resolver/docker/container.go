package docker

import (
	"context"
	"errors"

	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
	"github.com/sirupsen/logrus"
)

var ErrNotRunning = errors.New("no longer running")

// Container inspects the given container id and submits the results to the registry.
// When the container is shut down it removes itself from the registry
type Container interface {
	ID() string
	Refresh() error
	Shutdown()
	Done() <-chan struct{}
}

func NewContainer(ctx context.Context, client *client.Client, registry Registry, id string) Container {
	log := pkglog.WithField("docker-id", id).WithField("component", "container")

	ctx, cancel := context.WithCancel(ctx)

	c := &container{
		id:        id,
		client:    client,
		registry:  registry,
		refreshch: make(chan struct{}),
		donech:    make(chan struct{}),
		log:       log,
		cancel:    cancel,
		ctx:       ctx,
	}

	go c.run()

	return c
}

type container struct {
	id        string
	client    *client.Client
	registry  Registry
	refreshch chan struct{}
	donech    chan struct{}
	log       logrus.FieldLogger
	cancel    context.CancelFunc
	ctx       context.Context
}

func (c *container) ID() string {
	return c.id
}

func (c *container) Refresh() error {
	select {
	case c.refreshch <- struct{}{}:
		return nil
	case <-c.ctx.Done():
		return ErrNotRunning
	}
}

func (c *container) Shutdown() {
	c.cancel()
}

func (c *container) Done() <-chan struct{} {
	return c.donech
}

func (c *container) run() {
	defer close(c.donech)
	defer c.log.Debug("done")

	runner := newContainerRunner(c.ctx, c.client, c.id)
	runnerch := runner.Done()

loop:
	for {
		select {

		case <-c.ctx.Done():
			break loop

		case <-runnerch:

			if err := runner.Err(); err != nil {
				c.log.WithError(err).Warn("runner failed")
				// todo: handle error
				continue
			}

			result := runner.Result().(types.ContainerJSON)

			if result.State == nil {
				c.log.Warn("incomplete state")
				continue
			}

			c.log.WithField("status", result.State.Status).
				WithField("running", result.State.Running).
				WithField("pid", result.State.Pid).
				Debug("runner complete")

			c.registry.Submit(result)

			runner = nil
			runnerch = nil

		case <-c.refreshch:

			if runner != nil {
				// todo: schedule in the future?
				continue
			}

			c.log.Debug("beginning refresh")

			// todo: throttle
			runner = newContainerRunner(c.ctx, c.client, c.id)
			runnerch = runner.Done()

		}
	}
	c.cancel()

	if runner != nil {
		<-runner.Done()
	}
}

func newContainerRunner(ctx context.Context, client *client.Client, id string) Runner {
	return NewRunner(ctx, func(ctx context.Context) (interface{}, error) {
		return client.ContainerInspect(ctx, id)
	})
}
