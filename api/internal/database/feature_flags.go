package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"api/internal/database/gen"
)

// GetFeatureFlags returns a tenant's flag overrides (key → enabled). The catalog
// of which flags exist and their defaults lives in the server; this is only the
// tenant's deviations. Returns an empty map when the tenant or column is unset.
func (s *service) GetFeatureFlags(ctx context.Context, orgID string) (map[string]bool, error) {
	raw, err := s.q.GetFeatureFlags(ctx, orgID)
	if errors.Is(err, sql.ErrNoRows) {
		return map[string]bool{}, nil
	}
	if err != nil {
		return nil, err
	}
	flags := map[string]bool{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &flags); err != nil {
			return nil, err
		}
	}
	return flags, nil
}

// SetFeatureFlag sets one flag override for the tenant (a deviation from default).
func (s *service) SetFeatureFlag(ctx context.Context, orgID, key string, enabled bool) error {
	return s.q.SetFeatureFlag(ctx, gen.SetFeatureFlagParams{
		ZitadelOrgID: orgID, Key: key, Enabled: enabled,
	})
}

// RemoveFeatureFlag drops a flag override so the tenant falls back to the default.
func (s *service) RemoveFeatureFlag(ctx context.Context, orgID, key string) error {
	return s.q.RemoveFeatureFlag(ctx, gen.RemoveFeatureFlagParams{
		ZitadelOrgID: orgID, Key: key,
	})
}
