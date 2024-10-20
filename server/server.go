package server

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/BitcoinSchema/go-bap-indexer/database"
	"github.com/BitcoinSchema/go-bap-indexer/types"
	"github.com/GorillaPool/go-junglebus"
	"github.com/GorillaPool/go-junglebus/models"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var TRUE = true
var FALSE = false
var conn *database.Connection
var idColl, atColl, proColl *mongo.Collection
var jb *junglebus.Client
var currentBlock *models.BlockHeader

func Start() {
	var err error
	if jb, err = junglebus.New(
		junglebus.WithHTTP("https://junglebus.gorillapool.io"),
	); err != nil {
		log.Fatalln(err.Error())
	}

	conn = database.GetConnection()
	idColl = conn.Database("bap").Collection("id")
	atColl = conn.Database("bap").Collection("attest")
	proColl = conn.Database("bap").Collection("profile")

	if currentBlock, err = jb.GetChaintip(context.Background()); err != nil {
		log.Println(err.Error())
	}

	go func() {
		ticker := time.NewTicker(time.Minute)
		for range ticker.C {
			if currentBlock, err = jb.GetChaintip(context.Background()); err != nil {
				log.Println(err.Error())
			}
		}
	}()

	// Initialize a new Fiber app
	app := fiber.New()

	// Enable CORS for all routes from any origin
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",                                      // Allow all domains
		AllowMethods: "GET,POST,HEAD,PUT,DELETE,PATCH,OPTIONS", // Allow all methods
	}))

	// Define a route for the GET method on the root path '/'
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World 👋!")
	})

	app.Post("/v1/attestation/get", func(c *fiber.Ctx) error {
		req := map[string]string{}
		c.BodyParser(&req)
		att := &types.Attestation{}

		if err := atColl.FindOne(c.Context(), bson.M{"_id": req["hash"]}).Decode(att); err != nil {
			return c.Status(fiber.StatusNotFound).JSON(Response{
				Status:  "ERROR",
				Message: "Attestation could not be found",
			})
		}

		return c.JSON(Response{
			Status: "OK",
			Result: att,
		})
	})

	// app.Post("/v1/identity/get", func(c *fiber.Ctx) error {
	// 	req := map[string]string{}
	// 	c.BodyParser(&req)
	// 	id := &types.Identity{}
	// 	if err := idColl.FindOne(c.Context(), bson.M{"_id": req["idKey"]}).Decode(id); err != nil {
	// 		return c.Status(fiber.StatusNotFound).JSON(Response{
	// 			Status:  "ERROR",
	// 			Message: "Identity could not be found",
	// 		})
	// 	}

	// 	return c.JSON(Response{
	// 		Status: "OK",
	// 		Result: id,
	// 	})
	// })

	app.Get("/v1/identity", func(c *fiber.Ctx) error {
		// Default pagination parameters
		offset := int64(0)
		limit := int64(20) // You can set a default limit

		// Parse 'offset' query parameter
		if offsetStr := c.Query("offset"); offsetStr != "" {
			if parsedOffset, err := strconv.ParseInt(offsetStr, 10, 64); err == nil {
				if parsedOffset >= 0 {
					offset = parsedOffset
				} else {
					return c.Status(fiber.StatusBadRequest).JSON(Response{
						Status:  "ERROR",
						Message: "Offset must be a non-negative integer",
					})
				}
			} else {
				return c.Status(fiber.StatusBadRequest).JSON(Response{
					Status:  "ERROR",
					Message: "Invalid offset parameter",
				})
			}
		}

		// Optionally, parse 'limit' query parameter
		if limitStr := c.Query("limit"); limitStr != "" {
			if parsedLimit, err := strconv.ParseInt(limitStr, 10, 64); err == nil {
				if parsedLimit > 0 && parsedLimit <= 100 {
					limit = parsedLimit
				} else {
					return c.Status(fiber.StatusBadRequest).JSON(Response{
						Status:  "ERROR",
						Message: "Limit must be a positive integer up to 100",
					})
				}
			} else {
				return c.Status(fiber.StatusBadRequest).JSON(Response{
					Status:  "ERROR",
					Message: "Invalid limit parameter",
				})
			}
		}

		// Set up options for pagination
		findOptions := options.Find()
		findOptions.SetSkip(offset)
		findOptions.SetLimit(limit)

		// Optionally, sort identities (e.g., by creation date)
		findOptions.SetSort(bson.D{{"created_at", -1}}) // Change the field as per your data model

		// Query the identities collection
		cursor, err := idColl.Find(c.Context(), bson.M{}, findOptions)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(Response{
				Status:  "ERROR",
				Message: "failed to fetch identities",
			})
		}
		defer cursor.Close(c.Context())

		// Collect identities into a slice
		var identities []types.Identity
		for cursor.Next(c.Context()) {
			var id types.Identity
			if err := cursor.Decode(&id); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(Response{
					Status:  "ERROR",
					Message: "error decoding identity",
				})
			}
			identities = append(identities, id)
		}

		if err := cursor.Err(); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(Response{
				Status:  "ERROR",
				Message: "cursor error",
			})
		}

		// Return the list of identities
		return c.JSON(Response{
			Status: "OK",
			Result: identities,
		})
	})

	app.Post("/v1/identity/history", func(c *fiber.Ctx) error {
		// Parse the request body
		req := map[string]string{}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(Response{
				Status:  "ERROR",
				Message: "Invalid request body",
			})
		}

		idKey := req["idKey"]
		if idKey == "" {
			return c.Status(fiber.StatusBadRequest).JSON(Response{
				Status:  "ERROR",
				Message: "idKey is required",
			})
		}

		// Find the identity to ensure it exists
		id := &types.Identity{}
		if err := idColl.FindOne(c.Context(), bson.M{"_id": idKey}).Decode(id); err != nil {
			return c.Status(fiber.StatusNotFound).JSON(Response{
				Status:  "ERROR",
				Message: "identity could not be found",
			})
		}

		// Set up options to sort profiles by timestamp (ascending)
		opts := options.Find()
		opts.SetSort(bson.D{{"timestamp", 1}}) // Change to -1 for descending order

		// Fetch all profiles associated with the identity
		cursor, err := proColl.Find(c.Context(), bson.M{"idKey": idKey}, opts)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(Response{
				Status:  "ERROR",
				Message: err.Error(),
			})
		}
		defer cursor.Close(c.Context())

		// Collect profiles into a slice
		var profiles []map[string]interface{}
		for cursor.Next(c.Context()) {
			var profile map[string]interface{}
			if err := cursor.Decode(&profile); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(Response{
					Status:  "ERROR",
					Message: err.Error(),
				})
			}
			profiles = append(profiles, profile)
		}

		if err := cursor.Err(); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(Response{
				Status:  "ERROR",
				Message: err.Error(),
			})
		}

		// Return the profiles in the response
		return c.JSON(Response{
			Status: "OK",
			Result: profiles,
		})
	})

	app.Post("/v1/identity/get", func(c *fiber.Ctx) error {
		req := map[string]string{}
		c.BodyParser(&req)
		id := &types.Identity{}
		if err := idColl.FindOne(c.Context(), bson.M{"_id": req["idKey"]}).Decode(id); err != nil {
			return c.Status(fiber.StatusNotFound).JSON(Response{
				Status:  "ERROR",
				Message: "Identity could not be found",
			})
		}

		// Fetch the profile associated with the identity
		profile := map[string]interface{}{}
		if err := proColl.FindOne(c.Context(), bson.M{"_id": id.IDKey}).Decode(profile); err != nil && err != mongo.ErrNoDocuments {
			return c.Status(fiber.StatusInternalServerError).JSON(Response{
				Status:  "ERROR",
				Message: err.Error(),
			})
		}

		// Assign the profile data to id.Identity
		if data, exists := profile["data"]; exists {
			id.Identity = data
		} else {
			id.Identity = nil
		}

		return c.JSON(Response{
			Status: "OK",
			Result: id,
		})
	})

	app.Post("/v1/identity/getByAddress", func(c *fiber.Ctx) error {
		req := map[string]string{}
		c.BodyParser(&req)
		id := &types.Identity{}

		if err := idColl.FindOne(c.Context(), bson.M{"addresses.address": req["address"]}).Decode(id); err != nil {
			return c.Status(fiber.StatusNotFound).JSON(Response{
				Status:  "ERROR",
				Message: "Identity could not be found",
			})
		}

		return c.JSON(Response{
			Status: "OK",
			Result: id,
		})
	})

	app.Post("/v1/identity/did", func(c *fiber.Ctx) error {
		return c.SendString("Get Identity DID")
	})

	app.Post("/identity/didByAddress", func(c *fiber.Ctx) error {
		return c.SendString("Get Identity DID By Address")
	})

	// app.Post("/v1/attestation/valid", func(c *fiber.Ctx) error {
	// 	req := &AttestationValidParams{}
	// 	c.BodyParser(&req)
	// 	att := &types.Attestation{}

	// 	if req.Address != "" {
	// 		id := &types.Identity{}
	// 		if err := idColl.FindOne(c.Context(), bson.M{"addresses.address": req.Address}).Decode(id); err == mongo.ErrNoDocuments {
	// 			return c.Status(fiber.StatusNotFound).JSON(Response{
	// 				Status:  "ERROR",
	// 				Message: "Identity could not be found",
	// 			})
	// 		} else if err != nil {
	// 			return c.Status(fiber.StatusInternalServerError).JSON(Response{
	// 				Status:  "ERROR",
	// 				Message: err.Error(),
	// 			})
	// 		}
	// 		req.IDKey = id.IDKey
	// 	}

	// 	if req.Hash != "" {
	// 		if err := atColl.FindOne(c.Context(), bson.M{"_id": req.Hash}).Decode(att); err == mongo.ErrNoDocuments {
	// 			return c.Status(fiber.StatusNotFound).JSON(Response{
	// 				Status:  "ERROR",
	// 				Message: "Attestation could not be found",
	// 			})
	// 		} else if err != nil {
	// 			return c.Status(fiber.StatusInternalServerError).JSON(Response{
	// 				Status:  "ERROR",
	// 				Message: err.Error(),
	// 			})
	// 		}
	// 	} else {
	// 		if req.Urn == "" {

	// 			req.Urn = fmt.Sprintf("urn:bap:id:%s:%s:%s", req.Attribute, req.Value, req.Nonce)
	// 		}
	// 		urnHash := sha256.Sum256([]byte(req.Urn))

	// 		if err := atColl.FindOne(c.Context(), bson.M{"_id": hex.EncodeToString(urnHash[:])}).Decode(att); err == mongo.ErrNoDocuments {
	// 			return c.Status(fiber.StatusNotFound).JSON(Response{
	// 				Status:  "ERROR",
	// 				Message: "Attestation could not be found",
	// 			})
	// 		} else if err != nil {
	// 			return c.Status(fiber.StatusInternalServerError).JSON(Response{
	// 				Status:  "ERROR",
	// 				Message: err.Error(),
	// 			})
	// 		}
	// 	}
	// 	att.Valid = &TRUE

	// 	return c.JSON(Response{
	// 		Status: "OK",
	// 		Result: att,
	// 	})
	// })

	app.Post("/v1/identity/valid", func(c *fiber.Ctx) error {
		return c.SendString("Validate Identity")
	})

	app.Post("/v1/identity/validByAddress", func(c *fiber.Ctx) error {
		req := &IdentityValidByAddressParams{}
		c.BodyParser(&req)

		id := &types.Identity{}
		if err := idColl.FindOne(c.Context(), bson.M{"addresses.address": req.Address}).Decode(id); err == mongo.ErrNoDocuments {
			return c.Status(fiber.StatusNotFound).JSON(Response{
				Status:  "ERROR",
				Message: "Identity could not be found",
			})
		} else if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(Response{
				Status:  "ERROR",
				Message: err.Error(),
			})
		}
		profile := map[string]interface{}{}
		if err := proColl.FindOne(c.Context(), bson.M{"_id": id.IDKey}).Decode(profile); err != nil && err != mongo.ErrNoDocuments {
			return c.Status(fiber.StatusInternalServerError).JSON(Response{
				Status:  "ERROR",
				Message: err.Error(),
			})
		}
		if req.Block == 0 && req.Timestamp == 0 {
			req.Block = currentBlock.Height
			req.Timestamp = currentBlock.Time
		}
		if req.Block > 0 {
			currentAddress := ""
			for _, addr := range id.Addresses {
				if addr.Block <= req.Block {
					currentAddress = addr.Address
				} else {
					break
				}
			}
			if currentAddress != req.Address {
				return c.JSON(Response{
					Status: "OK",
					Result: IdentityValidResponse{
						Identity: *id,
						ValidityRecord: ValidityRecord{
							Valid:     false,
							Block:     req.Block,
							Timestamp: req.Timestamp,
						},
					},
				})
			} else {
				return c.JSON(Response{
					Status: "OK",
					Result: IdentityValidResponse{
						Identity: *id,
						ValidityRecord: ValidityRecord{
							Valid:     true,
							Block:     req.Block,
							Timestamp: req.Timestamp,
						},
						Profile: profile["data"],
					},
				})
			}
		} else {
			currentAddress := ""
			for _, addr := range id.Addresses {
				if addr.Timestamp <= req.Timestamp {
					currentAddress = addr.Address
				} else {
					break
				}
			}
			if currentAddress != req.Address {
				return c.JSON(Response{
					Status: "OK",
					Result: IdentityValidResponse{
						Identity: *id,
						ValidityRecord: ValidityRecord{
							Valid:     false,
							Block:     req.Block,
							Timestamp: req.Timestamp,
						},
					},
				})
			} else {
				return c.JSON(Response{
					Status: "OK",
					Result: IdentityValidResponse{
						Identity: *id,
						ValidityRecord: ValidityRecord{
							Valid:     true,
							Block:     req.Block,
							Timestamp: req.Timestamp,
						},
						Profile: profile["data"],
					},
				})
			}
		}
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	addr := fmt.Sprintf(":%s", port)
	// Start the server on port 3000
	log.Fatal(app.Listen(addr))
}
