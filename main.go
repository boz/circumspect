package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/boz/circumspect/rpc"
)

var (
	flagSocket = flag.String("s", "/tmp/satest.sock", "socket path")
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: %v <client|server> [ -s socket-path ]\n", os.Args[0])
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
	case "client":
		err = rpc.RunClient(log, *flagSocket)
	case "server":
		err = rpc.RunServer(log, *flagSocket)
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
