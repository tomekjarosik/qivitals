package main

import (
	"github.com/tomekjarosik/one-status/cmd/sensorcli/cmd"
)

func main() {
	cmd.Execute(cmd.InitializeCommands())
}
