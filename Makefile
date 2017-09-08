DOCKER_IMAGE ?= circumspect

IMG_LDFLAGS := -w -linkmode external -extldflags "-static"

HOST_OS := $(shell uname -s | tr A-Z a-z)

MINIKUBE_VERSION = v0.22.0
MINIKUBE_URL     = https://storage.googleapis.com/minikube/releases/$(MINIKUBE_VERSION)/minikube-$(HOST_OS)-amd64
MINIKUBE         = _build/minikube

KUBECTL_VERSION  = v1.7.5
KUBECTL_URL      = https://storage.googleapis.com/kubernetes-release/release/$(KUBECTL_VERSION)/bin/linux/amd64/kubectl
HOST_KUBECTL_URL = https://storage.googleapis.com/kubernetes-release/release/$(KUBECTL_VERSION)/bin/$(HOST_OS)/amd64/kubectl
KUBECTL          = _build/kubectl

# kube wants a fully-qualified image name
MINIKUBE_DOCKER_REPO = circumspect.io
MINIKUBE_SOCKET_PATH = "/tmp/circumspect/socket.socket"

ifdef DEBUG
	DEBUG_ARGS=--log-level=debug
endif

build:
	go build .

ifeq ($(HOST_OS),darwin)
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

integration-minikube: minikube-setup      \
											minikube-reinstall  \
											minikube-run-server

minikube-setup: minikube-start minikube-configure

minikube-start: $(MINIKUBE)
	$(MINIKUBE) start

$(MINIKUBE):
	mkdir -p $(shell dirname $(MINIKUBE))
	curl -Lo $(MINIKUBE) $(MINIKUBE_URL)
	chmod a+x $(MINIKUBE)

$(KUBECTL):
	mkdir -p $(shell dirname $(KUBECTL))
	curl -Lo $(KUBECTL) $(HOST_KUBECTL_URL)
	chmod a+x $(KUBECTL)

# sudo KUBECONFIG=/var/lib/localkube/kubeconfig ./kubectl get pods

minikube-configure:
	$(MINIKUBE) ssh -- sudo chmod a+r /var/lib/localkube/certs/apiserver.key

minikube-install-kubectl:
	$(MINIKUBE) ssh -- curl -LO $(KUBECTL_URL)
	$(MINIKUBE) ssh -- chmod a+x ./kubectl

minikube-reinstall: minikube-reinstall-circumspect minikube-reinstall-image
minikube-reinstall-circumspect: build-linux minikube-install-circumspect
minikube-reinstall-image:       build-linux minikube-install-image

minikube-install-circumspect:
	scp -oStrictHostKeyChecking=no -oUserKnownHostsFile=/dev/null -i$$($(MINIKUBE) ssh-key) \
		circumspect-linux docker@$$($(MINIKUBE) ip):circumspect

minikube-install-image:
	eval $$($(MINIKUBE) docker-env)   && \
	docker build -t $(DOCKER_IMAGE) . && \
	docker tag $(DOCKER_IMAGE) $(MINIKUBE_DOCKER_REPO)/$(DOCKER_IMAGE)

minikube-run-server:
	$(MINIKUBE) ssh -- mkdir -p $(shell dirname $(MINIKUBE_SOCKET_PATH))
	$(MINIKUBE) ssh -- KUBECONFIG=/var/lib/localkube/kubeconfig ./circumspect $(DEBUG_ARGS) --kube server -s $(MINIKUBE_SOCKET_PATH)

minikube-run-docker:
	$(MINIKUBE) ssh -- docker run -it --rm --label foo=bar \
		-v$(MINIKUBE_SOCKET_PATH):$(MINIKUBE_SOCKET_PATH)    \
		$(DOCKER_IMAGE) client -s $(MINIKUBE_SOCKET_PATH)

minikube-create-pod: $(KUBECTL)
	$(KUBECTL) create -f _integration/pod.yml

minikube-delete-pod: $(KUBECTL)
	$(KUBECTL) delete -f _integration/pod.yml --grace-period=0 --force

integration-docker-compose: image
	docker-compose -f _integration/docker-compose.yml up --build

clean:
	rm -rf circumspect circumspect-linux _build

.PHONY: build build-linux image proto install-libs \
	integration-minikube minikube-setup minikube-start minikube-configure minikube-install-kubectl \
	minikube-install-circumspect minikube-install-image \
	minikube-reinstall minikube-reinstall-circumspect minikube-reinstall-image \
	minikube-run-server minikube-run-docker \
	minikube-create-pod minikube-delete-pod \
	integration-docker-compose \
	clean
