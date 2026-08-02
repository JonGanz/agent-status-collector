package main

import (
	"fmt"
	"os"

	"github.com/JonGanz/agent-status-collector/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "agent-status:", err)
		os.Exit(1)
	}
}
