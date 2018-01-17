// Package client which will connect to a server and run a Go command.
package cmd

import (
	"fmt"
	"io"
	"log"

	pb "github.com/eparis/remote-shell/api"
	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"github.com/kr/pretty"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	_ = pretty.Print
)

func init() {
	// runCmd represents the base command when called without any subcommands
	runCmd := &cobra.Command{
		Use:   "run",
		Short: "A REST API client which provides role based operational access to machines",
		RunE:  doRun,
	}
	rootCmd.AddCommand(runCmd)
}

func attachToken(ctx context.Context, token string) context.Context {
	md := metadata.Pairs("authorization", fmt.Sprintf("bearer %s", token))
	return metautils.NiceMD(md).ToOutgoing(ctx)
}

func doRun(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("no command to execute prodived")
	}
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigFile)
	if err != nil {
		log.Fatal("Unable to load kubeconfig: %v\n", err)
	}
	token := config.BearerToken

	creds, err := credentials.NewClientTLSFromFile("certs/CA.pem", serverAddr)
	if err != nil {
		log.Fatalf("Failed to create TLS credentials %v", err)
	}
	dopts := []grpc.DialOption{grpc.WithDefaultCallOptions()}
	dopts = append(dopts, grpc.WithTransportCredentials(creds))
	// Set up a connection to the server.
	conn, err := grpc.Dial(serverAddr, dopts...)

	if err != nil {
		log.Fatalf("Could not connect: %v", err)
	}

	// Close the connection after main returns.
	defer conn.Close()

	// Create the client
	c := pb.NewRemoteCommandClient(conn)

	cmdName := args[0]
	args = args[1:]

	// Gets the response of the shell comm and from the server.
	req := &pb.CommandRequest{
		CmdName: cmdName,
		CmdArgs: args,
	}
	ctx := context.Background()
	ctx = attachToken(ctx, token)
	stream, err := c.SendCommand(ctx, req)
	if err != nil {
		log.Fatalf("Command failed: %v", err)
	}

	for {
		res, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Fatalf("%v\n", err)
		}
		fmt.Printf("%s", res.Output)
	}
	return nil
}
