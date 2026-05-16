package main

import (
	"github.com/tomekjarosik/qivitals/cmd/qivitals-server/cmd"
)

func main() {
	cmd.Execute(cmd.InitializeCommands())
}
