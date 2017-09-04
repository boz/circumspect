package resolver

import (
	"context"
	"sync"

	"github.com/boz/circumspect/resolver/docker"
	"github.com/boz/circumspect/resolver/kube"
)

type Props interface {
	Docker() docker.Props
	Kube() kube.Props
}

type Set interface {
	Lookup(context.Context, int) (Props, error)
	Shutdown()
}

func NewSet(docker docker.Service, kube kube.Service) Set {
	return &resolverSet{docker, kube}
}

type resolverSet struct {
	docker docker.Service
	kube   kube.Service
}

func (rs *resolverSet) Shutdown() {
	var wg sync.WaitGroup

	wg.Add(2)

	go func() {
		defer wg.Done()
		rs.docker.Shutdown()
	}()

	go func() {
		defer wg.Done()
		rs.kube.Shutdown()
	}()

	wg.Wait()
}

func (rs *resolverSet) Lookup(ctx context.Context, pid int) (Props, error) {
	var err error
	props := props{}

	props.docker, err = rs.docker.Lookup(ctx, pidProps(pid))
	if err != nil {
		return props, nil
	}

	props.kube, err = rs.kube.Lookup(ctx, props.docker)
	return props, nil
}

type props struct {
	docker docker.Props
	kube   kube.Props
}

func (p props) Docker() docker.Props {
	return p.docker
}

func (p props) Kube() kube.Props {
	return p.kube
}

type pidProps int

func (p pidProps) Pid() int {
	return int(p)
}
