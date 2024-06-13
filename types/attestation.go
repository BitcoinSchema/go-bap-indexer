package types

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

type Attestation struct {
	Id        string `json:"_id" bson:"_id"`
	Hash      string `json:"hash" bson:"hash"`
	Address   string `json:"signingAddress" bson:"signingAddress"`
	Sequence  uint64 `json:"sequence" bson:"sequence"`
	Block     uint32 `json:"block" bson:"block"`
	Txid      string `json:"txId" bson:"txId"`
	Timestamp uint32 `json:"timestamp" bson:"timestamp"`
	Revoked   bool   `json:"revoked" bson:"revoked"`
}
