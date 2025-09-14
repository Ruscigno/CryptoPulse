-- CryptoPulse dYdX Order Routing Service - Initial Database Schema
-- Migration: 001_initial_schema.down.sql
-- Description: Drop initial tables for orders and order status history

-- Drop triggers first
DROP TRIGGER IF EXISTS log_order_status_change_trigger ON orders;
DROP TRIGGER IF EXISTS update_orders_updated_at ON orders;

-- Drop functions
DROP FUNCTION IF EXISTS log_order_status_change();
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop views
DROP VIEW IF EXISTS order_statistics;
DROP VIEW IF EXISTS active_orders;

-- Drop indexes (they will be dropped automatically with tables, but explicit for clarity)
DROP INDEX IF EXISTS idx_order_status_history_created_at;
DROP INDEX IF EXISTS idx_order_status_history_new_status;
DROP INDEX IF EXISTS idx_order_status_history_order_id;

DROP INDEX IF EXISTS idx_orders_status_created_at;
DROP INDEX IF EXISTS idx_orders_market_status;
DROP INDEX IF EXISTS idx_orders_tx_hash;
DROP INDEX IF EXISTS idx_orders_updated_at;
DROP INDEX IF EXISTS idx_orders_created_at;
DROP INDEX IF EXISTS idx_orders_type;
DROP INDEX IF EXISTS idx_orders_side;
DROP INDEX IF EXISTS idx_orders_status;
DROP INDEX IF EXISTS idx_orders_market;
DROP INDEX IF EXISTS idx_orders_order_id;
DROP INDEX IF EXISTS idx_orders_client_id;

-- Drop tables (order matters due to foreign key constraints)
DROP TABLE IF EXISTS order_status_history;
DROP TABLE IF EXISTS orders;

-- Drop enum types
DROP TYPE IF EXISTS time_in_force;
DROP TYPE IF EXISTS order_status;
DROP TYPE IF EXISTS order_type;
DROP TYPE IF EXISTS order_side;

-- Drop extension (be careful with this in production)
-- DROP EXTENSION IF EXISTS "uuid-ossp";
