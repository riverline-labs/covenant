package engine

import (
	"fmt"
	"sync"
	"testing"
)

func TestFactSet_SetGet_returnsStoredValue(t *testing.T) {
	fs := NewFactSet()
	fs.Set("foo", 42)
	got, ok := fs.Get("foo")
	if !ok {
		t.Fatal("expected fact to be found")
	}
	if got != 42 {
		t.Fatalf("expected 42, got %v", got)
	}
}

func TestFactSet_Get_missingReturnsFalse(t *testing.T) {
	fs := NewFactSet()
	_, ok := fs.Get("missing")
	if ok {
		t.Fatal("expected false for missing fact")
	}
}

func TestFactSet_SetGet_concurrent(t *testing.T) {
	fs := NewFactSet()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := fmt.Sprintf("fact.%d", n)
			fs.Set(key, n)
			fs.Get(key)
		}(i)
	}
	wg.Wait()
}

func TestFactSet_GetPath_exactMatch(t *testing.T) {
	fs := NewFactSet()
	fs.Set("payment.amount", 500.0)
	got, ok := fs.GetPath("payment.amount")
	if !ok {
		t.Fatal("expected to find exact match")
	}
	if got != 500.0 {
		t.Fatalf("expected 500.0, got %v", got)
	}
}

func TestFactSet_GetPath_navigatesIntoNestedMap(t *testing.T) {
	fs := NewFactSet()
	fs.Set("payment", map[string]any{
		"amount": map[string]any{"value": 500, "currency": "USD"},
	})
	got, ok := fs.GetPath("payment.amount.value")
	if !ok {
		t.Fatal("expected to navigate into nested map")
	}
	if got != 500 {
		t.Fatalf("expected 500, got %v", got)
	}
}

func TestFactSet_GetPath_prefixFactWithNavigation(t *testing.T) {
	fs := NewFactSet()
	fs.Set("payment.amount", map[string]any{"value": 250, "currency": "EUR"})
	got, ok := fs.GetPath("payment.amount.currency")
	if !ok {
		t.Fatal("expected to find currency via prefix navigation")
	}
	if got != "EUR" {
		t.Fatalf("expected EUR, got %v", got)
	}
}

func TestFactSet_GetPath_notFoundReturnsFalse(t *testing.T) {
	fs := NewFactSet()
	got, ok := fs.GetPath("nonexistent.path")
	if ok {
		t.Fatalf("expected not found, got %v", got)
	}
}

func TestFactSet_GetPath_navigationIntoNonMapReturnsFalse(t *testing.T) {
	fs := NewFactSet()
	fs.Set("amount", 42.0) // scalar, not a map
	_, ok := fs.GetPath("amount.value")
	if ok {
		t.Fatal("expected false when navigating into a non-map fact")
	}
}

func TestFactSet_Snapshot_returnsIndependentCopy(t *testing.T) {
	fs := NewFactSet()
	fs.Set("a", 1)
	snap := fs.Snapshot()
	snap["a"] = 999 // mutate the copy
	got, _ := fs.Get("a")
	if got == 999 {
		t.Fatal("snapshot mutation should not affect original FactSet")
	}
}

func TestFactSet_Snapshot_containsAllFacts(t *testing.T) {
	fs := NewFactSet()
	fs.Set("x", "hello")
	fs.Set("y", 42)
	snap := fs.Snapshot()
	if snap["x"] != "hello" || snap["y"] != 42 {
		t.Fatalf("snapshot missing facts: got %v", snap)
	}
}
