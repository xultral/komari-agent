package main

import (
	"os"

	"github.com/xultral/komari-agent/cmd"
)

func main() {
	cmd.Execute()
	os.Exit(0)
}
