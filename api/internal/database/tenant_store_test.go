package database

import (
	"context"
	"errors"
	"sync"
	"testing"
)

// newStore opens a service against the TestMain container (migrations run on New)
// and closes it when the test ends. Tests isolate by a unique org id per test.
func newStore(t *testing.T) Service {
	t.Helper()
	srv, err := New(testCfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	return srv
}

func TestGetOrCreateTenant(t *testing.T) {
	srv := newStore(t)
	ctx := context.Background()
	org := "org_" + t.Name()

	tn, err := srv.GetOrCreateTenant(ctx, org)
	if err != nil {
		t.Fatal(err)
	}
	if tn.ID == "" {
		t.Fatal("expected a generated id")
	}
	if tn.OnboardedAt != nil {
		t.Fatal("a fresh tenant must not be onboarded")
	}
	if tn.Name != nil {
		t.Fatal("a fresh tenant has no name")
	}
	if len(tn.UseCases) != 0 {
		t.Fatalf("a fresh tenant has no use cases, got %v", tn.UseCases)
	}

	again, err := srv.GetOrCreateTenant(ctx, org)
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != tn.ID {
		t.Fatalf("get-or-create must be idempotent: %s != %s", again.ID, tn.ID)
	}
}

func TestCompleteOnboarding(t *testing.T) {
	srv := newStore(t)
	ctx := context.Background()
	org := "org_" + t.Name()

	tn, err := srv.CompleteOnboarding(ctx, org, "Acme", "11-50", "us-east", []string{"support", "other"}, "custom thing")
	if err != nil {
		t.Fatal(err)
	}
	if tn.OnboardedAt == nil {
		t.Fatal("tenant should be onboarded")
	}
	if tn.Name == nil || *tn.Name != "Acme" {
		t.Fatalf("name not persisted: %v", tn.Name)
	}
	if tn.Region == nil || *tn.Region != "us-east" {
		t.Fatalf("region not persisted: %v", tn.Region)
	}
	if tn.UseCaseOther == nil || *tn.UseCaseOther != "custom thing" {
		t.Fatalf("use_case_other not persisted: %v", tn.UseCaseOther)
	}
	if len(tn.UseCases) != 2 {
		t.Fatalf("use_cases did not round-trip: %v", tn.UseCases)
	}

	stamp := *tn.OnboardedAt
	// Re-submit (same region) is idempotent: onboarded_at stamped once; fields update;
	// use_case_other clears when "other" is no longer selected.
	tn2, err := srv.CompleteOnboarding(ctx, org, "Acme Inc", "51-200", "us-east", []string{"support"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if *tn2.OnboardedAt != stamp {
		t.Fatalf("onboarded_at must be stamped once: %s != %s", *tn2.OnboardedAt, stamp)
	}
	if tn2.Name == nil || *tn2.Name != "Acme Inc" {
		t.Fatalf("name should update: %v", tn2.Name)
	}
	if tn2.UseCaseOther != nil {
		t.Fatalf("use_case_other should clear: %v", tn2.UseCaseOther)
	}
}

func TestCompleteOnboarding_RegionWriteOnce(t *testing.T) {
	srv := newStore(t)
	ctx := context.Background()
	org := "org_" + t.Name()

	if _, err := srv.CompleteOnboarding(ctx, org, "Acme", "11-50", "us-east", []string{"support"}, ""); err != nil {
		t.Fatal(err)
	}
	_, err := srv.CompleteOnboarding(ctx, org, "Acme", "11-50", "eu-central", []string{"support"}, "")
	if !errors.Is(err, ErrRegionImmutable) {
		t.Fatalf("expected ErrRegionImmutable, got %v", err)
	}

	tn, err := srv.GetOrCreateTenant(ctx, org)
	if err != nil {
		t.Fatal(err)
	}
	if tn.Region == nil || *tn.Region != "us-east" {
		t.Fatalf("region must stay us-east, got %v", tn.Region)
	}
}

// TestGetOrCreateTenant_Concurrent proves the insert-if-absent + select is
// race-safe: many goroutines racing on a brand-new org all resolve to the same
// single row, none error (the unique constraint, not ordering, guarantees it).
func TestGetOrCreateTenant_Concurrent(t *testing.T) {
	srv := newStore(t)
	ctx := context.Background()
	org := "org_" + t.Name()

	const n = 16
	ids := make([]string, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := range n {
		go func(i int) {
			defer wg.Done()
			tn, err := srv.GetOrCreateTenant(ctx, org)
			ids[i], errs[i] = tn.ID, err
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d errored: %v", i, err)
		}
		if ids[i] != ids[0] {
			t.Fatalf("concurrent get-or-create returned different ids: %s != %s", ids[i], ids[0])
		}
	}
}

// TestCompleteOnboarding_ConcurrentRegions proves region write-once holds under a
// race: many goroutines onboard the same fresh org with different regions at once;
// exactly one region may ever succeed, every other attempt gets ErrRegionImmutable,
// and the stored region is that one winner.
func TestCompleteOnboarding_ConcurrentRegions(t *testing.T) {
	srv := newStore(t)
	ctx := context.Background()
	org := "org_" + t.Name()

	regions := []string{"us-east", "eu-central", "asia-south", "us-west"}
	const perRegion = 4

	type res struct {
		region string
		err    error
	}
	results := make(chan res, len(regions)*perRegion)
	var wg sync.WaitGroup
	for _, r := range regions {
		for range perRegion {
			wg.Add(1)
			go func(region string) {
				defer wg.Done()
				_, err := srv.CompleteOnboarding(ctx, org, "Acme", "11-50", region, []string{"support"}, "")
				results <- res{region, err}
			}(r)
		}
	}
	wg.Wait()
	close(results)

	winners := map[string]bool{}
	for r := range results {
		switch {
		case r.err == nil:
			winners[r.region] = true
		case errors.Is(r.err, ErrRegionImmutable):
			// expected for the losing regions
		default:
			t.Fatalf("unexpected error: %v", r.err)
		}
	}
	if len(winners) != 1 {
		t.Fatalf("write-once violated: more than one region succeeded: %v", winners)
	}

	tn, err := srv.GetOrCreateTenant(ctx, org)
	if err != nil {
		t.Fatal(err)
	}
	if tn.Region == nil || !winners[*tn.Region] {
		t.Fatalf("stored region %v is not the winning region %v", tn.Region, winners)
	}
}

func TestCompleteOnboarding_FromScratch(t *testing.T) {
	srv := newStore(t)
	ctx := context.Background()
	org := "org_" + t.Name()

	// Onboarding an org with no prior get-or-create still works (upsert), and a nil
	// use-case list round-trips to empty.
	tn, err := srv.CompleteOnboarding(ctx, org, "New", "1-10", "asia-south", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if tn.OnboardedAt == nil {
		t.Fatal("tenant should be onboarded")
	}
	if len(tn.UseCases) != 0 {
		t.Fatalf("nil use_cases should round-trip to empty, got %v", tn.UseCases)
	}
}
