package create_subscription_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/user/subscription-proj/contracts"
	"github.com/user/subscription-proj/domain"
	"github.com/user/subscription-proj/pkg/clock"
	"github.com/user/subscription-proj/pkg/commitplan"
	"github.com/user/subscription-proj/usecases/create_subscription"
)

type mockBilling struct{ mock.Mock }

func (m *mockBilling) ValidateCustomer(ctx context.Context, customerID string) error {
	return m.Called(ctx, customerID).Error(0)
}
func (m *mockBilling) IssueRefund(ctx context.Context, subID string, amount int64) error {
	return m.Called(ctx, subID, amount).Error(0)
}

type mockRepo struct{ mock.Mock }

func (m *mockRepo) GetByID(ctx context.Context, id string) (*domain.Subscription, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Subscription), args.Error(1)
}
func (m *mockRepo) InsertMut(sub *domain.Subscription) commitplan.Mutation {
	return m.Called(sub).Get(0)
}
func (m *mockRepo) UpdateMut(sub *domain.Subscription) commitplan.Mutation {
	return m.Called(sub).Get(0)
}

type mockOutbox struct{ mock.Mock }

func (m *mockOutbox) InsertEventMut(event domain.DomainEvent) commitplan.Mutation {
	return m.Called(event).Get(0)
}

type mockCommitter struct{ mock.Mock }

func (m *mockCommitter) Apply(ctx context.Context, plan *commitplan.Plan) error {
	return m.Called(ctx, plan).Error(0)
}

var _ contracts.BillingClient = (*mockBilling)(nil)
var _ contracts.SubscriptionRepository = (*mockRepo)(nil)
var _ contracts.OutboxRepository = (*mockOutbox)(nil)
var _ commitplan.Committer = (*mockCommitter)(nil)

func TestExecute_Success(t *testing.T) {
	billing := new(mockBilling)
	repo := new(mockRepo)
	outbox := new(mockOutbox)
	committer := new(mockCommitter)
	now := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)

	billing.On("ValidateCustomer", mock.Anything, "cust-1").Return(nil)
	repo.On("InsertMut", mock.Anything).Return("insert-mut")
	outbox.On("InsertEventMut", mock.Anything).Return("outbox-mut")
	committer.On("Apply", mock.Anything, mock.Anything).Return(nil)

	it := create_subscription.NewInteractor(billing, repo, outbox, committer, clock.FixedClock{Time: now})

	id, err := it.Execute(context.Background(), create_subscription.Request{
		CustomerID:  "cust-1",
		PlanID:      "plan-1",
		AmountCents: 2999,
	})

	require.NoError(t, err)
	assert.NotEmpty(t, id)
	billing.AssertExpectations(t)
	committer.AssertExpectations(t)
}

func TestExecute_InvalidCustomer(t *testing.T) {
	billing := new(mockBilling)
	repo := new(mockRepo)
	outbox := new(mockOutbox)
	committer := new(mockCommitter)
	now := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)

	billing.On("ValidateCustomer", mock.Anything, "bad-cust").Return(domain.ErrCustomerValidationFailed)

	it := create_subscription.NewInteractor(billing, repo, outbox, committer, clock.FixedClock{Time: now})

	_, err := it.Execute(context.Background(), create_subscription.Request{
		CustomerID:  "bad-cust",
		PlanID:      "plan-1",
		AmountCents: 2999,
	})

	assert.ErrorIs(t, err, domain.ErrCustomerValidationFailed)
	committer.AssertNotCalled(t, "Apply")
}

func TestExecute_InvalidAmount(t *testing.T) {
	billing := new(mockBilling)
	repo := new(mockRepo)
	outbox := new(mockOutbox)
	committer := new(mockCommitter)
	now := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)

	billing.On("ValidateCustomer", mock.Anything, "cust-1").Return(nil)

	it := create_subscription.NewInteractor(billing, repo, outbox, committer, clock.FixedClock{Time: now})

	_, err := it.Execute(context.Background(), create_subscription.Request{
		CustomerID:  "cust-1",
		PlanID:      "plan-1",
		AmountCents: -100,
	})

	assert.ErrorIs(t, err, domain.ErrInvalidAmount)
	committer.AssertNotCalled(t, "Apply")
}
