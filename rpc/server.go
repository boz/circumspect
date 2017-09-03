package rpc

import (
	"errors"
	"log"
	"net"
	"os"
	"syscall"

	context "golang.org/x/net/context"

	"github.com/boz/circumspect/probes/docker"
	ucgrpc "github.com/boz/circumspect/probes/ucred/grpc"
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

	docker, err := docker.NewService(context.Background())
	if err != nil {
		return err
	}

	s := grpc.NewServer(grpc.Creds(ucgrpc.NewCredentials()))

	RegisterWorkloadServer(s, &server{log, docker})

	return s.Serve(sock)
}

type server struct {
	log    *log.Logger
	docker docker.Service
}

func (s *server) Register(ctx context.Context, req *Request) (*Response, error) {
	props, ok := ucgrpc.PropsFromContext(ctx)

	if !ok {
		s.log.Printf("no properties for peer")
		return &Response{}, errors.New("no peer properties")
	}

	s.log.Printf("register request from [pid: %v uid: %v gid: %v]", props.Pid(), props.Uid(), props.Gid())

	dprops, err := s.docker.Lookup(ctx, props)
	if err != nil {
		s.log.Printf("error getting docker properties: %v", err)
		return &Response{}, err
	}

	log.Printf("found docker container %v", dprops.DockerID())

	docker.PrintProps(os.Stdout, dprops)

	return &Response{}, nil
}
