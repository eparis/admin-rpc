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

build: protobuf
	go install github.com/eparis/remote-shell/server
	go install github.com/eparis/remote-shell/client

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
deploy: docker-push deployment
	kubectl apply -f $(OBJECTDIR)/service.yaml
	kubectl apply -f $(OBJECTDIR)/local.deployment.yaml --record

# updates the deployment.yaml with current build information and sets it to --dry-run
deployment:
	sed -e 's|@@IMAGE@@|$(CONTAINER)|' $(OBJECTDIR)/deployment.yaml > $(OBJECTDIR)/local.deployment.yaml
