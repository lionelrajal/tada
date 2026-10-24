package cmd

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// priorityLabel renders a priority for display, tolerating values that were
// hand-edited into the data file.
func priorityLabel(priority int) string {
	switch priority {
	case 0:
		return "low"
	case 1:
		return "medium"
	case 2:
		return "high"
	default:
		return fmt.Sprintf("%d", priority)
	}
}

func newListCmd(app *App) *cobra.Command {
	var all bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tasks",
		Long:  "List pending tasks. Use --all to include completed ones.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			tasks, err := app.Store().List()
			if err != nil {
				return err
			}
			if len(tasks) == 0 {
				app.printf("No tasks yet. Add one with `tada add \"Buy milk\"`.\n")
				return nil
			}

			w := tabwriter.NewWriter(app.Out, 0, 4, 2, ' ', 0)
			shown := 0
			for _, task := range tasks {
				if task.Completed && !all {
					continue
				}

				status := "[ ]"
				if task.Completed {
					status = "[x]"
				}
				due := task.DueDate
				if due == "" {
					due = "-"
				}

				fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", task.ID, status, priorityLabel(task.Priority), due, task.Title)
				shown++
			}

			if shown == 0 {
				app.printf("No pending tasks. Use --all to include completed ones.\n")
				return nil
			}
			return w.Flush()
		},
	}

	cmd.Flags().BoolVarP(&all, "all", "a", false, "Include completed tasks")

	return cmd
}
