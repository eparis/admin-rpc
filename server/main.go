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

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type serverConfig struct {
	cfgDir      string
	serviceName string
}

var (
	srvCfg = serverConfig{
		cfgDir:      "/etc/remote-shell",
		serviceName: "remote-shell.eparis.svc",
	}

	rootCmd = &cobra.Command{
		Use:   filepath.Base(os.Args[0]),
		Short: "A REST API daemon which provides role based operational access to machines",
		RunE:  mainFunc,
	}
)

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&srvCfg.cfgDir, "config-dir", srvCfg.cfgDir, "config directory")
}

func initConfig() {
	viper.AddConfigPath(srvCfg.cfgDir)
	viper.SetConfigName("config")

	viper.AutomaticEnv() // read in environment variables that match

	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
