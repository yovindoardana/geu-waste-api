package domain

import (
	"time"
)

// Valid pickup waste types
const (
	WasteTypeOrganic    = "organic"
	WasteTypePlastic    = "plastic"
	WasteTypePaper      = "paper"
	WasteTypeElectronic = "electronic"
)

// Valid pickup status states
const (
	PickupStatusPending   = "pending"
	PickupStatusScheduled = "scheduled"
	PickupStatusCompleted = "completed"
	PickupStatusCanceled  = "canceled"
)

// ValidWasteTypes map for fast lookup
var ValidWasteTypes = map[string]bool{
	WasteTypeOrganic:    true,
	WasteTypePlastic:    true,
	WasteTypePaper:      true,
	WasteTypeElectronic: true,
}

// ValidPickupStatuses map for fast lookup
var ValidPickupStatuses = map[string]bool{
	PickupStatusPending:   true,
	PickupStatusScheduled: true,
	PickupStatusCompleted: true,
	PickupStatusCanceled:  true,
}

// Pickup represents a waste pickup record.
type Pickup struct {
	ID          string     `json:"id"`
	HouseholdID string     `json:"household_id"`
	Type        string     `json:"type"`
	Status      string     `json:"status"`
	PickupDate  *time.Time `json:"pickup_date"`
	SafetyCheck *bool      `json:"safety_check"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// CreatePickupRequest represents payload for creating a new waste pickup.
type CreatePickupRequest struct {
	HouseholdID *string `json:"household_id"`
	Type        *string `json:"type"`
	SafetyCheck *bool   `json:"safety_check,omitempty"`
}

// SchedulePickupRequest represents payload for scheduling a pickup.
type SchedulePickupRequest struct {
	PickupDate  *string `json:"pickup_date"`
	SafetyCheck *bool   `json:"safety_check,omitempty"`
}

// PickupFilter represents query parameters for filtering pickups.
type PickupFilter struct {
	Page        int
	Limit       int
	Status      *string
	HouseholdID *string
}
