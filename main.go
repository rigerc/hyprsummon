package main

import (
	"os"

	"hyprsummon/cmd"
)

func main() {
	os.Exit(cmd.Execute(os.Args[1:]))
}
