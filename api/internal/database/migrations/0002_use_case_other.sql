-- 0002 — free-text "Something else" use case (design-doc 0005).
-- When a tenant picks the "other" use case during onboarding, the description
-- they type is kept here rather than discarded. Null when "other" isn't chosen.
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS use_case_other text;
