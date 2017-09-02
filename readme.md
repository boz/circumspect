## grpc-over-uds peer pid

Example grpc credentials which capture `{pid,uid,gid}` of client.

### with vagrant

```sh
make linux
vagrant up
vagrant ssh
/vagrant/pidattest-linux agent &
/vagrant/pidattest-linux worker
kill %1
```

### see also

 * https://groups.google.com/forum/#!topic/golang-dev/OgfhJ8Ujabo
 * https://github.com/golang/go/issues/1101
