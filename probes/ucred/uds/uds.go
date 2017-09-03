package uds

import "errors"

var (
	ErrInvalidConnection = errors.New("invalid connection")
	ErrNotSupported      = errors.New("unsupported host OS")
)

type Props interface {
	Pid() int
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
