DROP TRIGGER IF EXISTS update_orders_updated_at ON orders;
DROP FUNCTION IF EXISTS update_updated_at_column();

DROP TABLE IF EXISTS withdrawals;
DROP TABLE IF EXISTS balances;
DROP TABLE IF EXISTS orders;
DROP TABLE IF EXISTS users;