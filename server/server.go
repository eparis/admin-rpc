// Package server will execute all commands issued by connected clients.
package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"strings"

	pb "github.com/eparis/remote-shell/api"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/grpc-ecosystem/go-grpc-middleware/auth"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/logrus"
	"github.com/grpc-ecosystem/go-grpc-middleware/tags"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/kr/pretty"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	//"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	authnv1 "k8s.io/api/authentication/v1"
	//authzv1 "k8s.io/api/authorization/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/eparis/remote-shell/operations/command"
	"github.com/eparis/remote-shell/operations/util"
)

var (
	_          = pretty.Print
	kubeConfig *rest.Config
	localAddr  = fmt.Sprintf("localhost:%d", port)
)

func parseToken(token string) (*authnv1.TokenReview, error) {
	clientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return nil, err
	}
	tr := &authnv1.TokenReview{
		Spec: authnv1.TokenReviewSpec{
			Token: token,
		},
	}
	tr, err = clientset.AuthenticationV1().TokenReviews().Create(tr)
	if err != nil {
		return nil, err
	}

	return tr, nil
}

func userNameFromToken(tr *authnv1.TokenReview) string {
	return tr.Status.User.Username
}

func uidFromToken(tr *authnv1.TokenReview) string {
	return tr.Status.User.UID
}

func exampleAuthFunc(ctx context.Context) (context.Context, error) {
	token, err := grpc_auth.AuthFromMD(ctx, "bearer")
	if err != nil {
		return nil, err
	}
	tokenInfo, err := parseToken(token)
	if err != nil {
		return nil, grpc.Errorf(codes.Unauthenticated, "invalid auth token: %v", err)
	}
	// adds auth.username and auth.uid to the audit messages
	grpc_ctxtags.Extract(ctx).Set("auth.username", userNameFromToken(tokenInfo))
	grpc_ctxtags.Extract(ctx).Set("auth.uid", uidFromToken(tokenInfo))
	// store the token for later
	newCtx := util.PutToken(ctx, tokenInfo)
	return newCtx, nil
}

// grpcHandlerFunc returns an http.Handler that delegates to grpcServer on incoming gRPC
// connections or otherHandler otherwise.
func grpcHandlerFunc(grpcServer *grpc.Server, otherHandler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proto := r.ProtoMajor
		contentType := r.Header.Get("Content-Type")
		if proto == 2 && strings.Contains(contentType, "application/grpc") {
			grpcServer.ServeHTTP(w, r)
		} else {
			otherHandler.ServeHTTP(w, r)
		}
	})
}

func mainFunc(cmd *cobra.Command, args []string) error {
	pretty.Println(srvCfg)
	serverKubeConfig := filepath.Join(srvCfg.cfgDir, "serverKubeConfig")
	config, err := clientcmd.BuildConfigFromFlags("", serverKubeConfig)
	if err != nil {
		log.Fatal("Unable to load kubeconfig: %v\n", err)
	}
	kubeConfig = config

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
		grpc.Creds(credentials.NewClientTLSFromCert(demoCertPool, localAddr)),
		grpc_middleware.WithStreamServerChain(
			grpc_ctxtags.StreamServerInterceptor(),
			grpc_logrus.StreamServerInterceptor(logrus.NewEntry(logrus.New()), logrusOpts...),
			grpc_prometheus.StreamServerInterceptor,
			grpc_auth.StreamServerInterceptor(exampleAuthFunc),
		),
		grpc_middleware.WithUnaryServerChain(
			grpc_ctxtags.UnaryServerInterceptor(),
			grpc_logrus.UnaryServerInterceptor(logrus.NewEntry(logrus.New()), logrusOpts...),
			grpc_prometheus.UnaryServerInterceptor,
			grpc_auth.UnaryServerInterceptor(exampleAuthFunc),
		),
	}
	// Initializes the gRPC server.
	grpcServer := grpc.NewServer(serverOpts...)

	// Register the SendCommand with gRPC.
	sndCmd := command.NewSendCommand()
	pb.RegisterRemoteCommandServer(grpcServer, sndCmd)

	// Register reflection service on gRPC server.
	reflection.Register(grpcServer)

	// After all your registrations, make sure all of the Prometheus metrics are initialized.
	grpc_prometheus.Register(grpcServer)

	mux := http.NewServeMux()

	gwmux := runtime.NewServeMux()

	dcreds := credentials.NewTLS(&tls.Config{
		ServerName: localAddr,
		RootCAs:    demoCertPool,
	})
	dopts := []grpc.DialOption{
		grpc.WithTransportCredentials(dcreds),
	}

	err = pb.RegisterRemoteCommandHandlerFromEndpoint(ctx, gwmux, localAddr, dopts)
	if err != nil {
		log.Fatal("RegisterRemoteCommandHandlerFromEndpoint: %v\n", err)
	}

	// Register Prometheus metrics handler.
	mux.Handle("/metrics", promhttp.Handler())
	mux.Handle("/", gwmux)

	conn, err := net.Listen("tcp", srvCfg.bindAddr)
	if err != nil {
		log.Fatalf("Failed to listen %v", err)
	}

	srv := &http.Server{
		Addr:    localAddr,
		Handler: grpcHandlerFunc(grpcServer, mux),
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{*demoKeyPair},
			NextProtos:   []string{"h2"},
		},
	}

	if err := srv.Serve(tls.NewListener(conn, srv.TLSConfig)); err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
	return nil
}
