package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	invoicev1 "github.com/accounting-microservices/gen/go/invoice/v1"
	statementv1 "github.com/accounting-microservices/gen/go/statement/v1"
	"github.com/accounting-microservices/services/statement/internal/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

func main() {
	port := os.Getenv("STATEMENT_PORT")
	if port == "" {
		port = "50055"
	}

	invoiceURL := os.Getenv("INVOICE_SERVICE_URL")
	if invoiceURL == "" {
		invoiceURL = "localhost:50054"
	}

	var invoiceClient invoicev1.InvoiceServiceClient
	conn, err := grpc.NewClient(invoiceURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("Warning: Failed to connect to Invoicing Service (%v), running in standalone mode", err)
	} else {
		defer conn.Close()
		invoiceClient = invoicev1.NewInvoiceServiceClient(conn)
		log.Printf("Statement Service connected to Invoicing Service at %s", invoiceURL)
	}

	statementServer := service.NewStatementServiceServer(invoiceClient)

	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatalf("Failed to listen on port %s: %v", port, err)
	}

	grpcServer := grpc.NewServer()
	statementv1.RegisterStatementServiceServer(grpcServer, statementServer)
	reflection.Register(grpcServer)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Shutting down Statement gRPC server...")
		grpcServer.GracefulStop()
	}()

	log.Printf("Statement & Analytics Service gRPC server listening on :%s", port)
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Statement gRPC serve error: %v", err)
	}
}
