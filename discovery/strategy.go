package discovery

import (
	"context"
	"errors"
	"sync"

	"github.com/boz/circumspect/propset"
	"github.com/boz/circumspect/resolver/docker"
	"github.com/boz/circumspect/resolver/kube"
	"github.com/boz/circumspect/resolver/uds"
)

type Strategy interface {
	Lookup(context.Context, uds.PidProps) (propset.PropSet, error)
	Shutdown()
}

func Build(ctx context.Context, enableDocker bool, enableKube bool) (Strategy, error) {

	if enableKube && !enableDocker {
		return nil, errors.New("kube resolver requires docker to be enabled")
	}

	s := &strategy{}
	var err error

	if enableDocker {
		s.docker, err = docker.NewService(ctx)
		if err != nil {
			return nil, err
		}
	}

	if enableKube {
		s.kube, err = kube.NewService(ctx)
		if err != nil {
			s.docker.Shutdown()
			return nil, err
		}
	}

	return s, nil
}

type strategy struct {
	docker docker.Service
	kube   kube.Service
}

func (d *strategy) Shutdown() {
	var wg sync.WaitGroup

	if d.docker != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d.docker.Shutdown()
		}()
	}

	if d.kube != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d.kube.Shutdown()
		}()
	}

	wg.Wait()
}

func (d *strategy) Lookup(ctx context.Context, pprops uds.PidProps) (propset.PropSet, error) {
	var err error
	var dprops docker.Props

	pset := pprops.PropSet()

	if d.docker != nil {
		dprops, err = d.docker.Lookup(ctx, pprops)
		if err != nil {
			return pset, nil
		}
		pset.Merge(dprops.PropSet())
	}

	if d.kube != nil {
		kprops, err := d.kube.Lookup(ctx, dprops)
		if err != nil {
			return pset, nil
		}
		pset.Merge(kprops.PropSet())
	}

	return pset, nil
}
