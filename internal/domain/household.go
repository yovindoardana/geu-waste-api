package domain

import (
	"time"
)

// Household represents a registered household domain model.
type Household struct {
	ID        string    `json:"id"`
	OwnerName string    `json:"owner_name"`
	Address   string    `json:"address"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateHouseholdRequest defines payload for creating a new household.
type CreateHouseholdRequest struct {
	OwnerName *string `json:"owner_name"`
	Address   *string `json:"address"`
}
