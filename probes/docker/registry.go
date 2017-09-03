package docker

import (
	"context"
	"errors"

	"github.com/docker/docker/api/types"
	ps "github.com/mitchellh/go-ps"
)

var ErrInvalidPid = errors.New("Invalid PID")

type Registry interface {
	Lookup(context.Context, int) (Props, error)
	Submit(types.ContainerJSON) error

	Shutdown()
	Done() <-chan struct{}
}

type registry struct {
	lookupch chan *registryLookupRequest
	submitch chan types.ContainerJSON
	purgech  chan *registryLookup

	// todo: optimize
	pendingLookups []*registryLookup
	containers     map[string]types.ContainerJSON

	donech chan struct{}
	cancel context.CancelFunc
	ctx    context.Context
}

type registryLookupRequest struct {
	pid    int
	ch     chan<- Props
	donech chan struct{}
}

func NewRegistry(ctx context.Context) Registry {
	ctx, cancel := context.WithCancel(ctx)

	r := &registry{
		lookupch: make(chan *registryLookupRequest),
		submitch: make(chan types.ContainerJSON),
		purgech:  make(chan *registryLookup),

		containers: make(map[string]types.ContainerJSON),

		donech: make(chan struct{}),
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

	ch := make(chan Props, 1)
	donech := make(chan struct{})
	defer close(donech)

	req := &registryLookupRequest{pid, ch, donech}

	select {
	case <-r.ctx.Done():
		return nil, ErrNotRunning
	case <-ctx.Done():
		return nil, ctx.Err()
	case r.lookupch <- req:
	}

	select {
	case <-r.ctx.Done():
		return nil, ErrNotRunning
	case <-ctx.Done():
		return nil, ctx.Err()
	case props, ok := <-ch:
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

	for len(r.pendingLookups) > 0 {
		r.purgeLookup(<-r.purgech)
	}

}

func (r *registry) doSubmit(c types.ContainerJSON) {
	pid := c.State.Pid

	for _, lookup := range r.pendingLookups {
		if lookup.accept(pid) {
			lookup.resolve(c)
		}
	}

	r.containers[c.ID] = c
}

func (r *registry) doLookup(req *registryLookupRequest) {
	var pids []int

	pid := req.pid

	for pid > 1 {

		for _, c := range r.containers {
			if c.State.Pid == pid {
				req.ch <- makeProps(c)
				return
			}
		}

		pids = append(pids, pid)

		p, err := ps.FindProcess(pid)
		if err != nil {
			break
		}

		pid = p.PPid()

	}

	if len(pids) == 0 {
		close(req.ch)
		return
	}

	lookup := &registryLookup{req, pids}

	r.pendingLookups = append(r.pendingLookups, lookup)

	go func() {
		<-req.donech
		r.purgech <- lookup
	}()

}

func (r *registry) purgeLookup(lookup *registryLookup) {
	for idx, item := range r.pendingLookups {
		if item == lookup {
			r.pendingLookups = append(r.pendingLookups[:idx], r.pendingLookups[idx+1:]...)
			return
		}
	}
}

type registryLookup struct {
	request *registryLookupRequest
	pids    []int
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
	select {
	case lookup.request.ch <- makeProps(c):
	case <-lookup.request.donech:
	}
}
