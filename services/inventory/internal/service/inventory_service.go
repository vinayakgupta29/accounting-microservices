package service

import (
	"context"
	"errors"
	"strings"

	inventoryv1 "github.com/accounting-microservices/gen/go/inventory/v1"
	"github.com/accounting-microservices/services/inventory/internal/repository"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type InventoryServiceServer struct {
	inventoryv1.UnimplementedInventoryServiceServer
	repo repository.InventoryRepository
}

func NewInventoryServiceServer(repo repository.InventoryRepository) *InventoryServiceServer {
	return &InventoryServiceServer{repo: repo}
}

func protoFromProduct(p *repository.Product) *inventoryv1.Product {
	return &inventoryv1.Product{
		Id:          p.ID,
		TenantId:    p.TenantID,
		ProductName: p.ProductName,
		Quantity:    p.Quantity,
		UnitPrice:   p.UnitPrice,
		CreatedAt:   p.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:   p.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func (s *InventoryServiceServer) AddProduct(ctx context.Context, req *inventoryv1.AddProductRequest) (*inventoryv1.AddProductResponse, error) {
	tenantID := strings.TrimSpace(req.GetTenantId())
	if tenantID == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant_id is required")
	}

	productName := strings.TrimSpace(req.GetProductName())
	if productName == "" {
		return nil, status.Error(codes.InvalidArgument, "product_name is required")
	}

	p := &repository.Product{
		TenantID:    tenantID,
		ProductName: productName,
		Quantity:    req.GetQuantity(),
		UnitPrice:   req.GetUnitPrice(),
	}

	if err := s.repo.Add(ctx, p); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to add product: %v", err)
	}

	return &inventoryv1.AddProductResponse{
		Product: protoFromProduct(p),
		Message: "Product added successfully",
	}, nil
}

func (s *InventoryServiceServer) GetProduct(ctx context.Context, req *inventoryv1.GetProductRequest) (*inventoryv1.Product, error) {
	tenantID := strings.TrimSpace(req.GetTenantId())
	if tenantID == "" || req.GetProductId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "tenant_id and product_id are required")
	}

	p, err := s.repo.FindByID(ctx, tenantID, req.GetProductId())
	if err != nil {
		if errors.Is(err, repository.ErrProductNotFound) {
			return nil, status.Error(codes.NotFound, "product not found")
		}
		return nil, status.Errorf(codes.Internal, "database error: %v", err)
	}

	return protoFromProduct(p), nil
}

func (s *InventoryServiceServer) ListProducts(ctx context.Context, req *inventoryv1.ListProductsRequest) (*inventoryv1.ListProductsResponse, error) {
	tenantID := strings.TrimSpace(req.GetTenantId())
	if tenantID == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant_id is required")
	}

	products, err := s.repo.List(ctx, tenantID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list products: %v", err)
	}

	var protoList []*inventoryv1.Product
	for _, p := range products {
		protoList = append(protoList, protoFromProduct(p))
	}

	return &inventoryv1.ListProductsResponse{
		Products: protoList,
	}, nil
}

func (s *InventoryServiceServer) ReserveStock(ctx context.Context, req *inventoryv1.ReserveStockRequest) (*inventoryv1.ReserveStockResponse, error) {
	tenantID := strings.TrimSpace(req.GetTenantId())
	resID := strings.TrimSpace(req.GetReservationId())

	if tenantID == "" || resID == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant_id and reservation_id are required")
	}

	if len(req.GetItems()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "at least one stock item is required")
	}

	var items []repository.ReservationItem
	for _, it := range req.GetItems() {
		items = append(items, repository.ReservationItem{
			ProductID: it.GetProductId(),
			Quantity:  it.GetQuantity(),
		})
	}

	reserved, err := s.repo.ReserveStock(ctx, tenantID, resID, items)
	if err != nil {
		if errors.Is(err, repository.ErrInsufficientStock) {
			return &inventoryv1.ReserveStockResponse{
				Success:       false,
				ReservationId: resID,
				Message:       "Insufficient stock available for requested quantity",
			}, nil
		}
		if errors.Is(err, repository.ErrProductNotFound) {
			return nil, status.Error(codes.NotFound, "one or more products not found in inventory")
		}
		return nil, status.Errorf(codes.Internal, "reservation failed: %v", err)
	}

	var protoProducts []*inventoryv1.Product
	for _, p := range reserved {
		protoProducts = append(protoProducts, protoFromProduct(p))
	}

	return &inventoryv1.ReserveStockResponse{
		Success:       true,
		ReservationId: resID,
		Message:       "Stock reserved successfully",
		Products:      protoProducts,
	}, nil
}

func (s *InventoryServiceServer) CommitStock(ctx context.Context, req *inventoryv1.CommitStockRequest) (*inventoryv1.CommitStockResponse, error) {
	tenantID := strings.TrimSpace(req.GetTenantId())
	resID := strings.TrimSpace(req.GetReservationId())

	if err := s.repo.CommitStock(ctx, tenantID, resID); err != nil {
		if errors.Is(err, repository.ErrReservationNotFound) {
			return nil, status.Error(codes.NotFound, "reservation not found or already processed")
		}
		return nil, status.Errorf(codes.Internal, "commit failed: %v", err)
	}

	return &inventoryv1.CommitStockResponse{
		Success: true,
		Message: "Stock reservation committed",
	}, nil
}

func (s *InventoryServiceServer) ReleaseStock(ctx context.Context, req *inventoryv1.ReleaseStockRequest) (*inventoryv1.ReleaseStockResponse, error) {
	tenantID := strings.TrimSpace(req.GetTenantId())
	resID := strings.TrimSpace(req.GetReservationId())

	if err := s.repo.ReleaseStock(ctx, tenantID, resID); err != nil {
		if errors.Is(err, repository.ErrReservationNotFound) {
			return nil, status.Error(codes.NotFound, "reservation not found or already processed")
		}
		return nil, status.Errorf(codes.Internal, "release failed: %v", err)
	}

	return &inventoryv1.ReleaseStockResponse{
		Success: true,
		Message: "Stock reservation released",
	}, nil
}
