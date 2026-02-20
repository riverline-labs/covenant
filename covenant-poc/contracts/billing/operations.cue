// Billing domain operation contracts.

operations: {
	"ProcessPayment": {
		constrained_by: [
			"no-payments-closed-accounts",
			"insufficient-funds",
			"processor-down",
			"large-payment-flag",
		]
		transitions: [
			{entity: "invoice", from: "approved", to: "paid"},
		]
	}

	"GetInvoice": {
		constrained_by: []
		transitions:    []
	}
}
