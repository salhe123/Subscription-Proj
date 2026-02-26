package contracts

import "context"

type BillingClient interface {
	ValidateCustomer(ctx context.Context, customerID string) error
	IssueRefund(ctx context.Context, subscriptionID string, amountCents int64) error
}
