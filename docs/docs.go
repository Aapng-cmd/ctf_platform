package docs

import "github.com/swaggo/swag"

const docTemplate = `{
  "swagger": "2.0",
  "info": {
    "description": "CTFd-like REST API for this Capture The Flag platform. Swagger UI is only reachable from localhost.",
    "title": "CTF Platform API",
    "contact": {},
    "version": "1.0"
  },
  "basePath": "/",
  "paths": {
    "/api/v1/challenges": {
      "get": {
        "tags": ["challenges"],
        "summary": "List visible challenges",
        "responses": {"200": {"description": "OK"}}
      }
    },
    "/api/v1/challenges/{id}": {
      "get": {
        "tags": ["challenges"],
        "summary": "Get a challenge",
        "parameters": [{"name": "id", "in": "path", "required": true, "type": "integer"}],
        "responses": {"200": {"description": "OK"}}
      }
    },
    "/api/v1/challenges/{id}/attempt": {
      "post": {
        "tags": ["challenges"],
        "summary": "Submit a flag",
        "parameters": [
          {"name": "id", "in": "path", "required": true, "type": "integer"},
          {"name": "body", "in": "body", "schema": {"type": "object", "properties": {"submission": {"type": "string"}}}}
        ],
        "responses": {"200": {"description": "OK"}}
      }
    },
    "/api/v1/challenges/{id}/solves": {
      "get": {
        "tags": ["challenges"],
        "summary": "List solves for a challenge",
        "parameters": [{"name": "id", "in": "path", "required": true, "type": "integer"}],
        "responses": {"200": {"description": "OK"}}
      }
    },
    "/api/v1/scoreboard": {
      "get": {
        "tags": ["scoreboard"],
        "summary": "Get the scoreboard",
        "responses": {"200": {"description": "OK"}}
      }
    },
    "/api/v1/scoreboard/top/{count}": {
      "get": {
        "tags": ["scoreboard"],
        "summary": "Top N score graph data",
        "parameters": [{"name": "count", "in": "path", "required": true, "type": "integer"}],
        "responses": {"200": {"description": "OK"}}
      }
    },
    "/api/v1/users": {
      "get": {
        "tags": ["users"],
        "summary": "List users",
        "responses": {"200": {"description": "OK"}}
      }
    },
    "/api/v1/users/me": {
      "get": {
        "tags": ["users"],
        "summary": "Current authenticated user",
        "responses": {"200": {"description": "OK"}}
      }
    },
    "/api/v1/users/{id}": {
      "get": {
        "tags": ["users"],
        "summary": "Get a user",
        "parameters": [{"name": "id", "in": "path", "required": true, "type": "integer"}],
        "responses": {"200": {"description": "OK"}}
      }
    },
    "/api/v1/teams": {
      "get": {
        "tags": ["teams"],
        "summary": "List teams",
        "responses": {"200": {"description": "OK"}}
      }
    },
    "/api/v1/teams/{id}": {
      "get": {
        "tags": ["teams"],
        "summary": "Get a team",
        "parameters": [{"name": "id", "in": "path", "required": true, "type": "integer"}],
        "responses": {"200": {"description": "OK"}}
      }
    },
    "/api/v1/notifications": {
      "get": {
        "tags": ["notifications"],
        "summary": "List notifications",
        "responses": {"200": {"description": "OK"}}
      }
    },
    "/api/v1/unlocks": {
      "post": {
        "tags": ["unlocks"],
        "summary": "Unlock a hint",
        "parameters": [{"name": "body", "in": "body", "schema": {"type": "object", "properties": {"target": {"type": "integer"}, "type": {"type": "string"}}}}],
        "responses": {"200": {"description": "OK"}}
      }
    },
    "/api/v1/configs": {
      "get": {
        "tags": ["config"],
        "summary": "Public configuration",
        "responses": {"200": {"description": "OK"}}
      }
    },
    "/api/v1/ad/scoreboard": {
      "get": {
        "tags": ["attack-defense"],
        "summary": "Attack-Defense scoreboard",
        "responses": {"200": {"description": "OK"}}
      }
    },
    "/api/v1/ad/status": {
      "get": {
        "tags": ["attack-defense"],
        "summary": "Current AD round status",
        "responses": {"200": {"description": "OK"}}
      }
    },
    "/api/v1/ad/flags": {
      "post": {
        "tags": ["attack-defense"],
        "summary": "Submit a stolen AD flag",
        "parameters": [{"name": "body", "in": "body", "schema": {"type": "object", "properties": {"submission": {"type": "string"}}}}],
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`

var SwaggerInfo = &swag.Spec{
	Version:          "1.0",
	Host:             "",
	BasePath:         "/",
	Schemes:          []string{},
	Title:            "CTF Platform API",
	Description:      "CTFd-like REST API. Swagger UI is restricted to localhost.",
	InfoInstanceName: "swagger",
	SwaggerTemplate:  docTemplate,
}

func init() {
	swag.Register(SwaggerInfo.InstanceName(), SwaggerInfo)
}
