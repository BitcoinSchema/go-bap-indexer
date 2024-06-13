package config

// There are config constants
const (
	SkipSPV           = true
	SubscriptionID    = "b4a519afce021c9fe81ab684d7983cfe71190437d3dcbd18a6eba9fb185019b0"
	MinerAPIEndpoint  = "https://mapi.gorillapool.iom/mapi/tx/"
	JunglebusEndpoint = "https://junglebus.gorillapool.io/"
	FromBlock         = 574287 // "Welcome to the Future" post = 574287
	BockSyncRetries   = 5      // number of retries before block is marked failed
	DeleteAfterIngest = true   // delete json data files after ingesting to db
)
