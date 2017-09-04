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

### see also

 * https://groups.google.com/forum/#!topic/golang-dev/OgfhJ8Ujabo
 * https://github.com/golang/go/issues/1101
