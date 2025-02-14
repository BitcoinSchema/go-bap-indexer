package server

import "github.com/BitcoinSchema/go-bap-indexer/types"

// Response represents the standard API response format
// @Description Standard API response wrapper
type Response struct {
	// Status of the response ("OK" or "ERROR")
	Status string `json:"status" example:"OK"`
	// Optional error message
	Message string `json:"message,omitempty" example:"Operation completed successfully"`
	// Response payload
	Result interface{} `json:"result,omitempty"`
}

// @Description Parameters for validating an attestation
type AttestationValidParams struct {
	// Blockchain address of the attestor
	Address string `json:"address" example:"1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"`
	// Identity key
	IDKey string `json:"idKey" example:"3QxhyGy6ZE5SUpzXVb6AwnXYwH8g"`
	// Attribute being attested
	Attribute string `json:"attribute" example:"name"`
	// Value of the attestation
	Value string `json:"value" example:"John Doe"`
	// Nonce for uniqueness
	Nonce string `json:"nonce" example:"random123"`
	// URN identifier
	Urn string `json:"urn" example:"urn:bap:attestation:123"`
	// Hash of the attestation
	Hash string `json:"hash" example:"abc123def456"`
	// Block height for validation
	Block uint32 `json:"block" example:"123456"`
	// Timestamp for validation
	Timestamp uint32 `json:"timestamp" example:"1612137600"`
}

// IdentitiesRequest represents the request format for fetching multiple identities
// @Description Request format for retrieving multiple identities
type IdentitiesRequest struct {
	// List of identity keys to fetch
	IdKeys []string `json:"idKeys" example:"['id1', 'id2']"`
	// List of blockchain addresses to fetch identities for
	Addresses []string `json:"addresses" example:"['addr1', 'addr2']"`
}

// IdentityValidByAddressParams represents parameters for identity validation
// @Description Parameters for validating an identity at a specific point in time
type IdentityValidByAddressParams struct {
	// Blockchain address to validate
	Address string `json:"address" example:"1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"`
	// Block height to validate at (optional)
	Block uint32 `json:"block" example:"123456"`
	// Timestamp to validate at (optional)
	Timestamp uint32 `json:"timestamp" example:"1612137600"`
}

// ValidityRecord represents the validity status of an identity
// @Description Record indicating the validity of an identity at a point in time
type ValidityRecord struct {
	// Whether the identity is valid
	Valid bool `json:"valid" example:"true"`
	// Block height at which validity was checked
	Block uint32 `json:"block" example:"123456"`
	// Timestamp at which validity was checked
	Timestamp uint32 `json:"timestamp" example:"1612137600"`
}

// @Description Response for attestation validation
type AttestationValidResponse struct {
	// The attestation being validated
	types.Attestation
	// Validity status record
	ValidityRecord
}

// IdentityValidResponse represents the response for identity validation
// @Description Response containing identity validation results
type IdentityValidResponse struct {
	// The identity being validated
	Identity types.Identity `json:"identity"`
	// Validity status record
	ValidityRecord ValidityRecord `json:"validityRecord"`
	// Associated profile data if valid
	Profile interface{} `json:"profile,omitempty"`
}
