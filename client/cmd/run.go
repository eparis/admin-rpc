// Package client which will connect to a server and run a Go command.
package cmd

import (
	"fmt"
	"io"
	"log"

	"github.com/kr/pretty"
	"github.com/spf13/cobra"

	rpcapi "github.com/eparis/admin-rpc/api"
)

var (
	_    = pretty.Print
	node string
)

func addNodeFlag(cmd *cobra.Command) {
	cmd.Flags().StringVar(&node, "node", "", "Where to run the command")
	cmd.MarkFlagRequired("node")
	cmd.MarkFlagCustom("node", "__client_get_nodes")
}

func init() {
	// runCmd represents the base command when called without any subcommands
	runCmd := &cobra.Command{
		Use:   "run --node=NODE Command [args]",
		Short: "A REST API client which provides role based operational access to machines",
		RunE:  doRun,
	}
	runCmd.Flags().SetInterspersed(false)
	addNodeFlag(runCmd)
	rootCmd.AddCommand(runCmd)
}

func doRun(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("Must include both a node and a command")
	}
	client, ctx, err := GetGRPCClient(node)
	if err != nil {
		return err
	}

	cmdName := args[0]
	args = args[1:]

	// Gets the response of the shell comm and from the server.
	req := &rpcapi.ExecRequest{
		CmdName: cmdName,
		CmdArgs: args,
	}
	stream, err := client.SendExec(ctx, req)
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
