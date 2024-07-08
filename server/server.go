package server

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/BitcoinSchema/go-bap-indexer/database"
	"github.com/BitcoinSchema/go-bap-indexer/types"
	"github.com/GorillaPool/go-junglebus"
	"github.com/GorillaPool/go-junglebus/models"
	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
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
		if err := proColl.FindOne(c.Context(), bson.M{"_id": profile}).Decode(profile); err == mongo.ErrNoDocuments {
			profile = nil
		} else if err != nil {
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
						Profile: profile,
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
						Profile: profile,
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
