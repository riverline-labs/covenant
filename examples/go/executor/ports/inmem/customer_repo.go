package inmem

import (
	"context"
	"fmt"
	"sync"
)

type CustomerRepo struct {
	mu        sync.RWMutex
	customers map[string]customer
}

type customer struct {
	id     string
	status string // "active" | "suspended" | "closed"
}

func NewCustomerRepo() *CustomerRepo {
	return &CustomerRepo{
		customers: map[string]customer{
			"cust_123": {id: "cust_123", status: "active"},
			"cust_456": {id: "cust_456", status: "closed"},
			"cust_789": {id: "cust_789", status: "suspended"},
		},
	}
}

func (r *CustomerRepo) Get(_ context.Context, fact string, input map[string]any) (any, error) {
	id, _ := input["customer.id"].(string)
	if id == "" {
		return nil, fmt.Errorf("customer.id missing from input")
	}

	r.mu.RLock()
	c, ok := r.customers[id]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("customer %q not found", id)
	}

	switch fact {
	case "customer.status":
		return c.status, nil
	default:
		return nil, fmt.Errorf("unknown fact %q", fact)
	}
}

func (r *CustomerRepo) Execute(_ context.Context, operation string, _ map[string]any) (map[string]any, error) {
	return nil, fmt.Errorf("customerRepo does not execute operation %q", operation)
}
