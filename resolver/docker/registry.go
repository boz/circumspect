package docker

import (
	"context"
	"errors"
	"time"

	"github.com/docker/engine-api/types"
	ps "github.com/mitchellh/go-ps"
	"github.com/sirupsen/logrus"
)

const (
	// todo: configurable
	registryLookupTimeout = time.Second
)

var ErrInvalidPid = errors.New("Invalid PID")
var ErrNotFound = errors.New("Not found")

// Registry contains a set of running containers and
// allows for finding which container a PID belongs to, if any.
type Registry interface {

	// Lookup will try to find the container that is running the given PID.
	// If no container is found it will block until one becomes known
	// or the given context has been cancelled.
	Lookup(ctx context.Context, pid int) (Props, error)

	// Submit notifies the registry of a new or updated
	// container
	Submit(types.ContainerJSON) error

	Shutdown()
	Done() <-chan struct{}
}

type registry struct {
	lookupch chan *registryLookupRequest
	submitch chan types.ContainerJSON
	purgech  chan *registryLookup

	// todo: optimize
	waitingLookups []*registryLookup
	containers     map[string]types.ContainerJSON

	donech chan struct{}
	log    logrus.FieldLogger
	cancel context.CancelFunc
	ctx    context.Context
}

type registryLookupRequest struct {
	pid    int
	ch     chan<- Props
	donech <-chan struct{}
}

func NewRegistry(ctx context.Context) Registry {
	ctx, cancel := context.WithCancel(ctx)

	log := pkglog.WithField("component", "registry")

	r := &registry{
		lookupch: make(chan *registryLookupRequest),
		submitch: make(chan types.ContainerJSON),
		purgech:  make(chan *registryLookup),

		containers: make(map[string]types.ContainerJSON),

		donech: make(chan struct{}),
		log:    log,
		cancel: cancel,
		ctx:    ctx,
	}

	go r.run()

	return r
}

func (r *registry) Shutdown() {
	r.cancel()
	<-r.donech
}

func (r *registry) Done() <-chan struct{} {
	return r.donech
}

func (r *registry) Lookup(ctx context.Context, pid int) (Props, error) {
	ctx, cancel := context.WithTimeout(ctx, registryLookupTimeout)
	defer cancel()

	ch := make(chan Props, 1)

	req := &registryLookupRequest{pid, ch, ctx.Done()}

	// submit request
	select {
	case <-r.ctx.Done():
		return nil, ErrNotRunning
	case <-ctx.Done():
		return nil, ErrNotFound
	case r.lookupch <- req:
	}

	// wait for response or timeout
	select {
	case <-r.ctx.Done():
		return nil, ErrNotRunning
	case <-ctx.Done():
		return nil, ctx.Err()
	case props, ok := <-ch:

		// ch is only closed if an invalid PID is given.
		if !ok {
			return nil, ErrInvalidPid
		}
		return props, nil
	}

}

func (r *registry) Submit(c types.ContainerJSON) error {
	select {
	case r.submitch <- c:
		return nil
	case <-r.ctx.Done():
		return ErrNotRunning
	}
}

func (r *registry) run() {
	defer close(r.donech)
	defer r.log.Debug("done")

loop:
	for {

		select {

		case <-r.ctx.Done():
			break loop

		case req := <-r.submitch:
			r.doSubmit(req)

		case req := <-r.lookupch:
			r.doLookup(req)

		case lookup := <-r.purgech:
			r.purgeLookup(lookup)

		}
	}

	r.log.Debugf("draining %v lookups", len(r.waitingLookups))

	// drain waiting lookups
	for len(r.waitingLookups) > 0 {
		r.purgeLookup(<-r.purgech)
	}
}

func (r *registry) doSubmit(c types.ContainerJSON) {
	pid := c.State.Pid

	// see if there are any lookups waiting for the PID of this container.
	for _, lookup := range r.waitingLookups {
		if lookup.accept(pid) {
			lookup.resolve(c)
		}
	}

	r.containers[c.ID] = c
}

func (r *registry) doLookup(req *registryLookupRequest) {
	log := pkglog.WithField("request-pid", req.pid)
	log.Debug("looking up container")

	var pids []int

	pid := req.pid

	// starting with the given pid, check if there are any containers
	// whose root process has the same pid.
	// repeat with the parent pid until a container is found
	// or the pid is init (pid == 1).
	for pid > 1 {

		for _, c := range r.containers {
			if c.State.Pid == pid {

				log.WithField("pid", pid).
					WithField("docker-id", c.ID).
					Debugf("match found")

				req.ch <- makeProps(c)
				return
			}
		}

		pids = append(pids, pid)

		p, err := ps.FindProcess(pid)
		if err != nil || p == nil {
			break
		}

		pid = p.PPid()

	}

	// if no valid pids were found,
	// this was somehow a bogus PID.
	if len(pids) == 0 {
		close(req.ch)
		return
	}

	log.Debug("no match found.  waiting for new containers")

	// no containers were found.
	// save all pid generations and wait for a new
	// container to be submitted that matches

	lookup := &registryLookup{req, pids, log.WithField("waiting", true)}

	r.waitingLookups = append(r.waitingLookups, lookup)

	go func() {
		<-req.donech
		r.purgech <- lookup
	}()

}

func (r *registry) purgeLookup(lookup *registryLookup) {
	r.log.WithField("request-pid", lookup.request.pid).Debugf("purging lookup")

	for idx, item := range r.waitingLookups {
		if item == lookup {
			r.waitingLookups = append(r.waitingLookups[:idx], r.waitingLookups[idx+1:]...)
			return
		}
	}
}

type registryLookup struct {
	request *registryLookupRequest
	pids    []int
	log     logrus.FieldLogger
}

func (lookup *registryLookup) accept(pid int) bool {
	for _, item := range lookup.pids {
		if item == pid {
			return true
		}
	}
	return false
}

func (lookup *registryLookup) resolve(c types.ContainerJSON) {
	lookup.log.WithField("pid", c.State.Pid).
		WithField("docker-id", c.ID).
		Debugf("match found")
	select {
	case lookup.request.ch <- makeProps(c):
	case <-lookup.request.donech:
	}
}
