package main

import (
	"github.com/tomekjarosik/qivitals/cmd/qivitals-cli/cmd"
)

func main() {
	cmd.Execute(cmd.InitializeCommands())
}
