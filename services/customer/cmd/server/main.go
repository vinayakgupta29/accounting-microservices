package main

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	customerv1 "github.com/accounting-microservices/gen/go/customer/v1"
	"github.com/accounting-microservices/services/customer/internal/repository"
	"github.com/accounting-microservices/services/customer/internal/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	port := os.Getenv("CUSTOMER_PORT")
	if port == "" {
		port = "50052"
	}

	dbURL := os.Getenv("CUSTOMER_DATABASE_URL")
	var custRepo repository.CustomerRepository

	if dbURL != "" {
		db, err := sql.Open("pgx", dbURL)
		if err != nil {
			log.Fatalf("Failed to open database: %v", err)
		}
		defer db.Close()

		if err := db.Ping(); err != nil {
			log.Printf("Warning: Customer DB ping failed (%v), falling back to in-memory store", err)
			custRepo = repository.NewMemoryCustomerRepository()
		} else {
			log.Println("Connected to PostgreSQL for Customer Service")
			custRepo = repository.NewPostgresCustomerRepository(db)
		}
	} else {
		log.Println("CUSTOMER_DATABASE_URL not set, using thread-safe in-memory customer repository")
		custRepo = repository.NewMemoryCustomerRepository()
	}

	customerServer := service.NewCustomerServiceServer(custRepo)

	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatalf("Failed to listen on port %s: %v", port, err)
	}

	grpcServer := grpc.NewServer()
	customerv1.RegisterCustomerServiceServer(grpcServer, customerServer)
	reflection.Register(grpcServer)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Shutting down Customer gRPC server...")
		grpcServer.GracefulStop()
	}()

	log.Printf("Customer Service gRPC server listening on :%s", port)
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Customer gRPC serve error: %v", err)
	}
}
