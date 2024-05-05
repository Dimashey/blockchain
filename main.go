package main

import (
	"os"

	"github.com/Dimashey/blockchain/commandline"
)

func main() {
	defer os.Exit(0)
	cli := commandline.New()
	cli.Run()
}
