# circumspect: discover peer process attributes

`circumspect` is small POC for determining attributes of a peer process on a linux.

There is a client and server which communicate over a unix domain socket.  When the client connects,
the server is able to determine:

 * pid, uid, gid of the client.
 * if the client is running in docker, attributes of the container that it's running in
 * ~~if the client is running in kubernetes, attributes of the kubernetes pod and container~~ _disabled; see [here](#kubernetes)_

## Usage

### Connect to server from a client running in docker

server terminal

```sh
$ ./circumspect server
```

client terminal

```sh
$ make integration
```

Once the client connects, the server should output its properties:

```sh
process 22295 properties:

docker-id      7c528a3b375f454add356a962dadd1bd57a454223cb182cccf4665153f2d2b60
docker-image   sha256:6084f5234b59e5767eb8b8a684bd83cf75c2352b951efc02dea23759d68eb0ad
docker-labels  com.docker.compose.config-hash       7e309c49308ffbb07d6e428134f303c77faa784777f595583ec8b82837c57929
               com.docker.compose.container-number  1
               com.docker.compose.oneoff            False
               com.docker.compose.project           circumspect
               com.docker.compose.service           worker
               com.docker.compose.version           1.15.0
               not-a-hacker                         tremendously
docker-path    ./circumspect
docker-pid     22295
gid            0
pid            22295
uid            0
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

### Kubernetes

 * This POC was developed with recent docker release libraries.
 * Minikube (and docker in general) can't uses the latest docker release.  See [here](https://github.com/kubernetes/kubernetes/issues/40182) and [here](https://github.com/kubernetes/minikube/pull/1542)
 * Downgrading the docker client library involves using a pre-moby release with a lot of changes.

It's just too much of a PITA for a dumb POC.

### Other Platforms

 * https://groups.google.com/forum/#!topic/golang-dev/OgfhJ8Ujabo
 * https://github.com/golang/go/issues/1101
