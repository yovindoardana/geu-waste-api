package domain

import "errors"

// Common domain errors
var (
	ErrNotFound                 = errors.New("resource not found")
	ErrHouseholdHasDependencies = errors.New("household has dependencies and cannot be deleted")
	ErrHouseholdPendingPayment  = errors.New("household has pending payment")
	ErrInvalidStateTransition   = errors.New("invalid state transition")
	ErrSafetyCheckRequired      = errors.New("safety check is required for electronic waste")
	ErrPaymentConflict          = errors.New("payment conflict with existing invoice")
	ErrValidation               = errors.New("validation error")
	ErrCommitUncertain          = errors.New("commit outcome uncertain")
)
