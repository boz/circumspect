// +build !linux

package agent

import (
	"log"
	"net"

	"google.golang.org/grpc/credentials"
)

func credentialsFromSocket(log *log.Logger, conn net.Conn) credentials.AuthInfo {
	log.Print("ucred credentials not supported")
	return unknownPeerInfo{}
}
