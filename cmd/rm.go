package cmd

import (
	"github.com/spf13/cobra"
)

func newRemoveCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:     "rm <id>...",
		Aliases: []string{"remove", "delete"},
		Short:   "Delete tasks",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ids, err := parseIDs(args)
			if err != nil {
				return err
			}

			for _, id := range ids {
				// Read the title first so the confirmation still names the
				// task after it is gone.
				task, err := app.Store().Get(id)
				if err != nil {
					return err
				}
				if err := app.Store().Delete(id); err != nil {
					return err
				}
				app.printf("Removed task %d: %s\n", task.ID, task.Title)
			}
			return nil
		},
	}
}
