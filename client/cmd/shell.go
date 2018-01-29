// Package client which will connect to a server and run a Go command.
package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	pb "github.com/eparis/remote-shell/api"
	"github.com/kr/pretty"
	"github.com/mattn/go-shellwords"
	"github.com/spf13/cobra"
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
	addNodeFlag(shellCmd)
	rootCmd.AddCommand(shellCmd)
}

func doShell(cmd *cobra.Command, args []string) error {
	client, ctx, err := GetGRPCClient(node)
	if err != nil {
		return err
	}
	fmt.Printf("\nYou have successfully connected to %s! To disconnect, hit ctrl+c or type exit.\n", serverAddr)

	// Read in the user's command.
	r := bufio.NewReader(os.Stdin)

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
		stream, err := client.SendCommand(ctx, req)
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
