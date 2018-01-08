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
	"path/filepath"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	port = 12021
)

var (
	caFile         = "certs/CA.pem"
	cfgDir         string
	cfgFile        string
	kubeConfigFile string
	serverAddr     = fmt.Sprintf("localhost:%d", port)
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   filepath.Base(os.Args[0]),
	Short: "A REST API client which provides role based operational access to machines",
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&caFile, "ca-file", caFile, "path to ca certificate to authenticate server")
	rootCmd.PersistentFlags().StringVar(&kubeConfigFile, "kubeconfig-file", kubeConfigFile, fmt.Sprintf("kubeconfig file (default $HOME/.kube/config)"))
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config-file", cfgFile, fmt.Sprintf("config file (default $HOME/.ops-client.yaml)"))
	rootCmd.PersistentFlags().StringVar(&cfgDir, "config-dir", cfgDir, "config directory (default $HOME)")
	rootCmd.PersistentFlags().StringVar(&serverAddr, "server", serverAddr, "URL of server")
}

func initConfig() {
	// Find home directory.
	home, err := homedir.Dir()
	if err != nil && (cfgFile == "" || kubeConfigFile == "") {
		fmt.Printf("--kubeconfig-file or --config-file is unset and unable to determine $HOME: %v\n", err)
		os.Exit(1)
	}
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		if cfgDir == "" {
			cfgDir = home
		}

		// Search config in home directory with name ".kube-access-client" (without extension).
		viper.AddConfigPath(cfgDir)
		viper.SetConfigName(".kube-access-client")
	}
	if kubeConfigFile == "" {
		kubeConfigFile = filepath.Join(home, ".kube/config")
	}

	viper.AutomaticEnv() // read in environment variables that match

	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
