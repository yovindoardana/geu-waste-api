package domain

import (
	"encoding/json"
	"time"

	"github.com/shopspring/decimal"
)

// Valid payment statuses
const (
	PaymentStatusPending = "pending"
	PaymentStatusPaid    = "paid"
	PaymentStatusFailed  = "failed"
)

// Standard tariffs
var (
	TariffStandard   = decimal.NewFromInt(50000)
	TariffElectronic = decimal.NewFromInt(100000)
)

// GetTariffForType returns standard decimal tariff for a given waste type.
func GetTariffForType(wasteType string) decimal.Decimal {
	if wasteType == WasteTypeElectronic {
		return TariffElectronic
	}
	return TariffStandard
}

// Payment represents a payment record.
type Payment struct {
	ID           string          `json:"id"`
	HouseholdID  string          `json:"household_id"`
	WasteID      string          `json:"waste_id"`
	Amount       DecimalAmount   `json:"amount"`
	PaymentDate  *time.Time      `json:"payment_date"`
	Status       string          `json:"status"`
	ProofFileURL *string         `json:"proof_file_url"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

// DecimalAmount wraps decimal.Decimal to guarantee exact 2 decimal places JSON string serialization.
type DecimalAmount struct {
	decimal.Decimal
}

// MarshalJSON marshals DecimalAmount to a JSON string with 2 decimal places.
func (d DecimalAmount) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.StringFixed(2))
}

// UnmarshalJSON unmarshals JSON string to DecimalAmount.
func (d *DecimalAmount) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	dec, err := decimal.NewFromString(s)
	if err != nil {
		return err
	}
	d.Decimal = dec
	return nil
}

// ValueFromDecimal creates DecimalAmount from decimal.Decimal.
func ValueFromDecimal(d decimal.Decimal) DecimalAmount {
	return DecimalAmount{Decimal: d}
}

// PickupCompleteResult holds composite response for pickup completion.
type PickupCompleteResult struct {
	Pickup  *Pickup  `json:"pickup"`
	Payment *Payment `json:"payment"`
}

// CreatePaymentRequest defines payload for POST /api/payments.
type CreatePaymentRequest struct {
	HouseholdID *string `json:"household_id"`
	WasteID     *string `json:"waste_id"`
	Amount      *string `json:"amount"`
}

// PaymentFilter represents query parameters for filtering payments.
type PaymentFilter struct {
	Page        int
	Limit       int
	Status      *string
	HouseholdID *string
	StartDate   *time.Time
	EndDate     *time.Time
}
