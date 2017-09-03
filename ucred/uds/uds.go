package uds

import "errors"

var (
	ErrInvalidConnection = errors.New("invalid connection")
	ErrNotSupported      = errors.New("unsupported host OS")
)

type Props interface {
	Pid() int32
	Uid() uint32
	Gid() uint32
}

func newProps(pid int32, uid uint32, gid uint32) Props {
	return &props{pid, uid, gid}
}

type props struct {
	pid int32
	uid uint32
	gid uint32
}

func (p *props) Pid() int32 {
	return p.pid
}

func (p *props) Uid() uint32 {
	return p.uid
}

func (p *props) Gid() uint32 {
	return p.gid
}
