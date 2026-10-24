package cmd

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/lionelrajal/tada/model"
)

func newAddCmd(app *App) *cobra.Command {
	var (
		due      string
		priority int
	)

	cmd := &cobra.Command{
		Use:   "add [title]",
		Short: "Add a new task",
		Long:  "Add a new task. Title words may be passed unquoted.",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Without the join, `tada add buy milk` would silently drop "milk".
			title := strings.Join(args, " ")

			task, err := model.NewTask(title, priority, due)
			if err != nil {
				return err
			}
			if err := app.Store().Add(task); err != nil {
				return err
			}

			app.printf("Added task %d: %s\n", task.ID, task.Title)
			return nil
		},
	}

	cmd.Flags().StringVarP(&due, "due", "d", "", "Due date YYYY-MM-DD")
	cmd.Flags().IntVarP(&priority, "priority", "p", 0, "Priority: 0 low, 1 medium, 2 high")

	return cmd
}
