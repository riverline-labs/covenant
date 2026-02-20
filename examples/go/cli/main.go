package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

func main() {
	op := flag.String("op", "", "Operation name (e.g. ProcessPayment, GetInvoice)")
	customerID := flag.String("customer", "cust_123", "Customer ID")
	invoiceID := flag.String("invoice", "inv_001", "Invoice ID")
	amount := flag.Float64("amount", 100.0, "Payment amount (USD)")
	dryRun := flag.Bool("dry-run", false, "Dry run — evaluate rules only, no side effects")
	executorURL := flag.String("executor", "http://localhost:26860", "Executor base URL")
	contractURL := flag.String("contracts", "http://localhost:26861", "Contract server base URL")
	flag.Parse()

	if *op == "" {
		fmt.Fprintln(os.Stderr, "Error: --op is required")
		fmt.Fprintln(os.Stderr, "\nOperations: ProcessPayment, GetInvoice")
		flag.Usage()
		os.Exit(1)
	}

	// Fetch discovery so we know the contract ETag.
	disc, err := fetchDiscovery(*contractURL)
	if err != nil {
		log.Fatalf("Contract server unreachable: %v", err)
	}
	fmt.Printf("Service:  %s\n", disc.Service)
	fmt.Printf("ETag:     %s\n", disc.ContractETag)
	fmt.Printf("Persona:  %s\n\n", disc.Persona)

	// Build input based on operation.
	input := map[string]any{
		"customer.id": *customerID,
		"invoice.id":  *invoiceID,
	}
	if *op == "ProcessPayment" {
		input["payment.amount"] = map[string]any{
			"value":    *amount,
			"currency": "USD",
		}
	}

	req := map[string]any{
		"operation":     *op,
		"input":         input,
		"dry_run":       *dryRun,
		"contract_etag": disc.ContractETag,
	}

	if *dryRun {
		fmt.Printf("Dry run: %s\n", *op)
	} else {
		fmt.Printf("Executing: %s\n", *op)
	}

	resp, err := execute(*executorURL, req)
	if err != nil {
		log.Fatalf("Executor error: %v", err)
	}

	printResponse(resp)
}

type discoveryDoc struct {
	Service      string `json:"service"`
	ContractETag string `json:"contract_etag"`
	Persona      string `json:"persona"`
}

func fetchDiscovery(baseURL string) (*discoveryDoc, error) {
	resp, err := http.Get(baseURL + "/.well-known/covenant")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var d discoveryDoc
	return &d, json.NewDecoder(resp.Body).Decode(&d)
}

func execute(baseURL string, req map[string]any) (map[string]any, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(baseURL+"/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w (body: %s)", err, raw)
	}
	return result, nil
}

func printResponse(resp map[string]any) {
	outcome, _ := resp["outcome"].(string)

	switch outcome {
	case "executed":
		fmt.Println("✓ Executed")
		if out, ok := resp["output"]; ok {
			pretty, _ := json.MarshalIndent(out, "  ", "  ")
			fmt.Printf("  Output: %s\n", pretty)
		}

	case "denied":
		fmt.Println("✗ Denied")
		if e, ok := resp["error"].(map[string]any); ok {
			fmt.Printf("  Code:    %v\n", e["code"])
			fmt.Printf("  Message: %v\n", e["message"])
			if s, ok := e["suggestion"]; ok && s != "" {
				fmt.Printf("  Hint:    %v\n", s)
			}
		}

	case "would_execute", "would_deny", "would_escalate", "would_execute_with_flags":
		fmt.Printf("Dry-run outcome: %s\n", outcome)
		if verdicts, ok := resp["verdicts"].([]any); ok && len(verdicts) > 0 {
			fmt.Println("  Rules matched:")
			for _, v := range verdicts {
				vm, _ := v.(map[string]any)
				fmt.Printf("    [%v] %v\n", vm["type"], vm["reason"])
			}
		}
		if outcome == "would_execute" || outcome == "would_execute_with_flags" {
			fmt.Println("  (would proceed to execution)")
		}

	default:
		fmt.Printf("Outcome: %s\n", outcome)
		if e, ok := resp["error"].(map[string]any); ok {
			fmt.Printf("  Error: %v\n", e["message"])
		}
	}

	// Always show flags if present alongside other outcomes.
	if verdicts, ok := resp["verdicts"].([]any); ok {
		for _, v := range verdicts {
			vm, _ := v.(map[string]any)
			if vm["type"] == "flag" {
				fmt.Printf("  Flag: [%v] %v\n", vm["code"], vm["reason"])
			}
		}
	}
}
