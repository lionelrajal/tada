package cmd

import (
	"errors"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lionelrajal/tada/model"
)

func newEditCmd(app *App) *cobra.Command {
	var (
		title    string
		due      string
		priority int
	)

	cmd := &cobra.Command{
		Use:   "edit <id>...",
		Short: "Change tasks without changing their IDs",
		Long: `Change the title, due date or priority of existing tasks.

Task IDs come first; everything after the first non-numeric word is the new
title, so it needs no quoting: ` + "`tada edit 3 buy oat milk`" + `.

At least one change is required. An empty --due clears the due date, while
omitting --due leaves it as it is. --due accepts YYYY-MM-DD, today, tomorrow,
a weekday such as fri, next-fri, or an offset such as +3d. Every ID is checked
before any task is written, so a mistyped ID changes nothing.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ids, words, err := splitIDsAndTitle(args)
			if err != nil {
				return err
			}

			dueChanged := cmd.Flags().Changed("due")
			priorityChanged := cmd.Flags().Changed("priority")
			titleChanged := cmd.Flags().Changed("title") || len(words) > 0

			// Normalise a written date before anything is validated or
			// written, so the store keeps receiving ISO dates only.
			if dueChanged {
				resolved, err := model.ParseDueDate(due, app.Now())
				if err != nil {
					return err
				}
				due = resolved
			}

			// Editing with nothing to change is almost always a mistake, and
			// silently doing nothing would look like success.
			if !titleChanged && !dueChanged && !priorityChanged {
				return errors.New("nothing to change: pass --title, --due or --priority")
			}

			newTitle := strings.Join(words, " ")
			if cmd.Flags().Changed("title") {
				// --title may be followed by unquoted words, exactly as add
				// accepts them.
				if len(words) > 0 {
					newTitle = title + " " + newTitle
				} else {
					newTitle = title
				}
			}

			// Resolve every ID before writing anything, so an unknown ID in
			// the list cannot leave the earlier tasks half-edited.
			tasks := make([]*model.Task, 0, len(ids))
			for _, id := range ids {
				task, err := app.Store().Get(id)
				if err != nil {
					return err
				}
				tasks = append(tasks, task)
			}

			// Validate the same way add does before the first write, so an
			// invalid date or priority changes nothing at all.
			updates := make([]*model.Task, 0, len(tasks))
			for _, task := range tasks {
				edited, err := editTask(task, newTitle, titleChanged, due, dueChanged, priority, priorityChanged)
				if err != nil {
					return err
				}
				updates = append(updates, edited)
			}

			for _, edited := range updates {
				if err := app.Store().Update(edited); err != nil {
					return err
				}
				app.printf("Updated task %d: %s\n", edited.ID, edited.Title)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "New title")
	cmd.Flags().StringVarP(&due, "due", "d", "", "New due date; empty clears it (accepts YYYY-MM-DD, tomorrow, fri, +3d)")
	cmd.Flags().IntVarP(&priority, "priority", "p", 0, "Priority: 0 low, 1 medium, 2 high")

	return cmd
}

// editTask returns a validated copy of task with the requested fields replaced.
// Fields whose flag was not given keep their current value, so an edit touches
// only what it names.
func editTask(task *model.Task, title string, titleChanged bool, due string, dueChanged bool, priority int, priorityChanged bool) (*model.Task, error) {
	if !titleChanged {
		title = task.Title
	}
	if !dueChanged {
		due = task.DueDate
	}
	if !priorityChanged {
		priority = task.Priority
	}

	// Reusing NewTask keeps edit bound to the same rules as add: a title is
	// never empty, priorities stay in range and due dates stay YYYY-MM-DD.
	edited, err := model.NewTask(title, priority, due)
	if err != nil {
		return nil, err
	}
	edited.ID = task.ID
	edited.Completed = task.Completed
	return edited, nil
}

// splitIDsAndTitle divides edit arguments into task IDs and title words. IDs
// are the leading numeric arguments; the first non-numeric word starts the
// title, so numbers inside a title ("2 apples") are never mistaken for IDs.
func splitIDsAndTitle(args []string) ([]int64, []string, error) {
	var ids []int64
	for i, arg := range args {
		id, err := strconv.ParseInt(arg, 10, 64)
		if err != nil || id <= 0 {
			if len(ids) == 0 {
				return nil, nil, errors.New("expected a task ID first: tada edit <id>... [title words]")
			}
			return ids, args[i:], nil
		}
		ids = append(ids, id)
	}
	return ids, nil, nil
}
