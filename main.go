package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/boz/circumspect/agent"
	"github.com/boz/circumspect/worker"
)

var (
	flagSocket = flag.String("s", "/tmp/satest.sock", "socket path")
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: %v <worker|agent> [ -s socket-path ]\n", os.Args[0])
	flag.PrintDefaults()
	os.Exit(1)
}

func main() {

	flag.Parse()

	if flag.NArg() != 1 {
		usage()
	}

	log := openLog()

	var err error

	switch flag.Arg(0) {
	case "worker":
		err = worker.Run(log, *flagSocket)
	case "agent":
		err = agent.Run(log, *flagSocket)
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
