package crawler

import (
	"log"

	"github.com/BitcoinSchema/go-bap-indexer/state"
	"github.com/b-open-io/go-junglebus"
	"github.com/ttacon/chalk"
)

// map of block height to tx count
var blocksDone = make(chan map[uint32]uint32, 1000)

var txCount uint32

func eventListener(subscription *junglebus.Subscription) {
	// var crawlHeight uint32
	// var wg sync.WaitGroup
	for event := range eventChannel {
		switch event.Type {
		case "transaction":
			txCount++
			// log.Printf("%sTransaction %s %s\n", chalk.Green, event.Id, chalk.Reset)
			processTransactionEvent(event.Transaction, event.Height, event.Time)

		case "status":
			switch event.Status {
			case "disconnected":
				txCount = 0
				log.Printf("%sDisconnected from Junglebus. Reset tx counter.%s\n", chalk.Green, chalk.Reset)
				continue
			case "connected":
				log.Printf("%sConnected to Junglebus%s\n", chalk.Green, chalk.Reset)

				continue
			case "block-done":
				// copy the var
				var count = txCount
				if count > 0 {
					log.Printf("%sBlock %d done with %d transactions%s\n", chalk.Green, event.Height, count, chalk.Reset)
					state.SaveProgress(event.Height)
					// blocksDone <- map[uint32]uint32{event.Height: count}
				}
				txCount = 0
				continue
			}
		case "mempool":
			processMempoolEvent(event.Transaction)
		case "error":
			log.Printf("%sERROR: %s%s\n", chalk.Green, event.Error.Error(), chalk.Reset)
		}
	}
}

func ProcessDone() {
	for heightMap := range blocksDone {
		// loop over single entry map
		for height, txCount := range heightMap {
			if txCount > 0 {
				processBlockDoneEvent(height, txCount)
			}
			break
		}
	}
}
