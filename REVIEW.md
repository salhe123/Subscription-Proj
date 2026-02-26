# Code Review: Broken Subscription Service

## Issue 1: God Service — No Layer Separation

- Category: Layer Violation
- Severity: CRITICAL
- Problem: `SubscriptionService` handles HTTP calls, SQL queries, business logic, and domain rules in a single struct. There are no distinct layers (domain, usecase, repository, adapter).
- Why it matters: Every change touches the same file. You can't swap the database or billing provider without rewriting business logic. New developers can't reason about where a bug lives.

## Issue 2: Direct HTTP Call in Business Logic

- Category: Layer Violation
- Severity: CRITICAL
- Problem: `CreateSubscription` calls `s.client.Get("https://api.billing.com/validate/...")` directly. The usecase is coupled to a specific HTTP endpoint and transport protocol.
- Why it matters: You can't test subscription creation without a live billing API. If the billing API changes its URL or auth scheme, you have to modify business logic code.

## Issue 3: Direct SQL in Business Logic

- Category: Layer Violation
- Severity: CRITICAL
- Problem: Raw SQL statements (`INSERT INTO subscriptions VALUES (?, ...)`) live next to domain logic. The service depends on `*sql.DB` directly.
- Why it matters: Database schema changes break business logic. You can't switch databases or use a different persistence strategy without rewriting the service. SQL injection risk if inputs are ever interpolated.

## Issue 4: Hardcoded External URL

- Category: Layer Violation
- Severity: WARNING
- Problem: `"https://api.billing.com/validate/"` and `"https://api.billing.com/refund"` are string literals inside business methods.
- Why it matters: No way to configure different environments (staging, prod). Can't mock or redirect without changing the source code.

## Issue 5: Anemic Domain Model — Public Struct Fields

- Category: Domain Purity
- Severity: CRITICAL
- Problem: `Subscription` is a plain data struct with all public fields. Any code can set `Status`, `Price`, or `StartDate` to anything. There are no invariants enforced by the aggregate.
- Why it matters: Nothing prevents setting `Status = "BANANA"` or `Price = -500`. The domain offers zero protection against invalid state.

## Issue 6: No Domain Events

- Category: Domain Purity
- Severity: WARNING
- Problem: State changes (creation, cancellation) don't produce domain events. There's no way for other bounded contexts to react to subscription lifecycle changes.
- Why it matters: You can't implement event-driven features (notifications, analytics, refund processing) without polling the database.

## Issue 7: Status as Raw String

- Category: Domain Purity
- Severity: WARNING
- Problem: Status is `string` with magic literals `"ACTIVE"` and `"CANCELLED"` scattered through the code. No type safety or exhaustiveness checking.
- Why it matters: Typos compile fine. Adding a new status means grepping for string literals across the codebase.

## Issue 8: Ignoring HTTP Error — `resp, _ :=`

- Category: Error Handling
- Severity: CRITICAL
- Problem: In `CreateSubscription`, the error from `s.client.Get(...)` is discarded with `_`. If the request fails (network error, DNS failure, timeout), `resp` is nil and `resp.Body.Close()` panics.
- Why it matters: A nil pointer dereference crashes the entire process. An invalid customer could slip through because the validation call never actually completed.

## Issue 9: Ignoring `row.Scan` Error

- Category: Error Handling
- Severity: CRITICAL
- Problem: In `CancelSubscription`, `row.Scan(...)` error is discarded. If the subscription doesn't exist, `sub` contains zero values and the code proceeds to calculate a refund on a phantom subscription.
- Why it matters: Cancelling a nonexistent subscription silently succeeds with a refund of zero. No feedback to the caller that the subscription wasn't found.

## Issue 10: Ignoring Refund API Response

- Category: Error Handling
- Severity: WARNING
- Problem: `s.client.Post(...)` for the refund has its error and response discarded. There's no way to know if the refund succeeded or failed.
- Why it matters: Customers may never receive their refund. There's no retry mechanism and no audit trail.

## Issue 11: float64 for Money

- Category: Money Handling
- Severity: CRITICAL
- Problem: `Price` is `float64`. The refund calculation `sub.Price * (30 - daysUsed) / 30` uses floating-point arithmetic. IEEE 754 floats cannot represent many decimal values exactly (e.g., 0.1 + 0.2 != 0.3).
- Why it matters: Rounding errors accumulate over thousands of transactions. A customer charged $29.99 might get refunded $19.993333... which truncates differently depending on the payment processor. Financial audits will show discrepancies.

## Issue 12: `time.Now()` and `time.Since()` — Untestable Time

- Category: Testability
- Severity: CRITICAL
- Problem: `time.Now()` is called directly in `CreateSubscription` and `time.Since()` in `CancelSubscription`. Tests can't control what "now" means.
- Why it matters: Refund calculation tests are non-deterministic. You can't verify that a subscription created "10 days ago" yields the correct refund amount because the clock keeps ticking.

## Issue 13: No Interfaces for Dependencies

- Category: Testability
- Severity: CRITICAL
- Problem: The service depends on concrete `*sql.DB` and `*http.Client`. There are no interfaces for the repository or billing client.
- Why it matters: Unit tests require a real database and real HTTP server. You can't inject mocks or fakes. Integration tests are the only option, which are slow and flaky.

## Issue 14: Refund in Same Transaction as Cancel

- Category: Layer Violation
- Severity: WARNING
- Problem: The cancel flow does DB update and refund API call sequentially. If the refund call fails after the DB update, the subscription is cancelled but the customer never gets their money back. If the refund succeeds but the DB update fails, the refund is issued for a still-active subscription.
- Why it matters: No atomicity between the local state change and the external side effect. The system can end up in an inconsistent state that's hard to detect and recover from.
