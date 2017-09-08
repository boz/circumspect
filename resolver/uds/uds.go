package uds

import (
	"errors"

	"github.com/boz/circumspect/propset"
)

var (
	ErrInvalidConnection = errors.New("invalid connection")
	ErrNotSupported      = errors.New("unsupported host OS")
)

type PidProps interface {
	Pid() int
	PropSet() propset.PropSet
}

type Props interface {
	PidProps
	Uid() uint
	Gid() uint
}

func newProps(pid int, uid uint, gid uint) Props {
	return &props{pid, uid, gid}
}

type props struct {
	pid int
	uid uint
	gid uint
}

func (p *props) Pid() int {
	return p.pid
}

func (p *props) Uid() uint {
	return p.uid
}

func (p *props) Gid() uint {
	return p.gid
}

func (p *props) PropSet() propset.PropSet {
	return propset.New().
		AddInt("system-pid", p.Pid()).
		AddInt("system-uid", int(p.Uid())).
		AddInt("system-gid", int(p.Gid()))
}

func NewPidProps(pid int) PidProps {
	return pidProps(pid)
}

type pidProps int

func (p pidProps) Pid() int {
	return int(p)
}

func (p pidProps) PropSet() propset.PropSet {
	return propset.New().AddInt("system-pid", int(p))
}
