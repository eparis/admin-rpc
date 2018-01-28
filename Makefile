all: docker-build

DATE := $(shell date +%F)
GIT := $(shell git rev-parse --short HEAD)
TAG ?= $(DATE)-$(GIT)

REPO ?= docker.io/eparis
APP ?= remote-shell
CONTAINER := $(REPO)/$(APP):$(TAG)

KUBECONFIG ?= $(HOME)/.kube/config
CERTDIR ?= $(CURDIR)/certs

OBJECTDIR ?= $(CURDIR)/objects

# Actual Build Stuff
test: build
	go test ./...

server:
	go install github.com/eparis/remote-shell/server

client:
	go install github.com/eparis/remote-shell/client

build: protobuf server client

clean:
	-rm client/client
	-rm server/server

generate:
	go generate github.com/eparis/remote-shell/...

protobuf: generate
	protoc -I/usr/local/include -I. \
		-I${GOPATH}/src \
		-I${GOPATH}/src/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis \
		--go_out=plugins=grpc:. \
		api/services.proto
	protoc -I/usr/local/include -I. \
		-I${GOPATH}/src \
		-I${GOPATH}/src/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis \
		--grpc-gateway_out=logtostderr=true:. \
		api/services.proto
	protoc -I/usr/local/include -I. \
		-I${GOPATH}/src \
		-I${GOPATH}/src/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis \
		--swagger_out=logtostderr=true:. \
		api/services.proto

bin-copy: build
	cp ${GOPATH}/bin/server bin/
	cp ${GOPATH}/bin/client bin/

test-cmd:
	client run ls -l /
	curl -s -k -H "Authorization: Bearer ${TOKEN}" -d '{"cmdName": "ls", "cmdArgs": ["-l", "/"]}' https://127.0.0.1:12021/v1/command | jq -r '.result.output' | base64 -d

# Mucking with docker containers
docker-build: bin-copy test
	docker build . -t $(CONTAINER)

docker-run: docker-build
	docker run --rm --privileged -v $(KUBECONFIG):/etc/remote-shell/serverKubeConfig -v $(CERTDIR):/etc/remote-shell/certs/ --pid=host --network=host --log-driver=none $(CONTAINER)

docker-push: docker-build
	docker push $(CONTAINER)

docker-clean:
	./docker-clean.sh


# Launching containers on kubernetes
deploy: docker-push
	kubectl apply -f $(OBJECTDIR)/service.yaml
	sed -e 's|@@IMAGE@@|$(CONTAINER)|' $(OBJECTDIR)/daemonset.yaml > $(OBJECTDIR)/local.daemonset.yaml
	kubectl apply -f $(OBJECTDIR)/local.daemonset.yaml --record

.PHONY: client server
