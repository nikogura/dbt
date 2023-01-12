// +build exclude {{.ProjectName}}

package cmd

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"os"
	"strings"
)

const (
	EnvPrefix = "{{.EnvPrefix}}"
)

var configFile string
var configSearchPath []string

func init() {
	logrus.SetFormatter(&logrus.JSONFormatter{})
}

func NewRootCommand() *cobra.Command {
	// Store the result of binding cobra flags and viper config. In a
	// real application these would be data structures, most likely
	// custom structs per command. This is simplified for the demo app and is
	// not recommended that you use one-off variables. The point is that we
	// aren't retrieving the values directly from viper or flags, we read the values
	// from standard Go data structures.

	// Define our command
	rootCmd := &cobra.Command{
		Use:   "{{.ProjectName}}",
		Short: "{{.ProjectShortDesc}}",
		Long:  `{{.ProjectLongDesc}}`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// You can bind cobra and viper in a few locations, but PersistencePreRunE on the root command works well
			return initializeConfig(cmd)
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return validateConfig(cmd)
		},
	}

	SetupRootFlags(rootCmd)
	rootCmd.AddCommand(NewServerCommand())
	return rootCmd
}

func initializeConfig(cmd *cobra.Command) error {
	v := viper.New()

	cfgFile, err := cmd.Flags().GetString("config_file")
	if err != nil {
		return err
	}

	if f, ok := os.LookupEnv(fmt.Sprintf("%s_CONFIG_FILE", EnvPrefix)); ok {
		cfgFile = f
	}
	// Set the base name of the config file, without the file extension.
	v.SetConfigName(cfgFile)

	cfgSearchPath, err := cmd.Flags().GetStringSlice("config_search_path")
	if err != nil {
		return err
	}

	if p, ok := os.LookupEnv(fmt.Sprintf("%s_CONFIG_SEARCH_PATH", EnvPrefix)); ok {
		cfgSearchPath = strings.FieldsFunc(p, func(r rune) bool {
			return r == ',' || r == ' '
		})
	}
	// Set as many paths as you like where viper should look for the
	// config file. We are only looking in the current working directory.
	for _, path := range cfgSearchPath {
		v.AddConfigPath(path)
	}

	// Attempt to read the config file, gracefully ignoring errors
	// caused by a config file not being found. Return an error
	// if we cannot parse the config file.
	if err := v.ReadInConfig(); err != nil {
		// It's okay if there isn't a config file
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return err
		}
	}

	// When we bind flags to environment variables expect that the
	// environment variables are prefixed, e.g. a flag like --number
	// binds to an environment variable {{.EnvPrefix}}_NUMBER. This helps
	// avoid conflicts.
	v.SetEnvPrefix(EnvPrefix)

	// Bind to environment variables
	// Works great for simple config names, but needs help for names
	// like --favorite-color which we fix in the bindFlags function
	v.AutomaticEnv()

	// Bind the current command's flags to viper
	bindFlags(cmd, v)

	return nil
}

// Bind each cobra flag to its associated viper configuration (config file and environment variable)
func bindFlags(cmd *cobra.Command, v *viper.Viper) {
	v.BindPFlags(cmd.PersistentFlags())
	v.BindPFlags(cmd.Flags())

	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		// Environment variables can't have dashes in them, so bind them to their equivalent
		// keys with underscores, e.g. --pg-host to {{.EnvPrefix}}_PG_HOST
		if strings.Contains(f.Name, "-") {
			envVarSuffix := strings.ToUpper(strings.ReplaceAll(f.Name, "-", "_"))
			v.BindEnv(f.Name, fmt.Sprintf("%s_%s", EnvPrefix, envVarSuffix))
		}

		// Apply the viper config value to the flag when the flag is not set and viper has a value
		if !f.Changed && v.IsSet(f.Name) {
			val := v.Get(f.Name)
			cmd.Flags().Set(f.Name, fmt.Sprintf("%v", val))
		}
	})
}

func SetupRootFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVar(&configFile, "config_file", "{{.ProjectName}}_config", "the configuration file, no extension")
	cmd.PersistentFlags().StringSliceVar(&configSearchPath, "config_search_path", []string{".", "~"}, "the list of paths to search for the config file")
}

func validateConfig(cmd *cobra.Command) error {
	// Check for valid flag configurations here
	return nil
}
