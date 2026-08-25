// Package toolbox reports which gears capabilities exist in this workspace and
// what state they are in.
//
// The point is discovery, not instruction: hydration tells an agent that a tool
// exists and where its own help lives, and the agent takes it from there. That
// keeps the command surface documented in exactly one place - the CLI's own
// --help output - so nothing can drift out of sync with the code.
//
// Every check here must be local and instant. Hydration runs at the start of
// every session, so no probe may open the database or touch the network.
package toolbox

import (
	"os"
	"path/filepath"

	"github.com/Syluxso/gears/internal/kanboard"
)

// Capability is one entry in the toolbox report.
type Capability struct {
	Name   string // short label, e.g. "kanboard"
	State  string // one-line state, e.g. "configured"
	Detail string // optional extra context, e.g. the board URL
	Help   string // the command that reveals the full surface
}

var (
	dbPath          = filepath.Join(".gears", ".gearbox", "gears.db")
	watchStatusPath = filepath.Join(".gears", ".gearbox", "watch.status")
)

// Detect returns the capabilities available in the current workspace.
func Detect() []Capability {
	caps := []Capability{}

	// Kanboard: config presence only, never a live call.
	configured, url, detail := kanboard.Describe()
	kan := Capability{Name: "kanboard", State: detail, Help: "gears kan -h"}
	if configured {
		kan.Detail = url
	} else if url != "" {
		kan.Detail = url
	}
	caps = append(caps, kan)

	// Backlog: file presence only. Opening the database here would risk lock
	// contention with a running watch process.
	backlog := Capability{Name: "backlog", Help: "gears backlog -h"}
	if fileExists(dbPath) {
		backlog.State = "local sqlite"
	} else {
		backlog.State = "not initialised"
		backlog.Detail = "run gears init"
	}
	caps = append(caps, backlog)

	// Watch state is deliberately hedged. The status file is written on start
	// and removed on a clean stop, but a killed process leaves it behind and
	// nothing records a PID, so its presence does not prove the watcher is
	// alive. Reporting "running" here would state something we cannot verify.
	watch := Capability{Name: "watch", Help: "gears watch -h"}
	if info, err := os.Stat(watchStatusPath); err == nil && !info.IsDir() {
		watch.State = "status file present"
		watch.Detail = "started " + info.ModTime().Format("2006-01-02 15:04") + ", liveness unverified"
	} else {
		watch.State = "not running"
	}
	caps = append(caps, watch)

	caps = append(caps,
		Capability{Name: "story", State: "available", Help: "gears story -h"},
		Capability{Name: "adr", State: "available", Help: "gears adr -h"},
		Capability{Name: "session", State: "available", Help: "gears session -h"},
		Capability{Name: "consult", State: "available", Help: "gears consult -h"},
		Capability{Name: "inbox", State: "available", Help: "gears inbox -h"},
	)

	return caps
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
