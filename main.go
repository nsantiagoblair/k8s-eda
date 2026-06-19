// Chapter 1 — Go Basics
//
// This program exercises the core trading domain model without any HTTP or
// message broker. The goal is to get comfortable with the types and behaviour
// we'll be building on throughout the tutorial.
package main

import (
	"errors"
	"fmt"
	"log"

	"github.com/nick-blair/k8s-eda/trade"
)

func main() {
	store := trade.NewInMemoryStore()

	fmt.Println("=== Placing trades ===")

	// Place a few trades and save them to the store.
	orders := []struct {
		asset  string
		side   trade.Side
		qty    int
		limit  float64
	}{
		{"AAPL", trade.Buy, 10, 195.00},
		{"TSLA", trade.Buy, 5, 250.00},
		{"NVDA", trade.Sell, 3, 890.00},
		{"MSFT", trade.Buy, 0, 420.00}, // invalid — quantity must be positive
	}

	var placed []*trade.Trade
	for _, o := range orders {
		t, err := trade.New(o.asset, o.side, o.qty, o.limit)
		if err != nil {
			fmt.Printf("  ✗ failed to create trade for %s: %v\n", o.asset, err)
			continue
		}
		if err := store.Save(t); err != nil {
			log.Fatalf("unexpected store error: %v", err)
		}
		fmt.Printf("  ✓ %s\n", t)
		placed = append(placed, t)
	}

	fmt.Println("\n=== Advancing trade lifecycle ===")

	if len(placed) > 0 {
		simulateLifecycle(store, placed[0], trade.Fulfilled)
	}
	if len(placed) > 1 {
		simulateLifecycle(store, placed[1], trade.Rejected)
	}

	fmt.Println("\n=== Attempting an invalid transition ===")

	if len(placed) > 0 {
		// placed[0] is already Fulfilled — trying to submit it again should fail.
		err := placed[0].Transition(trade.Submitted)
		if err != nil {
			fmt.Printf("  ✗ transition blocked (expected): %v\n", err)
		}
	}

	fmt.Println("\n=== Looking up a trade by ID ===")

	if len(placed) > 0 {
		found, err := store.FindByID(placed[0].ID)
		if err != nil {
			log.Fatalf("unexpected error: %v", err)
		}
		fmt.Printf("  found: %s\n", found)
	}

	fmt.Println("\n=== Looking up a non-existent trade ===")

	_, err := store.FindByID("does-not-exist")
	var notFound *trade.ErrNotFound
	if errors.As(err, &notFound) {
		fmt.Printf("  ✗ not found (expected): %v\n", err)
	}

	fmt.Println("\n=== All trades by status ===")

	printByStatus(store, trade.Pending)
	printByStatus(store, trade.Fulfilled)
	printByStatus(store, trade.Rejected)
}

// simulateLifecycle moves a trade through PENDING → SUBMITTED → final.
func simulateLifecycle(store trade.Store, t *trade.Trade, final trade.Status) {
	steps := []trade.Status{trade.Submitted, final}
	for _, next := range steps {
		if err := t.Transition(next); err != nil {
			fmt.Printf("  ✗ %s → %s failed: %v\n", t.Asset, next, err)
			return
		}
		if err := store.Save(t); err != nil {
			log.Fatalf("unexpected store error: %v", err)
		}
		fmt.Printf("  ✓ %s → %s\n", t.Asset, next)
	}
}

func printByStatus(store trade.Store, status trade.Status) {
	trades, err := store.FindByStatus(status)
	if err != nil {
		log.Fatalf("unexpected error: %v", err)
	}
	fmt.Printf("  %s (%d):\n", status, len(trades))
	for _, t := range trades {
		fmt.Printf("    • %s\n", t)
	}
}
