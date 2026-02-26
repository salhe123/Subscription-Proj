package contracts

import (
	"context"

	"github.com/user/subscription-proj/domain"
	"github.com/user/subscription-proj/pkg/commitplan"
)

type SubscriptionRepository interface {
	GetByID(ctx context.Context, id string) (*domain.Subscription, error)
	InsertMut(sub *domain.Subscription) commitplan.Mutation
	UpdateMut(sub *domain.Subscription) commitplan.Mutation
}

type OutboxRepository interface {
	InsertEventMut(event domain.DomainEvent) commitplan.Mutation
}
