package cmd

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Syluxso/gears/internal/kanboard"
	"github.com/spf13/cobra"
)

var (
	kanSubtaskAssignee string
	kanSubtaskEstimate float64
	kanLinkExternal    string
	kanLinkTitle       string
)

// --- users ---

var kanUsersCmd = &cobra.Command{
	Use:   "users",
	Short: "List Kanboard users (for assignment)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := kanboard.NewFromConfig()
		if err != nil {
			return err
		}
		users, err := client.GetAllUsers()
		if err != nil {
			return err
		}

		fmt.Println("👤 Kanboard Users")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		for _, u := range users {
			admin := ""
			if u.IsAdmin.Int() == 1 {
				admin = "  [admin]"
			}
			name := u.Name
			if name == "" {
				name = u.Username
			}
			fmt.Printf("%d. %s (%s)%s\n", u.ID.Int(), name, u.Username, admin)
		}
		fmt.Printf("\nTotal: %d user(s)\n", len(users))
		fmt.Println("\nAssign with:  gears kan edit <task> --assignee <username>")
		return nil
	},
}

// --- subtasks ---

var kanSubtaskCmd = &cobra.Command{
	Use:   "subtask",
	Short: "Manage subtasks on a task",
}

var kanSubtaskListCmd = &cobra.Command{
	Use:   "list <task-id|reference>",
	Short: "List a task's subtasks",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, task, err := resolveKanTask(args[0])
		if err != nil {
			return err
		}
		subtasks, err := client.GetAllSubtasks(task.ID.Int())
		if err != nil {
			return err
		}

		fmt.Printf("☑  Subtasks of [%d] %s\n", task.ID.Int(), oneLine(task.Title))
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		if len(subtasks) == 0 {
			fmt.Println("None.")
			fmt.Printf("\nAdd one: gears kan subtask new %s \"Subtask title\"\n", args[0])
			return nil
		}
		for _, s := range subtasks {
			timing := ""
			if s.TimeEstimated > 0 || s.TimeSpent > 0 {
				timing = fmt.Sprintf("  (%.2gh spent / %.2gh est)", s.TimeSpent, s.TimeEstimated)
			}
			fmt.Printf("  [%d] %-10s %s%s\n", s.ID.Int(), s.StatusLabel(), oneLine(s.Title), timing)
		}
		fmt.Printf("\nTotal: %d subtask(s)\n", len(subtasks))
		return nil
	},
}

var kanSubtaskNewCmd = &cobra.Command{
	Use:   "new <task-id|reference> <title>",
	Short: "Add a subtask",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, task, err := resolveKanTask(args[0])
		if err != nil {
			return err
		}
		title := strings.Join(args[1:], " ")

		params := map[string]any{"task_id": task.ID.Int(), "title": title}
		if kanSubtaskAssignee != "" {
			user, err := client.ResolveUser(kanSubtaskAssignee)
			if err != nil {
				return err
			}
			params["user_id"] = user.ID.Int()
		}
		if kanSubtaskEstimate > 0 {
			params["time_estimated"] = kanSubtaskEstimate
		}

		id, err := client.CreateSubtask(params)
		if err != nil {
			return err
		}
		fmt.Printf("✓ Added subtask [%d] %s to task [%d]\n", id, title, task.ID.Int())
		return nil
	},
}

var kanSubtaskRemoveCmd = &cobra.Command{
	Use:   "rm <subtask-id>",
	Short: "Remove a subtask",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := kanboard.NewFromConfig()
		if err != nil {
			return err
		}
		id, err := strconv.Atoi(strings.TrimSpace(args[0]))
		if err != nil {
			return fmt.Errorf("subtask id must be a number, got %q\n\n  Find it: gears kan subtask list <task>", args[0])
		}
		if err := client.RemoveSubtask(id); err != nil {
			return err
		}
		fmt.Printf("✓ Removed subtask [%d]\n", id)
		return nil
	},
}

// --- links ---

var kanLinkCmd = &cobra.Command{
	Use:   "link",
	Short: "Manage links between tasks, and to external URLs",
}

var kanLinkTypesCmd = &cobra.Command{
	Use:   "types",
	Short: "List the available relation types",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := kanboard.NewFromConfig()
		if err != nil {
			return err
		}
		links, err := client.GetAllLinks()
		if err != nil {
			return err
		}
		fmt.Println("🔗 Relation Types")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		for _, l := range links {
			fmt.Printf("%d. %s\n", l.ID.Int(), l.Label)
		}
		return nil
	},
}

var kanLinkListCmd = &cobra.Command{
	Use:   "list <task-id|reference>",
	Short: "List a task's links (internal and external)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, task, err := resolveKanTask(args[0])
		if err != nil {
			return err
		}

		fmt.Printf("🔗 Links on [%d] %s\n", task.ID.Int(), oneLine(task.Title))
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

		internal, err := client.GetAllTaskLinks(task.ID.Int())
		if err != nil {
			return err
		}
		if len(internal) > 0 {
			fmt.Println("Internal:")
			for _, l := range internal {
				state := ""
				if l.IsActive.Int() == 0 {
					state = " [closed]"
				}
				fmt.Printf("  [%d] %s task %d: %s%s\n",
					l.ID.Int(), l.Label, l.LinkedTaskID.Int(), oneLine(l.Title), state)
			}
		}

		external, err := client.GetAllExternalTaskLinks(task.ID.Int())
		if err != nil {
			return err
		}
		if len(external) > 0 {
			if len(internal) > 0 {
				fmt.Println()
			}
			fmt.Println("External:")
			for _, l := range external {
				title := l.Title
				if title == "" {
					title = "(no title)"
				}
				fmt.Printf("  [%d] %s  %s\n", l.ID.Int(), title, l.URL)
			}
		}

		if len(internal) == 0 && len(external) == 0 {
			fmt.Println("None.")
			fmt.Printf("\n  gears kan link add %s blocks <other-task>\n", args[0])
			fmt.Printf("  gears kan link add %s --url https://example.com --title \"Spec\"\n", args[0])
		}
		return nil
	},
}

var kanLinkAddCmd = &cobra.Command{
	Use:   "add <task-id|reference> [relation] [other-task]",
	Short: "Link a task to another task, or to an external URL",
	Long: `Link a task to another task, or attach an external URL.

Internal:
  gears kan link add 42 blocks 43
  gears kan link add 42 "relates to" 43

External:
  gears kan link add 42 --url https://example.com/spec --title "Spec"

Run 'gears kan link types' for the available relations.`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, task, err := resolveKanTask(args[0])
		if err != nil {
			return err
		}

		// External link
		if kanLinkExternal != "" {
			if len(args) > 1 {
				return fmt.Errorf("--url creates an external link; do not also pass a relation and task")
			}
			title := kanLinkTitle
			if title == "" {
				title = kanLinkExternal
			}
			id, err := client.CreateExternalTaskLink(task.ID.Int(), kanLinkExternal, title, "related", "weblink")
			if err != nil {
				return err
			}
			fmt.Printf("✓ External link [%d] added to [%d]: %s\n", id, task.ID.Int(), kanLinkExternal)
			return nil
		}

		// Internal link
		if len(args) < 3 {
			return fmt.Errorf("need a relation and a target task\n\n  gears kan link add %s blocks <other-task>\n  gears kan link types   # to see relations", args[0])
		}
		relation := args[1]
		linkType, err := client.ResolveLinkType(relation)
		if err != nil {
			return err
		}
		other, err := client.ResolveTask(task.ProjectID.Int(), args[2])
		if err != nil {
			return err
		}
		if other.ID.Int() == task.ID.Int() {
			return fmt.Errorf("cannot link a task to itself")
		}

		id, err := client.CreateTaskLink(task.ID.Int(), other.ID.Int(), linkType.ID.Int())
		if err != nil {
			return err
		}
		fmt.Printf("✓ Link [%d]: task %d %s task %d\n",
			id, task.ID.Int(), linkType.Label, other.ID.Int())
		return nil
	},
}

var kanLinkRemoveCmd = &cobra.Command{
	Use:   "rm <link-id>",
	Short: "Remove an internal task link",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := kanboard.NewFromConfig()
		if err != nil {
			return err
		}
		id, err := strconv.Atoi(strings.TrimSpace(args[0]))
		if err != nil {
			return fmt.Errorf("link id must be a number\n\n  Find it: gears kan link list <task>")
		}
		if err := client.RemoveTaskLink(id); err != nil {
			return err
		}
		fmt.Printf("✓ Removed link [%d]\n", id)
		return nil
	},
}

// --- attachments ---

var kanAttachCmd = &cobra.Command{
	Use:   "attach <task-id|reference> <file>",
	Short: "Attach a local file to a task",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, task, err := resolveKanTask(args[0])
		if err != nil {
			return err
		}
		id, err := client.CreateTaskFileFromPath(task.ProjectID.Int(), task.ID.Int(), args[1])
		if err != nil {
			return err
		}
		fmt.Printf("✓ Attached %s to [%d] as file [%d]\n", args[1], task.ID.Int(), id)
		return nil
	},
}

var kanFilesCmd = &cobra.Command{
	Use:   "files <task-id|reference>",
	Short: "List a task's attachments",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, task, err := resolveKanTask(args[0])
		if err != nil {
			return err
		}
		files, err := client.GetAllTaskFiles(task.ID.Int())
		if err != nil {
			return err
		}

		fmt.Printf("📎 Attachments on [%d] %s\n", task.ID.Int(), oneLine(task.Title))
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		if len(files) == 0 {
			fmt.Println("None.")
			fmt.Printf("\nAdd one: gears kan attach %s ./path/to/file\n", args[0])
			return nil
		}
		sort.SliceStable(files, func(i, j int) bool { return files[i].ID.Int() < files[j].ID.Int() })
		for _, f := range files {
			fmt.Printf("  [%d] %-40s %s\n", f.ID.Int(), f.Name, humanSize(f.Size.Int()))
		}
		fmt.Printf("\nTotal: %d file(s)\n", len(files))
		return nil
	},
}

// --- helpers ---

// resolveKanTask builds a client and resolves a task in one step, honouring the
// --project hint when the caller addressed the task by reference.
func resolveKanTask(ref string) (*kanboard.Client, *kanboard.Task, error) {
	client, err := kanboard.NewFromConfig()
	if err != nil {
		return nil, nil, err
	}
	projectID, err := optionalProjectID(client)
	if err != nil {
		return nil, nil, err
	}
	task, err := client.ResolveTask(projectID, ref)
	if err != nil {
		return nil, nil, err
	}
	return client, task, nil
}

func humanSize(bytes int) string {
	switch {
	case bytes >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1<<20))
	case bytes >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(bytes)/(1<<10))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func init() {
	kanCmd.AddCommand(kanUsersCmd, kanSubtaskCmd, kanLinkCmd, kanAttachCmd, kanFilesCmd)

	kanSubtaskCmd.AddCommand(kanSubtaskListCmd, kanSubtaskNewCmd, kanSubtaskRemoveCmd)
	kanSubtaskNewCmd.Flags().StringVar(&kanSubtaskAssignee, "assignee", "", "Assign the subtask to a user (id, username, or name)")
	kanSubtaskNewCmd.Flags().Float64Var(&kanSubtaskEstimate, "estimate", 0, "Estimated hours for the subtask")

	kanLinkCmd.AddCommand(kanLinkTypesCmd, kanLinkListCmd, kanLinkAddCmd, kanLinkRemoveCmd)
	kanLinkAddCmd.Flags().StringVar(&kanLinkExternal, "url", "", "Create an external link to this URL instead of a task link")
	kanLinkAddCmd.Flags().StringVar(&kanLinkTitle, "title", "", "Title for the external link")

	for _, c := range []*cobra.Command{kanSubtaskListCmd, kanSubtaskNewCmd,
		kanLinkListCmd, kanLinkAddCmd, kanAttachCmd, kanFilesCmd} {
		c.Flags().StringVar(&kanProject, "project", "", "Project hint when addressing a task by reference")
	}

	// Match the rest of the kan tree: actionable errors, printed once.
	for _, c := range []*cobra.Command{kanUsersCmd, kanSubtaskCmd, kanLinkCmd,
		kanAttachCmd, kanFilesCmd, kanSubtaskListCmd, kanSubtaskNewCmd,
		kanSubtaskRemoveCmd, kanLinkTypesCmd, kanLinkListCmd, kanLinkAddCmd,
		kanLinkRemoveCmd} {
		c.SilenceUsage = true
		c.SilenceErrors = true
	}
}
