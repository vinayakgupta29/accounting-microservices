package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	authv1 "github.com/accounting-microservices/gen/go/auth/v1"
	customerv1 "github.com/accounting-microservices/gen/go/customer/v1"
	inventoryv1 "github.com/accounting-microservices/gen/go/inventory/v1"
	invoicev1 "github.com/accounting-microservices/gen/go/invoice/v1"
	statementv1 "github.com/accounting-microservices/gen/go/statement/v1"
	"github.com/accounting-microservices/gateway/internal/docs"
	"github.com/accounting-microservices/gateway/internal/handlers"
	"github.com/accounting-microservices/gateway/internal/middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	authAddr := os.Getenv("AUTH_SERVICE_ADDR")
	if authAddr == "" {
		authAddr = "localhost:50051"
	}

	customerAddr := os.Getenv("CUSTOMER_SERVICE_ADDR")
	if customerAddr == "" {
		customerAddr = "localhost:50052"
	}

	inventoryAddr := os.Getenv("INVENTORY_SERVICE_ADDR")
	if inventoryAddr == "" {
		inventoryAddr = "localhost:50053"
	}

	invoiceAddr := os.Getenv("INVOICE_SERVICE_ADDR")
	if invoiceAddr == "" {
		invoiceAddr = "localhost:50054"
	}

	statementAddr := os.Getenv("STATEMENT_SERVICE_ADDR")
	if statementAddr == "" {
		statementAddr = "localhost:50055"
	}

	// Dial gRPC clients
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}

	authConn, err := grpc.NewClient(authAddr, opts...)
	if err != nil {
		log.Printf("Warning: Auth client dial failed: %v", err)
	} else {
		defer authConn.Close()
	}

	custConn, err := grpc.NewClient(customerAddr, opts...)
	if err != nil {
		log.Printf("Warning: Customer client dial failed: %v", err)
	} else {
		defer custConn.Close()
	}

	invConn, err := grpc.NewClient(inventoryAddr, opts...)
	if err != nil {
		log.Printf("Warning: Inventory client dial failed: %v", err)
	} else {
		defer invConn.Close()
	}

	invoiceConn, err := grpc.NewClient(invoiceAddr, opts...)
	if err != nil {
		log.Printf("Warning: Invoice client dial failed: %v", err)
	} else {
		defer invoiceConn.Close()
	}

	stmtConn, err := grpc.NewClient(statementAddr, opts...)
	if err != nil {
		log.Printf("Warning: Statement client dial failed: %v", err)
	} else {
		defer stmtConn.Close()
	}

	authClient := authv1.NewAuthServiceClient(authConn)
	customerClient := customerv1.NewCustomerServiceClient(custConn)
	inventoryClient := inventoryv1.NewInventoryServiceClient(invConn)
	invoiceClient := invoicev1.NewInvoiceServiceClient(invoiceConn)
	statementClient := statementv1.NewStatementServiceClient(stmtConn)

	h := &handlers.GatewayHandlers{
		AuthClient:      authClient,
		CustomerClient:  customerClient,
		InventoryClient: inventoryClient,
		InvoiceClient:   invoiceClient,
		StatementClient: statementClient,
	}

	mux := http.NewServeMux()

	// Swagger documentation routes
	mux.HandleFunc("/docs", docs.ServeSwaggerUI)
	mux.HandleFunc("/docs/", docs.ServeSwaggerUI)
	mux.HandleFunc("/docs/swagger.json", docs.ServeSwaggerJSON)

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","gateway":"healthy","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`))
	})

	// Public Auth endpoints
	mux.HandleFunc("/api/v1/auth/signup", h.HandleSignup)
	mux.HandleFunc("/api/v1/auth/login", h.HandleLogin)

	// Protected routes (support both direct token authorization or default tenant query)
	authGuard := middleware.AuthMiddleware(authClient)

	mux.Handle("/api/v1/auth/profile", authGuard(http.HandlerFunc(h.HandleProfile)))

	// For Customer, Inventory, Invoices, Statements: allow authenticated requests or fallback tenant param
	mux.HandleFunc("/api/v1/customers", h.HandleCustomers)
	mux.HandleFunc("/api/v1/customers/", h.HandleCustomers)

	mux.HandleFunc("/api/v1/inventory", h.HandleInventory)
	mux.HandleFunc("/api/v1/inventory/", h.HandleInventory)

	mux.HandleFunc("/api/v1/invoices", h.HandleInvoices)
	mux.HandleFunc("/api/v1/invoices/", h.HandleInvoices)

	mux.HandleFunc("/api/v1/statements", h.HandleStatements)
	mux.HandleFunc("/api/v1/analytics", h.HandleAnalytics)

	// Apply CORS and Gzip Compression
	handler := corsMiddleware(middleware.GzipMiddleware(mux))

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutting down API Gateway HTTP server...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()

	log.Printf("Accounting Microservices API Gateway running on http://localhost:%s", port)
	log.Printf("Interactive Swagger UI Documentation available at http://localhost:%s/docs", port)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Gateway server error: %v", err)
	}
}
