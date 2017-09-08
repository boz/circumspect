# circumspect: discover peer process attributes

`circumspect` is small POC for determining attributes of a peer process on a linux.

There is a client and server which communicate over a unix domain socket.  When the client connects,
the server is able to determine:

 * pid, uid, gid of the client.
 * if the client is running in docker, attributes of the container that it's running in
 * if the client is running in kubernetes, attributes of the kubernetes pod and container

## Usage

### Start server inside minikube

After [building](#building), boot up minikube and install binary and docker image:

```sh
$ make integration-minikube
```

### Connect from a docker container

In another terminal, launch a docker container inside minikube

```sh
$ make minikube-run-docker
```

The `make integration-minikube` terminal should display:

```
process 4050 properties:

docker-id      97f529ffdb257633e7f3bb46d210d9761b00e29f07e23879ad90d7cb45451f30
docker-image   sha256:c32901baff489930b3ad0ad03ff709547452eee49cc6cfe1fe78af65f81fc918
docker-labels  foo  bar
docker-path    ./circumspect
docker-pid     4050
gid            0
pid            4050
uid            0
```

### Connect from a kubernetes pod

Create a pod which connects as a client

```sh
$ make minikube-create-pod
```

The `make integration-minikube` terminal should display:

```
process 4386 properties:

docker-id            72580b1439f87913edd0d9c1d5b622105ea47911f642a70ed0b36e94dc6ecf7e
docker-image         sha256:c32901baff489930b3ad0ad03ff709547452eee49cc6cfe1fe78af65f81fc918
docker-labels        io.kubernetes.container.name                                 worker-container
                     io.kubernetes.pod.namespace                                  default
                     annotation.io.kubernetes.container.hash                      2eef9918
                     annotation.io.kubernetes.container.restartCount              0
                     annotation.io.kubernetes.container.terminationMessagePath    /dev/termination-log
                     annotation.io.kubernetes.container.terminationMessagePolicy  File
                     annotation.io.kubernetes.pod.terminationGracePeriod          30
                     io.kubernetes.container.logpath                              /var/log/pods/7b4a6c6b-9470-11e7-9e14-08002740d2fd/worker-container_0.log
                     io.kubernetes.docker.type                                    container
                     io.kubernetes.pod.name                                       worker
                     io.kubernetes.pod.uid                                        7b4a6c6b-9470-11e7-9e14-08002740d2fd
                     io.kubernetes.sandbox.id                                     85739086a895027958e2e2f77acc120da80d1229a0c2071f5eb6df88e81e873f
docker-path          /bin/sh
docker-pid           4323
gid                  0
kube-annotations     this-is-a-worker  true
kube-container-name  worker-container
kube-labels          foo  bar
kube-namespace       default
kube-pod-name        worker
pid                  4386
uid                  0
```

The [pod](_integration/pod.yml) connects every five seconds.  Stop it with

```sh
$ make minikube-delete-pod
```

### Inspect arbitrary pids on the system

Mostly just for testing.  Give it some PIDs and it will inspect them.

Inspect all docker processes:

```sh
$ docker ps --quiet | xargs docker inspect --format '{{.State.Pid}}' | xargs ./circumspect pid
```

Inspect all processes:

```sh
$ ps -eo pid | sed 1d | xargs ./circumspect pid
```

## Commands

```
usage: circumspect [<flags>] <command> [<args> ...]

Inspect process properties

Flags:
  -h, --help            Show context-sensitive help (also try --help-long and
                        --help-man).
  -l, --log-level=info  log level
      --docker          enable docker discovery
      --kube            enable kube discovery

Commands:
  help [<command>...]
    Show help.


  client [<flags>]
    run rpc client

    -s, --socket="/tmp/circumspect.sock"  
      rpc socket path

  server [<flags>]
    run rpc server

    -s, --socket="/tmp/circumspect.sock"  
      rpc socket path

  pid [<pid>...]
    inspect given pid(s)

```

## Building

Checkout into `$GOPATH/src/github.com/boz/circumspect` and install dependencies:

```sh
go get -u github.com/kardianos/govendor
govendor sync
make
```

## Status

Only works on linux.  More info here:

 * https://groups.google.com/forum/#!topic/golang-dev/OgfhJ8Ujabo
 * https://github.com/golang/go/issues/1101
