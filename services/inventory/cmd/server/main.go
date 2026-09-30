package main

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	inventoryv1 "github.com/accounting-microservices/gen/go/inventory/v1"
	"github.com/accounting-microservices/services/inventory/internal/repository"
	"github.com/accounting-microservices/services/inventory/internal/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	port := os.Getenv("INVENTORY_PORT")
	if port == "" {
		port = "50053"
	}

	dbURL := os.Getenv("INVENTORY_DATABASE_URL")
	var invRepo repository.InventoryRepository

	if dbURL != "" {
		db, err := sql.Open("pgx", dbURL)
		if err != nil {
			log.Fatalf("Failed to open inventory database: %v", err)
		}
		defer db.Close()

		if err := db.Ping(); err != nil {
			log.Printf("Warning: Inventory DB ping failed (%v), falling back to in-memory store", err)
			invRepo = repository.NewMemoryInventoryRepository()
		} else {
			log.Println("Connected to PostgreSQL for Inventory Service")
			invRepo = repository.NewPostgresInventoryRepository(db)
		}
	} else {
		log.Println("INVENTORY_DATABASE_URL not set, using thread-safe in-memory inventory repository")
		invRepo = repository.NewMemoryInventoryRepository()
	}

	inventoryServer := service.NewInventoryServiceServer(invRepo)

	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatalf("Failed to listen on port %s: %v", port, err)
	}

	grpcServer := grpc.NewServer()
	inventoryv1.RegisterInventoryServiceServer(grpcServer, inventoryServer)
	reflection.Register(grpcServer)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Shutting down Inventory gRPC server...")
		grpcServer.GracefulStop()
	}()

	log.Printf("Inventory Service gRPC server listening on :%s", port)
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Inventory gRPC serve error: %v", err)
	}
}
