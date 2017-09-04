package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/boz/circumspect/discovery"
	"github.com/boz/circumspect/propset"
	"github.com/boz/circumspect/resolver/uds"
	"github.com/boz/circumspect/rpc"
	"github.com/sirupsen/logrus"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var (
	flagLogLevel = kingpin.Flag("log-level", "log level").
			Short('l').
			Default("info").
			Enum("debug", "info", "warn", "error")

	flagEnableDocker = kingpin.Flag("docker", "enable docker discovery").
				Default("true").
				Bool()

	flagEnableKube = kingpin.Flag("kube", "enable kube discovery").
			Default("false").
			Bool()

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

func main() {

	kingpin.CommandLine.HelpFlag.Short('h')
	kingpin.CommandLine.Help = "Inspect process properties"

	command := kingpin.Parse()

	configureLogger()

	var wg sync.WaitGroup
	defer wg.Wait()

	ctx, cancel := context.WithCancel(context.Background())
	watchSignals(ctx, cancel, &wg)

	if command == "client" {
		defer cancel()
		runClient(ctx)
		return
	}

	rset := openResolver(ctx)
	defer rset.Shutdown()
	defer cancel()

	switch command {
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

func watchSignals(
	ctx context.Context, cancel context.CancelFunc, wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		sigch := make(chan os.Signal, 1)
		signal.Notify(sigch, syscall.SIGINT, syscall.SIGHUP)
		defer signal.Stop(sigch)

		select {
		case <-ctx.Done():
		case <-sigch:
			cancel()
		}
	}()
}

func openResolver(ctx context.Context) discovery.Strategy {
	discovery, err := discovery.Build(ctx, *flagEnableDocker, *flagEnableKube)
	kingpin.FatalIfError(err, "error opening discovery")
	return discovery
}

func runClient(ctx context.Context) {
	rpc.RunClient(ctx, *flagClientSocket)
}

func runServer(ctx context.Context, rset discovery.Strategy) {
	rpc.RunServer(ctx, *flagServerSocket, func(ctx context.Context, props uds.Props) {
		pset, err := rset.Lookup(ctx, props)
		displayProps(props, pset, err)
	})
}

func runPid(ctx context.Context, rset discovery.Strategy) {
	var wg sync.WaitGroup

	wg.Add(len(*flagPids))

	for _, pid := range *flagPids {
		go func(pid int) {
			defer wg.Done()
			props := uds.NewPidProps(pid)
			pset, err := rset.Lookup(ctx, props)
			displayProps(props, pset, err)
		}(pid)
	}

	wg.Wait()
}

var printMtx = &sync.Mutex{}

func displayProps(pprops uds.PidProps, pset propset.PropSet, err error) {
	printMtx.Lock()
	defer printMtx.Unlock()

	fmt.Printf("\nprocess %v properties:\n\n", pprops.Pid())
	propset.Fprint(os.Stdout, pset)

}
