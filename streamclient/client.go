// Package client which will connect to a server and run a Go command.
package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	pb "github.com/eparis/remote-shell/api"
	"github.com/kr/pretty"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

const (
	port = ":12021"
)

var (
	_ = pretty.Print
)

func main() {
	// Read in the user's command.
	r := bufio.NewReader(os.Stdin)

	address := "127.0.0.1" + port

	// Set up a connection to the server.
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Could not connect: %v", err)
	}
	defer conn.Close()

	// Create the client
	c := pb.NewRemoteCommandClient(conn)
	fmt.Printf("\nYou have successfully connected to %s! To disconnect, hit ctrl+c or type exit.\n", address)

	stream, err := c.RemoteShell(context.Background())
	if err != nil {
		log.Fatal("Call to %v.RemoteShell() failed: %v", c, err)
	}
	defer stream.CloseSend()

	go func() {
		for {
			res, err := stream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				log.Fatalf("%v.RemoteShell(_) = _, %v", c, err)
			}
			log.Printf("    %s", res.Output)
		}
	}()

	// Keep connection alive until ctrl+c or exit is entered.
	for {
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

		// Send the request to the shell server:
		req := &pb.CommandRequest{
			CmdName: cmdName,
			CmdArgs: cmdArgs,
		}
		err := stream.Send(req)
		if err != nil {
			log.Fatalf("Command failed: %v", err)
		}
	}
}
