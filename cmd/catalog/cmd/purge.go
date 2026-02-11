package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/nikogura/dbt/pkg/dbt"
	"github.com/spf13/cobra"
)

//nolint:gochecknoglobals // Cobra requires global variables for flags
var purgeAll bool

//nolint:gochecknoglobals // Cobra requires global variables for flags
var purgeOlderThan string

//nolint:gochecknoglobals // Cobra requires global variables for flags
var purgeKeep int

//nolint:gochecknoglobals // Cobra requires global variables for flags
var purgeKeepLatest bool

//nolint:gochecknoglobals // Cobra requires global variables for flags
var purgeDryRun bool

//nolint:gochecknoglobals // Cobra requires global variables for flags
var purgeYes bool

// purgeCmd represents the purge command.
//
//nolint:gochecknoglobals // Cobra boilerplate
var purgeCmd = &cobra.Command{
	Use:   "purge <toolname>",
	Short: "Purge tool versions from the repository.",
	Long: `
Purge tool versions from the repository.

Deletes old or unwanted versions of a tool. Requires either --all or --older-than.

Examples:
  catalog purge mytool --all                    # delete entire tool
  catalog purge mytool --older-than 30d         # delete versions older than 30 days
  catalog purge mytool --older-than 30d --keep 3  # age-based with minimum retention
  catalog purge mytool --keep-latest            # keep only the latest version
  catalog purge mytool --dry-run --older-than 7d  # preview without deleting
`,
	Args: cobra.ExactArgs(1),
	Run:  runPurge,
}

func runPurge(cmd *cobra.Command, args []string) {
	toolName := args[0]

	if !purgeAll && purgeOlderThan == "" && !purgeKeepLatest {
		fmt.Println("Error: specify --all, --older-than, or --keep-latest")
		os.Exit(1)
	}

	serverFlag := os.Getenv("DBT_SERVER")

	dbtObj, _, err := dbt.NewDbtWithServer("", serverFlag)
	if err != nil {
		log.Fatalf("Error creating DBT object: %s", err)
	}

	if toolsRepo := os.Getenv("DBT_TOOLS_REPO"); toolsRepo != "" {
		dbtObj.Config.Tools.Repo = toolsRepo
	}

	dbtObj.SetVerbose(verbose)

	opts := dbt.PurgeOptions{
		ToolName: toolName,
		All:      purgeAll,
		Keep:     purgeKeep,
		DryRun:   purgeDryRun,
		Yes:      purgeYes,
	}

	if purgeKeepLatest {
		opts.Keep = 1
	}

	if purgeOlderThan != "" {
		duration, parseErr := dbt.ParseDuration(purgeOlderThan)
		if parseErr != nil {
			log.Fatalf("Invalid duration %q: %s", purgeOlderThan, parseErr)
		}
		opts.OlderThan = duration
	}

	err = dbtObj.PurgeTool(opts)
	if err != nil {
		fmt.Printf("Error running purge: %s\n", err)
		os.Exit(1)
	}
}

//nolint:gochecknoinits // Cobra boilerplate
func init() {
	RootCmd.AddCommand(purgeCmd)

	purgeCmd.Flags().BoolVar(&purgeAll, "all", false, "Delete the entire tool and all versions")
	purgeCmd.Flags().StringVar(&purgeOlderThan, "older-than", "", "Delete versions older than duration (e.g., 30d, 2w, 24h)")
	purgeCmd.Flags().IntVar(&purgeKeep, "keep", 0, "Minimum number of versions to retain")
	purgeCmd.Flags().BoolVar(&purgeKeepLatest, "keep-latest", false, "Keep only the latest version (shortcut for --keep 1)")
	purgeCmd.Flags().BoolVar(&purgeDryRun, "dry-run", false, "Preview deletions without actually deleting")
	purgeCmd.Flags().BoolVarP(&purgeYes, "yes", "y", false, "Skip confirmation prompt")
}
