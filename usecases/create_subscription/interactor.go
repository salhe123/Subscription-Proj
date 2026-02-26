package create_subscription

import (
	"context"

	"github.com/user/subscription-proj/contracts"
	"github.com/user/subscription-proj/domain"
	"github.com/user/subscription-proj/pkg/clock"
	"github.com/user/subscription-proj/pkg/commitplan"
)

type Request struct {
	CustomerID  string
	PlanID      string
	AmountCents int64
}

type Interactor struct {
	billing    contracts.BillingClient
	repo       contracts.SubscriptionRepository
	outboxRepo contracts.OutboxRepository
	committer  commitplan.Committer
	clock      clock.Clock
}

func NewInteractor(
	billing contracts.BillingClient,
	repo contracts.SubscriptionRepository,
	outboxRepo contracts.OutboxRepository,
	committer commitplan.Committer,
	clock clock.Clock,
) *Interactor {
	return &Interactor{
		billing:    billing,
		repo:       repo,
		outboxRepo: outboxRepo,
		committer:  committer,
		clock:      clock,
	}
}

func (it *Interactor) Execute(ctx context.Context, req Request) (string, error) {
	if err := it.billing.ValidateCustomer(ctx, req.CustomerID); err != nil {
		return "", err
	}

	sub, err := domain.NewSubscription(req.CustomerID, req.PlanID, req.AmountCents, it.clock.Now())
	if err != nil {
		return "", err
	}

	plan := commitplan.NewPlan()
	plan.Add(it.repo.InsertMut(sub))

	for _, event := range sub.DomainEvents() {
		plan.Add(it.outboxRepo.InsertEventMut(event))
	}

	if err := it.committer.Apply(ctx, plan); err != nil {
		return "", err
	}

	return sub.ID(), nil
}
