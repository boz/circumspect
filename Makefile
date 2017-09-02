all: circumspect
linux: circumspect-linux

circumspect:
	go build .

circumspect-linux:
	GOOS=linux go build -o circumspect-linux .

proto:
	protoc --go_out=plugins=grpc:. proto/satest.proto

clean:
	rm circumspect circumspect-linux 2>/dev/null || true

.PHONY: linux all circumspect circumspect-linux proto clean
