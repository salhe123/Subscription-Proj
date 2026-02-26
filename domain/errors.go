package domain

import "errors"

var (
	ErrSubscriptionNotFound    = errors.New("subscription not found")
	ErrSubscriptionNotActive   = errors.New("subscription is not active")
	ErrAlreadyCancelled        = errors.New("subscription is already cancelled")
	ErrInvalidCustomerID       = errors.New("invalid customer ID")
	ErrInvalidPlanID           = errors.New("invalid plan ID")
	ErrInvalidAmount           = errors.New("amount must be positive")
	ErrCustomerValidationFailed = errors.New("customer validation failed")
)
