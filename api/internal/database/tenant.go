package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
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

// ErrRegionImmutable is returned when onboarding tries to change an
// already-set storage region.
var ErrRegionImmutable = errors.New("region is set and cannot be changed")

const tenantCols = `id, zitadel_org_id, name, size, region, use_cases, use_case_other, onboarded_at`

func scanTenant(row interface{ Scan(...any) error }) (Tenant, error) {
	var (
		t            Tenant
		name, size   sql.NullString
		region       sql.NullString
		useCasesJSON []byte
		useCaseOther sql.NullString
		onboardedAt  sql.NullTime
	)
	if err := row.Scan(&t.ID, &t.OrgID, &name, &size, &region, &useCasesJSON, &useCaseOther, &onboardedAt); err != nil {
		return Tenant{}, err
	}
	if name.Valid {
		t.Name = &name.String
	}
	if size.Valid {
		t.Size = &size.String
	}
	if region.Valid {
		t.Region = &region.String
	}
	if useCaseOther.Valid {
		t.UseCaseOther = &useCaseOther.String
	}
	if onboardedAt.Valid {
		v := onboardedAt.Time.UTC().Format("2006-01-02T15:04:05Z07:00")
		t.OnboardedAt = &v
	}
	if len(useCasesJSON) > 0 {
		_ = json.Unmarshal(useCasesJSON, &t.UseCases)
	}
	if t.UseCases == nil {
		t.UseCases = []string{}
	}
	return t, nil
}

// GetOrCreateTenant returns the tenant for a ZITADEL org id, creating an empty
// (un-onboarded) row the first time an org is seen.
func (s *service) GetOrCreateTenant(ctx context.Context, orgID string) (Tenant, error) {
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO tenants (zitadel_org_id) VALUES ($1) ON CONFLICT (zitadel_org_id) DO NOTHING`,
		orgID); err != nil {
		return Tenant{}, err
	}
	row := s.db.QueryRowContext(ctx,
		`SELECT `+tenantCols+` FROM tenants WHERE zitadel_org_id = $1`, orgID)
	return scanTenant(row)
}

// CompleteOnboarding records the onboarding answers and stamps onboarded_at.
// Idempotent. region is write-once: a different region is rejected; the same (or
// first) region is accepted.
func (s *service) CompleteOnboarding(ctx context.Context, orgID, name, size, region string, useCases []string, useCaseOther string) (Tenant, error) {
	var existing sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT region FROM tenants WHERE zitadel_org_id = $1`, orgID).Scan(&existing)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Tenant{}, err
	}
	if existing.Valid && existing.String != "" && existing.String != region {
		return Tenant{}, ErrRegionImmutable
	}

	useCasesJSON, _ := json.Marshal(useCases)
	row := s.db.QueryRowContext(ctx,
		`INSERT INTO tenants (zitadel_org_id, name, size, region, use_cases, use_case_other, onboarded_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), now(), now())
		 ON CONFLICT (zitadel_org_id) DO UPDATE SET
		     name           = EXCLUDED.name,
		     size           = EXCLUDED.size,
		     region         = COALESCE(tenants.region, EXCLUDED.region),
		     use_cases      = EXCLUDED.use_cases,
		     use_case_other = EXCLUDED.use_case_other,
		     onboarded_at   = COALESCE(tenants.onboarded_at, now()),
		     updated_at     = now()
		 RETURNING `+tenantCols,
		orgID, name, size, region, useCasesJSON, useCaseOther)
	return scanTenant(row)
}
