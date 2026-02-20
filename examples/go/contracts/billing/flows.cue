// Billing domain flows — persona-scoped sequences of operations.

flows: [
	{
		id:      "pay-invoice"
		persona: "customer"
		goal:    "Pay an outstanding invoice"

		steps: [
			{
				operation: "GetInvoice"
				produces: {entity: "invoice", state: "approved"}
			},
			{
				operation: "ProcessPayment"
				requires: {entity: "invoice", state: "approved"}
				produces: {entity: "invoice", state: "paid"}
			},
		]
	},
]
