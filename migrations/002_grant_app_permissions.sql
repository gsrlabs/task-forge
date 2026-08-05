-- +goose Up
-- +goose StatementBegin

GRANT USAGE ON SCHEMA public
    TO taskforge_app;

GRANT SELECT, INSERT, UPDATE, DELETE
    ON ALL TABLES IN SCHEMA public
    TO taskforge_app;

GRANT USAGE, SELECT
    ON ALL SEQUENCES IN SCHEMA public
    TO taskforge_app;

ALTER DEFAULT PRIVILEGES FOR ROLE taskforge_migrator
IN SCHEMA public
GRANT SELECT, INSERT, UPDATE, DELETE
    ON TABLES TO taskforge_app;

ALTER DEFAULT PRIVILEGES FOR ROLE taskforge_migrator
IN SCHEMA public
GRANT USAGE, SELECT
    ON SEQUENCES TO taskforge_app;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

REVOKE SELECT, INSERT, UPDATE, DELETE
    ON ALL TABLES IN SCHEMA public
    FROM taskforge_app;

REVOKE USAGE, SELECT
    ON ALL SEQUENCES IN SCHEMA public
    FROM taskforge_app;

REVOKE USAGE
    ON SCHEMA public
    FROM taskforge_app;

-- +goose StatementEnd