package service

import (
	"context"
	"strings"

	invoicev1 "github.com/accounting-microservices/gen/go/invoice/v1"
	statementv1 "github.com/accounting-microservices/gen/go/statement/v1"
	"github.com/accounting-microservices/services/statement/internal/aggregator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type StatementServiceServer struct {
	statementv1.UnimplementedStatementServiceServer
	invoiceClient invoicev1.InvoiceServiceClient
}

func NewStatementServiceServer(invoiceClient invoicev1.InvoiceServiceClient) *StatementServiceServer {
	return &StatementServiceServer{
		invoiceClient: invoiceClient,
	}
}

func (s *StatementServiceServer) GenerateStatement(ctx context.Context, req *statementv1.GenerateStatementRequest) (*statementv1.StatementResponse, error) {
	tenantID := strings.TrimSpace(req.GetTenantId())
	if tenantID == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant_id is required")
	}

	action := req.GetAction()
	if action == "" {
		action = "all"
	}

	var matchedInvoices []*invoicev1.Invoice

	if s.invoiceClient != nil {
		listResp, err := s.invoiceClient.ListInvoices(ctx, &invoicev1.ListInvoicesRequest{
			TenantId:  tenantID,
			Action:    action,
			StartDate: req.GetStartDate(),
			EndDate:   req.GetEndDate(),
		})

		if err == nil && listResp != nil {
			for _, inv := range listResp.GetInvoices() {
				if aggregator.MatchesFilter(inv.GetDateTime(), action, req.GetStartDate(), req.GetEndDate()) {
					matchedInvoices = append(matchedInvoices, inv)
				}
			}
		}
	}

	summary, entries := aggregator.ComputeSummary(tenantID, matchedInvoices, req.GetStartDate(), req.GetEndDate())

	return &statementv1.StatementResponse{
		Summary: summary,
		Entries: entries,
	}, nil
}

func (s *StatementServiceServer) StreamStatement(req *statementv1.GenerateStatementRequest, stream statementv1.StatementService_StreamStatementServer) error {
	tenantID := strings.TrimSpace(req.GetTenantId())
	if tenantID == "" {
		return status.Error(codes.InvalidArgument, "tenant_id is required")
	}

	if s.invoiceClient == nil {
		return nil
	}

	listResp, err := s.invoiceClient.ListInvoices(stream.Context(), &invoicev1.ListInvoicesRequest{
		TenantId:  tenantID,
		Action:    req.GetAction(),
		StartDate: req.GetStartDate(),
		EndDate:   req.GetEndDate(),
	})
	if err != nil {
		return status.Errorf(codes.Internal, "failed to query invoices: %v", err)
	}

	for _, inv := range listResp.GetInvoices() {
		if aggregator.MatchesFilter(inv.GetDateTime(), req.GetAction(), req.GetStartDate(), req.GetEndDate()) {
			entry := &statementv1.StatementEntry{
				TransactionId:   inv.GetTransactionId(),
				DateTime:        inv.GetDateTime(),
				CustomerId:      inv.GetCustomerId(),
				CustomerName:    inv.GetCustomerName(),
				TaxableAmount:   inv.GetTaxableAmount(),
				GrandTotal:      inv.GetGrandTotal(),
				MethodOfPayment: inv.GetMethodOfPayment(),
				LineItemsCount:  int32(len(inv.GetLines())),
			}
			if err := stream.Send(entry); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *StatementServiceServer) GetAnalytics(ctx context.Context, req *statementv1.AnalyticsRequest) (*statementv1.AnalyticsResponse, error) {
	tenantID := strings.TrimSpace(req.GetTenantId())
	if tenantID == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant_id is required")
	}

	var dataPoints []*statementv1.AnalyticsDataPoint

	if s.invoiceClient != nil {
		listResp, err := s.invoiceClient.ListInvoices(ctx, &invoicev1.ListInvoicesRequest{
			TenantId:  tenantID,
			Action:    "all",
			StartDate: req.GetStartDate(),
			EndDate:   req.GetEndDate(),
		})

		if err == nil && listResp != nil {
			for _, inv := range listResp.GetInvoices() {
				dateStr := inv.GetDateTime()
				if len(dateStr) >= 10 {
					dateStr = dateStr[:10]
				}
				for _, line := range inv.GetLines() {
					dataPoints = append(dataPoints, &statementv1.AnalyticsDataPoint{
						Date:      dateStr,
						ItemName:  line.GetProductName(),
						Quantity:  line.GetQuantity(),
						Amount:    line.GetAmount(),
						UnitPrice: line.GetUnitPrice(),
					})
				}
			}
		}
	}

	return &statementv1.AnalyticsResponse{
		DataPoints: dataPoints,
	}, nil
}
