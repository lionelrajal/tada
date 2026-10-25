package cmd

import (
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/lionelrajal/tada/store"
)

// App holds what the commands need, so they can be run against a temporary
// data file and writer in tests instead of the process environment.
type App struct {
	// Out receives command results.
	Out io.Writer
	// Err receives diagnostics such as verbose output.
	Err io.Writer

	dataFile string
	verbose  bool
	store    store.Store
}

// NewApp returns an App that writes results to out and diagnostics to stderr.
func NewApp(out io.Writer) *App {
	return &App{Out: out, Err: os.Stderr}
}

// Store returns the persistence layer resolved from the --data-file flag.
func (a *App) Store() store.Store {
	return a.store
}

func (a *App) printf(format string, args ...any) {
	fmt.Fprintf(a.Out, format, args...)
}

func newRootCmd(app *App) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:     "tada",
		Short:   "A command-line todo list",
		Long:    "Tada keeps a todo list in a JSON file.",
		Version: versionString(),
		// main prints the error once, and a runtime failure should not dump
		// the help text over it.
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			s, err := store.NewJSONStore(app.dataFile)
			if err != nil {
				return err
			}
			app.store = s
			if app.verbose {
				fmt.Fprintf(app.Err, "using data file %s\n", app.dataFile)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	rootCmd.SetVersionTemplate("tada {{.Version}}\n")

	rootCmd.PersistentFlags().StringVar(&app.dataFile, "data-file", "tasks.json", "Path to the data file")
	rootCmd.PersistentFlags().BoolVar(&app.verbose, "verbose", false, "Enable verbose output")

	rootCmd.AddCommand(
		newAddCmd(app),
		newEditCmd(app),
		newListCmd(app),
		newDoneCmd(app),
		newRemoveCmd(app),
	)

	return rootCmd
}

// parseIDs converts task ID arguments, rejecting anything that is not a
// positive integer.
func parseIDs(args []string) ([]int64, error) {
	ids := make([]int64, 0, len(args))
	for _, arg := range args {
		id, err := strconv.ParseInt(arg, 10, 64)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("invalid task ID %q", arg)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// Execute runs the CLI with the arguments from the command line.
func Execute() error {
	return newRootCmd(NewApp(os.Stdout)).Execute()
}
