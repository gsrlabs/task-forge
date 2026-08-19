-- 000-create-app-user.sql

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT FROM pg_roles
        WHERE rolname = 'taskforge_app'
    ) THEN
        CREATE ROLE taskforge_app
            WITH LOGIN
            PASSWORD 'postgres_dev_password';
    END IF;

    IF NOT EXISTS (
        SELECT FROM pg_roles
        WHERE rolname = 'taskforge_migrator'
    ) THEN
        CREATE ROLE taskforge_migrator
            WITH LOGIN
            PASSWORD 'migrator_dev_password';
    END IF;
END
$$;

CREATE EXTENSION IF NOT EXISTS citext;

GRANT CONNECT ON DATABASE taskforge_db
    TO taskforge_app;

GRANT CONNECT ON DATABASE taskforge_db
    TO taskforge_migrator;

GRANT USAGE, CREATE ON SCHEMA public
    TO taskforge_migrator;
