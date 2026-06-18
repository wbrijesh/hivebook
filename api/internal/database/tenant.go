package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"

	"api/internal/database/gen"
)

// Tenant is one customer workspace — one ZITADEL organization (design-doc 0005).
type Tenant struct {
	ID           string
	OrgID        string
	Name         *string
	Size         *string
	Region       *string
	UseCases     []string
	UseCaseOther *string // free-text detail when "other" is among UseCases
	OnboardedAt  *string // RFC3339 timestamp, nil until onboarding completes
}

// ErrRegionImmutable is returned when onboarding tries to change an already-set
// storage region (write-once, ADR-0014).
var ErrRegionImmutable = errors.New("region is set and cannot be changed")

func strPtr(s sql.NullString) *string {
	if !s.Valid {
		return nil
	}
	v := s.String
	return &v
}

// tenantRow is the shape both generated tenant rows share; mapping is one place.
func toTenant(id, org string, name, size, region, useCaseOther sql.NullString, useCases json.RawMessage, onboardedAt sql.NullTime) Tenant {
	t := Tenant{
		ID:           id,
		OrgID:        org,
		Name:         strPtr(name),
		Size:         strPtr(size),
		Region:       strPtr(region),
		UseCaseOther: strPtr(useCaseOther),
		UseCases:     []string{},
	}
	if len(useCases) > 0 {
		if err := json.Unmarshal(useCases, &t.UseCases); err != nil {
			// Shouldn't happen — the column is written as a JSON array by this
			// package — but a corrupt value shouldn't crash the read; log and
			// fall back to empty.
			slog.Error("tenant_use_cases_unmarshal_failed", "tenant_id", id, "error", err.Error())
		}
	}
	if t.UseCases == nil {
		t.UseCases = []string{}
	}
	if onboardedAt.Valid {
		v := onboardedAt.Time.UTC().Format("2006-01-02T15:04:05Z07:00")
		t.OnboardedAt = &v
	}
	return t
}

// GetOrCreateTenant returns the tenant for a ZITADEL org id, creating an empty
// (un-onboarded) row the first time an org is seen. Two statements (insert-if-
// absent, then select) rather than one upsert-returning, for readability — it is
// race-safe via the unique constraint on zitadel_org_id, not via ordering.
func (s *service) GetOrCreateTenant(ctx context.Context, orgID string) (Tenant, error) {
	if err := s.q.CreateTenantIfAbsent(ctx, orgID); err != nil {
		return Tenant{}, err
	}
	r, err := s.q.GetTenantByOrg(ctx, orgID)
	if err != nil {
		return Tenant{}, err
	}
	return toTenant(r.ID, r.ZitadelOrgID, r.Name, r.Size, r.Region, r.UseCaseOther, r.UseCases, r.OnboardedAt), nil
}

// CompleteOnboarding records the onboarding answers and stamps onboarded_at.
// Idempotent. region is write-once (ADR-0014): the upsert only updates when the
// stored region is unset or equals the requested one, so a conflicting region
// returns no row — which we surface as ErrRegionImmutable. Atomic and race-safe;
// no partial write on conflict.
func (s *service) CompleteOnboarding(ctx context.Context, orgID, name, size, region string, useCases []string, useCaseOther string) (Tenant, error) {
	if useCases == nil {
		useCases = []string{}
	}
	useCasesJSON, _ := json.Marshal(useCases)

	r, err := s.q.CompleteOnboarding(ctx, gen.CompleteOnboardingParams{
		ZitadelOrgID: orgID,
		Name:         sql.NullString{String: name, Valid: true},
		Size:         sql.NullString{String: size, Valid: true},
		Region:       sql.NullString{String: region, Valid: true},
		UseCases:     useCasesJSON,
		UseCaseOther: useCaseOther, // NULLIF(...,'') → NULL when empty
	})
	if errors.Is(err, sql.ErrNoRows) {
		return Tenant{}, ErrRegionImmutable
	}
	if err != nil {
		return Tenant{}, err
	}
	return toTenant(r.ID, r.ZitadelOrgID, r.Name, r.Size, r.Region, r.UseCaseOther, r.UseCases, r.OnboardedAt), nil
}

// UpdateTenantProfile edits the mutable workspace profile (name, size, use cases)
// from settings. Region is never touched — write-once (ADR-0014). The tenant must
// already exist; sql.ErrNoRows surfaces if the org is unknown.
func (s *service) UpdateTenantProfile(ctx context.Context, orgID, name, size string, useCases []string, useCaseOther string) (Tenant, error) {
	if useCases == nil {
		useCases = []string{}
	}
	useCasesJSON, _ := json.Marshal(useCases)

	r, err := s.q.UpdateTenantProfile(ctx, gen.UpdateTenantProfileParams{
		ZitadelOrgID: orgID,
		Name:         sql.NullString{String: name, Valid: true},
		Size:         sql.NullString{String: size, Valid: true},
		UseCases:     useCasesJSON,
		UseCaseOther: useCaseOther, // NULLIF(...,'') → NULL when empty
	})
	if err != nil {
		return Tenant{}, err
	}
	return toTenant(r.ID, r.ZitadelOrgID, r.Name, r.Size, r.Region, r.UseCaseOther, r.UseCases, r.OnboardedAt), nil
}
