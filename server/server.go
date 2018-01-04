// Package server will execute all commands issued by connected clients.
package main

import (
	"bytes"
	//"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"

	pb "github.com/eparis/remote-shell/api"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/logrus"
	"github.com/grpc-ecosystem/go-grpc-middleware/tags"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/kr/pretty"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var (
	_ = pretty.Print
)

// The port the server is listening on.
const (
	port = 12021
)

var (
	localAddr = fmt.Sprintf("localhost:%d", port)
)

type streamWriter struct {
	stream pb.RemoteCommand_SendCommandServer
}

func (sw streamWriter) Write(p []byte) (int, error) {
	cr := &pb.CommandReply{
		Output: p,
	}
	if err := sw.stream.Send(cr); err != nil {
		return 0, err
	}
	return len(p), nil
}

// Server is used to implement the RemoteCommandServer
type server struct{}

func ExecuteCmdNamespace(cmdName string, args []string, stream pb.RemoteCommand_SendCommandServer) error {

	outPipe, pw, err := os.Pipe()
	if err != nil {
		return err
	}

	cmd := exec.Command(cmdName, args...)
	cmd.Stdout = pw
	cmd.Stderr = pw

	if err := cmd.Start(); err != nil {
		return err
	}

	finished := make(chan bool)

	// When the process ends, close the pipe. This will cause the io.Copy() to
	// hit EOF and return.
	go func() {
		cmd.Wait()
		pw.Close()
	}()

	// If the client closes the stream mark that we are finished so we may
	// stop the exec early if needed.
	go func() {
		select {
		case <-stream.Context().Done():
			finished <- true
		}
	}()

	// If the io.Copy() returned that means we either hit an error or outPipe
	// return EOF. In either case, we've done all we can do, so indicate we
	// are finished and should return.
	go func() {
		defer func() {
			finished <- true
			outPipe.Close()
		}()
		for {
			sw := streamWriter{
				stream: stream,
			}
			buf := bytes.Buffer{}
			l, err := io.Copy(sw, outPipe)
			if err != nil || l == 0 {
				return
			}
			cr := &pb.CommandReply{
				Output: buf.Bytes(),
			}
			if err := stream.Send(cr); err != nil {
				return
			}
		}
	}()

	select {
	case <-finished:
		// If the process is still running after we are finished
		// we should kill it. After the call to Wait() cmd.ProcessState
		// should be non-nil.
		if cmd.ProcessState == nil || !cmd.ProcessState.Exited() {
			cmd.Process.Kill()
		}
	}

	return nil
}

// SendCommand receives the command from the client and then executes it server-side.
// It returns a commmand reply consisting of the output of the command.
func (s *server) SendCommand(in *pb.CommandRequest, stream pb.RemoteCommand_SendCommandServer) error {
	var cmdName = in.CmdName
	var cmdArgs = in.CmdArgs

	return ExecuteCmdNamespace(cmdName, cmdArgs, stream)
}

func (s *server) RemoteShell(stream pb.RemoteCommand_RemoteShellServer) error {
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		cmdName := in.CmdName
		cmdArgs := in.CmdArgs

		if err := ExecuteCmdNamespace(cmdName, cmdArgs, stream); err != nil {
			return err
		}
	}
	return nil
}

// grpcHandlerFunc returns an http.Handler that delegates to grpcServer on incoming gRPC
// connections or otherHandler otherwise. Copied from cockroachdb.
func grpcHandlerFunc(grpcServer *grpc.Server, otherHandler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ProtoMajor == 2 && strings.Contains(r.Header.Get("Content-Type"), "application/grpc") {
			grpcServer.ServeHTTP(w, r)
		} else {
			otherHandler.ServeHTTP(w, r)
		}
	})
}

func main() {
	ctx := context.Background()

	logrusOpts := []grpc_logrus.Option{
		grpc_logrus.WithDecider(func(methodFullName string, err error) bool {
			// will not log gRPC calls if it was a call to healthcheck and no error was raised
			if err == nil && methodFullName == "blah.foo.healthcheck" {
				return false
			}

			// by default you will log all calls
			return true
		}),
	}

	serverOpts := []grpc.ServerOption{
		grpc_middleware.WithStreamServerChain(
			grpc_ctxtags.StreamServerInterceptor(),
			grpc_logrus.StreamServerInterceptor(logrus.NewEntry(logrus.New()), logrusOpts...),
			grpc_prometheus.StreamServerInterceptor,
		),
		grpc_middleware.WithUnaryServerChain(
			grpc_ctxtags.UnaryServerInterceptor(),
			grpc_logrus.UnaryServerInterceptor(logrus.NewEntry(logrus.New()), logrusOpts...),
			grpc_prometheus.UnaryServerInterceptor,
		),
	}
	// Initializes the gRPC server.
	grpcServer := grpc.NewServer(serverOpts...)

	// Register the server with gRPC.
	pb.RegisterRemoteCommandServer(grpcServer, &server{})

	// Register reflection service on gRPC server.
	reflection.Register(grpcServer)

	// After all your registrations, make sure all of the Prometheus metrics are initialized.
	grpc_prometheus.Register(grpcServer)

	mux := http.NewServeMux()

	gwmux := runtime.NewServeMux()

	dopts := []grpc.DialOption{
		grpc.WithInsecure(),
	}
	err := pb.RegisterRemoteCommandHandlerFromEndpoint(ctx, gwmux, localAddr, dopts)
	if err != nil {
		fmt.Printf("RegisterRemoteCommandHandlerFromEndpoint: %v\n", err)
		return
	}

	// Register Prometheus metrics handler.
	mux.Handle("/metrics", promhttp.Handler())
	mux.Handle("/", gwmux)

	conn, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("Failed to listen %v", err)
	}

	srv := &http.Server{
		Addr:    localAddr,
		Handler: grpcHandlerFunc(grpcServer, mux),
		//TLSConfig: &tls.Config{
		//Certificates: []tls.Certificate{*demoKeyPair},
		//NextProtos:   []string{"h2"},
		//},
	}

	fmt.Printf("grpc on port: %d\n", port)
	if err := srv.Serve(conn); err != nil {
		log.Fatal("ListenAndServe: ", err)
	}

	//if err := s.Serve(conn); err != nil {
	//log.Fatalf("Failed to serve: %v", err)
	//}
}
