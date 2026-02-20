package inmem

import (
	"context"
	"fmt"
	"math/rand/v2"
	"sync"
)

type InvoiceRepo struct {
	mu       sync.RWMutex
	invoices map[string]*invoice
}

type invoice struct {
	id         string
	status     string
	balance    float64
	currency   string
	customerID string
}

func NewInvoiceRepo() *InvoiceRepo {
	return &InvoiceRepo{
		invoices: map[string]*invoice{
			"inv_001": {id: "inv_001", status: "approved", balance: 1500.00, currency: "USD", customerID: "cust_123"},
			"inv_002": {id: "inv_002", status: "draft", balance: 250.00, currency: "USD", customerID: "cust_123"},
			"inv_003": {id: "inv_003", status: "approved", balance: 25000.00, currency: "USD", customerID: "cust_456"},
		},
	}
}

func (r *InvoiceRepo) Get(_ context.Context, fact string, input map[string]any) (any, error) {
	id, _ := input["invoice.id"].(string)
	if id == "" {
		return nil, fmt.Errorf("invoice.id missing from input")
	}

	r.mu.RLock()
	inv, ok := r.invoices[id]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("invoice %q not found", id)
	}

	switch fact {
	case "invoice.balance":
		return map[string]any{"value": inv.balance, "currency": inv.currency}, nil
	case "invoice.status":
		return inv.status, nil
	default:
		return nil, fmt.Errorf("unknown fact %q", fact)
	}
}

func (r *InvoiceRepo) Execute(_ context.Context, operation string, input map[string]any) (map[string]any, error) {
	id, _ := input["invoice.id"].(string)
	if id == "" {
		return nil, fmt.Errorf("invoice.id missing")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	inv, ok := r.invoices[id]
	if !ok {
		return nil, fmt.Errorf("invoice %q not found", id)
	}

	switch operation {
	case "ProcessPayment":
		amount, _ := input["payment.amount"].(map[string]any)
		if amount == nil {
			return nil, fmt.Errorf("payment.amount missing")
		}
		inv.status = "paid"
		inv.balance = 0
		return map[string]any{
			"payment_id":  "pay_" + randString(8),
			"status":      "completed",
			"new_balance": map[string]any{"value": 0, "currency": inv.currency},
		}, nil

	case "GetInvoice":
		return map[string]any{
			"id":          inv.id,
			"status":      inv.status,
			"balance":     map[string]any{"value": inv.balance, "currency": inv.currency},
			"customer_id": inv.customerID,
		}, nil

	default:
		return nil, fmt.Errorf("unknown operation %q", operation)
	}
}

func randString(n int) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = chars[rand.IntN(len(chars))]
	}
	return string(b)
}
