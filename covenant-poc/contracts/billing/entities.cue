// Billing domain entity state machines.

entities: {
	"invoice": {
		states:   ["draft", "submitted", "approved", "paid", "cancelled"]
		initial:  "draft"
		terminal: ["paid", "cancelled"]

		transitions: [
			{from: "draft",     to: "submitted", via: "SubmitInvoice"},
			{from: "submitted", to: "approved",  via: "ApproveInvoice"},
			{from: "approved",  to: "paid",      via: "ProcessPayment"},
			{from: "*",         to: "cancelled", via: "CancelInvoice"},
		]
	}
}
