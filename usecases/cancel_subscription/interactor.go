package cancel_subscription

import (
	"context"

	"github.com/user/subscription-proj/contracts"
	"github.com/user/subscription-proj/pkg/clock"
	"github.com/user/subscription-proj/pkg/commitplan"
)

type Request struct {
	SubscriptionID string
}

type Interactor struct {
	repo       contracts.SubscriptionRepository
	outboxRepo contracts.OutboxRepository
	committer  commitplan.Committer
	clock      clock.Clock
}

func NewInteractor(
	repo contracts.SubscriptionRepository,
	outboxRepo contracts.OutboxRepository,
	committer commitplan.Committer,
	clock clock.Clock,
) *Interactor {
	return &Interactor{
		repo:       repo,
		outboxRepo: outboxRepo,
		committer:  committer,
		clock:      clock,
	}
}

func (it *Interactor) Execute(ctx context.Context, req Request) error {
	sub, err := it.repo.GetByID(ctx, req.SubscriptionID)
	if err != nil {
		return err
	}

	if err := sub.Cancel(it.clock.Now()); err != nil {
		return err
	}

	plan := commitplan.NewPlan()

	if mut := it.repo.UpdateMut(sub); mut != nil {
		plan.Add(mut)
	}

	for _, event := range sub.DomainEvents() {
		plan.Add(it.outboxRepo.InsertEventMut(event))
	}

	if err := it.committer.Apply(ctx, plan); err != nil {
		return err
	}

	return nil
}
