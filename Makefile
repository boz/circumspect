DOCKER_IMAGE ?= circumspect

IMG_LDFLAGS := -w -linkmode external -extldflags "-static"

build:
	go build .

ifeq ($(shell uname -s),Darwin)
build-linux:
	GOOS=linux GOARCH=amd64 go build -o circumspect-linux .
else
build-linux:
	CC=$$(which musl-gcc) go build --ldflags '$(IMG_LDFLAGS)' -o circumspect-linux .
endif

image: build-linux
	docker build -t $(DOCKER_IMAGE) .

proto:
	protoc --go_out=plugins=grpc:. rpc/rpc.proto

clean:
	rm circumspect circumspect-linux 2>/dev/null || true

.PHONY: build build-linux image proto clean
