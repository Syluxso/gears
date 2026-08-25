package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Agent harnesses each look for their own instruction file. Rather than invent
// a gears-specific name that nothing loads, write the conventional ones and let
// each harness find the one it already reads.
//
// These files stay deliberately small. They carry the bootstrap directive only
// - run hydrate, process the inbox - and never a list of commands. The command
// surface is discovered at runtime from hydration and `gears <cmd> -h`, so
// there is no second copy of it to fall out of date.
const (
	agentsDoorway  = "AGENTS.md"
	claudeDoorway  = "CLAUDE.md"
	copilotDoorway = ".github/copilot-instructions.md"

	blockStart = "<!-- gears:start -->"
	blockEnd   = "<!-- gears:end -->"
)

const managedBody = `## Gears Workspace

This workspace is managed by [gears](https://github.com/Syluxso/gears). Its
memory, stories, decisions, and session history live in ` + "`.gears/`" + `.

### Start every session with

` + "```bash" + `
gears hydrate --chat
` + "```" + `

Then process inbox messages before continuing with the user's request:

- **urgent** - handle immediately
- **action** - execute the requested action before continuing
- **info** - acknowledge and continue

### Discovering what gears can do

Run ` + "`gears hydrate`" + ` for the full startup report. Its **Toolbox** section
lists the capabilities available in this workspace and their current state.

Do not rely on any hard-coded list of commands: run ` + "`gears -h`" + `, or
` + "`gears <command> -h`" + ` for a specific one. The CLI's help is generated from
the commands themselves, so it is always current.

### At handoff

1. Update today's ` + "`.gears/sessions/YYYY-MM-DD.md`" + `
2. Update ` + "`.gears/context/index.md`" + `
3. Add an ADR to ` + "`.gears/decisions/index.md`" + ` if the architecture changed
`

const claudeBody = `## Gears Workspace

See @AGENTS.md for the workspace contract.

Start every session with ` + "`gears hydrate --chat`" + ` and process the inbox
before continuing. Run ` + "`gears hydrate`" + ` for the full report, including a
**Toolbox** section listing what is available here.
`

// DoorwayResult reports what happened to one file.
type DoorwayResult struct {
	Path   string
	Action string // "created", "updated", "appended", or "unchanged"
}

// EnsureAgentDoorways writes (or refreshes) the agent instruction files.
//
// Content between the gears markers is owned by gears and rewritten in place.
// Anything outside the markers is hand-written and always preserved - which is
// what lets this be run repeatedly by `gears update` rather than only once at
// init time.
func EnsureAgentDoorways() ([]DoorwayResult, error) {
	targets := []struct {
		path string
		body string
	}{
		{agentsDoorway, managedBody},
		{claudeDoorway, claudeBody},
		{copilotDoorway, managedBody},
	}

	results := make([]DoorwayResult, 0, len(targets))
	for _, t := range targets {
		res, err := writeManagedBlock(t.path, t.body)
		if err != nil {
			return results, err
		}
		results = append(results, res)
	}
	return results, nil
}

// legacyDirective is the exact unmarked file written by gears before managed
// blocks existed. Kept verbatim so it can be recognised and replaced.
const legacyDirective = "# Workspace Instructions\n" +
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

// isLegacyGearsContent reports whether a file holds *only* the unmarked
// directive gears used to write.
//
// This must be an exact match, not a heuristic. A "looks close enough" test
// (matching a prefix, or capping the length) silently deletes hand-written
// content that happens to start the same way - so anything that is not
// byte-identical to what gears itself produced is treated as hand-edited and
// appended to instead.
func isLegacyGearsContent(content string) bool {
	normalise := func(s string) string {
		return strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n"))
	}
	return normalise(content) == normalise(legacyDirective)
}

func writeManagedBlock(path, body string) (DoorwayResult, error) {
	block := blockStart + "\n" + body + blockEnd + "\n"

	existing, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		if dir := filepath.Dir(path); dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return DoorwayResult{Path: path}, fmt.Errorf("failed to create %s: %w", dir, err)
			}
		}
		if err := os.WriteFile(path, []byte(block), 0644); err != nil {
			return DoorwayResult{Path: path}, fmt.Errorf("failed to write %s: %w", path, err)
		}
		return DoorwayResult{Path: path, Action: "created"}, nil
	}
	if err != nil {
		return DoorwayResult{Path: path}, fmt.Errorf("failed to read %s: %w", path, err)
	}

	content := string(existing)
	startIdx := strings.Index(content, blockStart)
	endIdx := strings.Index(content, blockEnd)

	// Older gears versions wrote an unmarked inbox directive. That content is
	// ours and is fully superseded by the block below, so replace it instead of
	// appending - otherwise every workspace created before markers existed ends
	// up carrying two copies of the same instruction.
	if startIdx == -1 && isLegacyGearsContent(content) {
		if err := os.WriteFile(path, []byte(block), 0644); err != nil {
			return DoorwayResult{Path: path}, fmt.Errorf("failed to update %s: %w", path, err)
		}
		return DoorwayResult{Path: path, Action: "migrated"}, nil
	}

	// No markers: this is a hand-written file that predates gears (or was
	// written by another tool). Append rather than overwrite.
	if startIdx == -1 || endIdx == -1 || endIdx < startIdx {
		updated := content
		if !strings.HasSuffix(updated, "\n") {
			updated += "\n"
		}
		updated += "\n" + block
		if err := os.WriteFile(path, []byte(updated), 0644); err != nil {
			return DoorwayResult{Path: path}, fmt.Errorf("failed to update %s: %w", path, err)
		}
		return DoorwayResult{Path: path, Action: "appended"}, nil
	}

	// block already ends in a newline, so trim the leading blank lines the old
	// block left behind rather than accumulating one on every refresh.
	suffix := strings.TrimLeft(content[endIdx+len(blockEnd):], "\n")
	updated := content[:startIdx] + block
	if suffix != "" {
		updated += "\n" + suffix
	}
	updated = strings.TrimRight(updated, "\n") + "\n"

	if updated == content {
		return DoorwayResult{Path: path, Action: "unchanged"}, nil
	}
	if err := os.WriteFile(path, []byte(updated), 0644); err != nil {
		return DoorwayResult{Path: path}, fmt.Errorf("failed to update %s: %w", path, err)
	}
	return DoorwayResult{Path: path, Action: "updated"}, nil
}
