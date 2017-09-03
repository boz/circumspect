package docker

import (
	"fmt"
	"io"

	"github.com/docker/docker/api/types"
)

type Props interface {
	DockerID() string
	DockerPid() int
	DockerImage() string
	DockerPath() string
	DockerLabels() map[string]string
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

func PrintProps(w io.Writer, props Props) {
	fmt.Fprintf(w, "Docker ID: %v\n", props.DockerID())
	fmt.Fprintf(w, "Docker Image: %v\n", props.DockerImage())
	fmt.Fprintf(w, "Docker Path: %v\n", props.DockerPath())
	fmt.Fprintf(w, "Docker Labels: %#v\n", props.DockerLabels())
}
