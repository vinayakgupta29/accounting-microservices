.PHONY: proto build run run-docker docker-up docker-down clean clean-all docker-clean docker-clean-all help

# Default target displays available commands
help:
	@echo "=========================================================================="
	@echo "                   ACCOUNTING MICROSERVICES CLI                           "
	@echo "=========================================================================="
	@echo " Orchestration & Execution:"
	@echo "   make run              - Run ALL microservices and API Gateway simultaneously"
	@echo "   make docker-run       - Run entire stack with Docker Compose"
	@echo "   make docker-down      - Stop all Docker Compose containers"
	@echo ""
	@echo " Build & Code Generation:"
	@echo "   make build            - Compile Protobuf, all Go services & Rust engine"
	@echo "   make proto            - Regenerate Protocol Buffer stubs for Go & Rust"
	@echo ""
	@echo " Cleaning Targets:"
	@echo "   make clean            - Clean all compiled binaries, executables & Rust target"
	@echo "   make docker-clean     - Remove Docker containers, networks, and wipe DB volumes"
	@echo "   make docker-clean-all - Remove containers, volumes, DB, and DELETE all images"
	@echo "   make clean-all        - Full reset: cleans all local builds AND all Docker images"
	@echo "=========================================================================="

# Generate Protobuf stubs for Go and Rust
proto:
	@echo "--> Compiling Protocol Buffer contracts..."
	@mkdir -p gen/go
	@PATH="$(HOME)/go/bin:$(PATH)" protoc \
		--proto_path=proto \
		--go_out=gen/go --go_opt=paths=source_relative \
		--go-grpc_out=gen/go --go-grpc_opt=paths=source_relative \
		proto/auth/v1/auth.proto \
		proto/customer/v1/customer.proto \
		proto/inventory/v1/inventory.proto \
		proto/invoice/v1/invoice.proto \
		proto/statement/v1/statement.proto
	@echo "--> Protobuf stubs generated successfully."

# Compile all Go microservices and Rust Invoicing Engine
build: proto
	@echo "--> Compiling Go microservices and API Gateway..."
	@mkdir -p bin
	@go build -o bin/auth-service ./services/auth/cmd/server
	@go build -o bin/customer-service ./services/customer/cmd/server
	@go build -o bin/inventory-service ./services/inventory/cmd/server
	@go build -o bin/statement-service ./services/statement/cmd/server
	@go build -o bin/api-gateway ./gateway/cmd/server
	@echo "--> Compiling Rust Invoicing Engine (tonic + rust_decimal)..."
	@cargo build --manifest-path services/invoicing/Cargo.toml
	@echo "--> All microservices compiled successfully into bin/."

# Parent command to run ALL services simultaneously with automated process management
run: build
	@echo "=========================================================================="
	@echo " Starting all microservices mesh locally..."
	@echo "   [Auth Service]        :50051 (gRPC)"
	@echo "   [Customer Service]    :50052 (gRPC)"
	@echo "   [Inventory Service]   :50053 (gRPC)"
	@echo "   [Invoicing Engine]    :50054 (Rust gRPC)"
	@echo "   [Statement Service]   :50055 (gRPC)"
	@echo "   [API Gateway]         http://localhost:8080"
	@echo "   [Swagger UI Docs]     http://localhost:8080/docs"
	@echo " Press CTRL+C to gracefully stop all services."
	@echo "=========================================================================="
	@bash -c '\
		trap "echo -e \"\n--> Terminating all microservice processes...\"; kill 0" SIGINT SIGTERM EXIT; \
		AUTH_PORT=50051 ./bin/auth-service & \
		CUSTOMER_PORT=50052 ./bin/customer-service & \
		INVENTORY_PORT=50053 ./bin/inventory-service & \
		INVOICING_PORT=50054 ./services/invoicing/target/debug/invoicing-service & \
		STATEMENT_PORT=50055 ./bin/statement-service & \
		sleep 1; \
		PORT=8080 ./bin/api-gateway'

# Docker Compose execution
docker-run:
	@echo "--> Launching all microservices and PostgreSQL with Docker Compose..."
	docker compose up --build -d
	@echo "--> Stack running! Access Swagger UI at http://localhost:8080/docs"

docker-up: docker-run

docker-down:
	@echo "--> Stopping Docker containers..."
	docker compose down

# Clean all local build artifacts, executables, and cargo target files
clean:
	@echo "--> Cleaning compiled executables and build artifacts..."
	@rm -rf bin/
	@cargo clean --manifest-path services/invoicing/Cargo.toml
	@rm -f *.log services/*/*.log gateway/*.log
	@echo "--> Cleaned all local build artifacts."

# Clean Docker containers and wipe all PostgreSQL database contents (volumes)
docker-clean:
	@echo "--> Stopping containers and wiping all persistent database volumes..."
	docker compose down -v --remove-orphans
	@echo "--> Docker containers and database volumes wiped clean."

# Clean Docker containers, DB volumes, AND delete all composed and downloaded images
docker-clean-all:
	@echo "--> Stopping containers, wiping database volumes, and deleting all Docker images..."
	docker compose down -v --rmi all --remove-orphans
	@echo "--> All Docker containers, volumes, and images deleted."

# Full nuclear reset: cleans all local build artifacts AND all Docker containers/images/DBs
clean-all: clean docker-clean-all
	@echo "--> Complete project cleanup complete (all binaries, caches, DBs, and images removed)."
