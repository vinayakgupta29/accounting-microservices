package main

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	authv1 "github.com/accounting-microservices/gen/go/auth/v1"
	"github.com/accounting-microservices/services/auth/internal/repository"
	"github.com/accounting-microservices/services/auth/internal/service"
	"github.com/accounting-microservices/services/auth/internal/token"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	port := os.Getenv("AUTH_PORT")
	if port == "" {
		port = "50051"
	}

	secretKey := os.Getenv("JWT_SECRET")
	if secretKey == "" {
		secretKey = "super-secret-jwt-key-for-accounting-microservices"
	}

	dbURL := os.Getenv("AUTH_DATABASE_URL")
	var userRepo repository.UserRepository

	if dbURL != "" {
		db, err := sql.Open("pgx", dbURL)
		if err != nil {
			log.Fatalf("Failed to open database: %v", err)
		}
		defer db.Close()

		if err := db.Ping(); err != nil {
			log.Printf("Warning: Database ping failed (%v), falling back to in-memory store", err)
			userRepo = repository.NewMemoryUserRepository()
		} else {
			log.Println("Connected to PostgreSQL database for Auth Service")
			userRepo = repository.NewPostgresUserRepository(db)
		}
	} else {
		log.Println("AUTH_DATABASE_URL not set, using thread-safe in-memory repository")
		userRepo = repository.NewMemoryUserRepository()
	}

	tokenManager := token.NewManager(secretKey, 24*time.Hour)
	authServer := service.NewAuthServiceServer(userRepo, tokenManager)

	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatalf("Failed to listen on port %s: %v", port, err)
	}

	grpcServer := grpc.NewServer()
	authv1.RegisterAuthServiceServer(grpcServer, authServer)
	reflection.Register(grpcServer)

	// Graceful shutdown handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Shutting down Auth gRPC server...")
		grpcServer.GracefulStop()
	}()

	log.Printf("Auth Service gRPC server listening on :%s", port)
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("gRPC serve error: %v", err)
	}
}
