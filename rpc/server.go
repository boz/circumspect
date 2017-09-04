package rpc

import (
	"errors"
	"net"

	context "golang.org/x/net/context"

	udsgrpc "github.com/boz/circumspect/resolver/uds/grpc"
	"github.com/sirupsen/logrus"
	grpc "google.golang.org/grpc"
)

var log = logrus.StandardLogger().WithField("package", "rpc")

func RunServer(ctx context.Context, path string, fn func(context.Context, int)) error {
	log := log.WithField("component", "server")

	sock, err := net.Listen("unix", path)
	if err != nil {
		log.WithError(err).Errorf("error listening on %v", path)
		return err
	}

	donech := make(chan struct{})
	defer func() { <-donech }()

	go func() {
		defer close(donech)
		<-ctx.Done()
		sock.Close()
	}()

	s := grpc.NewServer(grpc.Creds(udsgrpc.NewCredentials()))

	RegisterWorkloadServer(s, &server{log, fn})

	return s.Serve(sock)
}

type server struct {
	log logrus.FieldLogger
	fn  func(context.Context, int)
}

func (s *server) Register(ctx context.Context, req *Request) (*Response, error) {
	props, ok := udsgrpc.PropsFromContext(ctx)

	if !ok {
		s.log.Warnf("no properties for peer")
		return &Response{}, errors.New("no peer properties")
	}

	s.log.Debugf("register request from [pid: %v uid: %v gid: %v]", props.Pid(), props.Uid(), props.Gid())

	s.fn(ctx, props.Pid())

	return &Response{}, nil
}
