package cmd

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/lionelrajal/tada/model"
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
	var (
		all       bool
		overdue   bool
		dueBefore string
		priority  int
		sortBy    string
		asJSON    bool
	)

	cmd := &cobra.Command{
		Use:   "list [term...]",
		Short: "List tasks",
		Long: `List pending tasks.

Terms are matched against the title, ignoring case, and every term must match:
` + "`tada list milk shop`" + ` shows only tasks containing both. A term written
with a leading ~ is excluded instead, so ` + "`tada list milk ~shop`" + ` hides
tasks mentioning "shop".

--overdue, --due-before and --priority narrow the list by field and combine
with the terms. Use --all to include completed tasks, --sort to reorder the
result, and --json to print it as a JSON array.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Reject a bad --sort before reading the data file or printing
			// anything, so a typo cannot quietly change the order.
			mode, err := validateSort(sortBy)
			if err != nil {
				return err
			}

			// --priority 0 must be distinguishable from the flag being absent.
			var priorityFilter *int
			if cmd.Flags().Changed("priority") {
				priorityFilter = &priority
			}

			filter, err := newListFilter(args, overdue, dueBefore, priorityFilter, app.Now())
			if err != nil {
				return err
			}

			tasks, err := app.Store().List()
			if err != nil {
				return err
			}

			today := app.Now().Format(model.DueDateLayout)
			selected := make([]*model.Task, 0, len(tasks))
			for _, task := range tasks {
				// --overdue asks about work still outstanding, so it never
				// shows completed tasks even when --all is given.
				if task.Completed && (!all || overdue) {
					continue
				}
				if !filter.match(task, today) {
					continue
				}
				selected = append(selected, task)
			}

			selected = sortTasks(selected, mode)

			if asJSON {
				// Always an array, so `jq` and encoding/json see the same
				// shape whether or not anything matched.
				return encodeJSON(app.Out, selected)
			}

			if len(tasks) == 0 {
				app.printf("No tasks yet. Add one with `tada add \"Buy milk\"`.\n")
				return nil
			}
			if len(selected) == 0 {
				app.printf("No pending tasks. Use --all to include completed ones.\n")
				return nil
			}

			w := tabwriter.NewWriter(app.Out, 0, 4, 2, ' ', 0)
			for _, task := range selected {
				status := "[ ]"
				if task.Completed {
					status = "[x]"
				}
				due := task.DueDate
				if due == "" {
					due = "-"
				}

				fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", task.ID, status, priorityLabel(task.Priority), due, task.Title)
			}
			return w.Flush()
		},
	}

	cmd.Flags().BoolVarP(&all, "all", "a", false, "Include completed tasks")
	cmd.Flags().BoolVar(&overdue, "overdue", false, "Only tasks whose due date has already passed")
	cmd.Flags().StringVar(&dueBefore, "due-before", "", "Only tasks due before this date (YYYY-MM-DD, or a word like tomorrow)")
	cmd.Flags().IntVarP(&priority, "priority", "p", 0, "Only tasks with this priority: 0 low, 1 medium, 2 high")
	cmd.Flags().StringVarP(&sortBy, "sort", "s", "", "Sort by due, priority or created")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Print tasks as a JSON array")

	// -term reaches the parser as an unknown flag rather than a search term,
	// so point at the syntax that does work instead of a bare failure.
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		if strings.Contains(err.Error(), "unknown shorthand flag") {
			return fmt.Errorf("%w (to hide matching tasks, write the term as ~term)", err)
		}
		return err
	})

	return cmd
}
