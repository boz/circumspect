package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/boz/circumspect/probes/docker"
	"github.com/boz/circumspect/rpc"
)

var (
	flagSocket = flag.String("s", "/tmp/circumspect.sock", "socket path")
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: %v <client|server|docker> [ -s socket-path ]\n", os.Args[0])
	flag.PrintDefaults()
	os.Exit(1)
}

func main() {

	flag.Parse()

	if flag.NArg() < 1 {
		usage()
	}

	log := openLog()

	var err error

	switch flag.Arg(0) {
	case "client":
		err = rpc.RunClient(log, *flagSocket)
	case "server":
		err = rpc.RunServer(log, *flagSocket)
	case "docker":
		err = runDocker(log)
	default:
		usage()
	}

	if err != nil {
		log.Fatalf("error: %v", err)
	}
}

func openLog() *log.Logger {
	prefix := fmt.Sprintf("[%v %6d] ", flag.Arg(0), os.Getpid())
	return log.New(os.Stderr, prefix, log.Lshortfile)
}

func runDocker(log *log.Logger) error {
	ctx := context.Background()
	svc, err := docker.NewService(ctx)
	if err != nil {
		return err
	}

	for i := 1; i < flag.NArg(); i++ {
		pid, err := strconv.Atoi(flag.Arg(i))

		if err != nil {
			log.Printf("invalid pid: %v", flag.Arg(i))
			continue
		}

		log.Printf("looking up pid %v", pid)

		props, err := svc.Lookup(ctx, pidProps(pid))

		if err != nil {
			log.Printf("error looking up %v: %v", pid, err)
			continue
		}

		log.Printf("FOUND CONTAINER FOR PID %v", pid)
		docker.PrintProps(os.Stdout, props)
	}

	svc.Shutdown()
	return nil
}

type pidProps int

func (p pidProps) Pid() int32 {
	return int32(p)
}
