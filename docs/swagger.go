// Package docs contains the root Swagger annotations for the Zenstats HTTP API.
//
// This file is the swagger entrypoint used by `make swagger` and may be safely
// kept under version control. Generated artifacts are docs.go, swagger.json and
// swagger.yaml.
package docs

// @title           Zenstats API
// @version         1.0
// @description     Zenstats provides privacy-friendly web analytics APIs, including event ingestion, site management, API key management, authentication and Plausible-compatible statistics endpoints.
// @description     Most management APIs return a unified JSON envelope: `{ "code": 200, "message": "success", "data": ... }`. Error responses use `{ "code": <business_status>, "message": "...", "error": "..." }`.
// @description     Statistics APIs under `/stats/{domain}` accept either a JWT bearer token or an API key through the `Authorization` header.
// @termsOfService  https://github.com/zenstats/zenstats

// @contact.name   Zenstats
// @contact.url    https://github.com/zenstats/zenstats
// @contact.email  wrpota@gmail.com

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:8080
// @BasePath  /api

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token, for example: Bearer eyJhbGciOi...

// @securityDefinitions.apikey APIKeyAuth
// @in header
// @name Authorization
// @description API key authentication for statistics APIs. Use the API key value in the Authorization header.

// @externalDocs.description  OpenAPI
// @externalDocs.url          https://swagger.io/resources/open-api/
