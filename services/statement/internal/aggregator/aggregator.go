package aggregator

import (
	"time"

	invoicev1 "github.com/accounting-microservices/gen/go/invoice/v1"
	statementv1 "github.com/accounting-microservices/gen/go/statement/v1"
)

// MatchesFilter checks if an invoice date matches the requested temporal action.
func MatchesFilter(dateStr, action string, startDate, endDate string) bool {
	t, err := time.Parse(time.RFC3339, dateStr)
	if err != nil {
		t, err = time.Parse("2006-01-02T15:04:05Z07:00", dateStr)
		if err != nil {
			return true // Fallback include if unparseable
		}
	}

	now := time.Now()

	switch action {
	case "today":
		return t.Year() == now.Year() && t.YearDay() == now.YearDay()
	case "thisWeek":
		y1, w1 := t.ISOWeek()
		y2, w2 := now.ISOWeek()
		return y1 == y2 && w1 == w2
	case "thisMonth":
		return t.Year() == now.Year() && t.Month() == now.Month()
	case "thisQuarter":
		return t.Year() == now.Year() && (int(t.Month())-1)/3 == (int(now.Month())-1)/3
	case "thisYear":
		return t.Year() == now.Year()
	case "toAndFromDate":
		if startDate != "" && endDate != "" {
			s, err1 := time.Parse(time.RFC3339, startDate)
			e, err2 := time.Parse(time.RFC3339, endDate)
			if err1 == nil && err2 == nil {
				return !t.Before(s) && !t.After(e)
			}
		}
		return true
	case "beforeDate":
		if endDate != "" {
			e, err := time.Parse(time.RFC3339, endDate)
			if err == nil {
				return !t.After(e)
			}
		}
		return true
	case "afterDate":
		if startDate != "" {
			s, err := time.Parse(time.RFC3339, startDate)
			if err == nil {
				return !t.Before(s)
			}
		}
		return true
	case "all":
		fallthrough
	default:
		return true
	}
}

// ComputeSummary aggregates totals across a list of matched invoices.
func ComputeSummary(tenantID string, invoices []*invoicev1.Invoice, startDate, endDate string) (*statementv1.StatementSummary, []*statementv1.StatementEntry) {
	var totalRevenue float64
	var totalTax float64
	var totalDiscount float64
	var entries []*statementv1.StatementEntry

	for _, inv := range invoices {
		totalRevenue += inv.GetGrandTotal()
		totalTax += inv.GetTaxCollectedAtSource()
		totalDiscount += inv.GetTotalDiscount()

		entries = append(entries, &statementv1.StatementEntry{
			TransactionId:   inv.GetTransactionId(),
			DateTime:        inv.GetDateTime(),
			CustomerId:      inv.GetCustomerId(),
			CustomerName:    inv.GetCustomerName(),
			TaxableAmount:   inv.GetTaxableAmount(),
			GrandTotal:      inv.GetGrandTotal(),
			MethodOfPayment: inv.GetMethodOfPayment(),
			LineItemsCount:  int32(len(inv.GetLines())),
		})
	}

	summary := &statementv1.StatementSummary{
		TenantId:      tenantID,
		PeriodStart:   startDate,
		PeriodEnd:     endDate,
		TotalInvoices: int64(len(invoices)),
		TotalRevenue:  totalRevenue,
		TotalTax:      totalTax,
		TotalDiscount: totalDiscount,
	}

	return summary, entries
}
