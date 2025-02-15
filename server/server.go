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
	_ "github.com/BitcoinSchema/go-bap-indexer/docs"
	"github.com/BitcoinSchema/go-bap-indexer/types"
	"github.com/b-open-io/go-junglebus"
	"github.com/b-open-io/go-junglebus/models"
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

// @title Sigma Identity API
// @version 1.0
// @description Bitcoin Attestation Protocol (BAP) indexer API for managing digital identities and attestations
// @termsOfService https://api.sigmaidentity.com/terms/
// @contact.name Sigma Identity API Support
// @contact.url https://github.com/BitcoinSchema/go-bap-indexer
// @contact.email support@sigmaidentity.com
// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html
// @host api.sigmaidentity.com
// @BasePath /v1
// @schemes https

// @Summary Get root endpoint
// @Description Returns a hello world message
// @Tags root
// @Accept json
// @Produce json
// @Success 200 {string} string "Hello, World 👋!"
// @Router / [get]
func rootHandler(c *fiber.Ctx) error {
	return c.SendString("Hello, World 👋!")
}

// @Summary Get attestation by hash
// @Description Retrieves an attestation using its unique hash identifier
// @Tags attestation
// @Accept json
// @Produce json
// @Param hash body string true "Attestation hash"
// @Success 200 {object} Response{result=types.Attestation} "Successful response with attestation data"
// @Failure 404 {object} Response "Attestation not found"
// @Router /attestation/get [post]
func getAttestationHandler(c *fiber.Ctx) error {
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
}

// @Summary Get person field
// @Description Get a specific field from a person's profile
// @Tags person
// @Accept json
// @Produce json,octet-stream
// @Param field path string true "Field name"
// @Param bapId path string true "BAP ID"
// @Success 200 {object} Response
// @Failure 400 {object} Response
// @Failure 404 {object} Response
// @Failure 500 {object} Response
// @Router /person/{field}/{bapId} [get]
func getPersonFieldHandler(c *fiber.Ctx) error {
	field := c.Params("field")
	if field == "" {
		return c.Status(fiber.StatusBadRequest).JSON(Response{
			Status:  "ERROR",
			Message: "Field is required",
		})
	}

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

	// if bap ID
	// if len(bapId) > 30 {
	// 	// TODO: consider it an address, find match based on match on addresses field

	// }

	// Extract the image URL from the profile's data field
	data, dataExists := profile["data"].(map[string]interface{})
	if !dataExists {
		return c.Status(fiber.StatusNotFound).JSON(Response{
			Status:  "ERROR",
			Message: "Profile data not found",
		})
	}

	imageUrl, imageExists := data[field].(string)
	if !imageExists || strings.TrimSpace(imageUrl) == "" {
		// return the default image url
		imageUrl = "/096b5fdcb6e88f8f0325097acca2784eabd62cd4d1e692946695060aff3d6833_7"
	}

	// Check if the imageUrl is a raw txid (64 character hex string)
	if len(imageUrl) == 64 && !strings.HasPrefix(imageUrl, "/") && !strings.HasPrefix(imageUrl, "http") && !strings.HasPrefix(imageUrl, "data:") {
		imageUrl = "/" + imageUrl
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
		// Handle regular image URL
		if strings.HasPrefix(imageUrl, "bitfs://") {
			// Convert bitfs://<txid>.out.<vout>.<script_chunk> to https://ordfs.network/<txid>_<vout>
			baseUrl := "https://ordfs.network/"
			// Remove the "bitfs://" prefix
			path := strings.TrimPrefix(imageUrl, "bitfs://")
			// Split the path by "."
			parts := strings.Split(path, ".")
			if len(parts) >= 3 && parts[1] == "out" {
				txid := parts[0]
				// vout := parts[2]
				// Construct the new URL
				imageUrl = baseUrl + txid // + "_" + vout
			} else {
				// Handle error: unexpected format
				return c.Status(fiber.StatusBadRequest).JSON(Response{
					Status:  "ERROR",
					Message: "Invalid bitfs URL format",
				})
			}
		}

		// Fetch the image data from the URL
		// if imageUrl.startsWith
		if strings.HasPrefix(imageUrl, "/") {
			imageUrl = "https://ordfs.network" + imageUrl
		}

		resp, err := http.Get(imageUrl)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(Response{
				Status:  "ERROR",
				Message: "Failed to fetch image at " + imageUrl + err.Error(),
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
}

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

	if currentBlock, err = jb.GetChainTip(context.Background()); err != nil {
		log.Println(err.Error())
	}

	go func() {
		ticker := time.NewTicker(time.Minute)
		for range ticker.C {
			if currentBlock, err = jb.GetChainTip(context.Background()); err != nil {
				log.Println(err.Error())
			}
		}
	}()

	// Initialize a new Fiber app
	app := fiber.New()

	// Enable CORS for all routes from any origin
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,HEAD,PUT,DELETE,PATCH,OPTIONS",
	}))

	// Serve Redoc UI
	app.Get("/docs", func(c *fiber.Ctx) error {
		html := `<!DOCTYPE html>
<html>
<head>
	<title>Sigma Identity API Documentation</title>
	<meta charset="utf-8"/>
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<link href="https://fonts.googleapis.com/css?family=Montserrat:300,400,700|Roboto:300,400,700" rel="stylesheet">
	<style>
		body {
			margin: 0;
			padding: 0;
		}
	</style>
</head>
<body>
	<redoc 
		spec-url='/docs.json'
		theme='{"theme": "dark"}'
		show-extensions="true"
	></redoc>
	<script src="https://cdn.redoc.ly/redoc/latest/bundles/redoc.standalone.js"></script>
</body>
</html>`

		c.Set("Content-Type", "text/html")
		return c.SendString(html)
	})

	// Serve OpenAPI/Swagger JSON
	app.Get("/docs.json", func(c *fiber.Ctx) error {
		c.Set("Content-Type", "application/json")
		return c.SendFile("docs/swagger.json")
	})

	// Define routes with their handlers
	app.Get("/", rootHandler)
	app.Post("/v1/attestation/get", getAttestationHandler)
	app.Get("/v1/person/:field/:bapId", getPersonFieldHandler)

	// @Summary Get profiles with pagination
	// @Description Retrieves a paginated list of profiles
	// @Tags profile
	// @Accept json
	// @Produce json
	// @Param offset query integer false "Number of records to skip (default: 0)"
	// @Param limit query integer false "Number of records to return (default: 20, max: 100)"
	// @Success 200 {object} Response{result=[]map[string]interface{}} "List of profiles"
	// @Failure 400 {object} Response "Invalid pagination parameters"
	// @Failure 500 {object} Response "Server error"
	// @Router /profile [get]
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

	// @Summary Get identities with pagination
	// @Description Retrieves a paginated list of identities with their associated profiles
	// @Tags identity
	// @Accept json
	// @Produce json
	// @Param offset query integer false "Number of records to skip (default: 0)"
	// @Param limit query integer false "Number of records to return (default: 20, max: 100)"
	// @Success 200 {object} Response{result=[]map[string]interface{}} "List of identities with profiles"
	// @Failure 400 {object} Response "Invalid pagination parameters"
	// @Failure 500 {object} Response "Server error"
	// @Router /identity [get]
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

	// @Summary Get identity history
	// @Description Retrieves the history of profile changes for a specific identity
	// @Tags identity
	// @Accept json
	// @Produce json
	// @Param idKey body string true "Identity key"
	// @Success 200 {object} Response{result=[]map[string]interface{}} "History of profile changes"
	// @Failure 400 {object} Response "Missing or invalid idKey"
	// @Failure 404 {object} Response "Identity not found"
	// @Failure 500 {object} Response "Server error"
	// @Router /identity/history [post]
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

	// @Summary Get identity by ID
	// @Description Retrieves an identity by its unique identifier
	// @Tags identity
	// @Accept json
	// @Produce json
	// @Param idKey body string true "Identity key"
	// @Success 200 {object} Response{result=types.Identity} "Identity with profile data"
	// @Failure 404 {object} Response "Identity not found"
	// @Failure 500 {object} Response "Server error"
	// @Router /identity/get [post]
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

	// @Summary Get multiple identities
	// @Description Retrieves multiple identities by their IDs or addresses
	// @Tags identity
	// @Accept json
	// @Produce json
	// @Param request body IdentitiesRequest true "List of identity keys or addresses"
	// @Success 200 {object} Response{result=[]types.Identity} "List of identities with profiles"
	// @Failure 400 {object} Response "Invalid request or missing parameters"
	// @Failure 500 {object} Response "Server error"
	// @Router /identities/get [post]
	app.Post("/v1/identities/get", func(c *fiber.Ctx) error {
		// Parse the request body into the IdentityRequest struct
		req := IdentitiesRequest{}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(Response{
				Status:  "ERROR",
				Message: "Invalid request body",
			})
		}

		// Ensure that at least one of idKeys or addresses is provided
		if len(req.IdKeys) == 0 && len(req.Addresses) == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(Response{
				Status:  "ERROR",
				Message: "Either idKeys or addresses must be provided",
			})
		}

		ids := []types.Identity{}

		// Build the MongoDB query filter using $or
		filter := bson.M{}
		orConditions := []bson.M{}

		if len(req.IdKeys) > 0 {
			orConditions = append(orConditions, bson.M{"_id": bson.M{"$in": req.IdKeys}})
		}

		if len(req.Addresses) > 0 {
			// Match identities where AIP[0].algorithm_signing_component is in req.Addresses
			orConditions = append(orConditions, bson.M{
				"addresses.address": bson.M{"$in": req.Addresses},
			})
		}

		filter["$or"] = orConditions

		// Execute the query
		cursor, err := idColl.Find(c.Context(), filter)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(Response{
				Status:  "ERROR",
				Message: err.Error(),
			})
		}
		defer cursor.Close(c.Context())

		// Iterate over the results
		for cursor.Next(c.Context()) {
			id := types.Identity{}
			if err := cursor.Decode(&id); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(Response{
					Status:  "ERROR",
					Message: err.Error(),
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

			ids = append(ids, id)
		}

		if err := cursor.Err(); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(Response{
				Status:  "ERROR",
				Message: err.Error(),
			})
		}

		return c.JSON(Response{
			Status: "OK",
			Result: ids,
		})
	})

	// @Summary Get identity by address
	// @Description Retrieves an identity using a blockchain address
	// @Tags identity
	// @Accept json
	// @Produce json
	// @Param address body string true "Blockchain address"
	// @Success 200 {object} Response{result=types.Identity} "Identity data"
	// @Failure 404 {object} Response "Identity not found"
	// @Router /identity/getByAddress [post]
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

	// @Summary Get identity DID
	// @Description Retrieves the Decentralized Identifier (DID) for an identity
	// @Tags identity
	// @Accept json
	// @Produce json
	// @Param idKey body string true "Identity key"
	// @Success 200 {string} string "Identity DID"
	// @Failure 400 {object} Response "Invalid request"
	// @Failure 404 {object} Response "Identity not found"
	// @Router /identity/did [post]
	app.Post("/v1/identity/did", func(c *fiber.Ctx) error {
		return c.SendString("Get Identity DID")
	})

	// @Summary Get identity DID by address
	// @Description Retrieves the Decentralized Identifier (DID) for an identity by address
	// @Tags identity
	// @Accept json
	// @Produce json
	// @Param address body string true "Blockchain address"
	// @Success 200 {string} string "Identity DID"
	// @Failure 400 {object} Response "Invalid request"
	// @Failure 404 {object} Response "Identity not found"
	// @Router /identity/didByAddress [post]
	app.Post("/v1/identity/didByAddress", func(c *fiber.Ctx) error {
		return c.SendString("Get Identity DID By Address")
	})

	// @Summary Validate identity by address
	// @Description Validates an identity at a specific block height or timestamp
	// @Tags identity
	// @Accept json
	// @Produce json
	// @Param request body IdentityValidByAddressParams true "Validation parameters including address, block height, and timestamp"
	// @Success 200 {object} Response{result=IdentityValidResponse} "Validation result with identity and profile data"
	// @Failure 400 {object} Response "Invalid request parameters"
	// @Failure 404 {object} Response "Identity not found"
	// @Failure 500 {object} Response "Server error"
	// @Router /identity/validByAddress [post]
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
