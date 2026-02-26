package domain

import "time"

type DomainEvent interface {
	EventType() string
	OccurredAt() time.Time
	AggregateID() string
}

type baseEvent struct {
	aggregateID string
	occurredAt  time.Time
}

func (e baseEvent) OccurredAt() time.Time { return e.occurredAt }
func (e baseEvent) AggregateID() string   { return e.aggregateID }

type SubscriptionCreatedEvent struct {
	baseEvent
	CustomerID string
	PlanID     string
	AmountCents int64
}

func (e *SubscriptionCreatedEvent) EventType() string { return "subscription.created" }

type SubscriptionCancelledEvent struct {
	baseEvent
	RefundCents int64
}

func (e *SubscriptionCancelledEvent) EventType() string { return "subscription.cancelled" }
