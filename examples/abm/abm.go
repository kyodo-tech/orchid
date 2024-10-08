// Copyright 2024 Kyodo Tech合同会社
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"math/rand"
	"os"
	"time"

	"github.com/kyodo-tech/orchid"
)

// Define the activities for the marketplace
const (
	SellActivity        = "sell"
	BuyActivity         = "buy"
	UpdatePriceActivity = "updatePrice"

	MaxIterations = 10
)

type MarketOffer struct {
	Price      float64 `json:"price"`
	Quantity   int     `json:"quantity"`
	Iterations int     `json:"iterations"`
}

func startActivity(ctx context.Context, input []byte) ([]byte, error) {
	// Start conditions
	return json.Marshal(MarketOffer{
		Price:      100.0, // Starting price
		Quantity:   20,    // Starting quantity
		Iterations: MaxIterations,
	})
}

func sellerActivity(ctx context.Context, input []byte) ([]byte, error) {
	var offer MarketOffer
	json.Unmarshal(input, &offer)

	// Simulate adding/removing quantity
	quantity := rand.Intn(5) - 2
	offer.Quantity += quantity // Randomly add or subtract quantity
	if offer.Quantity < 0 {
		offer.Quantity = 0 // Ensure quantity is non-negative
	}

	fmt.Printf("Seller creates an offer: %+v (quantity %+d)\n", offer, quantity)
	time.Sleep(300 * time.Millisecond)

	return json.Marshal(offer)
}

func priceUpdateActivity(ctx context.Context, input []byte) ([]byte, error) {
	var offer MarketOffer
	json.Unmarshal(input, &offer)

	// Simulate price fluctuation based on quantity
	if offer.Quantity > 10 {
		offer.Price *= 0.95 // Decrease price if oversupply
	} else if offer.Quantity < 5 {
		offer.Price *= 1.1 // Increase price if high demand
	} else {
		// Random small fluctuation if supply and demand are balanced
		fluctuation := 1.0 + (rand.Float64()/10.0 - 0.05)
		offer.Price *= fluctuation
	}

	fmt.Printf("Marketplace updates prices: %+v\n", offer)
	time.Sleep(300 * time.Millisecond)

	return json.Marshal(offer)
}

func buyerActivity(ctx context.Context, input []byte) ([]byte, error) {
	var offer MarketOffer
	json.Unmarshal(input, &offer)

	// Simulate a purchase
	buyQuantity := rand.Intn(3) + 1 // Random quantity to buy
	if offer.Quantity >= buyQuantity {
		offer.Quantity -= buyQuantity
	} else {
		offer.Quantity = 0 // Sold out
	}

	fmt.Printf("Buyer places an order: %+v (bought %d)\n", offer, buyQuantity)

	offer.Iterations--
	time.Sleep(300 * time.Millisecond)

	if offer.Iterations <= 0 || offer.Quantity <= 0 {
		// End loop if no iterations left and sold out
		return []byte{}, &orchid.DynamicRoute{Key: "complete"}
	}

	out, _ := json.Marshal(offer)
	return out, &orchid.DynamicRoute{Key: SellActivity}
}

func main() {
	// Initialize a new workflow
	wf := orchid.NewWorkflow("Marketplace Simulation")
	wf.AddNode(orchid.NewNode("start"))
	wf.AddNode(orchid.NewNode(SellActivity))
	wf.AddNode(orchid.NewNode(BuyActivity))
	wf.AddNode(orchid.NewNode(UpdatePriceActivity))
	wf.AddNode(orchid.NewNode("complete"))

	// Define the flow: Sell → Update Price → Buy
	wf.Link("start", SellActivity)
	wf.Link(SellActivity, UpdatePriceActivity)
	wf.Link(UpdatePriceActivity, BuyActivity)
	wf.Link(BuyActivity, SellActivity) // Loop back to create a cycle
	wf.Link(BuyActivity, "complete")

	// Set up the orchestrator
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	o := orchid.NewOrchestrator(
		orchid.WithLogger(logger),
	)
	o.LoadWorkflow(wf)
	o.RegisterActivity("start", startActivity)
	o.RegisterActivity(SellActivity, sellerActivity)
	o.RegisterActivity(BuyActivity, buyerActivity)
	o.RegisterActivity(UpdatePriceActivity, priceUpdateActivity)
	o.RegisterActivity("complete", func(ctx context.Context, input []byte) ([]byte, error) {
		fmt.Println("Marketplace simulation completed")
		return nil, nil
	})

	// Execute the workflow
	ctx := context.Background()
	ctx = orchid.WithWorkflowID(ctx, "marketplace-simulation")
	if _, err := o.Start(ctx, nil); err != nil {
		log.Fatalf("Workflow execution failed: %v", err)
	}
}
