package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/boz/circumspect/resolver"
	"github.com/boz/circumspect/resolver/docker"
	"github.com/boz/circumspect/resolver/kube"
	"github.com/boz/circumspect/rpc"
	"github.com/sirupsen/logrus"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var (
	flagLogLevel = kingpin.Flag("log-level", "log level").
			Short('l').
			Default("info").
			Enum("debug", "info", "warn", "error")

	cmdClient        = kingpin.Command("client", "run rpc client")
	flagClientSocket = cmdClient.Flag("socket", "rpc socket path").
				Short('s').
				Default("/tmp/circumspect.sock").
				String()

	cmdServer        = kingpin.Command("server", "run rpc server")
	flagServerSocket = cmdServer.Flag("socket", "rpc socket path").
				Short('s').
				Default("/tmp/circumspect.sock").
				String()

	cmdPid   = kingpin.Command("pid", "inspect given pid(s)")
	flagPids = cmdPid.Arg("pid", "pid to inspect").
			Ints()
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: %v <client|server|docker> [ -s socket-path ]\n", os.Args[0])
	flag.PrintDefaults()
	os.Exit(1)
}

func main() {

	kingpin.CommandLine.HelpFlag.Short('h')
	kingpin.CommandLine.Help = "Inspect process properties"

	command := kingpin.Parse()

	configureLogger()

	var wg sync.WaitGroup
	defer wg.Wait()

	ctx, cancel := context.WithCancel(context.Background())

	rset := openResolver(ctx)

	defer rset.Shutdown()

	defer cancel()

	wg.Add(1)
	go func() {
		defer wg.Done()
		watchSignals(ctx, cancel)
	}()

	switch command {
	case "client":
		runClient(ctx, rset)
	case "server":
		runServer(ctx, rset)
	case "pid":
		runPid(ctx, rset)
	}

}

func configureLogger() {
	level, err := logrus.ParseLevel(*flagLogLevel)
	kingpin.FatalIfError(err, "Invalid log level")
	logrus.StandardLogger().Level = level
}

func watchSignals(ctx context.Context, cancel context.CancelFunc) {
	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, syscall.SIGINT, syscall.SIGHUP)
	defer signal.Stop(sigch)

	select {
	case <-ctx.Done():
	case <-sigch:
		cancel()
	}
}

func openResolver(ctx context.Context) resolver.Set {
	docker, err := docker.NewService(ctx)
	kingpin.FatalIfError(err, "error opening docker resolver")

	kube, err := kube.NewService(ctx)
	if err != nil {
		docker.Shutdown()
		kingpin.FatalIfError(err, "error opening kube resolver")
	}

	return resolver.NewSet(docker, kube)
}

func runClient(ctx context.Context, _ resolver.Set) {
	rpc.RunClient(ctx, *flagClientSocket)
}

func runServer(ctx context.Context, rset resolver.Set) {
	rpc.RunServer(ctx, *flagServerSocket, func(ctx context.Context, pid int) {
		props, err := rset.Lookup(ctx, pid)
		displayProps(pid, props, err)
	})
}

func runPid(ctx context.Context, rset resolver.Set) {
	var wg sync.WaitGroup

	wg.Add(len(*flagPids))

	for _, pid := range *flagPids {
		go func(pid int) {
			defer wg.Done()
			props, err := rset.Lookup(ctx, pid)
			displayProps(pid, props, err)
		}(pid)
	}

	wg.Wait()
}

var printMtx = &sync.Mutex{}

func displayProps(pid int, props resolver.Props, err error) {
	printMtx.Lock()
	defer printMtx.Unlock()

	fmt.Printf("\npid %v\n", pid)

	dprops := props.Docker()
	if dprops == nil {
		fmt.Printf("docker: no properties\n")
	} else {
		fmt.Printf("DockerID: %v\n", dprops.DockerID())
		fmt.Printf("DockerPid: %v\n", dprops.DockerPid())
		fmt.Printf("DockerImage: %v\n", dprops.DockerImage())
		fmt.Printf("DockerPath: %v\n", dprops.DockerPath())
		fmt.Printf("DockerLabels: %v\n", dprops.DockerLabels())
	}

	kprops := props.Kube()
	if kprops == nil {
		fmt.Printf("kube: no properties\n")
	} else {
		fmt.Printf("KubeNamespace: %v\n", kprops.KubeNamespace())
		fmt.Printf("KubePodName: %v\n", kprops.KubePodName())
		fmt.Printf("KubeLabels: %v\n", kprops.KubeLabels())
		fmt.Printf("KubeAnnotations: %v\n", kprops.KubeAnnotations())
		fmt.Printf("KubeContainerName: %v\n", kprops.KubeContainerName())
	}

}
