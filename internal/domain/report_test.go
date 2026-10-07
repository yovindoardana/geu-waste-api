package domain

import (
	"encoding/json"
	"testing"

	"github.com/shopspring/decimal"
)

func TestPaymentSummarySerialization(t *testing.T) {
	report := PaymentSummaryReport{
		Currency: "IDR",
		Items: []PaymentSummaryItem{
			{Status: PaymentStatusPending, Count: 1, TotalAmount: ValueFromDecimal(decimal.NewFromInt(50000))},
			{Status: PaymentStatusPaid, Count: 1, TotalAmount: ValueFromDecimal(decimal.NewFromInt(100000))},
			{Status: PaymentStatusFailed, Count: 1, TotalAmount: ValueFromDecimal(decimal.NewFromInt(50000))},
		},
		TotalPayments: 3,
		TotalRevenue:  ValueFromDecimal(decimal.NewFromInt(100000)),
	}

	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("failed to marshal report: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to unmarshal raw json: %v", err)
	}

	if raw["total_revenue"] != "100000.00" {
		t.Errorf("expected total_revenue '100000.00', got %v", raw["total_revenue"])
	}

	items := raw["items"].([]any)
	if len(items) != 3 {
		t.Errorf("expected 3 items, got %d", len(items))
	}
	firstItem := items[0].(map[string]any)
	if firstItem["total_amount"] != "50000.00" {
		t.Errorf("expected first item total_amount '50000.00', got %v", firstItem["total_amount"])
	}
}
