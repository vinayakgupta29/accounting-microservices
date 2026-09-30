package service

import (
	"context"
	"errors"
	"strings"

	customerv1 "github.com/accounting-microservices/gen/go/customer/v1"
	"github.com/accounting-microservices/services/customer/internal/repository"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// CustomerServiceServer implements the customer.v1.CustomerServiceServer interface.
type CustomerServiceServer struct {
	customerv1.UnimplementedCustomerServiceServer
	repo repository.CustomerRepository
}

// NewCustomerServiceServer creates a new CustomerServiceServer.
func NewCustomerServiceServer(repo repository.CustomerRepository) *CustomerServiceServer {
	return &CustomerServiceServer{repo: repo}
}

func protoFromCustomer(c *repository.Customer) *customerv1.Customer {
	return &customerv1.Customer{
		Id:          c.ID,
		TenantId:    c.TenantID,
		CustId:      c.CustID,
		Name:        c.Name,
		Address:     c.Address,
		PhoneNumber: c.PhoneNumber,
		Email:       c.Email,
		Gstin:       c.GSTIN,
		DealerType:  c.DealerType,
		PanCard:     c.PANCard,
		Aadhaar:     c.Aadhaar,
		CreatedAt:   c.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// CreateCustomer creates and assigns an incremental cust_XXX identifier.
func (s *CustomerServiceServer) CreateCustomer(ctx context.Context, req *customerv1.CreateCustomerRequest) (*customerv1.CreateCustomerResponse, error) {
	tenantID := strings.TrimSpace(req.GetTenantId())
	if tenantID == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant_id is required")
	}

	name := strings.TrimSpace(req.GetName())
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "customer name is required")
	}

	custID, err := s.repo.NextCustomerID(ctx, tenantID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to generate customer ID: %v", err)
	}

	cust := &repository.Customer{
		TenantID:    tenantID,
		CustID:      custID,
		Name:        name,
		Address:     strings.TrimSpace(req.GetAddress()),
		PhoneNumber: strings.TrimSpace(req.GetPhoneNumber()),
		Email:       strings.TrimSpace(req.GetEmail()),
		GSTIN:       strings.TrimSpace(req.GetGstin()),
		DealerType:  strings.TrimSpace(req.GetDealerType()),
		PANCard:     strings.TrimSpace(req.GetPanCard()),
		Aadhaar:     strings.TrimSpace(req.GetAadhaar()),
	}

	if cust.DealerType == "" {
		cust.DealerType = "Regular"
	}

	if err := s.repo.Create(ctx, cust); err != nil {
		if errors.Is(err, repository.ErrGSTINExists) {
			return nil, status.Error(codes.AlreadyExists, "GSTIN already exists for this tenant")
		}
		return nil, status.Errorf(codes.Internal, "failed to insert customer: %v", err)
	}

	return &customerv1.CreateCustomerResponse{
		Customer: protoFromCustomer(cust),
		Message:  "Customer created successfully",
	}, nil
}

// GetCustomer retrieves a single customer by cust_id.
func (s *CustomerServiceServer) GetCustomer(ctx context.Context, req *customerv1.GetCustomerRequest) (*customerv1.Customer, error) {
	tenantID := strings.TrimSpace(req.GetTenantId())
	custID := strings.TrimSpace(req.GetCustId())

	if tenantID == "" || custID == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant_id and cust_id are required")
	}

	cust, err := s.repo.FindByCustID(ctx, tenantID, custID)
	if err != nil {
		if errors.Is(err, repository.ErrCustomerNotFound) {
			return nil, status.Error(codes.NotFound, "customer not found")
		}
		return nil, status.Errorf(codes.Internal, "database error: %v", err)
	}

	return protoFromCustomer(cust), nil
}

// ListCustomers returns paginated customers for a tenant.
func (s *CustomerServiceServer) ListCustomers(ctx context.Context, req *customerv1.ListCustomersRequest) (*customerv1.ListCustomersResponse, error) {
	tenantID := strings.TrimSpace(req.GetTenantId())
	if tenantID == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant_id is required")
	}

	limit := int(req.GetLimit())
	if limit <= 0 {
		limit = 100
	}
	page := int(req.GetPage())
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit

	list, total, err := s.repo.List(ctx, tenantID, limit, offset)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list customers: %v", err)
	}

	var protoList []*customerv1.Customer
	for _, c := range list {
		protoList = append(protoList, protoFromCustomer(c))
	}

	return &customerv1.ListCustomersResponse{
		Customers:  protoList,
		TotalCount: total,
	}, nil
}

// VerifyCustomer validates existence of customer for invoicing sagas.
func (s *CustomerServiceServer) VerifyCustomer(ctx context.Context, req *customerv1.VerifyCustomerRequest) (*customerv1.VerifyCustomerResponse, error) {
	tenantID := strings.TrimSpace(req.GetTenantId())
	custID := strings.TrimSpace(req.GetCustId())

	cust, err := s.repo.FindByCustID(ctx, tenantID, custID)
	if err != nil {
		return &customerv1.VerifyCustomerResponse{
			Exists:   false,
			Customer: nil,
		}, nil
	}

	return &customerv1.VerifyCustomerResponse{
		Exists:   true,
		Customer: protoFromCustomer(cust),
	}, nil
}
