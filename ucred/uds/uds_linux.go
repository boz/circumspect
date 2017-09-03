// +build linux

package uds

import (
	"net"
	"syscall"
)

func FromConn(conn net.Conn) (Props, error) {
	uconn, ok := conn.(*net.UnixConn)
	if !ok {
		return nil, ErrInvalidConnection
	}

	file, err := uconn.File()
	if err != nil {
		return nil, err
	}
	defer file.Close()

	ucred, err := syscall.GetsockoptUcred(int(file.Fd()), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	if err != nil {
		return nil, err
	}

	return newProps(ucred.Pid, ucred.Uid, ucred.Gid), nil
}
