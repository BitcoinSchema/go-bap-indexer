# go-bap-indexer

A Bitcoin Attestation Protocol (BAP) indexer written in Go. Parses and indexes BAP transactions from the Bitcoin SV blockchain.

## Overview

This project is a blockchain indexer specifically designed for BAP (Bitcoin Attestation Protocol) transactions. It crawls the Bitcoin SV blockchain, processes BAP-related transactions, and maintains an indexed database of identities, attestations, and profiles.

## Features

- Real-time blockchain crawling using [JungleBus](https://github.com/b-open-io/go-junglebus)
- BAP transaction processing and indexing using [go-bmap](https://github.com/bitcoinschema/go-bmap)
- Identity and attestation management
- Profile data storage and retrieval
- RESTful API endpoints for data access
- State management for reliable indexing
- Support for various image formats (base64, bitfs://, ordfs.network)
- OpenAPI/Swagger documentation

## Components

### Core Components

- **Parsers**: [go-bmap](https://github.com/bitcoinschema/go-bmap) provides parsers
  - [go-bap](https://github.com/bitcoinschema/go-bap) - Parses BAP transactions, like ID, ATTEST, etc.
  - [go-bob](https://github.com/bitcoinschema/go-bob) - Splitting transactions into Tapes
  - [go-aip](https://github.com/bitcoinschema/go-aip) - Parses AIP tapes, validate signatures

- **Crawler**: Processes blockchain data in real-time
  - Handles transaction events
  - Processes BAP and AIP (Author Identity Protocol) data
  - Manages block synchronization

- **Server**: Provides HTTP API endpoints
  - Identity management
  - Profile retrieval
  - Attestation verification
  - Image handling

- **State Management**: Tracks indexer progress
  - Uses MongoDB `_state` collection
  - Allows for indexer rewinding
  - Maintains synchronization state

### Database Collections

- `bap.id`: Stores identity information
- `bap.attest`: Stores attestations
- `bap.profile`: Stores profile data
- `bap._state`: Tracks indexer state

## Configuration

The indexer can be configured through environment variables:

- `PORT`: API server port (default: 3000)
- `JUNGLEBUS_ENDPOINT`: JungleBus API endpoint
- `FROM_BLOCK`: Starting block height for indexing
- `SUBSCRIPTION_ID`: JungleBus subscription ID

## State Management

### The _state Collection

The indexer uses a special MongoDB collection called `_state` to track its progress. This collection contains a single document with the structure:

```json
{
  "_id": "_state",
  "height": <current_block_height>
}
```

### Rewinding the Indexer

To rewind the indexer to a previous block:

1. Access your MongoDB instance
2. Update the height in the _state collection:
   ```javascript
   db.bap._state.updateOne(
     { "_id": "_state" },
     { "$set": { "height": <target_block_height> } }
   )
   ```
3. Restart the indexer

The indexer will resume processing from the specified block height.

## API Documentation

The API documentation is available in two formats:

### Swagger UI
```
https://api.sigmaidentity.com/swagger/
```

The classic Swagger UI provides:
- Interactive API documentation
- Try-it-out functionality for testing endpoints
- Raw OpenAPI specification viewing

### Redoc (Modern Dark Theme)
```
https://api.sigmaidentity.com/docs
```

Redoc provides a modern, responsive interface with:
- Dark theme by default
- Better readability and organization
- Improved search functionality
- Three-column layout for better navigation
- Automatic code sample generation

Both interfaces use the same OpenAPI specification and provide:
- Complete endpoint documentation
- Request/response schemas
- Authentication requirements
- Example requests and responses

### API Endpoints

#### Identity Endpoints

- `GET /v1/identity`: List identities (paginated)
- `POST /v1/identity/get`: Get identity by ID
- `POST /v1/identity/getByAddress`: Get identity by address
- `POST /v1/identity/history`: Get identity history
- `POST /v1/identity/validByAddress`: Validate identity by address

#### Profile Endpoints

- `GET /v1/profile`: List profiles (paginated)
- `GET /v1/person/:field/:bapId`: Get specific field from a profile

#### Attestation Endpoints

- `POST /v1/attestation/get`: Get attestation by hash

## Development

### Prerequisites

- Go 1.21.3 or higher
- MongoDB
- Access to a JungleBus endpoint

### Building

```bash
go build
```

### Running

```bash
./go-bap-indexer
```

### Generating API Documentation

To regenerate the Swagger documentation after making API changes:

```bash
swag init -g server/server.go
```

## License

[Add your license information here]
