package types

import (
	"github.com/bitcoinschema/go-aip"
	"github.com/bitcoinschema/go-bap"
)

// {
// 	"idKey": "714a3c856435781fb48ca16a4cf0ba9bc1ef16dd7abbc060d3e18e7e900eec9f",
//     "firstSeen": 590194,
//     "rootAddress": "18nBpQeLxQpnvnZninJbbtGHgY7ru6Mboa",
//     "currentAddress": "134a6TXxzgQ9Az3w8BcvgdZyA5UqRL89da",
//     "addresses": [
//       {
//         "address": "134a6TXxzgQ9Az3w8BcvgdZyA5UqRL89da",
//         "txId": "aa9670f44439c45db24daa5d084021b6667ff317a550d3a5671f564fac4d724c",
//         "block": 590194
//       }
//     ]
// }

type Address struct {
	Address string `json:"address" bson:"address"`
	Txid    string `json:"txId" bson:"txId"`
	Block   uint32 `json:"block" bson:"block"`
}

type Identity struct {
	IDKey          string    `json:"idKey" bson:"_id"`
	FirstSeen      uint32    `json:"firstSeen" bson:"firstSeen"`
	RootAddress    string    `json:"rootAddress" bson:"rootAddress"`
	CurrentAddress string    `json:"currentAddress" bson:"currentAddress"`
	Addresses      []Address `json:"addresses" bson:"addresses"`
}

type BapAip struct {
	BAP *bap.Bap
	AIP *aip.Aip
}
