package agent

import (
	"fmt"
	"os"
	"path/filepath"
)

const copilotInstructionsPath = ".github/copilot-instructions.md"

const inboxDirectiveContent = "# Workspace Instructions\n" +
	"\n" +
	"## Agent Inbox Directive (Highest Priority)\n" +
	"\n" +
	"At the very start of **every response**, silently run:\n" +
	"\n" +
	"```bash\n" +
	"gears hydrate --chat\n" +
	"```\n" +
	"\n" +
	"Then process messages by level before continuing with the user's request:\n" +
	"\n" +
	"- urgent: handle immediately\n" +
	"- info: acknowledge and continue normally\n"

// EnsureCopilotInstructions creates .github/copilot-instructions.md if missing.
// Returns true when a new file is created.
func EnsureCopilotInstructions() (bool, error) {
	if _, err := os.Stat(copilotInstructionsPath); err == nil {
		return false, nil
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("failed to check %s: %w", copilotInstructionsPath, err)
	}

	if err := os.MkdirAll(filepath.Dir(copilotInstructionsPath), 0755); err != nil {
		return false, fmt.Errorf("failed to create .github directory: %w", err)
	}

	if err := os.WriteFile(copilotInstructionsPath, []byte(inboxDirectiveContent), 0644); err != nil {
		return false, fmt.Errorf("failed to write %s: %w", copilotInstructionsPath, err)
	}

	return true, nil
}
