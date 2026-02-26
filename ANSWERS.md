# Answers

## Q1: Where should the refund HTTP call happen?

**D) As a separate usecase triggered by `SubscriptionCancelledEvent`.**

The cancel usecase's job is to transition the subscription to "cancelled" and persist that state change atomically with an outbox event. The refund is a separate concern that involves calling an external API — an inherently unreliable operation that shouldn't block or couple to the local state transition.

With option D, the `Cancel` usecase writes a `SubscriptionCancelledEvent` to the outbox table in the same transaction that updates the subscription status. A background worker picks up the event and executes the refund via a dedicated usecase. If the refund API is down, the event stays in the outbox and gets retried.

Trade-offs of the other options:
- **A (in Cancel usecase):** Mixes local state change with external call. If the refund fails, do you roll back the cancellation? If you don't, the customer is cancelled without a refund.
- **B (in domain Cancel method):** Domain must never make HTTP calls. This violates the dependency rule — the domain layer has no knowledge of infrastructure.
- **C (after committer.Apply):** Better than A or B, but if the process crashes between the commit and the refund call, the refund is lost with no retry mechanism.

## Q2: If Cancel() works but the refund API is down

**What should happen to the subscription status?**
The subscription should remain cancelled. The cancellation is a business fact that already happened. The refund is a downstream consequence, not a precondition.

**Should we retry? When? How many times?**
Yes. The outbox worker retries with exponential backoff (e.g., 1s, 5s, 30s, 2m, 10m). Retry up to a configured limit — something like 5-10 attempts over a few hours.

**What if refund fails after all retries?**
Move the event to a dead-letter queue (or mark it as `FAILED` in the outbox). Alert the operations team. A human investigates and either triggers a manual refund or resolves the underlying issue. The subscription status doesn't change — the customer is still cancelled, and the pending refund is tracked as unresolved.

## Q3: Two problems with the refund calculation

**Problem 1: `time.Since()` uses the real clock.**
`time.Since(sub.StartDate)` calls `time.Now()` internally, which makes the calculation non-deterministic. In tests, the result changes every millisecond. You also can't calculate refunds for historical dates. The fix: inject a `Clock` interface and pass `now` as a parameter.

**Problem 2: float64 arithmetic loses precision.**
`sub.Price * (30 - daysUsed) / 30` uses floating-point math. IEEE 754 cannot represent all decimal fractions exactly. For example, $29.99 / 30 = 0.99966... which introduces rounding errors that compound across transactions.

Rewritten with int64 cents and injected clock:

```go
func (s *Subscription) RefundAmount(now time.Time) int64 {
    elapsed := now.Sub(s.startDate)
    daysUsed := int64(elapsed.Hours()) / 24
    if daysUsed >= 30 {
        return 0
    }
    return s.amountCents * (30 - daysUsed) / 30
}
```

Money is stored as `int64` cents (e.g., $29.99 = 2999 cents). Integer division truncates deterministically. The clock is passed in as an argument rather than called internally.

## Q4: Test design for CancelSubscription

```go
func TestCancelSubscription_Success(t *testing.T) {
    // Mock: repo.GetByID returns an active subscription (use ReconstructSubscription)
    // Mock: repo.UpdateMut returns a mutation
    // Mock: outbox.InsertEventMut returns a mutation
    // Mock: committer.Apply returns nil
    //
    // Act: call interactor.Execute
    //
    // Assert:
    //   - No error returned
    //   - Subscription status is CANCELLED
    //   - committer.Apply was called (plan committed)
}

func TestCancelSubscription_AlreadyCancelled(t *testing.T) {
    // Mock: repo.GetByID returns a cancelled subscription
    //
    // Act: call interactor.Execute
    //
    // Assert:
    //   - Error is ErrAlreadyCancelled
    //   - committer.Apply was NOT called
}

func TestCancelSubscription_NotFound(t *testing.T) {
    // Mock: repo.GetByID returns ErrSubscriptionNotFound
    //
    // Act: call interactor.Execute
    //
    // Assert:
    //   - Error is ErrSubscriptionNotFound
    //   - committer.Apply was NOT called
}

func TestRefundCalculation(t *testing.T) {
    // Table-driven test with fixed dates:
    //   start=Jan 1, now=Jan 11, amount=3000 → refund=2000 (10 days used)
    //   start=Jan 1, now=Jan 31, amount=3000 → refund=0    (30+ days)
    //   start=Jan 1, now=Jan 1,  amount=3000 → refund=3000 (same day)
    //   start=Jan 1, now=Jan 2,  amount=3000 → refund=2900 (1 day used)
    //
    // Use ReconstructSubscription with known values.
    // Call RefundAmount(now) directly on the aggregate.
    // Assert exact int64 cent values.
}
```

What to mock: `SubscriptionRepository`, `OutboxRepository`, `Committer`. Use `testify/mock` with compile-time interface checks. The domain aggregate is real — never mock it. The clock is a `FixedClock` with a deterministic time.

## Q5: Business problem with ignored validation error

```go
resp, _ := s.client.Get("https://api.billing.com/validate/" + req.CustomerID)
```

Beyond the nil pointer crash, the business problem is that **an invalid customer can create a subscription**. If the HTTP call fails for any reason — network timeout, DNS failure, billing API 500 — the error is discarded and execution continues. The `result.Valid` field will be its zero value (`false`), but only if the JSON decode also fails silently. If the response body is empty or malformed, `json.NewDecoder` may produce a zero-value struct where `Valid` is false, which would actually reject the customer.

But the more dangerous case: if the request fails and `resp` doesn't panic (e.g., the error is a redirect that returns a non-nil response with an unexpected body), the decode might succeed with garbage data. The real business risk is that you **cannot trust the validation result**. You're making a billing decision (creating a paid subscription) based on data from a call that may not have completed. This means:

1. Subscriptions could be created for customers who don't exist in the billing system
2. You can't collect payment because the billing provider doesn't recognize the customer
3. Revenue is booked for subscriptions that will never be paid
4. Reconciliation between your system and the billing provider will show mismatches
