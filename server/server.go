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
	"github.com/gorilla/mux"
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
	authnv1 "k8s.io/api/authentication/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	_ "net/http/pprof"

	"github.com/eparis/remote-shell/operations/util"

	// All of the rpc operations we support
	"github.com/eparis/remote-shell/operations/command"
)

var (
	_          = pretty.Print
	kubeConfig *rest.Config
	localAddr  = fmt.Sprintf("localhost:%d", port)
)

// validateToken will ask the Kubernetes API Server to do a TokenReview
func validateToken(clientset *kubernetes.Clientset, token string) (*authnv1.TokenReview, error) {
	tr := &authnv1.TokenReview{
		Spec: authnv1.TokenReviewSpec{
			Token: token,
		},
	}
	tr, err := clientset.AuthenticationV1().TokenReviews().Create(tr)
	if err != nil {
		return nil, err
	}
	if !tr.Status.Authenticated {
		if tr.Status.Error != "" {
			return nil, fmt.Errorf("%s", tr.Status.Error)
		}
		return nil, fmt.Errorf("Response from RokenReview was unauthenticated")
	}

	return tr, nil
}

// attachAuthnData will attach the kubernetes clientset and TokenReview to the context.Context
func attachAuthnData(ctx context.Context) (context.Context, error) {
	clientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return nil, err
	}

	token, err := grpc_auth.AuthFromMD(ctx, "bearer")
	if err != nil {
		return nil, err
	}
	tokenInfo, err := validateToken(clientset, token)
	if err != nil {
		return nil, grpc.Errorf(codes.Unauthenticated, "invalid auth token: %v", err)
	}
	// adds auth.username and auth.uid to the audit messages
	grpc_ctxtags.Extract(ctx).Set("auth.username", tokenInfo.Status.User.Username)
	grpc_ctxtags.Extract(ctx).Set("auth.uid", tokenInfo.Status.User.UID)
	// store the token for later
	ctx = util.PutToken(ctx, tokenInfo)
	ctx = util.PutClientset(ctx, clientset)
	return ctx, nil
}

// Register all of the operations which are defined with the server
func registerAllOperations(grpcServer *grpc.Server) error {
	sndCmd, err := command.NewSendCommand(srvCfg.cfgDir)
	if err != nil {
		return err
	}
	pb.RegisterRemoteCommandServer(grpcServer, sndCmd)

	return nil
}

// isGRPC returns true if the traffic is http/2 and Content-Type==application/grpc
func isGRPC(r *http.Request, rm *mux.RouteMatch) bool {
	if r.ProtoMajor != 2 {
		return false
	}
	contentType := r.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/grpc") {
		return false
	}
	return true
}

func mainFunc(cmd *cobra.Command, args []string) error {
	pretty.Println(srvCfg)
	serverKubeConfig := filepath.Join(srvCfg.cfgDir, "serverKubeConfig")
	config, err := clientcmd.BuildConfigFromFlags("", serverKubeConfig)
	if err != nil {
		log.Fatal("Unable to load kubeconfig: %v\n", err)
	}
	kubeConfig = config

	err = initCerts()
	if err != nil {
		return err
	}

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
			grpc_auth.StreamServerInterceptor(attachAuthnData),
		),
		grpc_middleware.WithUnaryServerChain(
			grpc_ctxtags.UnaryServerInterceptor(),
			grpc_logrus.UnaryServerInterceptor(logrus.NewEntry(logrus.New()), logrusOpts...),
			grpc_prometheus.UnaryServerInterceptor,
			grpc_auth.UnaryServerInterceptor(attachAuthnData),
		),
	}
	// Initializes the gRPC server.
	grpcServer := grpc.NewServer(serverOpts...)

	// This registers all of the things we can do!
	if err := registerAllOperations(grpcServer); err != nil {
		return err
	}

	// After all your registrations, make sure all of the Prometheus metrics are initialized.
	grpc_prometheus.Register(grpcServer)

	dcreds := credentials.NewTLS(&tls.Config{
		ServerName: localAddr,
		RootCAs:    demoCertPool,
	})
	dopts := []grpc.DialOption{
		grpc.WithTransportCredentials(dcreds),
	}

	// Load all GRPC endpoints into the json gateway mux
	gwmux := runtime.NewServeMux()
	err = pb.RegisterRemoteCommandHandlerFromEndpoint(ctx, gwmux, localAddr, dopts)
	if err != nil {
		log.Fatal("RegisterRemoteCommandHandlerFromEndpoint: %v\n", err)
	}

	// This is the main router for the remote-shell
	router := mux.NewRouter()

	// Send all grpc traffic to the grpc server
	router.PathPrefix("/").HandlerFunc(grpcServer.ServeHTTP).MatcherFunc(isGRPC)

	s := router.Methods("GET").Subrouter()
	// Register Prometheus metrics handler.
	s.PathPrefix("/metrics").Handler(promhttp.Handler())
	// Server the /static dir so users can download the client
	s.PathPrefix("/static").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("/static"))))
	// pprof loads itself to http.DefaultServeMux
	s.PathPrefix("/debug/pprof/").Handler(http.DefaultServeMux)

	// Send everything else to the json->grpc gateway mux
	router.PathPrefix("/").Handler(gwmux)

	conn, err := net.Listen("tcp", srvCfg.bindAddr)
	if err != nil {
		log.Fatalf("Failed to listen %v", err)
	}

	srv := &http.Server{
		Addr:    localAddr,
		Handler: router,
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
