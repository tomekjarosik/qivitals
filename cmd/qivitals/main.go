package main

import (
	"github.com/tomekjarosik/qivitals/cmd/qivitals/cmd"
)

func main() {
	cmd.Execute(cmd.InitializeCommands())
}
