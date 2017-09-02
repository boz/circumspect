package agent

import (
	"errors"
	"log"
	"net"
	"syscall"

	context "golang.org/x/net/context"

	"github.com/boz/circumspect/proto"

	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
)

func Run(log *log.Logger, path string) error {

	// delete socket if present
	syscall.Unlink(path)

	sock, err := net.Listen("unix", path)
	if err != nil {
		log.Printf("error listening on %v: %v", path, err)
		return err
	}
	defer sock.Close()

	s := grpc.NewServer(grpc.Creds(&creds{log: log}))

	proto.RegisterWorkloadServer(s, &server{log})

	return s.Serve(sock)
}

type server struct {
	log *log.Logger
}

func (s *server) Register(ctx context.Context, req *proto.Request) (*proto.Response, error) {
	peer, ok := peer.FromContext(ctx)

	if !ok {
		s.log.Printf("no peer info")
		return &proto.Response{}, errors.New("no peer info")
	}

	pinfo, ok := peer.AuthInfo.(ucredPeerInfo)
	if !ok {
		s.log.Printf("invalid credentials")
		return &proto.Response{}, errors.New("invalid credentials")
	}

	s.log.Printf("register request from [pid: %v uid: %v gid: %v]", pinfo.Pid, pinfo.Uid, pinfo.Gid)

	return &proto.Response{}, nil
}

type creds struct {
	log *log.Logger
}

func (c *creds) ClientHandshake(_ context.Context, _ string, conn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	return conn, unknownPeerInfo{}, nil
}

func (c *creds) ServerHandshake(conn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	pinfo := credentialsFromSocket(c.log, conn)

	return conn, pinfo, nil
}

func (c *creds) Info() credentials.ProtocolInfo {
	return credentials.ProtocolInfo{
		SecurityProtocol: "none",
		SecurityVersion:  "0.0",
		ServerName:       "socket",
	}
}

func (c *creds) Clone() credentials.TransportCredentials {
	return &(*c)
}

func (c *creds) OverrideServerName(_ string) error {
	return nil
}

type ucredPeerInfo struct {
	Pid int32
	Uid uint32
	Gid uint32
}

func (ucredPeerInfo) AuthType() string {
	return "ucred"
}

type unknownPeerInfo struct{}

func (unknownPeerInfo) AuthType() string {
	return "none"
}
