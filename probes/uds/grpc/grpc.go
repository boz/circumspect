package grpc

import (
	"net"

	"golang.org/x/net/context"

	"github.com/boz/circumspect/probes/uds"

	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
)

const (
	authType = "uds-grpc"
)

func NewCredentials() credentials.TransportCredentials {
	return &txCredentials{}
}

type txCredentials struct{}

func (c *txCredentials) ClientHandshake(_ context.Context, _ string, conn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	return conn, unknownAuthInfo{}, nil
}

func (c *txCredentials) ServerHandshake(conn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	props, err := uds.FromConn(conn)

	if err != nil {
		// returning an error here hung the connection
		return conn, unknownAuthInfo{err}, nil
	}

	return conn, authInfo{props}, nil
}

func (c *txCredentials) Info() credentials.ProtocolInfo {
	return credentials.ProtocolInfo{
		SecurityProtocol: authType,
		SecurityVersion:  "0.1",
		ServerName:       "",
	}
}

func (c *txCredentials) Clone() credentials.TransportCredentials {
	return &(*c)
}

func (c *txCredentials) OverrideServerName(_ string) error {
	return nil
}

func PropsFromContext(ctx context.Context) (uds.Props, bool) {
	peer, ok := peer.FromContext(ctx)
	if !ok {
		return nil, false
	}
	return PropsFromAuthInfo(peer.AuthInfo)
}

func PropsFromAuthInfo(ai credentials.AuthInfo) (uds.Props, bool) {
	if ai, ok := ai.(authInfo); ok {
		return ai.Props(), true
	}
	return nil, false
}

type unknownAuthInfo struct {
	err error
}

func (unknownAuthInfo) AuthType() string {
	return authType
}

type authInfo struct {
	props uds.Props
}

func (authInfo) AuthType() string {
	return authType
}

func (pi authInfo) Props() uds.Props {
	return pi.props
}
