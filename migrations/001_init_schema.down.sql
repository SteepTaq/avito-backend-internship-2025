-- Удаление триггеров
DROP TRIGGER IF EXISTS update_users_updated_at ON users;

-- Удаление функций
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Удаление таблиц
DROP TABLE IF EXISTS pull_requests;
DROP TABLE IF EXISTS users;

