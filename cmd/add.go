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
		asJSON   bool
	)

	cmd := &cobra.Command{
		Use:   "add [title]",
		Short: "Add a new task",
		Long: `Add a new task. Title words may be passed unquoted.

--due accepts YYYY-MM-DD, today, tomorrow, yesterday, a weekday such as fri,
next-fri, or an offset such as +3d or +2w. The stored date is always ISO.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Without the join, `tada add buy milk` would silently drop "milk".
			title := strings.Join(args, " ")

			// Normalise before validation so the store only ever sees ISO
			// dates, whatever the user typed.
			resolved, err := model.ParseDueDate(due, app.Now())
			if err != nil {
				return err
			}

			task, err := model.NewTask(title, priority, resolved)
			if err != nil {
				return err
			}
			if err := app.Store().Add(task); err != nil {
				return err
			}

			if asJSON {
				return encodeJSON(app.Out, task)
			}
			app.printf("Added task %d: %s\n", task.ID, task.Title)
			return nil
		},
	}

	cmd.Flags().StringVarP(&due, "due", "d", "", "Due date: YYYY-MM-DD, today, tomorrow, a weekday like fri, or +3d")
	cmd.Flags().IntVarP(&priority, "priority", "p", 0, "Priority: 0 low, 1 medium, 2 high")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Print the created task as JSON")

	return cmd
}
