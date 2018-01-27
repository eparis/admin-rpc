package cmd

import (
	"fmt"

	pb "github.com/eparis/remote-shell/api"
	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"k8s.io/client-go/tools/clientcmd"
)

func attachToken(ctx context.Context, token string) context.Context {
	md := metadata.Pairs("authorization", fmt.Sprintf("bearer %s", token))
	return metautils.NiceMD(md).ToOutgoing(ctx)
}

func GetGRPCCLient() (pb.RemoteCommandClient, context.Context, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigFile)
	if err != nil {
		return nil, nil, fmt.Errorf("Unable to load kubeconfig: %v\n", err)
	}
	token := config.BearerToken

	creds, err := credentials.NewClientTLSFromFile("certs/CA.crt", "remote-shell.eparis.svc")
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to create TLS credentials %v", err)
	}
	dopts := []grpc.DialOption{grpc.WithDefaultCallOptions()}
	dopts = append(dopts, grpc.WithTransportCredentials(creds))

	conn, err := grpc.Dial(serverAddr, dopts...)
	if err != nil {
		return nil, nil, fmt.Errorf("Could not connect: %v", err)
	}

	// Create the client
	c := pb.NewRemoteCommandClient(conn)

	ctx := context.Background()
	ctx = attachToken(ctx, token)

	return c, ctx, nil
}
