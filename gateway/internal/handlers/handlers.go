package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	authv1 "github.com/accounting-microservices/gen/go/auth/v1"
	customerv1 "github.com/accounting-microservices/gen/go/customer/v1"
	inventoryv1 "github.com/accounting-microservices/gen/go/inventory/v1"
	invoicev1 "github.com/accounting-microservices/gen/go/invoice/v1"
	statementv1 "github.com/accounting-microservices/gen/go/statement/v1"
	"github.com/accounting-microservices/gateway/internal/middleware"
)

type GatewayHandlers struct {
	AuthClient      authv1.AuthServiceClient
	CustomerClient  customerv1.CustomerServiceClient
	InventoryClient inventoryv1.InventoryServiceClient
	InvoiceClient   invoicev1.InvoiceServiceClient
	StatementClient statementv1.StatementServiceClient
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// ---------------- AUTH HANDLERS ----------------

type SignupRequest struct {
	Name     string `json:"name"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
	GSTIN    string `json:"gstin"`
	PAN      string `json:"pan"`
	Aadhaar  string `json:"aadhaar"`
	Phone    string `json:"phone"`
	Address  string `json:"address"`
}

func (h *GatewayHandlers) HandleSignup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req SignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	resp, err := h.AuthClient.Register(r.Context(), &authv1.RegisterRequest{
		Name:     req.Name,
		Username: req.Username,
		Email:    req.Email,
		Password: req.Password,
		Gstin:    req.GSTIN,
		Pan:      req.PAN,
		Aadhaar:  req.Aadhaar,
		Phone:    req.Phone,
		Address:  req.Address,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, resp)
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *GatewayHandlers) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	resp, err := h.AuthClient.Login(r.Context(), &authv1.LoginRequest{
		Username: req.Username,
		Password: req.Password,
	})
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Invalid username or password")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *GatewayHandlers) HandleProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	tenantID := middleware.GetTenantID(r.Context())
	username := middleware.GetUsername(r.Context())

	profile, err := h.AuthClient.GetUserProfile(r.Context(), &authv1.GetUserProfileRequest{
		UserId:   tenantID,
		Username: username,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "User profile not found")
		return
	}

	writeJSON(w, http.StatusOK, profile)
}

// ---------------- CUSTOMER HANDLERS ----------------

type CreateCustomerRequest struct {
	TenantID    string `json:"tenant_id"`
	Name        string `json:"name"`
	Address     string `json:"address"`
	PhoneNumber string `json:"phone_number"`
	Email       string `json:"email"`
	GSTIN       string `json:"gstin"`
	DealerType  string `json:"dealer_type"`
	PANCard     string `json:"pan_card"`
	Aadhaar     string `json:"aadhaar"`
}

func (h *GatewayHandlers) HandleCustomers(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())

	switch r.Method {
	case http.MethodPost:
		var req CreateCustomerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid JSON payload")
			return
		}
		if req.TenantID == "" {
			req.TenantID = tenantID
		}
		if req.TenantID == "" {
			req.TenantID = "default_tenant"
		}

		resp, err := h.CustomerClient.CreateCustomer(r.Context(), &customerv1.CreateCustomerRequest{
			TenantId:    req.TenantID,
			Name:        req.Name,
			Address:     req.Address,
			PhoneNumber: req.PhoneNumber,
			Email:       req.Email,
			Gstin:       req.GSTIN,
			DealerType:  req.DealerType,
			PanCard:     req.PANCard,
			Aadhaar:     req.Aadhaar,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, resp)

	case http.MethodGet:
		pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(pathParts) == 4 && pathParts[3] != "" {
			// Single customer GET /api/v1/customers/{cust_id}
			custID := pathParts[3]
			cust, err := h.CustomerClient.GetCustomer(r.Context(), &customerv1.GetCustomerRequest{
				TenantId: tenantID,
				CustId:   custID,
			})
			if err != nil {
				writeError(w, http.StatusNotFound, "Customer not found")
				return
			}
			writeJSON(w, http.StatusOK, cust)
			return
		}

		// List customers
		reqTenant := r.URL.Query().Get("tenant_id")
		if reqTenant == "" {
			reqTenant = tenantID
		}
		if reqTenant == "" {
			reqTenant = r.URL.Query().Get("username")
		}
		if reqTenant == "" {
			reqTenant = "default_tenant"
		}

		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

		resp, err := h.CustomerClient.ListCustomers(r.Context(), &customerv1.ListCustomersRequest{
			TenantId: reqTenant,
			Page:     int32(page),
			Limit:    int32(limit),
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, resp)

	default:
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// ---------------- INVENTORY HANDLERS ----------------

type AddProductRequest struct {
	TenantID    string  `json:"tenant_id"`
	ProductName string  `json:"product_name"`
	Quantity    int32   `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
}

func (h *GatewayHandlers) HandleInventory(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())

	switch r.Method {
	case http.MethodPost:
		var req AddProductRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid JSON payload")
			return
		}
		if req.TenantID == "" {
			req.TenantID = tenantID
		}
		if req.TenantID == "" {
			req.TenantID = "default_tenant"
		}

		resp, err := h.InventoryClient.AddProduct(r.Context(), &inventoryv1.AddProductRequest{
			TenantId:    req.TenantID,
			ProductName: req.ProductName,
			Quantity:    req.Quantity,
			UnitPrice:   req.UnitPrice,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, resp)

	case http.MethodGet:
		pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(pathParts) == 4 && pathParts[3] != "" {
			// Single product GET /api/v1/inventory/{id}
			prodID, err := strconv.ParseInt(pathParts[3], 10, 64)
			if err != nil {
				writeError(w, http.StatusBadRequest, "Invalid product ID")
				return
			}
			prod, err := h.InventoryClient.GetProduct(r.Context(), &inventoryv1.GetProductRequest{
				TenantId:  tenantID,
				ProductId: prodID,
			})
			if err != nil {
				writeError(w, http.StatusNotFound, "Product not found")
				return
			}
			writeJSON(w, http.StatusOK, prod)
			return
		}

		reqTenant := r.URL.Query().Get("tenant_id")
		if reqTenant == "" {
			reqTenant = tenantID
		}
		if reqTenant == "" {
			reqTenant = r.URL.Query().Get("username")
		}
		if reqTenant == "" {
			reqTenant = "default_tenant"
		}

		resp, err := h.InventoryClient.ListProducts(r.Context(), &inventoryv1.ListProductsRequest{
			TenantId: reqTenant,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, resp)

	default:
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// ---------------- INVOICE HANDLERS ----------------

type InvoiceLineDTO struct {
	ProductID int64 `json:"product_id"`
	Quantity  int32 `json:"quantity"`
}

type CreateInvoiceRequestDTO struct {
	TenantID             string           `json:"tenant_id"`
	CustomerID           string           `json:"customer_id"`
	DateTime             string           `json:"date_time"`
	TotalDiscount        float64          `json:"total_discount"`
	Packaging            float64          `json:"packaging"`
	Freight              float64          `json:"freight"`
	TaxCollectedAtSource float64          `json:"tax_collected_at_source"`
	RoundOff             float64          `json:"round_off"`
	MethodOfPayment      string           `json:"method_of_payment"`
	Lines                []InvoiceLineDTO `json:"lines"`
}

func (h *GatewayHandlers) HandleInvoices(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())

	switch r.Method {
	case http.MethodPost:
		var req CreateInvoiceRequestDTO
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid JSON payload")
			return
		}
		if req.TenantID == "" {
			req.TenantID = tenantID
		}
		if req.TenantID == "" {
			req.TenantID = "default_tenant"
		}

		var lines []*invoicev1.InvoiceLineInput
		for _, l := range req.Lines {
			lines = append(lines, &invoicev1.InvoiceLineInput{
				ProductId: l.ProductID,
				Quantity:  l.Quantity,
			})
		}

		resp, err := h.InvoiceClient.CreateInvoice(r.Context(), &invoicev1.CreateInvoiceRequest{
			TenantId:             req.TenantID,
			CustomerId:           req.CustomerID,
			DateTime:             req.DateTime,
			TotalDiscount:        req.TotalDiscount,
			Packaging:            req.Packaging,
			Freight:              req.Freight,
			TaxCollectedAtSource: req.TaxCollectedAtSource,
			RoundOff:             req.RoundOff,
			MethodOfPayment:      req.MethodOfPayment,
			Lines:                lines,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, resp)

	case http.MethodGet:
		pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(pathParts) == 4 && pathParts[3] != "" {
			// Single invoice GET /api/v1/invoices/{txn_id}
			txnID := pathParts[3]
			inv, err := h.InvoiceClient.GetInvoice(r.Context(), &invoicev1.GetInvoiceRequest{
				TenantId:      tenantID,
				TransactionId: txnID,
			})
			if err != nil {
				writeError(w, http.StatusNotFound, "Invoice not found")
				return
			}
			writeJSON(w, http.StatusOK, inv)
			return
		}

		reqTenant := r.URL.Query().Get("tenant_id")
		if reqTenant == "" {
			reqTenant = tenantID
		}
		if reqTenant == "" {
			reqTenant = r.URL.Query().Get("username")
		}
		if reqTenant == "" {
			reqTenant = "default_tenant"
		}

		action := r.URL.Query().Get("action")
		if action == "" {
			action = "all"
		}

		resp, err := h.InvoiceClient.ListInvoices(r.Context(), &invoicev1.ListInvoicesRequest{
			TenantId:  reqTenant,
			Action:    action,
			StartDate: r.URL.Query().Get("sdate"),
			EndDate:   r.URL.Query().Get("endate"),
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, resp)

	default:
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// ---------------- STATEMENT & ANALYTICS HANDLERS ----------------

func (h *GatewayHandlers) HandleStatements(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	tenantID := middleware.GetTenantID(r.Context())
	reqTenant := r.URL.Query().Get("tenant_id")
	if reqTenant == "" {
		reqTenant = tenantID
	}
	if reqTenant == "" {
		reqTenant = r.URL.Query().Get("username")
	}
	if reqTenant == "" {
		reqTenant = "default_tenant"
	}

	action := r.URL.Query().Get("action")
	if action == "" {
		action = "all"
	}

	resp, err := h.StatementClient.GenerateStatement(r.Context(), &statementv1.GenerateStatementRequest{
		TenantId:  reqTenant,
		Action:    action,
		StartDate: r.URL.Query().Get("sdate"),
		EndDate:   r.URL.Query().Get("endate"),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *GatewayHandlers) HandleAnalytics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	tenantID := middleware.GetTenantID(r.Context())
	reqTenant := r.URL.Query().Get("tenant_id")
	if reqTenant == "" {
		reqTenant = tenantID
	}
	if reqTenant == "" {
		reqTenant = r.URL.Query().Get("username")
	}
	if reqTenant == "" {
		reqTenant = "default_tenant"
	}

	resp, err := h.StatementClient.GetAnalytics(r.Context(), &statementv1.AnalyticsRequest{
		TenantId:  reqTenant,
		StartDate: r.URL.Query().Get("sdate"),
		EndDate:   r.URL.Query().Get("endate"),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}
