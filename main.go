package main

import (
	"os"

	"github.com/Dimashey/blockchain/blockchain"
	"github.com/Dimashey/blockchain/commandline"
)

func main() {
	defer os.Exit(0)
	chain := blockchain.InitBlockChain()
	defer chain.Database.Close()

	cli := commandline.New(chain)
	cli.Run()
}
