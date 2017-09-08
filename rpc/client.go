package rpc

import (
	"net"
	"time"

	context "golang.org/x/net/context"

	grpc "google.golang.org/grpc"
)

func RunClient(ctx context.Context, path string) error {
	log := pkglog.WithField("component", "client")

	log.Debugf("connecting to %v ...", path)

	dialer := func(addr string, timeout time.Duration) (net.Conn, error) {
		d := net.Dialer{Timeout: timeout}
		return d.DialContext(ctx, "unix", addr)
	}

	conn, err := grpc.DialContext(ctx, path, grpc.WithInsecure(), grpc.WithDialer(dialer))

	if err != nil {
		log.WithError(err).Errorf("error connecting to %v", path)
		return err
	}
	defer conn.Close()

	client := NewWorkloadClient(conn)

	_, err = client.Register(ctx, &Request{})
	if err != nil {
		log.WithError(err).Error("error registering")
		return err
	}

	return nil
}
