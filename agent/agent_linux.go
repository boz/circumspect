// +build linux
package agent

import (
	"log"
	"net"
	"syscall"

	"google.golang.org/grpc/credentials"
)

func credentialsFromSocket(log *log.Logger, conn net.Conn) credentials.AuthInfo {

	uconn, ok := conn.(*net.UnixConn)

	if !ok {
		log.Printf("invalid connection type: %#t", conn)
		return unknownPeerInfo{}
	}

	file, err := uconn.File()
	if err != nil {
		log.Printf("can't get connection file: %v", err)
		return unknownPeerInfo{}
	}
	defer file.Close()

	ucred, err := syscall.GetsockoptUcred(int(file.Fd()), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	if err != nil {
		log.Printf("unable to get peer credentials: %v", err)
		return unknownPeerInfo{}
	}

	log.Printf("connection established [pid: %v uid: %v gid: %v]", ucred.Pid, ucred.Uid, ucred.Gid)

	return ucredPeerInfo{
		Pid: ucred.Pid,
		Uid: ucred.Uid,
		Gid: ucred.Gid,
	}
}
