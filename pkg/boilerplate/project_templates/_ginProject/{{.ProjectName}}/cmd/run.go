// +build exclude {{.ProjectName}}

/*
Copyright © 2022 {{.MaintainerName}} <{{.MaintainerEmail}}>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"github.com/{{.ProjectName}}/pkg/config"
	"github.com/{{.ProjectName}}/pkg/{{.ProjectPackage}}"
)

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const DefaultServerPort = {{.DefaultServerPort}}

var addr string
var port int
var apiKey string

func NewServerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "{{.ServerShortDesc}}",
		Long:  `{{.ServerLongDesc}}`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// You can bind cobra and viper in a few locations, but PersistencePreRunE on the root command works well
			return initializeConfig(cmd)
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return validateConfig(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {

			config := config.Config{
				Addr:     addr,
				Port:     port,
			}

			logrus.WithFields(logrus.Fields{
				"version": cmd.Version,
				"name":    cmd.Name(),
				"desc":    cmd.Short,
				"config":  fmt.Sprintf("%+v", config),
			}).Info("starting {{.ProjectName}} server")
			return {{.ProjectPackage}}.Run(config)
		},
	}

	cmd.Flags().StringVar(&addr, "addr", "http://0.0.0.0", "server ip addr")
	cmd.Flags().IntVar(&port, "port", DefaultServerPort, "server port")

	return cmd
}
