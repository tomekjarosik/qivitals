package main

import (
	"github.com/tomekjarosik/one-status/cmd/statussvc/cmd"
)

func main() {
	cmd.Execute(cmd.InitializeCommands())
}
