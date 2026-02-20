package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"time"

	"covenant-poc/executor/engine"
	"covenant-poc/executor/ports"
	"covenant-poc/executor/ports/inmem"
)

func main() {
	contractServer := flag.String("contracts", "http://localhost:26861", "Contract server base URL")
	addr := flag.String("addr", ":26860", "Listen address")
	flag.Parse()

	// Build port registry.
	registry := ports.NewRegistry()
	registry.Register("customerRepo", inmem.NewCustomerRepo())
	registry.Register("paymentProcessor", inmem.NewPaymentProcessor())
	invoiceRepo := inmem.NewInvoiceRepo()
	registry.Register("invoiceRepo", invoiceRepo)

	eng := engine.NewEngine(registry)

	// Load contracts from the contract server.
	if err := refreshContracts(eng, *contractServer); err != nil {
		log.Fatalf("Initial contract load failed: %v", err)
	}

	// Poll for contract updates every 30 seconds.
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		for range ticker.C {
			if err := refreshContracts(eng, *contractServer); err != nil {
				log.Printf("Contract refresh error: %v", err)
			}
		}
	}()

	http.HandleFunc("POST /execute", func(w http.ResponseWriter, r *http.Request) {
		var req engine.Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		resp, err := eng.Evaluate(context.Background(), &req)
		if err != nil {
			log.Printf("eval error: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("encode error: %v", err)
		}

		log.Printf("op=%s outcome=%s dry_run=%v", req.Operation, resp.Outcome, req.DryRun)
	})

	log.Printf("Executor listening on %s (contracts: %s)", *addr, *contractServer)
	log.Fatal(http.ListenAndServe(*addr, nil))
}

func refreshContracts(eng *engine.Engine, serverURL string) error {
	disc, err := engine.FetchDiscovery(serverURL)
	if err != nil {
		return err
	}

	// Skip reload if ETag hasn't changed.
	if disc.ContractETag != "" && disc.ContractETag == eng.ETag() {
		return nil
	}

	contract, err := engine.LoadContract(serverURL, disc)
	if err != nil {
		return err
	}

	eng.LoadContract(contract, disc.ContractETag)
	log.Printf("Contracts loaded: etag=%s service=%s", disc.ContractETag, disc.Service)
	return nil
}
