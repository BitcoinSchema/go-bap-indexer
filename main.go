package main

import (
	"github.com/BitcoinSchema/go-bap-indexer/crawler"
	"github.com/BitcoinSchema/go-bap-indexer/server"
	"github.com/BitcoinSchema/go-bap-indexer/state"
)

func main() {
	currentBlock := state.LoadProgress()

	go server.Start()
	go crawler.ProcessDone()
	crawler.SyncBlocks(int(currentBlock))

	<-make(chan struct{})
}
