package domain

import (
	"crypto/rand"
	"fmt"
	"time"
)

type SubscriptionStatus string

const (
	StatusActive    SubscriptionStatus = "ACTIVE"
	StatusCancelled SubscriptionStatus = "CANCELLED"
)

const (
	FieldStatus      = "status"
	FieldCancelledAt = "cancelled_at"
)

type ChangeTracker struct {
	dirtyFields map[string]bool
}

func NewChangeTracker() *ChangeTracker {
	return &ChangeTracker{dirtyFields: make(map[string]bool)}
}

func (ct *ChangeTracker) MarkDirty(field string) {
	ct.dirtyFields[field] = true
}

func (ct *ChangeTracker) Dirty(field string) bool {
	return ct.dirtyFields[field]
}

func (ct *ChangeTracker) HasChanges() bool {
	return len(ct.dirtyFields) > 0
}

type Subscription struct {
	id          string
	customerID  string
	planID      string
	amountCents int64
	status      SubscriptionStatus
	startDate   time.Time
	cancelledAt *time.Time
	changes     *ChangeTracker
	events      []DomainEvent
}

func NewSubscription(customerID, planID string, amountCents int64, now time.Time) (*Subscription, error) {
	if customerID == "" {
		return nil, ErrInvalidCustomerID
	}
	if planID == "" {
		return nil, ErrInvalidPlanID
	}
	if amountCents <= 0 {
		return nil, ErrInvalidAmount
	}

	id := generateID()
	s := &Subscription{
		id:          id,
		customerID:  customerID,
		planID:      planID,
		amountCents: amountCents,
		status:      StatusActive,
		startDate:   now,
		changes:     NewChangeTracker(),
	}

	s.events = append(s.events, &SubscriptionCreatedEvent{
		baseEvent:   baseEvent{aggregateID: id, occurredAt: now},
		CustomerID:  customerID,
		PlanID:      planID,
		AmountCents: amountCents,
	})

	return s, nil
}

func ReconstructSubscription(
	id, customerID, planID string,
	amountCents int64,
	status SubscriptionStatus,
	startDate time.Time,
	cancelledAt *time.Time,
) *Subscription {
	return &Subscription{
		id:          id,
		customerID:  customerID,
		planID:      planID,
		amountCents: amountCents,
		status:      status,
		startDate:   startDate,
		cancelledAt: cancelledAt,
		changes:     NewChangeTracker(),
	}
}

func (s *Subscription) Cancel(now time.Time) error {
	if s.status == StatusCancelled {
		return ErrAlreadyCancelled
	}

	s.status = StatusCancelled
	s.cancelledAt = &now
	s.changes.MarkDirty(FieldStatus)
	s.changes.MarkDirty(FieldCancelledAt)

	s.events = append(s.events, &SubscriptionCancelledEvent{
		baseEvent:   baseEvent{aggregateID: s.id, occurredAt: now},
		RefundCents: s.RefundAmount(now),
	})

	return nil
}

func (s *Subscription) RefundAmount(now time.Time) int64 {
	elapsed := now.Sub(s.startDate)
	daysUsed := int64(elapsed.Hours()) / 24
	if daysUsed >= 30 {
		return 0
	}
	return s.amountCents * (30 - daysUsed) / 30
}

func (s *Subscription) ID() string                   { return s.id }
func (s *Subscription) CustomerID() string            { return s.customerID }
func (s *Subscription) PlanID() string                { return s.planID }
func (s *Subscription) AmountCents() int64            { return s.amountCents }
func (s *Subscription) Status() SubscriptionStatus    { return s.status }
func (s *Subscription) StartDate() time.Time          { return s.startDate }
func (s *Subscription) CancelledAt() *time.Time       { return s.cancelledAt }
func (s *Subscription) Changes() *ChangeTracker       { return s.changes }

func (s *Subscription) DomainEvents() []DomainEvent {
	events := s.events
	s.events = nil
	return events
}

func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
