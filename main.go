package main

import (
	"os"

	"github.com/anonvector/slipgate/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
