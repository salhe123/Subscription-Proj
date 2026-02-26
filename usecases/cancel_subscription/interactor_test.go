package cancel_subscription_test

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
	"github.com/user/subscription-proj/usecases/cancel_subscription"
)

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

var _ contracts.SubscriptionRepository = (*mockRepo)(nil)
var _ contracts.OutboxRepository = (*mockOutbox)(nil)
var _ commitplan.Committer = (*mockCommitter)(nil)

func TestExecute_Success(t *testing.T) {
	repo := new(mockRepo)
	outbox := new(mockOutbox)
	committer := new(mockCommitter)
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(2025, 1, 11, 0, 0, 0, 0, time.UTC)

	sub := domain.ReconstructSubscription("sub-1", "cust-1", "plan-1", 3000, domain.StatusActive, start, nil)

	repo.On("GetByID", mock.Anything, "sub-1").Return(sub, nil)
	repo.On("UpdateMut", mock.Anything).Return("update-mut")
	outbox.On("InsertEventMut", mock.Anything).Return("outbox-mut")
	committer.On("Apply", mock.Anything, mock.Anything).Return(nil)

	it := cancel_subscription.NewInteractor(repo, outbox, committer, clock.FixedClock{Time: now})

	err := it.Execute(context.Background(), cancel_subscription.Request{SubscriptionID: "sub-1"})

	require.NoError(t, err)
	assert.Equal(t, domain.StatusCancelled, sub.Status())
	committer.AssertExpectations(t)
}

func TestExecute_AlreadyCancelled(t *testing.T) {
	repo := new(mockRepo)
	outbox := new(mockOutbox)
	committer := new(mockCommitter)
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	cancelledAt := time.Date(2025, 1, 5, 0, 0, 0, 0, time.UTC)
	now := time.Date(2025, 1, 11, 0, 0, 0, 0, time.UTC)

	sub := domain.ReconstructSubscription("sub-1", "cust-1", "plan-1", 3000, domain.StatusCancelled, start, &cancelledAt)

	repo.On("GetByID", mock.Anything, "sub-1").Return(sub, nil)

	it := cancel_subscription.NewInteractor(repo, outbox, committer, clock.FixedClock{Time: now})

	err := it.Execute(context.Background(), cancel_subscription.Request{SubscriptionID: "sub-1"})

	assert.ErrorIs(t, err, domain.ErrAlreadyCancelled)
	committer.AssertNotCalled(t, "Apply")
}

func TestExecute_NotFound(t *testing.T) {
	repo := new(mockRepo)
	outbox := new(mockOutbox)
	committer := new(mockCommitter)
	now := time.Date(2025, 1, 11, 0, 0, 0, 0, time.UTC)

	repo.On("GetByID", mock.Anything, "missing").Return(nil, domain.ErrSubscriptionNotFound)

	it := cancel_subscription.NewInteractor(repo, outbox, committer, clock.FixedClock{Time: now})

	err := it.Execute(context.Background(), cancel_subscription.Request{SubscriptionID: "missing"})

	assert.ErrorIs(t, err, domain.ErrSubscriptionNotFound)
	committer.AssertNotCalled(t, "Apply")
}

func TestRefundCalculation(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		now         time.Time
		amountCents int64
		wantRefund  int64
	}{
		{
			name:        "10 days used",
			now:         time.Date(2025, 1, 11, 0, 0, 0, 0, time.UTC),
			amountCents: 3000,
			wantRefund:  2000,
		},
		{
			name:        "full month used",
			now:         time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC),
			amountCents: 3000,
			wantRefund:  0,
		},
		{
			name:        "same day cancel",
			now:         time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			amountCents: 3000,
			wantRefund:  3000,
		},
		{
			name:        "one day used",
			now:         time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC),
			amountCents: 3000,
			wantRefund:  2900,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := domain.ReconstructSubscription("sub-1", "cust-1", "plan-1", tt.amountCents, domain.StatusActive, start, nil)
			got := sub.RefundAmount(tt.now)
			assert.Equal(t, tt.wantRefund, got)
		})
	}
}
