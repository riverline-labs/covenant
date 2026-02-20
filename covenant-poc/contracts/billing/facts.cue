// Billing domain facts.
// No package declaration — compiled and unified by the executor.

facts: {
	"customer.id": {
		source:   "input"
		required: true
	}
	"invoice.id": {
		source:   "input"
		required: true
	}
	"payment.amount": {
		source:   "input"
		required: true
	}
	"customer.status": {
		source:     "port:customerRepo"
		required:   true
		on_missing: "deny"
	}
	"invoice.balance": {
		source:     "port:invoiceRepo"
		required:   true
		on_missing: "system_error"
	}
	"invoice.status": {
		source:     "port:invoiceRepo"
		required:   true
		on_missing: "system_error"
	}
	"payment.processor.status": {
		source:     "port:paymentProcessor"
		required:   true
		on_missing: "deny"
	}
}

derived_facts: {
	"payment.exceeds_balance": {
		derivation: {
			fn: "greater_than"
			args: [
				{fact: "payment.amount.value"},
				{fact: "invoice.balance.value"},
			]
		}
	}
	"customer.can_pay": {
		derivation: {
			fn: "and"
			args: [
				{fact: "customer.status", op: "equals", value: "active"},
				{fact: "payment.processor.status", op: "equals", value: "up"},
			]
		}
	}
}
