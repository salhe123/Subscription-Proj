package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/user/subscription-proj/contracts"
	"github.com/user/subscription-proj/domain"
)

var _ contracts.BillingClient = (*HTTPBillingClient)(nil)

type HTTPBillingClient struct {
	client  *http.Client
	baseURL string
}

func NewHTTPBillingClient(client *http.Client, baseURL string) *HTTPBillingClient {
	return &HTTPBillingClient{
		client:  client,
		baseURL: baseURL,
	}
}

func (c *HTTPBillingClient) ValidateCustomer(ctx context.Context, customerID string) error {
	url := fmt.Sprintf("%s/customers/%s/validate", c.baseURL, customerID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("calling billing API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return domain.ErrCustomerValidationFailed
	}

	var result struct{ Valid bool }
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}

	if !result.Valid {
		return domain.ErrCustomerValidationFailed
	}

	return nil
}

func (c *HTTPBillingClient) IssueRefund(ctx context.Context, subscriptionID string, amountCents int64) error {
	url := fmt.Sprintf("%s/subscriptions/%s/refund", c.baseURL, subscriptionID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("calling refund API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("refund API returned status %d", resp.StatusCode)
	}

	return nil
}
