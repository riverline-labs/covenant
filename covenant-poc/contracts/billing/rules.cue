// Billing domain business rules.
// Rules produce verdicts; absence of a deny/escalate/require is permission.

rules: [
	{
		id:         "no-payments-closed-accounts"
		applies_to: ["ProcessPayment"]

		when: {
			all: [
				{fact: "customer.status", equals: "closed"},
			]
		}

		verdict: deny: {
			code:   "ACCOUNT_CLOSED"
			reason: "Cannot process payments for closed accounts"
			error: {
				code:        "ACCOUNT_CLOSED"
				message:     "Account is closed and cannot accept payments"
				http_status: 422
				category:    "business_rule_violation"
				retryable:   false
				suggestion:  "Contact support to reactivate account"
			}
		}
	},

	{
		id:         "insufficient-funds"
		applies_to: ["ProcessPayment"]

		when: {
			all: [
				{fact: "payment.exceeds_balance", equals: true},
			]
		}

		verdict: deny: {
			code:   "INSUFFICIENT_FUNDS"
			reason: "Payment amount exceeds invoice balance"
			error: {
				code:        "INSUFFICIENT_FUNDS"
				message:     "Payment amount exceeds invoice balance"
				http_status: 402
				category:    "business_rule_violation"
				retryable:   true
				suggestion:  "Reduce payment amount or add funds to account"
			}
		}
	},

	{
		id:         "processor-down"
		applies_to: ["ProcessPayment"]

		when: {
			all: [
				{fact: "payment.processor.status", equals: "down"},
			]
		}

		verdict: deny: {
			code:   "PROCESSOR_UNAVAILABLE"
			reason: "Payment processor is temporarily unavailable"
			error: {
				code:        "PROCESSOR_UNAVAILABLE"
				message:     "Payment processor is temporarily unavailable"
				http_status: 503
				category:    "external_dependency"
				retryable:   true
				suggestion:  "Please try again in a few minutes"
			}
		}
	},

	{
		id:         "large-payment-flag"
		applies_to: ["ProcessPayment"]

		when: {
			all: [
				{fact: "payment.amount.value", greater_than: 10000},
			]
		}

		verdict: flag: {
			code:   "LARGE_PAYMENT"
			reason: "Payment amount over 10,000 — flagged for review"
		}
	},
]
