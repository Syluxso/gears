package cmd

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/Syluxso/gears/internal/kanboard"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const promptSentinel = "\x00prompt"

var (
	kanURL       string
	kanAPIKey    string
	kanUsername  string
	kanProject   string
	kanColumn    string
	kanDesc      string
	kanRef       string
	kanColor     string
	kanTags      []string
	kanPosition  int
	kanPriority  int
	kanTitle     string
	kanShowClose bool
)

var kanCmd = &cobra.Command{
	Use:   "kan",
	Short: "Work with a Kanboard instance (projects, columns, tasks)",
	Long: `CRUD against a Kanboard board over its JSON-RPC API.

Configure once:
  gears kan --url=https://your-board.example.com
  gears kan --set-api-key                 # prompts, stays out of shell history

Then use it:
  gears kan projects
  gears kan tasks DXC
  gears kan new DXC "Fix the login bug" --column Backlog
  gears kan move 42 --column Doing
  gears kan close 42

The board URL and username are stored in .gears/.gearbox/config.json.
The API token is stored in .gears/.gearbox/config-local.json (gitignored),
or read from the KANBOARD_API_TOKEN environment variable.`,
	// Without this, an unrecognised subcommand falls through to RunE and is
	// silently ignored - so a typo prints the status banner and looks like it
	// worked. Reject unknown args instead.
	Args: cobra.NoArgs,
	RunE: runKanRoot,
}

var kanInfoCmd = &cobra.Command{
	Use:     "info",
	Aliases: []string{"status"},
	Short:   "Show the configured connection and verify it works",
	Args:    cobra.NoArgs,
	RunE:    func(cmd *cobra.Command, args []string) error { return kanStatus() },
}

func init() {
	rootCmd.AddCommand(kanCmd)

	kanCmd.Flags().StringVar(&kanURL, "url", "", "Set the Kanboard base URL")
	kanCmd.Flags().StringVar(&kanAPIKey, "set-api-key", "", "Set the API token (omit the value to be prompted)")
	kanCmd.Flags().StringVar(&kanUsername, "username", "", "Set the API username (default: jsonrpc)")
	kanCmd.Flags().Lookup("set-api-key").NoOptDefVal = promptSentinel

	kanCmd.AddCommand(kanInfoCmd, kanProjectsCmd, kanColumnsCmd, kanTasksCmd,
		kanShowCmd, kanSearchCmd, kanNewCmd, kanMoveCmd, kanEditCmd, kanTagCmd,
		kanCommentCmd, kanCloseCmd, kanReopenCmd, kanRemoveCmd)

	kanTasksCmd.Flags().BoolVar(&kanShowClose, "closed", false, "Show closed tasks instead of open ones")
	kanTasksCmd.Flags().StringVar(&kanColumn, "column", "", "Only show tasks in this column (name or id)")

	kanNewCmd.Flags().StringVar(&kanDesc, "desc", "", "Task description (markdown)")
	kanNewCmd.Flags().StringVar(&kanColumn, "column", "", "Column to create the task in (name or id)")
	kanNewCmd.Flags().StringVar(&kanRef, "ref", "", "External reference (stable handle, e.g. a gears slug)")
	kanNewCmd.Flags().StringVar(&kanColor, "color", "", "Card colour (yellow, blue, green, red, ...)")
	kanNewCmd.Flags().StringSliceVar(&kanTags, "tag", nil, "Tags to attach (repeatable)")

	kanMoveCmd.Flags().StringVar(&kanColumn, "column", "", "Destination column (name or id)")
	kanMoveCmd.Flags().IntVar(&kanPosition, "position", 1, "Position within the column")
	kanMoveCmd.Flags().StringVar(&kanProject, "project", "", "Project hint when addressing a task by reference")

	kanEditCmd.Flags().StringVar(&kanTitle, "title", "", "New title")
	kanEditCmd.Flags().StringVar(&kanDesc, "desc", "", "New description")
	kanEditCmd.Flags().IntVar(&kanPriority, "priority", -1, "New priority")

	for _, c := range []*cobra.Command{kanShowCmd, kanTagCmd, kanCommentCmd,
		kanCloseCmd, kanReopenCmd, kanRemoveCmd, kanEditCmd} {
		c.Flags().StringVar(&kanProject, "project", "", "Project hint when addressing a task by reference")
	}

	// Errors here are actionable on their own; dumping full usage after each
	// one is noise, especially for agents parsing the output. SilenceErrors
	// also stops cobra printing the error, since main() already prints it -
	// without this every failure is shown twice.
	kanCmd.SilenceUsage = true
	kanCmd.SilenceErrors = true
	for _, c := range kanCmd.Commands() {
		c.SilenceUsage = true
		c.SilenceErrors = true
	}
}

// runKanRoot handles the config flags, or prints current status when given none.
func runKanRoot(cmd *cobra.Command, args []string) error {
	changed := false

	if kanURL != "" {
		if err := kanboard.SaveURL(kanURL); err != nil {
			return err
		}
		fmt.Printf("✓ Kanboard URL set: %s\n", strings.TrimRight(kanURL, "/"))
		changed = true
	}

	if kanUsername != "" {
		if err := kanboard.SaveUsername(kanUsername); err != nil {
			return err
		}
		fmt.Printf("✓ Kanboard username set: %s\n", kanUsername)
		changed = true
	}

	if cmd.Flags().Changed("set-api-key") {
		token := kanAPIKey
		if token == promptSentinel || strings.TrimSpace(token) == "" {
			var err error
			token, err = promptForToken()
			if err != nil {
				return err
			}
		}
		if strings.TrimSpace(token) == "" {
			return fmt.Errorf("no token entered")
		}
		if err := kanboard.SaveToken(token); err != nil {
			return err
		}
		fmt.Println("✓ API token saved to .gears/.gearbox/config-local.json (gitignored, mode 0600)")
		changed = true
	}

	if changed {
		fmt.Println("\nVerify the connection:")
		fmt.Println("  gears kan projects")
		return nil
	}

	return kanStatus()
}

// promptForToken reads a token from the terminal without echoing it.
func promptForToken() (string, error) {
	fmt.Print("Kanboard API token (input hidden): ")
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		raw, err := term.ReadPassword(fd)
		fmt.Println()
		if err != nil {
			return "", fmt.Errorf("could not read token: %w", err)
		}
		return strings.TrimSpace(string(raw)), nil
	}
	// Not a terminal (piped input): read a line.
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	fmt.Println()
	if err != nil && strings.TrimSpace(line) == "" {
		return "", fmt.Errorf("could not read token from stdin: %w", err)
	}
	return strings.TrimSpace(line), nil
}

// kanStatus prints the current configuration without revealing the token.
func kanStatus() error {
	fmt.Println("🗂  Kanboard")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	settings, err := kanboard.LoadSettings()
	if err != nil {
		fmt.Println("Not configured yet.")
		fmt.Println()
		fmt.Println("  gears kan --url=https://your-board.example.com")
		fmt.Println("  gears kan --set-api-key")
		return nil
	}

	client := kanboard.New(settings)
	fmt.Printf("URL:      %s\n", settings.URL)
	fmt.Printf("Endpoint: %s\n", client.Endpoint())
	fmt.Printf("Username: %s\n", settings.Username)
	fmt.Printf("Token:    configured (%d chars, hidden)\n", len(settings.Token))

	projects, err := client.GetAllProjects()
	if err != nil {
		fmt.Printf("\n⚠️  Connection failed: %v\n", err)
		return nil
	}
	fmt.Printf("\n✓ Connected. %d project(s) visible.\n", len(projects))
	fmt.Println("\nNext:")
	fmt.Println("  gears kan projects")
	return nil
}

// --- Read commands ---

var kanProjectsCmd = &cobra.Command{
	Use:   "projects",
	Short: "List all Kanboard projects",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := kanboard.NewFromConfig()
		if err != nil {
			return err
		}
		projects, err := client.GetAllProjects()
		if err != nil {
			return err
		}
		if len(projects) == 0 {
			fmt.Println("No projects visible to this API user.")
			return nil
		}

		fmt.Println("📁 Kanboard Projects")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		for _, p := range projects {
			state := ""
			if p.IsActive.Int() == 0 {
				state = "  [inactive]"
			}
			fmt.Printf("%d. %s (%s)%s\n", p.ID.Int(), p.Name, p.Identifier, state)
			if p.URL.Board != "" {
				fmt.Printf("   %s\n", p.URL.Board)
			}
		}
		fmt.Printf("\nTotal: %d project(s)\n", len(projects))
		fmt.Println("\nNext:")
		fmt.Println("  gears kan tasks <project>     # id, identifier, or name")
		return nil
	},
}

var kanColumnsCmd = &cobra.Command{
	Use:   "columns <project>",
	Short: "List the columns of a project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := kanboard.NewFromConfig()
		if err != nil {
			return err
		}
		project, err := client.ResolveProject(args[0])
		if err != nil {
			return err
		}
		columns, err := client.GetColumns(project.ID.Int())
		if err != nil {
			return err
		}

		fmt.Printf("🧱 Columns - %s\n", project.Name)
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		for _, col := range columns {
			limit := ""
			if col.TaskLimit.Int() > 0 {
				limit = fmt.Sprintf("  (limit %d)", col.TaskLimit.Int())
			}
			fmt.Printf("%d. %s%s\n", col.ID.Int(), col.Title, limit)
		}
		return nil
	},
}

var kanTasksCmd = &cobra.Command{
	Use:   "tasks <project>",
	Short: "List tasks on a project board",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := kanboard.NewFromConfig()
		if err != nil {
			return err
		}
		project, err := client.ResolveProject(args[0])
		if err != nil {
			return err
		}

		statusID := 1
		label := "open"
		if kanShowClose {
			statusID = 0
			label = "closed"
		}

		tasks, err := client.GetAllTasks(project.ID.Int(), statusID)
		if err != nil {
			return err
		}

		columns, err := client.GetColumns(project.ID.Int())
		if err != nil {
			return err
		}
		colNames := map[int]string{}
		for _, col := range columns {
			colNames[col.ID.Int()] = col.Title
		}

		if kanColumn != "" {
			col, err := client.ResolveColumn(project.ID.Int(), kanColumn)
			if err != nil {
				return err
			}
			filtered := tasks[:0]
			for _, t := range tasks {
				if t.ColumnID.Int() == col.ID.Int() {
					filtered = append(filtered, t)
				}
			}
			tasks = filtered
		}

		fmt.Printf("📋 %s - %s tasks\n", project.Name, label)
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		if len(tasks) == 0 {
			fmt.Println("Nothing here.")
			fmt.Printf("\nAdd one: gears kan new %s \"Task title\"\n", args[0])
			return nil
		}

		sort.SliceStable(tasks, func(i, j int) bool {
			if tasks[i].ColumnID.Int() != tasks[j].ColumnID.Int() {
				return tasks[i].ColumnID.Int() < tasks[j].ColumnID.Int()
			}
			return tasks[i].Position.Int() < tasks[j].Position.Int()
		})

		currentCol := -1
		for _, t := range tasks {
			if t.ColumnID.Int() != currentCol {
				currentCol = t.ColumnID.Int()
				name := colNames[currentCol]
				if name == "" {
					name = fmt.Sprintf("column %d", currentCol)
				}
				fmt.Printf("\n%s:\n", name)
			}
			ref := ""
			if t.Reference != "" {
				ref = fmt.Sprintf("  → %s", t.Reference)
			}
			fmt.Printf("  [%d] %s%s\n", t.ID.Int(), oneLine(t.Title), ref)
		}

		fmt.Printf("\nTotal: %d task(s)\n", len(tasks))
		fmt.Println("\nNext:")
		fmt.Println("  gears kan show <task-id>")
		return nil
	},
}

var kanShowCmd = &cobra.Command{
	Use:   "show <task-id|reference>",
	Short: "Show one task in full",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := kanboard.NewFromConfig()
		if err != nil {
			return err
		}
		projectID, err := optionalProjectID(client)
		if err != nil {
			return err
		}
		task, err := client.ResolveTask(projectID, args[0])
		if err != nil {
			return err
		}

		state := "open"
		if task.IsActive.Int() == 0 {
			state = "closed"
		}

		fmt.Printf("🗒  [%d] %s\n", task.ID.Int(), task.Title)
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Printf("Status:   %s\n", state)
		fmt.Printf("Priority: %d\n", task.Priority.Int())
		if col, err := client.ResolveColumn(task.ProjectID.Int(), strconv.Itoa(task.ColumnID.Int())); err == nil {
			fmt.Printf("Column:   %s\n", col.Title)
		}
		if task.Reference != "" {
			fmt.Printf("Ref:      %s\n", task.Reference)
		}
		if task.URL != "" {
			fmt.Printf("URL:      %s\n", task.URL)
		}

		if tags, err := client.GetTaskTags(task.ID.Int()); err == nil && len(tags) > 0 {
			names := make([]string, 0, len(tags))
			for _, v := range tags {
				names = append(names, v)
			}
			sort.Strings(names)
			fmt.Printf("Tags:     %s\n", strings.Join(names, ", "))
		}

		if strings.TrimSpace(task.Description) != "" {
			fmt.Println("\nDescription:")
			fmt.Println(task.Description)
		}

		if comments, err := client.GetAllComments(task.ID.Int()); err == nil && len(comments) > 0 {
			fmt.Printf("\nComments (%d):\n", len(comments))
			for _, c := range comments {
				who := c.Name
				if who == "" {
					who = c.Username
				}
				fmt.Printf("  - %s: %s\n", who, oneLine(c.Comment))
			}
		}
		return nil
	},
}

var kanSearchCmd = &cobra.Command{
	Use:   "search <project> <query>",
	Short: "Search tasks (e.g. \"status:open assignee:me\")",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := kanboard.NewFromConfig()
		if err != nil {
			return err
		}
		project, err := client.ResolveProject(args[0])
		if err != nil {
			return err
		}
		query := strings.Join(args[1:], " ")
		tasks, err := client.SearchTasks(project.ID.Int(), query)
		if err != nil {
			return err
		}

		fmt.Printf("🔎 %s - %q\n", project.Name, query)
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		if len(tasks) == 0 {
			fmt.Println("No matches.")
			return nil
		}
		for _, t := range tasks {
			fmt.Printf("  [%d] %s\n", t.ID.Int(), oneLine(t.Title))
		}
		fmt.Printf("\nTotal: %d match(es)\n", len(tasks))
		return nil
	},
}

// --- Write commands ---

var kanNewCmd = &cobra.Command{
	Use:   "new <project> <title>",
	Short: "Create a task",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := kanboard.NewFromConfig()
		if err != nil {
			return err
		}
		project, err := client.ResolveProject(args[0])
		if err != nil {
			return err
		}
		title := strings.Join(args[1:], " ")

		params := map[string]any{
			"project_id": project.ID.Int(),
			"title":      title,
		}
		if kanDesc != "" {
			params["description"] = kanDesc
		}
		if kanRef != "" {
			params["reference"] = kanRef
		}
		if kanColor != "" {
			params["color_id"] = kanColor
		}
		if kanColumn != "" {
			col, err := client.ResolveColumn(project.ID.Int(), kanColumn)
			if err != nil {
				return err
			}
			params["column_id"] = col.ID.Int()
		}

		id, err := client.CreateTask(params)
		if err != nil {
			return err
		}
		fmt.Printf("✓ Created task [%d] %s\n", id, title)

		if len(kanTags) > 0 {
			if err := client.SetTaskTags(project.ID.Int(), id, kanTags); err != nil {
				fmt.Printf("   (task created, but tagging failed: %v)\n", err)
			} else {
				fmt.Printf("   Tags: %s\n", strings.Join(kanTags, ", "))
			}
		}

		if task, err := client.GetTask(id); err == nil && task != nil && task.URL != "" {
			fmt.Printf("   %s\n", task.URL)
		}
		fmt.Println("\nNext:")
		fmt.Printf("  gears kan show %d\n", id)
		return nil
	},
}

var kanMoveCmd = &cobra.Command{
	Use:   "move <task-id|reference>",
	Short: "Move a task to another column",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if kanColumn == "" {
			return fmt.Errorf("--column is required\n\n  Example: gears kan move 42 --column Doing")
		}
		client, err := kanboard.NewFromConfig()
		if err != nil {
			return err
		}
		projectID, err := optionalProjectID(client)
		if err != nil {
			return err
		}
		task, err := client.ResolveTask(projectID, args[0])
		if err != nil {
			return err
		}
		col, err := client.ResolveColumn(task.ProjectID.Int(), kanColumn)
		if err != nil {
			return err
		}
		swimlane := task.SwimlaneID.Int()
		if swimlane == 0 {
			swimlane = 1
		}
		if err := client.MoveTaskPosition(task.ProjectID.Int(), task.ID.Int(),
			col.ID.Int(), kanPosition, swimlane); err != nil {
			return err
		}
		fmt.Printf("✓ Moved [%d] %s → %s\n", task.ID.Int(), oneLine(task.Title), col.Title)
		return nil
	},
}

var kanEditCmd = &cobra.Command{
	Use:   "edit <task-id|reference>",
	Short: "Edit a task's title, description, or priority",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := kanboard.NewFromConfig()
		if err != nil {
			return err
		}
		projectID, err := optionalProjectID(client)
		if err != nil {
			return err
		}
		task, err := client.ResolveTask(projectID, args[0])
		if err != nil {
			return err
		}

		params := map[string]any{"id": task.ID.Int()}
		if kanTitle != "" {
			params["title"] = kanTitle
		}
		if kanDesc != "" {
			params["description"] = kanDesc
		}
		if kanPriority >= 0 {
			params["priority"] = kanPriority
		}
		if len(params) == 1 {
			return fmt.Errorf("nothing to change\n\n  Use --title, --desc, or --priority")
		}

		if err := client.UpdateTask(params); err != nil {
			return err
		}
		fmt.Printf("✓ Updated task [%d]\n", task.ID.Int())
		return nil
	},
}

var kanTagCmd = &cobra.Command{
	Use:   "tag <task-id|reference> <tag>...",
	Short: "Replace a task's tags",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := kanboard.NewFromConfig()
		if err != nil {
			return err
		}
		projectID, err := optionalProjectID(client)
		if err != nil {
			return err
		}
		task, err := client.ResolveTask(projectID, args[0])
		if err != nil {
			return err
		}
		tags := args[1:]
		if err := client.SetTaskTags(task.ProjectID.Int(), task.ID.Int(), tags); err != nil {
			return err
		}
		fmt.Printf("✓ Tags on [%d]: %s\n", task.ID.Int(), strings.Join(tags, ", "))
		fmt.Println("   (setTaskTags replaces the full set, it does not append)")
		return nil
	},
}

var kanCommentCmd = &cobra.Command{
	Use:   "comment <task-id|reference> <text>",
	Short: "Add a comment to a task",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := kanboard.NewFromConfig()
		if err != nil {
			return err
		}
		projectID, err := optionalProjectID(client)
		if err != nil {
			return err
		}
		task, err := client.ResolveTask(projectID, args[0])
		if err != nil {
			return err
		}
		content := strings.Join(args[1:], " ")
		owner := task.OwnerID.Int()
		if owner == 0 {
			owner = 1
		}
		id, err := client.CreateComment(task.ID.Int(), owner, content)
		if err != nil {
			return err
		}
		fmt.Printf("✓ Comment %d added to [%d]\n", id, task.ID.Int())
		return nil
	},
}

var kanCloseCmd = &cobra.Command{
	Use:   "close <task-id|reference>",
	Short: "Close a task",
	Args:  cobra.ExactArgs(1),
	RunE:  func(cmd *cobra.Command, args []string) error { return kanSetState(args[0], true) },
}

var kanReopenCmd = &cobra.Command{
	Use:   "reopen <task-id|reference>",
	Short: "Reopen a closed task",
	Args:  cobra.ExactArgs(1),
	RunE:  func(cmd *cobra.Command, args []string) error { return kanSetState(args[0], false) },
}

func kanSetState(ref string, close bool) error {
	client, err := kanboard.NewFromConfig()
	if err != nil {
		return err
	}
	projectID, err := optionalProjectID(client)
	if err != nil {
		return err
	}
	task, err := client.ResolveTask(projectID, ref)
	if err != nil {
		return err
	}
	if close {
		if err := client.CloseTask(task.ID.Int()); err != nil {
			return err
		}
		fmt.Printf("✓ Closed [%d] %s\n", task.ID.Int(), oneLine(task.Title))
		return nil
	}
	if err := client.OpenTask(task.ID.Int()); err != nil {
		return err
	}
	fmt.Printf("✓ Reopened [%d] %s\n", task.ID.Int(), oneLine(task.Title))
	return nil
}

var kanRemoveCmd = &cobra.Command{
	Use:   "rm <task-id|reference>",
	Short: "Permanently delete a task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := kanboard.NewFromConfig()
		if err != nil {
			return err
		}
		projectID, err := optionalProjectID(client)
		if err != nil {
			return err
		}
		task, err := client.ResolveTask(projectID, args[0])
		if err != nil {
			return err
		}

		// Deletion is not reversible in Kanboard, so confirm unless piped.
		if term.IsTerminal(int(os.Stdin.Fd())) {
			fmt.Printf("Permanently delete [%d] %s? [y/N]: ", task.ID.Int(), oneLine(task.Title))
			answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
			if strings.ToLower(strings.TrimSpace(answer)) != "y" {
				fmt.Println("Cancelled.")
				return nil
			}
		}

		if err := client.RemoveTask(task.ID.Int()); err != nil {
			return err
		}
		fmt.Printf("✓ Deleted [%d]\n", task.ID.Int())
		return nil
	},
}

// --- helpers ---

// optionalProjectID resolves the --project hint, if one was given.
func optionalProjectID(client *kanboard.Client) (int, error) {
	if strings.TrimSpace(kanProject) == "" {
		return 0, nil
	}
	project, err := client.ResolveProject(kanProject)
	if err != nil {
		return 0, err
	}
	return project.ID.Int(), nil
}

// oneLine flattens a title or comment for single-line list output.
func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	if len(s) > 90 {
		return s[:87] + "..."
	}
	return s
}
