## grpc-over-uds peer pid

Example grpc credentials which capture `{pid,uid,gid}` of client.

```sh
docker ps --quiet | xargs docker inspect --format '{{.State.Pid}}'
```

### with vagrant

```sh
make linux
vagrant up
vagrant ssh
/vagrant/circumspect-linux server &
/vagrant/circumspect-linux client
kill %1
```

### docker + kube = fail

 * This POC was developed with recent docker release libraries.
 * Minikube (and docker in general) can't uses the latest docker release.  See [here](https://github.com/kubernetes/kubernetes/issues/40182) and [here](https://github.com/kubernetes/minikube/pull/1542)
 * Downgrading the docker client library involves using a pre-moby release with a lot of changes.

It's just too much of a PITA for a dumb POC.

### see also

 * https://groups.google.com/forum/#!topic/golang-dev/OgfhJ8Ujabo
 * https://github.com/golang/go/issues/1101
