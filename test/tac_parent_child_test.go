package test

import (
	"context"
	"fmt"
	"testing"

	"github.com/hsdfat8/eir/internal/adapters/memory"
	"github.com/hsdfat8/eir/internal/domain/ports"
	"github.com/hsdfat8/eir/internal/domain/service"
	"github.com/hsdfat8/eir/internal/logger"
)

func TestTacParentChildLinking(t *testing.T) {
	// Initialize logger
	_ = logger.New("test", "debug")

	// Create in-memory repository
	repo := memory.NewInMemoryIMEIRepository()

	// Create EIR service (with nil audit and cache repos for testing)
	eirService := service.NewEIRService(repo, nil, nil)

	ctx := context.Background()

	fmt.Println("\n=== Test: TAC Parent-Child Linking ===")
	fmt.Println("\nStep 1: Insert child range 133-135 (black)")

	// Insert child range first
	childTacInfo := &ports.TacInfo{
		StartRangeTac: "133",
		EndRangeTac:   "135",
		Color:         "black",
	}

	result1, err := eirService.InsertTac(ctx, childTacInfo)
	if err != nil {
		t.Fatalf("Failed to insert child TAC: %v", err)
	}

	fmt.Printf("Result: Status=%s, Error=%v\n", result1.Status, result1.Error)

	// Check stored TAC info
	allTacs := repo.ListAllTacInfo(ctx)
	fmt.Printf("\nTotal TAC records inserted: %d\n", len(allTacs))
	for _, tac := range allTacs {
		prevLink := "nil"
		if tac.PrevLink != nil {
			prevLink = *tac.PrevLink
		}
		fmt.Printf("  KeyTac: %s, StartRange: %s, EndRange: %s, Color: %s, PrevLink: %s\n",
			tac.KeyTac, tac.StartRangeTac, tac.EndRangeTac, tac.Color, prevLink)
	}

	fmt.Println("\n\nStep 2: Insert parent range 133-139 (grey)")

	// Insert parent range that encompasses the child
	parentTacInfo := &ports.TacInfo{
		StartRangeTac: "133",
		EndRangeTac:   "139",
		Color:         "grey",
	}

	result2, err := eirService.InsertTac(ctx, parentTacInfo)
	if err != nil {
		t.Fatalf("Failed to insert parent TAC: %v", err)
	}

	fmt.Printf("Result: Status=%s, Error=%v\n", result2.Status, result2.Error)

	// Check stored TAC info again
	allTacs = repo.ListAllTacInfo(ctx)
	fmt.Printf("\nTotal TAC records inserted: %d\n", len(allTacs))
	for _, tac := range allTacs {
		prevLink := "nil"
		if tac.PrevLink != nil {
			prevLink = *tac.PrevLink
		}
		fmt.Printf("  KeyTac: %s, StartRange: %s, EndRange: %s, Color: %s, PrevLink: %s\n",
			tac.KeyTac, tac.StartRangeTac, tac.EndRangeTac, tac.Color, prevLink)
	}

	fmt.Println("\n\nStep 3: Verify child's PrevLink points to parent")

	// Verify the child TAC now has PrevLink pointing to parent
	childKey := "133             -135ÿÿÿÿÿÿÿÿÿÿÿÿÿ"
	parentKey := "133             -139ÿÿÿÿÿÿÿÿÿÿÿÿÿ"

	childTac, found := repo.LookupTacInfo(ctx, childKey)
	if !found {
		t.Fatalf("Child TAC not found with key: %s", childKey)
	}

	if childTac.PrevLink == nil {
		t.Errorf("FAILED: Child TAC (133-135) should have PrevLink pointing to parent (133-139), but PrevLink is nil")
		fmt.Println("FAILED: Child's PrevLink is nil")
	} else if *childTac.PrevLink != parentKey {
		t.Errorf("FAILED: Child TAC PrevLink = %s, expected %s", *childTac.PrevLink, parentKey)
		fmt.Printf("FAILED: Child's PrevLink = %s, expected %s\n", *childTac.PrevLink, parentKey)
	} else {
		fmt.Printf("SUCCESS: Child TAC (133-135) has PrevLink pointing to parent (133-139)\n")
		fmt.Printf("  Child PrevLink: %s\n", *childTac.PrevLink)
		fmt.Printf("  Expected:       %s\n", parentKey)
	}
}
