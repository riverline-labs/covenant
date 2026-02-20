package inmem

import (
	"context"
	"fmt"
	"sync"
)

type PaymentProcessor struct {
	mu     sync.RWMutex
	status string // "up" | "down"
}

func NewPaymentProcessor() *PaymentProcessor {
	return &PaymentProcessor{status: "up"}
}

// SetStatus lets tests simulate processor downtime.
func (p *PaymentProcessor) SetStatus(s string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.status = s
}

func (p *PaymentProcessor) Get(_ context.Context, fact string, _ map[string]any) (any, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	switch fact {
	case "payment.processor.status":
		return p.status, nil
	default:
		return nil, fmt.Errorf("unknown fact %q", fact)
	}
}

func (p *PaymentProcessor) Execute(_ context.Context, operation string, _ map[string]any) (map[string]any, error) {
	return nil, fmt.Errorf("paymentProcessor does not execute operation %q", operation)
}
