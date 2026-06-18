-- Onboarding-flow redesign (design-doc 0012). Two changes:
--
-- 1. Newly discovered sync_units default to NOT selected. The user opts in to what
--    they want to sync rather than syncing everything on connect. UpsertSyncUnit
--    relies on this column default (its INSERT never names `selected`), so flipping
--    the default flips the behaviour for new units; re-discovery never touches the
--    column, preserving the user's choice.
-- 2. `onboarded` flags a connection whose source the user has configured. It flips
--    true the first time a selection is saved (SetProjectSelection). The frontend
--    routes a not-onboarded connection to its manage page, an onboarded one to its
--    details page.
ALTER TABLE sync_unit ALTER COLUMN selected SET DEFAULT false;
ALTER TABLE connector_connection ADD COLUMN onboarded boolean NOT NULL DEFAULT false;
