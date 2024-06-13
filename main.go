package main

import (
	"github.com/rohenaz/go-bap-indexer/crawler"
	"github.com/rohenaz/go-bap-indexer/state"
)

func main() {
	currentBlock := state.LoadProgress()

	go crawler.ProcessDone()
	crawler.SyncBlocks(int(currentBlock))

	<-make(chan struct{})
}
