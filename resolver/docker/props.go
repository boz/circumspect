package docker

import (
	"github.com/boz/circumspect/propset"
	"github.com/docker/docker/api/types"
)

type Props interface {
	DockerID() string
	DockerPid() int
	DockerImage() string
	DockerPath() string
	DockerLabels() map[string]string

	PropSet() propset.PropSet
}

type makeProps types.ContainerJSON

func (p makeProps) DockerID() string {
	return p.ID
}

func (p makeProps) DockerPid() int {
	return p.State.Pid
}

func (p makeProps) DockerImage() string {
	return p.Image
}

func (p makeProps) DockerPath() string {
	return p.Path
}

func (p makeProps) DockerLabels() map[string]string {
	return p.Config.Labels
}

func (p makeProps) PropSet() propset.PropSet {
	return propset.New().
		AddString("docker-id", p.DockerID()).
		AddInt("docker-pid", p.DockerPid()).
		AddString("docker-image", p.DockerImage()).
		AddString("docker-path", p.DockerPath()).
		AddMap("docker-labels", p.DockerLabels())
}
