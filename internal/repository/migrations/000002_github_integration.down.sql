DROP TABLE IF EXISTS app_databases;
DROP TABLE IF EXISTS user_tokens;
DROP INDEX IF EXISTS idx_apps_owner_login;
DROP INDEX IF EXISTS idx_apps_github_full_name;
ALTER TABLE apps
    DROP COLUMN IF EXISTS owner_login,
    DROP COLUMN IF EXISTS auto_deploy,
    DROP COLUMN IF EXISTS webhook_id,
    DROP COLUMN IF EXISTS github_full_name;
