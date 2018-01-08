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

docker-build: bin-copy test
	docker build . -t eparis/remote-shell:latest

docker-run: docker-build
	docker run --rm --privileged --pid=host --network=host --log-driver=none eparis/remote-shell:latest

docker-clean:
	./docker-clean.sh

