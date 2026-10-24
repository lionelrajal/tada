package cmd

import (
	"github.com/spf13/cobra"
)

func newDoneCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "done <id>...",
		Short: "Mark tasks as completed",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ids, err := parseIDs(args)
			if err != nil {
				return err
			}

			for _, id := range ids {
				task, err := app.Store().Get(id)
				if err != nil {
					return err
				}
				if task.Completed {
					app.printf("Task %d already completed: %s\n", task.ID, task.Title)
					continue
				}

				task.Completed = true
				if err := app.Store().Update(task); err != nil {
					return err
				}
				app.printf("Completed task %d: %s\n", task.ID, task.Title)
			}
			return nil
		},
	}
}
