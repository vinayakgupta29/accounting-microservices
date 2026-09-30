package docs

import (
	"fmt"
	"net/http"
)

// SwaggerJSON provides the OpenAPI 3.0 specification for the Gateway REST API.
const SwaggerJSON = `{
  "openapi": "3.0.3",
  "info": {
    "title": "Accounting Microservices API Gateway",
    "version": "1.0.0",
    "description": "Unified REST API Gateway for the polyglot accounting microservices architecture (Go, Rust, gRPC). Handles authentication, customer directories, inventory tracking, transactional invoicing with exact decimal arithmetic, and audit statements."
  },
  "servers": [
    {
      "url": "http://localhost:8080",
      "description": "Local API Gateway Server"
    }
  ],
  "components": {
    "securitySchemes": {
      "bearerAuth": {
        "type": "http",
        "scheme": "bearer",
        "bearerFormat": "JWT"
      }
    }
  },
  "paths": {
    "/api/v1/auth/signup": {
      "post": {
        "tags": ["Authentication"],
        "summary": "Register a new business tenant",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "required": ["name", "username", "email", "password"],
                "properties": {
                  "name": { "type": "string", "example": "Jane Doe" },
                  "username": { "type": "string", "example": "janedoe" },
                  "email": { "type": "string", "example": "jane@example.com" },
                  "password": { "type": "string", "example": "SecureSecret123!" },
                  "gstin": { "type": "string", "example": "29ABCDE1234F1Z5" },
                  "pan": { "type": "string", "example": "ABCDE1234F" },
                  "aadhaar": { "type": "string", "example": "123456789012" },
                  "phone": { "type": "string", "example": "9876543210" },
                  "address": { "type": "string", "example": "Tech Park, Bangalore" }
                }
              }
            }
          }
        },
        "responses": {
          "201": { "description": "Tenant registered successfully" }
        }
      }
    },
    "/api/v1/auth/login": {
      "post": {
        "tags": ["Authentication"],
        "summary": "Authenticate user credentials and acquire JWT token",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "required": ["username", "password"],
                "properties": {
                  "username": { "type": "string", "example": "janedoe" },
                  "password": { "type": "string", "example": "SecureSecret123!" }
                }
              }
            }
          }
        },
        "responses": {
          "200": { "description": "Login successful" },
          "401": { "description": "Invalid credentials" }
        }
      }
    },
    "/api/v1/auth/profile": {
      "get": {
        "tags": ["Authentication"],
        "summary": "Get authenticated user profile",
        "security": [{ "bearerAuth": [] }],
        "responses": {
          "200": { "description": "User profile data" }
        }
      }
    },
    "/api/v1/customers": {
      "post": {
        "tags": ["Customers"],
        "summary": "Register a new customer for the tenant",
        "security": [{ "bearerAuth": [] }],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "required": ["name", "email", "gstin"],
                "properties": {
                  "name": { "type": "string", "example": "Acme Global Corp" },
                  "address": { "type": "string", "example": "42 Boulevard St" },
                  "phone_number": { "type": "string", "example": "9876543210" },
                  "email": { "type": "string", "example": "billing@acme.com" },
                  "gstin": { "type": "string", "example": "29AAAAA0000A1Z5" },
                  "dealer_type": { "type": "string", "example": "Regular" },
                  "pan_card": { "type": "string", "example": "AAAAA0000A" },
                  "aadhaar": { "type": "string", "example": "987654321098" }
                }
              }
            }
          }
        },
        "responses": {
          "201": { "description": "Customer registered" }
        }
      },
      "get": {
        "tags": ["Customers"],
        "summary": "List tenant customers (paginated)",
        "security": [{ "bearerAuth": [] }],
        "parameters": [
          { "name": "page", "in": "query", "schema": { "type": "integer", "default": 1 } },
          { "name": "limit", "in": "query", "schema": { "type": "integer", "default": 50 } }
        ],
        "responses": {
          "200": { "description": "Customer list" }
        }
      }
    },
    "/api/v1/inventory": {
      "post": {
        "tags": ["Inventory"],
        "summary": "Add or stock a product item in tenant catalog",
        "security": [{ "bearerAuth": [] }],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "required": ["product_name", "quantity", "unit_price"],
                "properties": {
                  "product_name": { "type": "string", "example": "Precision Component X" },
                  "quantity": { "type": "integer", "example": 100 },
                  "unit_price": { "type": "number", "example": 45.50 }
                }
              }
            }
          }
        },
        "responses": {
          "200": { "description": "Product added" }
        }
      },
      "get": {
        "tags": ["Inventory"],
        "summary": "List all products in inventory",
        "security": [{ "bearerAuth": [] }],
        "responses": {
          "200": { "description": "Product catalog" }
        }
      }
    },
    "/api/v1/invoices": {
      "post": {
        "tags": ["Invoices"],
        "summary": "Transactionally create an invoice (coordinated with Rust Engine)",
        "security": [{ "bearerAuth": [] }],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "required": ["customer_id", "lines"],
                "properties": {
                  "customer_id": { "type": "string", "example": "cust_001" },
                  "total_discount": { "type": "number", "example": 50.00 },
                  "packaging": { "type": "number", "example": 20.00 },
                  "freight": { "type": "number", "example": 30.00 },
                  "tax_collected_at_source": { "type": "number", "example": 0.00 },
                  "round_off": { "type": "number", "example": 0.00 },
                  "method_of_payment": { "type": "string", "example": "Bank Transfer" },
                  "lines": {
                    "type": "array",
                    "items": {
                      "type": "object",
                      "required": ["product_id", "quantity"],
                      "properties": {
                        "product_id": { "type": "integer", "example": 1 },
                        "quantity": { "type": "integer", "example": 10 }
                      }
                    }
                  }
                }
              }
            }
          }
        },
        "responses": {
          "201": { "description": "Invoice created successfully" }
        }
      },
      "get": {
        "tags": ["Invoices"],
        "summary": "List invoices matching temporal action filters",
        "security": [{ "bearerAuth": [] }],
        "parameters": [
          {
            "name": "action",
            "in": "query",
            "schema": {
              "type": "string",
              "enum": ["today", "thisMonth", "thisWeek", "thisQuarter", "thisYear", "toAndFromDate", "all"],
              "default": "all"
            }
          },
          { "name": "sdate", "in": "query", "schema": { "type": "string" } },
          { "name": "endate", "in": "query", "schema": { "type": "string" } }
        ],
        "responses": {
          "200": { "description": "Invoices list" }
        }
      }
    },
    "/api/v1/statements": {
      "get": {
        "tags": ["Statements & Analytics"],
        "summary": "Generate financial statement ledger report",
        "security": [{ "bearerAuth": [] }],
        "parameters": [
          {
            "name": "action",
            "in": "query",
            "schema": {
              "type": "string",
              "enum": ["today", "thisMonth", "thisWeek", "thisQuarter", "thisYear", "toAndFromDate", "all"],
              "default": "thisMonth"
            }
          }
        ],
        "responses": {
          "200": { "description": "Statement summary and ledger entries" }
        }
      }
    },
    "/api/v1/analytics": {
      "get": {
        "tags": ["Statements & Analytics"],
        "summary": "Get item sales analytics points for charting",
        "security": [{ "bearerAuth": [] }],
        "responses": {
          "200": { "description": "Analytics data points" }
        }
      }
    }
  }
}`

// ServeSwaggerUI serves an interactive Swagger UI web interface.
func ServeSwaggerUI(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Accounting Microservices - Swagger UI</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.11.0/swagger-ui.css" />
  <style>
    body { margin: 0; background: #fafafa; }
    .topbar { display: none; }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5.11.0/swagger-ui-bundle.js"></script>
  <script>
    window.onload = function() {
      SwaggerUIBundle({
        url: "/docs/swagger.json",
        dom_id: '#swagger-ui',
        deepLinking: true,
        presets: [
          SwaggerUIBundle.presets.apis,
          SwaggerUIBundle.SwaggerUIStandalonePreset
        ],
        layout: "BaseLayout"
      });
    };
  </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, html)
}

// ServeSwaggerJSON serves the OpenAPI specification JSON.
func ServeSwaggerJSON(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, SwaggerJSON)
}
