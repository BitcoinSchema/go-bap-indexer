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
	Address   string `json:"address" bson:"address"`
	Txid      string `json:"txId" bson:"txId"`
	Block     uint32 `json:"block" bson:"block"`
	Timestamp uint32 `json:"-" bson:"timestamp"`
}

type Identity struct {
	IDKey          string      `json:"idKey" bson:"_id"`
	FirstSeen      uint32      `json:"firstSeen" bson:"firstSeen"`
	RootAddress    string      `json:"rootAddress" bson:"rootAddress"`
	CurrentAddress string      `json:"currentAddress" bson:"currentAddress"`
	Addresses      []Address   `json:"addresses" bson:"addresses"`
	Identity       interface{} `json:"identity" bson:"-"`
}

type BapAip struct {
	BAP *bap.Bap
	AIP *aip.Aip
}

// {
// "attribute": "name",
// "value": "John Doe",
// "nonce": "e2c6fb4063cc04af58935737eaffc938011dff546d47b7fbb18ed346f8c4d4fa",
// "urn": "urn:bap:id:name:John Doe:e2c6fb4063cc04af58935737eaffc938011dff546d47b7fbb18ed346f8c4d4fa",
// "hash": "b17c8e606afcf0d8dca65bdf8f33d275239438116557980203c82b0fae259838",
// "signers": [
//   {
// 	"idKey": "714a3c856435781fb48ca16a4cf0ba9bc1ef16dd7abbc060d3e18e7e900eec9f",
// 	"address": "134a6TXxzgQ9Az3w8BcvgdZyA5UqRL89da",
// 	"txId": "1fd626dc8286d449d4c2cf3b5b70d169728f5ffefd5c3a3205d4970e21fbf187",
// 	"block": 590230,
// 	"sequence": 0
//   }
// ],
// "block": 594320,
// "timestamp": 1565040382,
// "valid": true
// }

type Signer struct {
	IDKey     string `json:"idKey" bson:"idKey"`
	Address   string `json:"signingAddress" bson:"signingAddress"`
	Sequence  uint64 `json:"sequence" bson:"sequence"`
	Block     uint32 `json:"block" bson:"block"`
	Txid      string `json:"txId" bson:"txId"`
	Timestamp uint32 `json:"timestamp" bson:"timestamp"`
	Revoked   bool   `json:"revoked" bson:"revoked"`
}

type Attestation struct {
	Id        string    `json:"hash" bson:"_id"`
	Attribute string    `json:"attribute,omitempty" bson:"attribute,omitempty"`
	Value     string    `json:"value,omitempty" bson:"value,omitempty"`
	Nonce     string    `json:"nonce,omitempty" bson:"nonce,omitempty"`
	URN       string    `json:"urn,omitempty" bson:"urn,omitempty"`
	Signers   []*Signer `json:"signers" bson:"signers"`
}
