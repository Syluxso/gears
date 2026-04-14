package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/Syluxso/gears/internal/db"
	"github.com/Syluxso/gears/internal/inbox"
	"github.com/spf13/cobra"
)

var (
	inboxReadFlag  bool
	inboxListFlag  bool
	inboxClearFlag bool
	inboxLimit     int

	inboxAddLevel   string
	inboxAddTitle   string
	inboxAddMessage string
	inboxAddSuggest string
	inboxAddMeta    string
)

var inboxCmd = &cobra.Command{
	Use:   "inbox",
	Short: "Read and manage agent inbox directives",
	Long: `The inbox stores agent directives from gears watch and other background processes.

Examples:
  gears inbox --read
  gears inbox --list
  gears inbox --clear
  gears inbox add --level action --title "Update Context" --message "Refresh context from recent changes" --cmd "gears context update --auto"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := db.Initialize(); err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}

		switch {
		case inboxReadFlag:
			return runInboxRead()
		case inboxListFlag:
			return runInboxList()
		case inboxClearFlag:
			return runInboxClear()
		default:
			return cmd.Help()
		}
	},
}

var inboxAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add an inbox message",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := db.Initialize(); err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}

		if strings.TrimSpace(inboxAddMessage) == "" && len(args) > 0 {
			inboxAddMessage = strings.Join(args, " ")
		}

		m := &inbox.Message{
			Level:            inboxAddLevel,
			Title:            inboxAddTitle,
			Message:          inboxAddMessage,
			SuggestedCommand: inboxAddSuggest,
			Metadata:         inboxAddMeta,
		}

		if err := inbox.Add(db.GetDB(), m); err != nil {
			return err
		}

		fmt.Printf("✓ Added inbox message #%d (%s)\n", m.ID, m.Level)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(inboxCmd)
	inboxCmd.AddCommand(inboxAddCmd)

	inboxCmd.Flags().BoolVar(&inboxReadFlag, "read", false, "Read unread messages and mark them read")
	inboxCmd.Flags().BoolVar(&inboxListFlag, "list", false, "List inbox messages")
	inboxCmd.Flags().BoolVar(&inboxClearFlag, "clear", false, "Mark all unread messages as read")
	inboxCmd.Flags().IntVar(&inboxLimit, "limit", 100, "Maximum number of messages")

	inboxAddCmd.Flags().StringVar(&inboxAddLevel, "level", inbox.LevelInfo, "Message level: urgent|action|info")
	inboxAddCmd.Flags().StringVar(&inboxAddTitle, "title", "Inbox Message", "Message title")
	inboxAddCmd.Flags().StringVar(&inboxAddMessage, "message", "", "Message body")
	inboxAddCmd.Flags().StringVar(&inboxAddSuggest, "cmd", "", "Suggested command")
	inboxAddCmd.Flags().StringVar(&inboxAddMeta, "metadata", "", "Optional metadata JSON string")
}

func runInboxRead() error {
	msgs, err := inbox.ReadUnread(db.GetDB(), inboxLimit)
	if err != nil {
		return err
	}

	if len(msgs) == 0 {
		fmt.Println("Inbox is empty. No pending directives from Gears.")
		return nil
	}

	fmt.Printf("📥 Inbox (%d unread message(s))\n", len(msgs))
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	for _, m := range msgs {
		printInboxMessage(m)
	}

	return nil
}

func runInboxList() error {
	msgs, err := inbox.List(db.GetDB(), false, inboxLimit)
	if err != nil {
		return err
	}

	if len(msgs) == 0 {
		fmt.Println("Inbox is empty. No pending directives from Gears.")
		return nil
	}

	fmt.Printf("📋 Inbox (showing %d message(s))\n", len(msgs))
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	for _, m := range msgs {
		printInboxMessage(m)
	}

	return nil
}

func runInboxClear() error {
	count, err := inbox.ClearUnread(db.GetDB())
	if err != nil {
		return err
	}

	fmt.Printf("✓ Cleared %d unread inbox message(s)\n", count)
	return nil
}

func printInboxMessage(m inbox.Message) {
	readStatus := "unread"
	if m.IsRead {
		readStatus = "read"
	}

	level := strings.ToUpper(m.Level)
	emoji := "ℹ️"
	switch m.Level {
	case inbox.LevelUrgent:
		emoji = "🚨"
	case inbox.LevelAction:
		emoji = "⚡"
	case inbox.LevelInfo:
		emoji = "ℹ️"
	}

	fmt.Printf("%s [%s] %s (#%d, %s, %s)\n", emoji, level, m.Title, m.ID, readStatus, m.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("   %s\n", m.Message)
	if strings.TrimSpace(m.SuggestedCommand) != "" {
		fmt.Printf("   Suggested: %s\n", m.SuggestedCommand)
	}
	if m.ReadAt != nil {
		fmt.Printf("   Read At: %s\n", m.ReadAt.Format(time.RFC3339))
	}
	fmt.Println()
}
