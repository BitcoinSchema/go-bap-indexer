package server

import "github.com/rohenaz/go-bap-indexer/types"

type Response struct {
	Status  string `json:"status"`
	Result  any    `json:"result,omitempty"`
	Message string `json:"message,omitempty"`
}

type AttestationValidParams struct {
	Address   string `json:"address"`
	IDKey     string `json:"idKey"`
	Attribute string `json:"attribute"`
	Value     string `json:"value"`
	Nonce     string `json:"nonce"`
	Urn       string `json:"urn"`
	Hash      string `json:"hash"`
	Block     uint32 `json:"block"`
	Timestamp uint32 `json:"timestamp"`
}

type IdentityValidByAddressParams struct {
	Address   string `json:"address"`
	Block     uint32 `json:"block"`
	Timestamp uint32 `json:"timestamp"`
}

type ValidityRecord struct {
	Valid     bool   `json:"valid"`
	Block     uint32 `json:"block"`
	Timestamp uint32 `json:"timestamp"`
}

type AttestationValidResponse struct {
	types.Attestation
	ValidityRecord
}

type IdentityValidResponse struct {
	types.Identity
	ValidityRecord
}
