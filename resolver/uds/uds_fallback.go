// +build !linux

package uds

import "net"

func FromConn(conn net.Conn) (Props, error) {
	return nil, ErrNotSupported
}
