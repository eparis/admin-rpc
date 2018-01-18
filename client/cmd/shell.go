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
	"github.com/kr/pretty"
	"github.com/mattn/go-shellwords"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	_ = pretty.Print
)

func init() {
	// shellCmd represents the base command when called without any subcommands
	shellCmd := &cobra.Command{
		Use:   "shell",
		Short: "A simple \"shell\" on the remote server. Easy to run multiple commands in a row",
		RunE:  doShell,
	}
	rootCmd.AddCommand(shellCmd)
}

func doShell(cmd *cobra.Command, args []string) error {
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigFile)
	if err != nil {
		log.Fatal("Unable to load kubeconfig: %v\n", err)
	}
	token := config.BearerToken
	// Read in the user's command.
	r := bufio.NewReader(os.Stdin)

	creds, err := credentials.NewClientTLSFromFile("certs/CA.crt", serverAddr)
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

		args, err := shellwords.Parse(tCmd)
		if err != nil {
			fmt.Printf("Unable to parse command; %v", err)
			continue
		}
		if len(args) < 1 {
			// User hit enter with no command, just look for another one.
			continue
		}
		cmdName := args[0]
		cmdArgs := args[1:]

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
			fmt.Printf("Command failed: %v\n", err)
			continue
		}

		for {
			res, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					break
				}
				fmt.Printf("Command failed: %v\n", err)
				break
			}
			fmt.Printf("%s", res.Output)
		}
	}
	return nil
}
