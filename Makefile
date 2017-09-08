DOCKER_IMAGE ?= circumspect

IMG_LDFLAGS := -w -linkmode external -extldflags "-static"

MINIKUBE_URL := https://storage.googleapis.com/minikube/releases/latest/minikube-$(shell uname -s | tr A-Z a-z)-amd64
MINIKUBE     := _build/minikube

KUBECTL_VERSION = $(shell curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)
KUBECTL_URL     = https://storage.googleapis.com/kubernetes-release/release/$(KUBECTL_VERSION)/bin/linux/amd64/kubectl

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

install-libs:
	govendor build -i +local,^program

integration: image
	docker-compose up --build

$(MINIKUBE):
	mkdir -p $(shell dirname $(MINIKUBE))
	curl -Lo $(MINIKUBE) $(MINIKUBE_URL)
	chmod a+x $(MINIKUBE)

minikube-start: $(MINIKUBE)
	$(MINIKUBE) start

minikube-install-kubectl:
	$(MINIKUBE) ssh -- curl -LO $(KUBECTL_URL)
	$(MINIKUBE) ssh -- chmod a+x ./kubectl

minikube-install-circumspect: build-linux
	scp -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i $$($(MINIKUBE) ssh-key) circumspect-linux docker@$$($(MINIKUBE) ip):circumspect

minikube-install-image:
	eval $$($(MINIKUBE) docker-env) && docker build -t $(DOCKER_IMAGE) .

clean:
	rm circumspect circumspect-linux 2>/dev/null || true

.PHONY: build build-linux image proto clean \
	install-libs integration \
	$(MINIKUBE) minikube-install-kubectl \
	minikube-install-circumspect minikube-install-image
