package domain

// WasteSummaryItem represents an individual bucket item in waste report.
type WasteSummaryItem struct {
	Type   string `json:"type"`
	Status string `json:"status"`
	Count  int64  `json:"count"`
}

// WasteSummaryReport holds the aggregated waste pickup statistics.
type WasteSummaryReport struct {
	Items        []WasteSummaryItem `json:"items"`
	TotalPickups int64              `json:"total_pickups"`
}

// PaymentSummaryItem represents an individual status bucket in payment report.
type PaymentSummaryItem struct {
	Status      string        `json:"status"`
	Count       int64         `json:"count"`
	TotalAmount DecimalAmount `json:"total_amount"`
}

// PaymentSummaryReport holds the aggregated payment and revenue metrics.
type PaymentSummaryReport struct {
	Currency      string               `json:"currency"`
	Items         []PaymentSummaryItem `json:"items"`
	TotalPayments int64                `json:"total_payments"`
	TotalRevenue  DecimalAmount        `json:"total_revenue"`
}

// DefaultWasteTypesOrder defines the required canonical sequence of waste types.
var DefaultWasteTypesOrder = []string{
	WasteTypeOrganic,
	WasteTypePlastic,
	WasteTypePaper,
	WasteTypeElectronic,
}

// DefaultPickupStatusesOrder defines the required canonical sequence of pickup statuses.
var DefaultPickupStatusesOrder = []string{
	PickupStatusPending,
	PickupStatusScheduled,
	PickupStatusCompleted,
	PickupStatusCanceled,
}

// DefaultPaymentStatusesOrder defines the required canonical sequence of payment statuses.
var DefaultPaymentStatusesOrder = []string{
	PaymentStatusPending,
	PaymentStatusPaid,
	PaymentStatusFailed,
}
