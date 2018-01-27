// Package client which will connect to a server and run a Go command.
package cmd

import (
	"fmt"
	"io"
	"log"

	pb "github.com/eparis/remote-shell/api"
	"github.com/kr/pretty"
	"github.com/spf13/cobra"
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
	runCmd.Flags().SetInterspersed(false)
	rootCmd.AddCommand(runCmd)
}

func doRun(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("no command to execute prodived")
	}
	client, ctx, err := GetGRPCCLient()
	if err != nil {
		return err
	}

	cmdName := args[0]
	args = args[1:]

	// Gets the response of the shell comm and from the server.
	req := &pb.CommandRequest{
		CmdName: cmdName,
		CmdArgs: args,
	}
	stream, err := client.SendCommand(ctx, req)
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
