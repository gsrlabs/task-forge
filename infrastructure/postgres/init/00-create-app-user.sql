-- 00-create-app-user.sql

DO $$
BEGIN
   IF NOT EXISTS (
      SELECT FROM pg_roles WHERE rolname = 'taskforge_app'
   ) THEN
      CREATE ROLE taskforge_app
      WITH LOGIN PASSWORD 'taskforge_dev_password';
   END IF;
END
$$;

GRANT CONNECT ON DATABASE taskforge_db TO taskforge_app;