package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Syluxso/gears/internal/content"
	"github.com/Syluxso/gears/internal/db"
	"github.com/spf13/cobra"
)

var consultCmd = &cobra.Command{
	Use:   "consult",
	Short: "Manage agent-to-agent consult threads",
	Long:  `Create and review consult threads — structured conversations between AI agents held in .gears/consults/.`,
}

var consultNewCmd = &cobra.Command{
	Use:   "new <name>",
	Short: "Start a new consult thread",
	Long: `Creates a new consult file in .gears/consults/ using the format consults--<name>.md.

The file is the shared medium for agent-to-agent conversation.
Read .gears/consults/index.md for the turn protocol before participating.`,
	Args: cobra.ExactArgs(1),
	RunE: runConsultNew,
}

var consultLatestCmd = &cobra.Command{
	Use:   "latest",
	Short: "Show the latest consult thread (supports --full, --follow, --weaponsfree)",
	Long: `Displays orientation info and the most recently created consult file.

Options for active/running consults (to avoid manually running latest repeatedly):
  --full         Include the full text of the last agent's turn.
  --follow       Poll the consult file and reprint on changes (run in a side terminal).
  --weaponsfree  (or --wf) Autonomous "weapons free" mode: full last turn + explicit
                 instructions allowing the agent to chain multiple turns without
                 waiting for human prompts between them. "Go to town."

See .gears/consults/index.md for the turn protocol (always re-read it).`,
	RunE:  runConsultLatest,
}

var (
	consultLatestFull       bool
	consultLatestFollow     bool
	consultLatestWeaponsFree bool
)

func init() {
	rootCmd.AddCommand(consultCmd)
	consultCmd.AddCommand(consultNewCmd)
	consultCmd.AddCommand(consultLatestCmd)

	// Flags for latest (active consult UX improvements)
	consultLatestCmd.Flags().BoolVarP(&consultLatestFull, "full", "f", false, "Show the full text of the last turn (not just who/date)")
	consultLatestCmd.Flags().BoolVar(&consultLatestFollow, "follow", false, "Continuously poll and reprint when the consult file changes (side terminal watcher)")
	consultLatestCmd.Flags().BoolVar(&consultLatestWeaponsFree, "weaponsfree", false, "Weapons-free / autonomous mode: full context + instructions so agents can chain turns without human between each")
	consultLatestCmd.Flags().BoolVar(&consultLatestWeaponsFree, "wf", false, "Shorthand for --weaponsfree")
}

func runConsultNew(cmd *cobra.Command, args []string) error {
	if _, err := os.Stat(".gears"); os.IsNotExist(err) {
		return fmt.Errorf(".gears directory not found. Run 'gears init' first")
	}

	consultsDir := filepath.Join(".gears", "consults")
	if _, err := os.Stat(consultsDir); os.IsNotExist(err) {
		if err := os.MkdirAll(consultsDir, 0755); err != nil {
			return fmt.Errorf("failed to create .gears/consults directory: %w", err)
		}
		// Write the protocol index if it doesn't exist
		indexPath := filepath.Join(consultsDir, "index.md")
		if _, err := os.Stat(indexPath); os.IsNotExist(err) {
			indexContent, readErr := templateFS.ReadFile("templates/.gears/consults/index.md")
			if readErr == nil {
				_ = os.WriteFile(indexPath, indexContent, 0644)
			}
		}
		fmt.Println("✓ Created .gears/consults/ and index.md")
	}

	name := args[0]
	slug := content.NormalizeSlug(name)
	consultFile, err := content.BuildDefaultFilePath(content.TypeConsult, slug)
	if err != nil {
		return err
	}

	if err := db.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	if _, err := os.Stat(consultFile); err == nil {
		return fmt.Errorf("consult file already exists: %s", consultFile)
	}

	item, err := content.CreateItem(db.GetDB(), content.TypeConsult, "md", name, slug, "open", consultFile)
	if err != nil {
		return fmt.Errorf("failed to create consult metadata: %w", err)
	}

	today := time.Now().Format("2006-01-02")
	template := fmt.Sprintf(`# Consult: %s

**Started:** %s
**Status:** Open

Read .gears/consults/index.md for the turn protocol before writing your first entry.

---

`, name, today)

	if err := os.WriteFile(consultFile, []byte(template), 0644); err != nil {
		_ = content.SyncFromFiles(db.GetDB())
		return fmt.Errorf("failed to create consult file: %w", err)
	}

	_ = content.SyncFromFiles(db.GetDB())
	_ = content.UpdateSyncMetadata(db.GetDB(), item.UUID, "")

	fmt.Printf("✓ Created consult: %s\n", consultFile)
	fmt.Println()
	fmt.Println("Agent next:")
	fmt.Println("  1. Read .gears/consults/index.md (turn protocol)")
	fmt.Println("  2. Open the consult file above")
	fmt.Println("  3. Append your opening turn:")
	fmt.Println()
	fmt.Println("     ## {Your Name} — " + today)
	fmt.Println()
	fmt.Println("     {Your opening message}")
	fmt.Println()
	fmt.Println("     **End of {Your Name} output**")
	fmt.Println()
	fmt.Println("     ---")
	fmt.Println()
	fmt.Println("  4. Hand off to the next agent — share the file path or its contents")
	fmt.Println()
	fmt.Println("  For active threads (less manual re-running of latest):")
	fmt.Println("    gears consult latest --full        # see last turn body")
	fmt.Println("    gears consult latest --follow      # live watcher in another terminal")
	fmt.Println("    gears consult latest --weaponsfree # (or --wf) autonomous chaining mode")

	return nil
}

func runConsultLatest(cmd *cobra.Command, args []string) error {
	if _, err := os.Stat(".gears"); os.IsNotExist(err) {
		return fmt.Errorf(".gears directory not found. Run 'gears init' first")
	}

	if err := db.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	if err := content.SyncFromFiles(db.GetDB()); err != nil {
		return fmt.Errorf("failed to sync consults: %w", err)
	}

	items, err := content.GetByType(db.GetDB(), content.TypeConsult)
	if err != nil {
		return fmt.Errorf("failed to query consults: %w", err)
	}

	fmt.Println()
	fmt.Println("💬 Consults")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Consults are structured agent-to-agent conversations held")
	fmt.Println("through markdown files in .gears/consults/.")
	fmt.Println()

	if len(items) == 0 {
		fmt.Println("No consults found.")
		fmt.Println()
		fmt.Println("  Start one:    gears consult new \"topic-name\"")
		fmt.Println("  Protocol:     .gears/consults/index.md")
		return nil
	}

	// Latest is the last item (GetByType orders by created_at ASC)
	latest := items[len(items)-1]
	filePath := filepath.FromSlash(latest.FilePath)

	if consultLatestFollow {
		return followLatestConsult(latest, filePath)
	}

	return displayLatestConsult(latest, filePath, consultLatestFull, consultLatestWeaponsFree)
}

// consultParseLastEntry scans the consult file for the last ## AgentName — YYYY-MM-DD heading
// and returns the body of that turn (the content the agent wrote).
func consultParseLastEntry(filePath string) (agentName, date, body string) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", "", ""
	}

	lines := strings.Split(string(data), "\n")
	re := regexp.MustCompile(`^##\s+(.+?)\s+[—–-]+\s+(\d{4}-\d{2}-\d{2})`)
	endRe := regexp.MustCompile(`^\*\*End of .+? output\*\*`)

	lastHeaderIdx := -1
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if m := re.FindStringSubmatch(trim); m != nil {
			agentName = strings.TrimSpace(m[1])
			date = m[2]
			lastHeaderIdx = i
		}
	}
	if lastHeaderIdx == -1 {
		return "", "", ""
	}

	// Collect lines after the header until we hit an end marker or the next turn header.
	var bodyLines []string
	for j := lastHeaderIdx + 1; j < len(lines); j++ {
		trim := strings.TrimSpace(lines[j])
		if strings.HasPrefix(trim, "## ") && re.MatchString(trim) {
			break // next agent's turn started
		}
		if endRe.MatchString(trim) {
			break
		}
		bodyLines = append(bodyLines, lines[j])
	}
	body = strings.TrimSpace(strings.Join(bodyLines, "\n"))
	return agentName, date, body
}

// displayLatestConsult prints the standard latest info + optional full last turn + weapons free banner.
func displayLatestConsult(latest content.Item, filePath string, showFull, weaponsFree bool) error {
	fileName := filepath.Base(filePath)

	fmt.Println("Protocol: read .gears/consults/index.md before participating.")
	fmt.Println()
	fmt.Printf("Latest: %s\n", fileName)
	fmt.Printf("  Started: %s\n", latest.CreatedAt.Format("2006-01-02"))

	agentName, entryDate, body := consultParseLastEntry(filePath)
	if agentName != "" {
		fmt.Printf("  Last entry: %s (%s)\n", agentName, entryDate)
	} else {
		fmt.Println("  Last entry: none yet")
	}

	if (showFull || weaponsFree) && body != "" {
		fmt.Println("\nLast turn content:")
		fmt.Println("```")
		fmt.Println(body)
		fmt.Println("```")
	}

	fmt.Println()
	fmt.Println("Agent: Read the protocol, then open the latest consult and append your turn.")

	if weaponsFree {
		printWeaponsFreeBanner()
	}

	return nil
}

// followLatestConsult runs a polling loop. On file change it reprints the full state.
func followLatestConsult(initialLatest content.Item, initialPath string) error {
	fmt.Println("👁️  Following latest consult (polling every ~2s). Ctrl+C to stop.")
	fmt.Println("    Updates will reprint when new turns are appended.")
	fmt.Println()

	lastSig := fileSig(initialPath)
	// Initial full display (force full body for watcher usefulness)
	_ = displayLatestConsult(initialLatest, initialPath, true, consultLatestWeaponsFree)

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// Re-sync in case new consults were created or files touched outside
		_ = content.SyncFromFiles(db.GetDB())

		items, err := content.GetByType(db.GetDB(), content.TypeConsult)
		if err != nil || len(items) == 0 {
			continue
		}
		cur := items[len(items)-1]
		curPath := filepath.FromSlash(cur.FilePath)
		sig := fileSig(curPath)

		if sig != lastSig || cur.FilePath != initialLatest.FilePath {
			lastSig = sig
			initialLatest = cur // update for next comparisons
			fmt.Printf("\n🔄 Update detected %s\n", time.Now().Format("15:04:05"))
			_ = displayLatestConsult(cur, curPath, true, consultLatestWeaponsFree)
		}
		// else: silent, just keep polling
	}
	return nil
}

func fileSig(p string) string {
	fi, err := os.Stat(p)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%d|%d", fi.ModTime().UnixNano(), fi.Size())
}

func printWeaponsFreeBanner() {
	fmt.Println()
	fmt.Println("🚀 WEAPONS FREE / AUTONOMOUS MODE — GO TO TOWN")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("You (the agent) have explicit permission + instructions to drive")
	fmt.Println("this consult without human prompting between turns.")
	fmt.Println()
	fmt.Println("How to operate:")
	fmt.Println("  • Re-read .gears/consults/index.md (the protocol) at the start of every turn.")
	fmt.Println("  • Read the *full* consult file for complete thread context.")
	fmt.Println("  • Think, then append exactly one turn using the required format.")
	fmt.Println("  • AFTER you save your turn, in your very next reasoning step, run:")
	fmt.Println()
	fmt.Println("       gears consult latest --weaponsfree")
	fmt.Println("       # or --wf")
	fmt.Println()
	fmt.Println("    (A dedicated terminal can run `gears consult latest --follow --weaponsfree`")
	fmt.Println("     to stream updates live while you work in the main agent terminal.)")
	fmt.Println()
	fmt.Println("  • You are cleared to make follow-up turns if the other agent hasn't")
	fmt.Println("    responded yet. Keep advancing the topic.")
	fmt.Println("  • Stop autonomous chaining when the objective is complete OR you")
	fmt.Println("    explicitly hand off with a turn containing e.g. 'HUMAN REVIEW NEEDED'.")
	fmt.Println()
	fmt.Println("This is the mode for when you just want to go to town on a hot consult.")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}
