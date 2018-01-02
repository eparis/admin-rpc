build: test
	cd client; go build
	cd server; go build

test: protobuf
	go test ./...

clean:
	-rm client/client
	-rm server/server

generate:
	go generate github.com/eparis/remote-shell/...

protobuf: generate
	protoc services.proto --go_out=plugins=grpc:.

