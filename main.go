package main

import (
	"os"

	"github.com/rigerc/hyprsummon/cmd"
)

func main() {
	os.Exit(cmd.Execute(os.Args[1:]))
}
