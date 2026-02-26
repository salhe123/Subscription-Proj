package repo

import (
	"context"

	"github.com/user/subscription-proj/contracts"
	"github.com/user/subscription-proj/domain"
	"github.com/user/subscription-proj/pkg/commitplan"
)

var _ contracts.SubscriptionRepository = (*SubscriptionRepo)(nil)
var _ contracts.OutboxRepository = (*OutboxRepo)(nil)

type SubscriptionRepo struct{}

func NewSubscriptionRepo() *SubscriptionRepo {
	return &SubscriptionRepo{}
}

func (r *SubscriptionRepo) GetByID(_ context.Context, _ string) (*domain.Subscription, error) {
	return nil, domain.ErrSubscriptionNotFound
}

func (r *SubscriptionRepo) InsertMut(sub *domain.Subscription) commitplan.Mutation {
	row := map[string]interface{}{
		"id":           sub.ID(),
		"customer_id":  sub.CustomerID(),
		"plan_id":      sub.PlanID(),
		"amount_cents": sub.AmountCents(),
		"status":       string(sub.Status()),
		"start_date":   sub.StartDate(),
	}
	return row
}

func (r *SubscriptionRepo) UpdateMut(sub *domain.Subscription) commitplan.Mutation {
	updates := make(map[string]interface{})

	if sub.Changes().Dirty(domain.FieldStatus) {
		updates["status"] = string(sub.Status())
	}
	if sub.Changes().Dirty(domain.FieldCancelledAt) {
		if at := sub.CancelledAt(); at != nil {
			updates["cancelled_at"] = *at
		}
	}

	if len(updates) == 0 {
		return nil
	}

	updates["id"] = sub.ID()
	return updates
}

type OutboxRepo struct{}

func NewOutboxRepo() *OutboxRepo {
	return &OutboxRepo{}
}

func (r *OutboxRepo) InsertEventMut(event domain.DomainEvent) commitplan.Mutation {
	row := map[string]interface{}{
		"event_type":   event.EventType(),
		"aggregate_id": event.AggregateID(),
		"occurred_at":  event.OccurredAt(),
		"status":       "PENDING",
	}
	return row
}
