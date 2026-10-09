-- Baseline: the starting point of the schema history.
--
-- It deliberately creates no product tables. It exists so that every
-- environment records a migration version from day one, and so the migration
-- mechanism (including rolling back) is exercised by the integration tests
-- before any real table depends on it.

-- +goose Up
COMMENT ON SCHEMA public IS 'platform-backend: schema managed by goose migrations in db/migrations';

-- +goose Down
COMMENT ON SCHEMA public IS NULL;
