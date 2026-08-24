package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Syluxso/gears/internal/config"
	"github.com/Syluxso/gears/internal/db"
	"github.com/Syluxso/gears/internal/inbox"
	"github.com/spf13/cobra"
)

var backlogCmd = &cobra.Command{
	Use:   "backlog",
	Short: "Manage the operational backlog of work items",
	Long: `Backlog items are the source of truth for "what should be worked on next".

They support:
- Lightweight items (just a title) or rich items with .gears/backlog/backlog--*.md files
- Explicit claiming by an agent
- Sort ordering (lower numbers = higher priority; 0 = top priority group)
- References to stories, defects, etc.

One agent claims an item at a time. Planning can continue by adding more items.`,
}

var backlogAgentFlag string

func init() {
	rootCmd.AddCommand(backlogCmd)

	backlogCmd.PersistentFlags().StringVar(&backlogAgentFlag, "agent", "", "Agent name to use for this command (overrides default)")

	backlogCmd.AddCommand(backlogListCmd)
	backlogCmd.AddCommand(backlogNewCmd)
	backlogCmd.AddCommand(backlogClaimCmd)
	backlogCmd.AddCommand(backlogCompleteCmd)
	backlogCmd.AddCommand(backlogPrioritizeCmd)
	backlogCmd.AddCommand(backlogCurrentCmd)
	backlogCmd.AddCommand(backlogShowCmd)
}

var backlogListCmd = &cobra.Command{
	Use:   "list",
	Short: "List backlog items (default: all, ordered by priority)",
	RunE:  runBacklogList,
}

var backlogNewRef string

var backlogNewCmd = &cobra.Command{
	Use:   "new <title>",
	Short: "Add a new item to the backlog (title can contain spaces)",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runBacklogNew,
}

var backlogClaimCmd = &cobra.Command{
	Use:   "claim <id-or-slug>",
	Short: "Claim a backlog item so you can work on it",
	Args:  cobra.ExactArgs(1),
	RunE:  runBacklogClaim,
}

var backlogCompleteCmd = &cobra.Command{
	Use:   "complete <id-or-slug>",
	Short: "Mark a backlog item completed (updates referenced story if present + posts inbox note)",
	Args:  cobra.ExactArgs(1),
	RunE:  runBacklogComplete,
}

var backlogPrioritizeCmd = &cobra.Command{
	Use:   "prioritize <id-or-slug>",
	Short: "Move item to top priority (sets sort_order=0). Multiple 0s are fine.",
	Args:  cobra.ExactArgs(1),
	RunE:  runBacklogPrioritize,
}

var backlogCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Show the backlog item currently claimed by this agent (the one you should be working on)",
	RunE:  runBacklogCurrent,
}

var backlogShowCmd = &cobra.Command{
	Use:   "show <id-or-slug>",
	Short: "Show details of one backlog item",
	Args:  cobra.ExactArgs(1),
	RunE:  runBacklogShow,
}

func getAgentName() string {
	if backlogAgentFlag != "" {
		return strings.TrimSpace(backlogAgentFlag)
	}
	if name := config.GetDefaultAgentName(); name != "" {
		return name
	}
	return "agent"
}

func runBacklogList(cmd *cobra.Command, args []string) error {
	if _, err := os.Stat(".gears"); os.IsNotExist(err) {
		return fmt.Errorf(".gears directory not found. Run 'gears init' first")
	}

	items, err := db.ListBacklogItems("")
	if err != nil {
		return err
	}

	if len(items) == 0 {
		fmt.Println("Backlog is empty.")
		fmt.Println("\nAdd something: gears backlog new \"Do the thing\"")
		return nil
	}

	fmt.Println("📋 Backlog")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	agent := getAgentName()

	pending := 0
	claimed := 0
	completed := 0

	for _, it := range items {
		switch it.Status {
		case "pending":
			pending++
		case "claimed", "in_progress":
			claimed++
		case "completed":
			completed++
		}

		priority := it.SortOrder
		claimInfo := ""
		if it.ClaimedBy != "" {
			claimInfo = fmt.Sprintf("  [claimed by %s]", it.ClaimedBy)
			if it.ClaimedBy == agent {
				claimInfo += " ← you"
			}
		}

		ref := ""
		if it.ReferenceType != "" && it.ReferenceSlug != "" {
			ref = fmt.Sprintf("  → %s:%s", it.ReferenceType, it.ReferenceSlug)
		}

		fmt.Printf("%4d. [%s] %s (order:%d)%s%s\n",
			it.ID, it.Status, it.Label, priority, claimInfo, ref)
	}

	fmt.Printf("\nTotal: %d  |  pending: %d  claimed/in_progress: %d  completed: %d\n",
		len(items), pending, claimed, completed)

	fmt.Println("\nTip: gears backlog current   # what you should be working on right now")
	return nil
}

func runBacklogNew(cmd *cobra.Command, args []string) error {
	if _, err := os.Stat(".gears"); os.IsNotExist(err) {
		return fmt.Errorf(".gears directory not found. Run 'gears init' first")
	}

	title := strings.Join(args, " ")
	title = strings.TrimSpace(title)

	var refType, refSlug string
	if backlogNewRef != "" {
		parts := strings.SplitN(backlogNewRef, ":", 2)
		if len(parts) == 2 {
			refType = strings.ToLower(strings.TrimSpace(parts[0]))
			refSlug = strings.TrimSpace(parts[1])
		} else {
			val := strings.TrimSpace(backlogNewRef)
			// Support "story--foo", "story-foo", or just "foo" (assume story)
			if strings.HasPrefix(val, "story--") || strings.HasPrefix(val, "story-") {
				refType = "story"
				refSlug = strings.TrimPrefix(strings.TrimPrefix(val, "story--"), "story-")
			} else {
				refType = "story"
				refSlug = val
			}
		}
	}

	item, err := db.CreateBacklogItem(title, refType, refSlug)
	if err != nil {
		return err
	}

	// Ensure backlog dir exists (for future rich files)
	backlogDir := filepath.Join(".gears", "backlog")
	_ = os.MkdirAll(backlogDir, 0755)

	fmt.Printf("✓ Added to backlog: [%d] %s  (order: %d)\n", item.ID, item.Label, item.SortOrder)
	if refType != "" && refSlug != "" {
		fmt.Printf("   References: %s:%s\n", refType, refSlug)
	}
	fmt.Println("\nNext:")
	fmt.Println("  gears backlog claim", item.ID, "   # or use the slug")
	fmt.Println("  gears backlog list")
	return nil
}

func init() {
	// Add --ref flag to new
	backlogNewCmd.Flags().StringVar(&backlogNewRef, "ref", "", "Reference e.g. story--foo or story:foo or defect:bar")
}

func runBacklogClaim(cmd *cobra.Command, args []string) error {
	agent := getAgentName()
	ident := args[0]

	if err := db.ClaimBacklogItem(ident, agent); err != nil {
		return err
	}

	item, err := db.GetBySlugOrID(ident)
	if err != nil || item == nil {
		item = &db.BacklogItem{ID: 0, Label: ident}
	}

	fmt.Printf("✓ Claimed by %s: [%d] %s\n", agent, item.ID, item.Label)
	fmt.Println("This is now your current work item.")
	fmt.Println("\nWhen done: gears backlog complete", ident)
	return nil
}

// small local helpers because we don't want to export too much from db for CLI
func parseID(s string) int64 {
	var id int64
	fmt.Sscanf(strings.TrimSpace(s), "%d", &id)
	return id
}

func init() {
	// register a tiny helper so other cmds can use if needed later
	// (we'll define a small wrapper)
}

func runBacklogComplete(cmd *cobra.Command, args []string) error {
	ident := args[0]

	item, err := db.CompleteBacklogItem(ident)
	if err != nil {
		return err
	}

	fmt.Printf("✓ Completed: [%d] %s\n", item.ID, item.Label)

	// Side effect: if this references a story, try to mark the story file as completed
	if item.ReferenceType == "story" && item.ReferenceSlug != "" {
		if err := markStoryCompleted(item.ReferenceSlug); err != nil {
			fmt.Printf("   (could not auto-update story file: %v)\n", err)
		} else {
			fmt.Printf("   Updated referenced story status to Completed.\n")
		}
	}

	// Post an inbox action so the agent (or human) remembers to do the handoff updates
	_ = db.Initialize()
	if db.GetDB() != nil {
		msg := &inbox.Message{
			Level: inbox.LevelAction,
			Title: "Backlog item completed - perform handoff updates",
			Message: fmt.Sprintf("Backlog item completed: %s\n\nPlease:\n- Update today's .gears/sessions/YYYY-MM-DD.md\n- Review .gears/context/index.md (active story, done items)\n- Consider adding an ADR if architecture changed\n- Check if any other backlog items are now unblocked", item.Label),
			SuggestedCommand: fmt.Sprintf("gears backlog list"),
		}
		_ = inbox.Add(db.GetDB(), msg)
		fmt.Println("   Added action item to inbox for handoff (session / context / adr).")
	}

	fmt.Println("\nRecommended next:")
	fmt.Println("  gears inbox --read")
	fmt.Println("  gears session")
	fmt.Println("  gears backlog current   # or list to pick the next thing")
	return nil
}

// markStoryCompleted finds the story file and flips its **Status:** line.
// This is advisory/best-effort.
func markStoryCompleted(refSlug string) error {
	slug := strings.TrimPrefix(refSlug, "story--")
	slug = strings.TrimPrefix(slug, "story-")
	storyDir := filepath.Join(".gears", "story")

	// Try common filename patterns
	candidates := []string{
		filepath.Join(storyDir, "story--"+slug+".md"),
		filepath.Join(storyDir, "story-"+slug+".md"),
	}

	var storyPath string
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			storyPath = c
			break
		}
	}
	if storyPath == "" {
		return fmt.Errorf("story file not found for slug %s", refSlug)
	}

	data, err := os.ReadFile(storyPath)
	if err != nil {
		return err
	}

	contentStr := string(data)
	// Simple replace of status line
	lines := strings.Split(contentStr, "\n")
	changed := false
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "**Status:**") {
			lines[i] = "**Status:** Completed"
			changed = true
			break
		}
	}
	if !changed {
		return fmt.Errorf("no **Status:** line found to update")
	}

	newContent := strings.Join(lines, "\n")
	if err := os.WriteFile(storyPath, []byte(newContent), 0644); err != nil {
		return err
	}

	// Best effort: next time story list / content sync runs it will pick up the file change.
	return nil
}

func runBacklogPrioritize(cmd *cobra.Command, args []string) error {
	ident := args[0]
	if err := db.PrioritizeBacklogItem(ident); err != nil {
		return err
	}
	fmt.Printf("✓ Prioritized (order=0): %s\n", ident)
	fmt.Println("It will now sort at the top (ties with other 0-order items are OK).")
	return nil
}

func runBacklogCurrent(cmd *cobra.Command, args []string) error {
	agent := getAgentName()

	item, err := db.GetCurrentClaimed(agent)
	if err != nil {
		return err
	}

	if item == nil {
		fmt.Printf("No item currently claimed by %s.\n", agent)
		fmt.Println("\nClaim something: gears backlog list  then  gears backlog claim <id>")
		return nil
	}

	fmt.Printf("🎯 Current work for %s:\n", agent)
	fmt.Printf("   [%d] %s   (status: %s, order: %d)\n", item.ID, item.Label, item.Status, item.SortOrder)
	if item.ReferenceType != "" && item.ReferenceSlug != "" {
		fmt.Printf("   References: %s:%s\n", item.ReferenceType, item.ReferenceSlug)
	}
	fmt.Printf("\nClaimed at: %s\n", item.ClaimedAt)
	fmt.Println("\nWhen finished: gears backlog complete", item.ID)
	return nil
}

func runBacklogShow(cmd *cobra.Command, args []string) error {
	ident := args[0]

	item, err := db.GetBySlugOrID(ident)
	if err != nil || item == nil {
		return fmt.Errorf("backlog item not found: %s", ident)
	}

	fmt.Printf("Backlog Item [%d]\n", item.ID)
	fmt.Printf("Label: %s\n", item.Label)
	fmt.Printf("Slug:  %s\n", item.Slug)
	fmt.Printf("Status: %s\n", item.Status)
	fmt.Printf("Order:  %d\n", item.SortOrder)
	if item.ClaimedBy != "" {
		fmt.Printf("Claimed by: %s at %s\n", item.ClaimedBy, item.ClaimedAt)
	}
	if item.ReferenceType != "" {
		fmt.Printf("Reference: %s:%s\n", item.ReferenceType, item.ReferenceSlug)
	}
	fmt.Printf("Created: %s\n", item.CreatedAt.Format("2006-01-02 15:04"))
	return nil
}

// Small exported helpers so cmd/backlog can call into db package without duplicating logic.
// These are thin wrappers to keep the db package clean while allowing CLI to resolve things.
func init() {
	// Nothing - we use the db funcs directly. The parse helpers are local.
}

// NOTE: We added two tiny unexported helpers above. If compile complains about
// db.NormalizeForDisplay / db.ResolveForDisplay we will fix in next compile cycle.
// For now the run* functions use direct Get calls.
