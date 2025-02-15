package crawler

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"time"

	"fmt"

	"github.com/BitcoinSchema/go-bap-indexer/config"
	"github.com/BitcoinSchema/go-bap-indexer/database"
	"github.com/BitcoinSchema/go-bap-indexer/state"
	"github.com/BitcoinSchema/go-bap-indexer/types"
	"github.com/b-open-io/go-junglebus"
	"github.com/b-open-io/go-junglebus/models"
	"github.com/bitcoin-sv/go-sdk/transaction"
	"github.com/bitcoinschema/go-aip"
	"github.com/bitcoinschema/go-bap"
	"github.com/bitcoinschema/go-bob"
	"github.com/ttacon/chalk"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// var wgs map[uint32]*sync.WaitGroup
var cancelChannel chan int
var eventChannel chan *Event

var ctx = context.Background()

func SyncBlocks(height int) (newBlock int) {
	// Setup crawl timer
	crawlStart := time.Now()

	// Crawl will mutate currentBlock
	newBlock = Crawl(height)

	// Crawl complete
	diff := time.Since(crawlStart).Seconds()

	// TODO: I believe if we get here crawl has actually died
	fmt.Printf("Junglebus closed after %fs\nBlock height: %d\n", diff, height)
	return
}

type BlockState struct {
	Height  int
	Retries int
}

type CrawlState struct {
	Height int
	Blocks []BlockState
}

type Event struct {
	Type        string
	Error       error
	Height      uint32
	Time        uint32
	Id          string
	Transaction []byte
	Status      string
}

func init() {
	// TODO: Is this needed?
	// wgs = make(map[uint32]*sync.WaitGroup)
	// cancelChannel = make(chan int)
	eventChannel = make(chan *Event, 1000000) // Buffered channel
}

// Crawl loops over the new bmap transactions since the given block height
func Crawl(height int) (newHeight int) {

	// readyFiles := make(chan string, 1000) // Adjust buffer size as needed
	// make the first waitgroup for the initial block
	// hereafter we will add these in block done event
	// wgs[uint32(height)] = &sync.WaitGroup{}

	junglebusClient, err := junglebus.New(
		junglebus.WithHTTP(config.JunglebusEndpoint),
	)
	if err != nil {
		log.Fatalln(err.Error())
	}

	subscriptionID := config.SubscriptionID

	// get from block from block.tmp
	fromBlock := uint64(config.FromBlock)

	lastBlock := uint64(state.LoadProgress())

	if lastBlock > fromBlock {
		fromBlock = lastBlock
	}

	eventHandler := junglebus.EventHandler{
		// Mined tx callback
		OnTransaction: func(tx *models.TransactionResponse) {
			// log.Printf("[TX]: %d - %d: %v", tx.BlockHeight, len(tx.Transaction), tx.Id)

			eventChannel <- &Event{
				Type:        "transaction",
				Height:      tx.BlockHeight,
				Time:        tx.BlockTime,
				Transaction: tx.Transaction,
				Id:          tx.Id,
			}
		},
		// Mempool tx callback
		// OnMempool: func(tx *models.TransactionResponse) {
		// 	log.Printf("[MEM]: %d: %v", tx.BlockHeight, tx.Id)

		// 	eventChannel <- &Event{
		// 		Type:        "mempool",
		// 		Transaction: tx.Transaction,
		// 		Id:          tx.Id,
		// 	}
		// },
		OnStatus: func(status *models.ControlResponse) {
			if status.Status == "error" {
				log.Printf("[ERROR %d]: %v", status.StatusCode, status.Message)
				eventChannel <- &Event{Type: "error", Error: fmt.Errorf(status.Message)}
				return
			} else {
				eventChannel <- &Event{
					Type:   "status",
					Height: status.Block,
					Status: status.Status,
				}
			}
		},
		OnError: func(err error) {
			log.Printf("[ERROR]: %v", err)
			eventChannel <- &Event{Type: "error", Error: err}
		},
	}

	fmt.Printf("Initializing from block %d\n", fromBlock)

	var subscription *junglebus.Subscription
	if subscription, err = junglebusClient.Subscribe(ctx, subscriptionID, fromBlock, eventHandler); err != nil {
		log.Printf("ERROR: failed getting subscription %s", err.Error())
	}

	if err != nil {
		log.Printf("ERROR: failed getting subscription %s", err.Error())
		unsubscribeError := subscription.Unsubscribe()

		if err = subscription.Unsubscribe(); unsubscribeError != nil {
			log.Printf("ERROR: failed unsubscribing %s", err.Error())
		}
	}

	// wait indefinitely to make sure we dont stop
	// before more mempool txs come in
	go eventListener(subscription)

	// have a channel here listen for the stop signal, decrement the waitgroup
	// and return the new block height to resubscribe from

	// Print tx line to stdout
	// if err != nil {
	// 	fmt.Println(err)
	// }

	return
}

func CancelCrawl(newBlockHeight int) {
	log.Printf("%s[INFO]: Canceling crawl at block %d%s\n", chalk.Yellow, newBlockHeight, chalk.Reset)
	cancelChannel <- newBlockHeight
}

func processTransactionEvent(rawtx []byte, blockHeight uint32, blockTime uint32) {
	if len(rawtx) > 0 {
		// log.Printf("[TX]: %d: %s | Data Length: %d", blockHeight, tx.Id, len(tx.Transaction))
		t, err := transaction.NewTransactionFromBytes(rawtx)
		if err != nil {
			log.Printf("[ERROR]: %v", err)
			return
		}
		var bobTx *bob.Tx
		if bobTx, err = bob.NewFromTx(t); err != nil {
			return
		}

		bobTx.Blk.I = blockHeight
		bobTx.Blk.T = blockTime
		ProcessTx(bobTx)

	}
}

func processMempoolEvent(rawtx []byte) {
	t, err := transaction.NewTransactionFromBytes(rawtx)
	if err != nil {
		log.Printf("[ERROR]: %v", err)
		return
	}
	var bobTx *bob.Tx
	if bobTx, err = bob.NewFromTx(t); err != nil {
		return
	}

	ProcessTx(bobTx)
}

func processBlockDoneEvent(height uint32, count uint32) {

	filename := fmt.Sprintf("data/%d.json", height)

	// // check if the file exists at path
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		log.Printf("No block file found for %d ", height)
		return
	}

	ingest(filename)
	state.SaveProgress(height)
	if config.DeleteAfterIngest {
		err := os.Remove(filename)
		if err != nil {
			fmt.Printf("%s%s %s: %v%s\n", chalk.Cyan, "Error deleting file", filename, err, chalk.Reset)
		}
	}

	// log ingestions in green using chalk
	log.Printf("%sIngested %d txs from block %d%s", chalk.Cyan, count, height, chalk.Reset)

}

func ProcessTx(bobTx *bob.Tx) {
	baps := make([]types.BapAip, 0)
	for _, out := range bobTx.Out {
		var bapAip *types.BapAip
		for index, tape := range out.Tape {
			if len(tape.Cell) > 0 && tape.Cell[0].S != nil {
				prefixData := *tape.Cell[0].S
				switch prefixData {
				case bap.Prefix:
					if bapOut, err := bap.NewFromTape(&out.Tape[index]); err != nil {
						bapAip = nil
						continue
					} else {
						bapAip = &types.BapAip{
							BAP: bapOut,
						}
					}
				case aip.Prefix:
					if bapAip != nil {
						aipOut := aip.NewFromTape(tape)
						// TODO: Use the correct instance index
						aipOut.SetDataFromTapes(out.Tape, 0)
						bapAip.AIP = aipOut
						baps = append(baps, *bapAip)
						bapAip = nil
					}
				default:
					if bapAip != nil {
						bapAip = nil
					}
				}
			}
		}
	}

	conn := database.GetConnection()
	idColl := conn.Database("bap").Collection("id")
	atColl := conn.Database("bap").Collection("attest")
	proColl := conn.Database("bap").Collection("profile")

	for _, b := range baps {

		if valid, err := b.AIP.Validate(); err != nil {
			log.Printf("Error validating AIP: %s %v", bobTx.Tx.Tx.H, err)
			continue
		} else if !valid {
			continue
		}

		id := &types.Identity{}
		if err := idColl.FindOne(
			ctx,
			bson.M{"currentAddress": b.AIP.AlgorithmSigningComponent},
		).Decode(&id); err == mongo.ErrNoDocuments {
			id = nil
		} else if err != nil {
			panic(err)
		}

		switch b.BAP.Type {
		case bap.ID:
			if id == nil {
				id = &types.Identity{
					IDKey:          b.BAP.IDKey,
					FirstSeen:      bobTx.Tx.Blk.I,
					RootAddress:    b.AIP.AlgorithmSigningComponent,
					CurrentAddress: b.BAP.Address,
					Addresses: []types.Address{
						{
							Address: b.BAP.Address,
							Txid:    bobTx.Tx.Tx.H,
							Block:   bobTx.Tx.Blk.I,
						},
					},
				}
				if _, err := idColl.InsertOne(ctx, id); err != nil && !mongo.IsDuplicateKeyError(err) {
					panic(err)
				}
			} else if id.CurrentAddress == b.AIP.AlgorithmSigningComponent {
				if _, err := idColl.UpdateOne(ctx, bson.M{"_id": id.IDKey}, bson.M{
					"$set": bson.M{"currentAddress": b.BAP.Address},
					"$addToSet": bson.M{"addresses": types.Address{
						Address: b.BAP.Address,
						Txid:    bobTx.Tx.Tx.H,
						Block:   bobTx.Tx.Blk.I,
					}},
				}); err != nil {
					panic(err)
				}
			}
		case bap.ATTEST:
			if id == nil {
				log.Println("ATTEST without ID", bobTx.Tx.Tx.H)
				continue
				// panic()
			}
			signer := &types.Signer{
				IDKey:     id.IDKey,
				Address:   b.AIP.AlgorithmSigningComponent,
				Txid:      bobTx.Tx.Tx.H,
				Block:     bobTx.Tx.Blk.I,
				Timestamp: bobTx.Tx.Blk.T,
				Revoked:   false,
			}
			var att *types.Attestation
			if err := atColl.FindOne(ctx, bson.M{"_id": b.BAP.URNHash}).Decode(&att); err == mongo.ErrNoDocuments {
				att = &types.Attestation{
					Id:      b.BAP.URNHash,
					Signers: []*types.Signer{signer},
				}
				if _, err := atColl.InsertOne(ctx, att); err != nil {
					panic(err)
				}
			} else if err != nil {
				panic(err)
			} else {
				found := false
				for i, s := range att.Signers {
					if s.IDKey == signer.IDKey {
						if s.Sequence < signer.Sequence {
							log.Println("UPDATING ATTEST signer", bobTx.Tx.Tx.H)
							if _, err := atColl.UpdateOne(ctx,
								bson.M{
									"_id": b.BAP.URNHash,
								},
								bson.M{"$set": bson.M{fmt.Sprintf("signers.%d", i): signer}},
							); err != nil {
								panic(err)
							}
						} else {
							log.Println("Bad ATTEST signer sequence", bobTx.Tx.Tx.H)
						}
						found = true
						break
					}
				}
				if !found {
					log.Println("Adding ATTEST signer", bobTx.Tx.Tx.H)
					if _, err := atColl.UpdateOne(ctx,
						bson.M{
							"_id": b.BAP.URNHash,
						},
						bson.M{"$push": bson.M{"signers": signer}},
					); err != nil {
						panic(err)
					}
				}
			}

		case bap.REVOKE:
			if id == nil {
				log.Println("REVOKE without ID", bobTx.Tx.Tx.H)
				continue
			}
			if _, err := idColl.UpdateOne(ctx,
				bson.M{"_id": b.BAP.URNHash},
				bson.M{
					"$pull": bson.M{
						"signers": bson.M{
							"idKey":    id.IDKey,
							"sequence": bson.M{"$lt": b.BAP.Sequence},
						},
					},
				},
			); err != nil {
				panic(err)
			}
		case bap.ALIAS:
			if id == nil {
				// log.Println("ALIAS without ID", bobTx.Tx.Tx.H)
				l := map[string]interface{}{
					"txid": bobTx.Tx.Tx.H,
					"bap":  b.BAP,
					"aip":  b.AIP,
				}
				j, _ := json.MarshalIndent(l, "", "  ")
				log.Println("ALIAS without ID", bobTx.Tx.Tx.H, string(j))
				continue
			}
			if len(b.BAP.Profile) > 0 && b.BAP.IDKey == id.IDKey {
				profile := make(map[string]interface{})
				if err := json.Unmarshal([]byte(b.BAP.Profile), &profile); err != nil {
					panic(err)
				} else if _, err := proColl.UpdateOne(ctx, bson.M{"_id": id.IDKey}, bson.M{"$set": bson.M{"data": profile}}, options.Update().SetUpsert(true)); err != nil {
					panic(err)
				}
			} else {
				l := map[string]interface{}{
					"txid": bobTx.Tx.Tx.H,
					"bap":  b.BAP,
					"aip":  b.AIP,
				}
				j, _ := json.MarshalIndent(l, "", "  ")
				log.Panicln("ALIAS without ID match", string(j))
			}
		}
	}
}
