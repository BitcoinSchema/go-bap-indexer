package server

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"strconv"
	"strings"
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

	app.Get("/v1/person/image/:bapId", func(c *fiber.Ctx) error {
		bapId := c.Params("bapId")
		if bapId == "" {
			return c.Status(fiber.StatusBadRequest).JSON(Response{
				Status:  "ERROR",
				Message: "BAPID is required",
			})
		}

		// Fetch the profile associated with the BAPID
		profile := map[string]interface{}{}
		if err := proColl.FindOne(c.Context(), bson.M{"_id": bapId}).Decode(&profile); err == mongo.ErrNoDocuments {
			return c.Status(fiber.StatusNotFound).JSON(Response{
				Status:  "ERROR",
				Message: "Profile not found",
			})
		} else if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(Response{
				Status:  "ERROR",
				Message: err.Error(),
			})
		}

		// Extract the image URL from the profile's data field
		data, dataExists := profile["data"].(map[string]interface{})
		if !dataExists {
			return c.Status(fiber.StatusNotFound).JSON(Response{
				Status:  "ERROR",
				Message: "Profile data not found",
			})
		}

		imageUrl, imageExists := data["image"].(string)
		if !imageExists || strings.TrimSpace(imageUrl) == "" {
			return c.Status(fiber.StatusNotFound).JSON(Response{
				Status:  "ERROR",
				Message: "Image URL not found in profile",
			})
		}

		if strings.HasPrefix(imageUrl, "data:") {
			// Handle base64-encoded data URL
			commaIndex := strings.Index(imageUrl, ",")
			if commaIndex < 0 {
				return c.Status(fiber.StatusBadRequest).JSON(Response{
					Status:  "ERROR",
					Message: "Invalid data URL format",
				})
			}

			// Extract the metadata and data
			// Remove "data:" prefix from metaData
			metaData := strings.TrimPrefix(imageUrl[:commaIndex], "data:")
			// metadata = image/jpeg;base64
			metaDataParts := strings.Split(metaData, ";")

			metaData = metaDataParts[0]
			// metadata = image/jpeg

			base64Data := imageUrl[commaIndex+1:]

			// Parse the media type from the metadata
			mediaType, _, err := mime.ParseMediaType(metaData)
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(Response{
					Status:  "ERROR",
					Message: "Invalid media type in data URL " + metaData + " " + err.Error(),
				})
			}

			// image/jpeg;base64
			log.Println(("Data URL: " + base64Data))

			// Decode the base64 data
			imgData, err := base64.StdEncoding.DecodeString(base64Data)
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(Response{
					Status:  "ERROR",
					Message: "Failed to decode base64 image data",
				})
			}

			// Set the Content-Type header
			c.Set("Content-Type", mediaType)

			// Return the image data
			return c.Send(imgData)
		} else {
			// Handle regular image URL
			// If the image URL uses a custom protocol (e.g., bitfs://), handle it accordingly
			if strings.HasPrefix(imageUrl, "bitfs://") {
				// Convert bitfs://<hash> to a valid HTTP URL
				imageUrl = "https://ordfs.network/" + strings.TrimPrefix(imageUrl, "bitfs://")
			}

			// Fetch the image data from the URL
			resp, err := http.Get(imageUrl)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(Response{
					Status:  "ERROR",
					Message: "Failed to fetch image",
				})
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return c.Status(fiber.StatusNotFound).JSON(Response{
					Status:  "ERROR",
					Message: "Image not found at the specified URL",
				})
			}

			// Read the image data
			imgData, err := io.ReadAll(resp.Body)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(Response{
					Status:  "ERROR",
					Message: "Failed to read image data",
				})
			}

			// Determine the content type
			contentType := resp.Header.Get("Content-Type")
			if contentType == "" {
				// Fallback to detecting content type from data
				contentType = http.DetectContentType(imgData)
			}

			// Set the appropriate content type header
			c.Set("Content-Type", contentType)

			// Return the image data as the response
			return c.Send(imgData)
		}
	})

	app.Get("/v1/profile", func(c *fiber.Ctx) error {
		// Default pagination parameters
		offset := int64(0)
		limit := int64(20) // Set a default limit

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
		findOptions.SetSort(bson.D{{"timestamp", -1}}) // Adjust sorting as needed

		// Query the profiles collection
		cursor, err := proColl.Find(c.Context(), bson.M{}, findOptions)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(Response{
				Status:  "ERROR",
				Message: "Failed to fetch profiles",
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
					Message: "Error decoding profile",
				})
			}
			profiles = append(profiles, profile)
		}

		if err := cursor.Err(); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(Response{
				Status:  "ERROR",
				Message: "Cursor error",
			})
		}

		// Return the list of profiles
		return c.JSON(Response{
			Status: "OK",
			Result: profiles,
		})
	})

	app.Get("/v1/identity", func(c *fiber.Ctx) error {
		// Default pagination parameters
		offset := int64(0)
		limit := int64(20) // Set a default limit

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
		findOptions.SetSort(bson.D{{"firstSeen", -1}}) // Adjust sorting as needed

		// Query the identities collection
		cursor, err := idColl.Find(c.Context(), bson.M{}, findOptions)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(Response{
				Status:  "ERROR",
				Message: "Failed to fetch identities",
			})
		}
		defer cursor.Close(c.Context())

		// Collect identities into a slice
		var identities []map[string]interface{}
		for cursor.Next(c.Context()) {
			var id types.Identity
			if err := cursor.Decode(&id); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(Response{
					Status:  "ERROR",
					Message: "Error decoding identity",
				})
			}

			// Fetch the profile associated with the identity
			profile := map[string]interface{}{}
			if err := proColl.FindOne(c.Context(), bson.M{"_id": id.IDKey}).Decode(&profile); err != nil && err != mongo.ErrNoDocuments {
				return c.Status(fiber.StatusInternalServerError).JSON(Response{
					Status:  "ERROR",
					Message: err.Error(),
				})
			}

			// Extract the 'data' field from the profile
			var identityData interface{} = nil
			if data, exists := profile["data"]; exists {
				identityData = data
			}

			// Build the response object
			identityResponse := map[string]interface{}{
				"idKey":          id.IDKey,
				"firstSeen":      id.FirstSeen,
				"rootAddress":    id.RootAddress,
				"currentAddress": id.CurrentAddress,
				"addresses":      id.Addresses,
				"identity":       identityData,
			}

			identities = append(identities, identityResponse)
		}

		if err := cursor.Err(); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(Response{
				Status:  "ERROR",
				Message: "Cursor error",
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
