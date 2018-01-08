// Package client which will connect to a server and run a Go command.
package cmd

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

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

func attachToken(ctx context.Context, token string) context.Context {
	md := metadata.Pairs("authorization", fmt.Sprintf("bearer %s", token))
	return metautils.NiceMD(md).ToOutgoing(ctx)
}

func doIt(cmd *cobra.Command, args []string) error {
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigFile)
	if err != nil {
		log.Fatal("Unable to load kubeconfig: %v\n", err)
	}
	token := config.BearerToken
	// Read in the user's command.
	r := bufio.NewReader(os.Stdin)

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

	fmt.Printf("\nYou have successfully connected to %s! To disconnect, hit ctrl+c or type exit.\n", serverAddr)

	// Keep connection alive until ctrl+c or exit is entered.
	for true {
		fmt.Print("$ ")
		tCmd, _ := r.ReadString('\n')

		// This strips off any trailing whitespace/carriage returns.
		tCmd = strings.TrimSpace(tCmd)
		tCmd2 := strings.Split(tCmd, " ")

		// Parse their input.
		cmdName := tCmd2[0]

		//cmdArgs := []string{}
		cmdArgs := tCmd2[1:]

		// Close the connection if the user enters exit.
		if cmdName == "exit" {
			break
		}

		// Gets the response of the shell comm and from the server.
		req := &pb.CommandRequest{
			CmdName: cmdName,
			CmdArgs: cmdArgs,
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
				log.Fatalf("%v.SendCommand(_) = _, %v", c, err)
			}
			fmt.Printf("%s", res.Output)
		}
	}
	return nil
}
