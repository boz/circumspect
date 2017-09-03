package rpc

import (
	"errors"
	"log"
	"net"
	"syscall"

	context "golang.org/x/net/context"

	ucgrpc "github.com/boz/circumspect/ucred/grpc"
	grpc "google.golang.org/grpc"
)

func RunServer(log *log.Logger, path string) error {

	// delete socket if present
	syscall.Unlink(path)

	sock, err := net.Listen("unix", path)
	if err != nil {
		log.Printf("error listening on %v: %v", path, err)
		return err
	}
	defer sock.Close()

	s := grpc.NewServer(grpc.Creds(ucgrpc.NewCredentials()))

	RegisterWorkloadServer(s, &server{log})

	return s.Serve(sock)
}

type server struct {
	log *log.Logger
}

func (s *server) Register(ctx context.Context, req *Request) (*Response, error) {
	props, ok := ucgrpc.PropsFromContext(ctx)

	if !ok {
		s.log.Printf("no properties for peer")
		return &Response{}, errors.New("no peer properties")
	}

	s.log.Printf("register request from [pid: %v uid: %v gid: %v]", props.Pid(), props.Uid(), props.Gid())

	return &Response{}, nil
}
