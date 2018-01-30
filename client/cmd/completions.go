// Copyright © 2017 Eric Paris <eparis@redhat.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const (
	customBashCompletionFuncs = `# Print all nodes where the rpc pod is running"
__client_get_nodes()
{
    local client_out
    if client_out=($(client bashcompletion nodes 2>/dev/null)); then
        COMPREPLY=( $( compgen -W "${client_out[*]}" -- "$cur" ) )
    fi
}
`
)

func init() {
	completionsCmd := &cobra.Command{
		Use:    "bashcompletion",
		Short:  "Generate bash completions",
		Hidden: true,
		RunE: func(cmd *cobra.Command, cmdArgs []string) error {
			return rootCmd.GenBashCompletion(os.Stdout)
		},
	}
	rootCmd.AddCommand(completionsCmd)

	getNodesCmd := &cobra.Command{
		Use: "nodes",
		RunE: func(cmd *cobra.Command, cmdArgs []string) error {
			_, clientset, err := getClientset()
			if err != nil {
				return err
			}
			pods, err := getPods(clientset, namespace)
			if err != nil {
				return err
			}
			nodes := getNodes(pods)

			for _, node := range nodes {
				fmt.Fprintf(os.Stdout, "%s ", node)
			}
			return nil
		},
	}
	completionsCmd.AddCommand(getNodesCmd)
}
