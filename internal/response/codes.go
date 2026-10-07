package response

// Standard API Error Codes
const (
	CodeValidationError          = "VALIDATION_ERROR"
	CodeResourceNotFound         = "RESOURCE_NOT_FOUND"
	CodeMethodNotAllowed         = "METHOD_NOT_ALLOWED"
	CodeHouseholdPendingPayment  = "HOUSEHOLD_PENDING_PAYMENT"
	CodeInvalidStateTransition   = "INVALID_STATE_TRANSITION"
	CodeSafetyCheckRequired      = "SAFETY_CHECK_REQUIRED"
	CodeHouseholdHasDependencies = "HOUSEHOLD_HAS_DEPENDENCIES"
	CodePaymentConflict          = "PAYMENT_CONFLICT"
	CodePayloadTooLarge          = "PAYLOAD_TOO_LARGE"
	CodeUnsupportedMediaType     = "UNSUPPORTED_MEDIA_TYPE"
	CodeInternalError            = "INTERNAL_ERROR"
	CodeServiceUnavailable       = "SERVICE_UNAVAILABLE"
)
