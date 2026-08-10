-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- ENUM TYPES
-- ============================================================

CREATE TYPE team_role AS ENUM (
    'owner',
    'admin',
    'member'
);

CREATE TYPE task_status AS ENUM (
    'todo',
    'in_progress',
    'review',
    'done'
);

CREATE TYPE task_history_action AS ENUM (
    'created',
    'updated',
    'deleted'
);

-- ============================================================
-- UPDATED_AT TRIGGER FUNCTION
-- ============================================================

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- ============================================================
-- 1. USERS
-- ============================================================

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email CITEXT NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TRIGGER users_updated_at
BEFORE UPDATE ON users
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- 2. TEAMS
-- ============================================================

CREATE TABLE teams (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,

    created_by UUID NOT NULL
        REFERENCES users(id)
        ON DELETE RESTRICT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TRIGGER teams_updated_at
BEFORE UPDATE ON teams
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();


-- ============================================================
-- 3. TEAM MEMBERS
-- ============================================================

CREATE TABLE team_members (
    team_id UUID NOT NULL
        REFERENCES teams(id)
        ON DELETE CASCADE,

    user_id UUID NOT NULL
        REFERENCES users(id)
        ON DELETE RESTRICT,

    role team_role NOT NULL DEFAULT 'member',
    joined_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (team_id, user_id)
);

-- ============================================================
-- 4. TASKS
-- ============================================================

CREATE TABLE tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id UUID NOT NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    status task_status NOT NULL DEFAULT 'todo',
    assignee_id UUID,
    created_by UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT tasks_title_not_empty
        CHECK (length(trim(title)) > 0),

    CONSTRAINT fk_tasks_team
        FOREIGN KEY (team_id)
        REFERENCES teams(id)
        ON DELETE CASCADE,

    CONSTRAINT fk_tasks_created_by_member
        FOREIGN KEY (team_id, created_by)
        REFERENCES team_members(team_id, user_id)
        ON DELETE RESTRICT,

    CONSTRAINT fk_tasks_assignee_member
        FOREIGN KEY (team_id, assignee_id)
        REFERENCES team_members(team_id, user_id)
        ON DELETE RESTRICT
);

CREATE TRIGGER tasks_updated_at
BEFORE UPDATE ON tasks
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- 5. TASK HISTORY / AUDIT
-- ============================================================

CREATE TABLE task_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    task_id UUID NOT NULL
        REFERENCES tasks(id)
        ON DELETE CASCADE,

    changed_by UUID NOT NULL
        REFERENCES users(id)
        ON DELETE RESTRICT,

    action task_history_action NOT NULL,
    changes JSONB NOT NULL,
    changed_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- ============================================================
-- 6. TASK COMMENTS
-- ============================================================

CREATE TABLE task_comments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    task_id UUID NOT NULL
        REFERENCES tasks(id)
        ON DELETE CASCADE,

    user_id UUID NOT NULL
        REFERENCES users(id)
        ON DELETE RESTRICT,

    content TEXT NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT task_comments_content_not_empty
        CHECK (length(trim(content)) > 0)
);

CREATE TRIGGER task_comments_updated_at
BEFORE UPDATE ON task_comments
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- INDEXES
-- ============================================================

-- ------------------------------------------------------------
-- team_members
-- ------------------------------------------------------------

CREATE INDEX idx_team_members_user_id
    ON team_members(user_id);

-- ------------------------------------------------------------
-- tasks: основные фильтры
-- ------------------------------------------------------------

CREATE INDEX idx_tasks_team_status_assignee
    ON tasks(team_id, status, assignee_id);

-- ------------------------------------------------------------
-- tasks: assignee
-- ------------------------------------------------------------

CREATE INDEX idx_tasks_assignee_id
    ON tasks(assignee_id);

-- ------------------------------------------------------------
-- tasks: creator
-- ------------------------------------------------------------

CREATE INDEX idx_tasks_created_by
    ON tasks(created_by);

CREATE INDEX idx_tasks_done_updated_team
    ON tasks(updated_at, team_id)
    WHERE status = 'done';

-- ------------------------------------------------------------
-- tasks: Top-3 пользователей по созданным задачам за месяц
-- ------------------------------------------------------------

CREATE INDEX idx_tasks_created_at_team_creator
    ON tasks(created_at, team_id, created_by);

-- ------------------------------------------------------------
-- task_history
-- ------------------------------------------------------------

CREATE INDEX idx_task_history_task_time
    ON task_history(task_id, changed_at DESC);

-- ------------------------------------------------------------
-- task_comments
-- ------------------------------------------------------------

CREATE INDEX idx_task_comments_task_time
    ON task_comments(task_id, created_at ASC);

-- ============================================================
-- PERMISSIONS
-- ============================================================

  GRANT USAGE ON SCHEMA public
      TO taskforge_app;

  GRANT SELECT, INSERT, UPDATE, DELETE
      ON ALL TABLES IN SCHEMA public
      TO taskforge_app;

-- +goose StatementEnd

-- ============================================================
-- DOWN
-- ============================================================

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS task_comments;
DROP TABLE IF EXISTS task_history;
DROP TABLE IF EXISTS tasks;
DROP TABLE IF EXISTS team_members;
DROP TABLE IF EXISTS teams;
DROP TABLE IF EXISTS users;

DROP FUNCTION IF EXISTS set_updated_at();

DROP TYPE IF EXISTS task_history_action;
DROP TYPE IF EXISTS task_status;
DROP TYPE IF EXISTS team_role;


-- +goose StatementEnd
