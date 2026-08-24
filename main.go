package main

import (
	"fmt"
	"os"

	"github.com/Syluxso/gears/cmd"
)

func main() {
	cmd.Version = Version // Version comes from version.go in this package
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
