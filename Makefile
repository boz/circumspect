all: pidattest
linux: pidattest-linux

pidattest:
	go build .

pidattest-linux:
	GOOS=linux go build -o pidattest-linux .

proto:
	protoc --go_out=plugins=grpc:. proto/satest.proto

clean:
	rm pidattest pidattest-linux 2>/dev/null || true

.PHONY: linux all pidattest pidattest-linux proto clean
