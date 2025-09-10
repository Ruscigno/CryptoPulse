-- CryptoPulse dYdX Order Routing Service - Initial Database Schema
-- Migration: 001_initial_schema.sql
-- Description: Create initial tables for orders and order status history

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Create enum types for order-related fields
CREATE TYPE order_side AS ENUM ('BUY', 'SELL');
CREATE TYPE order_type AS ENUM ('MARKET', 'LIMIT', 'STOP_MARKET', 'STOP_LIMIT', 'TAKE_PROFIT_MARKET', 'TAKE_PROFIT_LIMIT');
CREATE TYPE order_status AS ENUM ('PENDING', 'OPEN', 'FILLED', 'CANCELLED', 'REJECTED', 'EXPIRED', 'PARTIALLY_FILLED');
CREATE TYPE time_in_force AS ENUM ('GTT', 'FOK', 'IOC');

-- Orders table - stores all order information
CREATE TABLE orders (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    
    -- Order identification
    client_id VARCHAR(255) NOT NULL UNIQUE,
    order_id VARCHAR(255) UNIQUE, -- dYdX order ID (set after successful placement)
    
    -- Order details
    market VARCHAR(50) NOT NULL,
    side order_side NOT NULL,
    type order_type NOT NULL,
    size DECIMAL(20, 8) NOT NULL CHECK (size > 0),
    price DECIMAL(20, 8), -- NULL for market orders
    
    -- dYdX specific fields
    quantums BIGINT, -- Size in quantums (dYdX internal representation)
    subticks BIGINT, -- Price in subticks (dYdX internal representation)
    
    -- Order execution parameters
    time_in_force time_in_force DEFAULT 'GTT',
    good_til_block INTEGER, -- Block number until which order is valid
    good_til_block_time TIMESTAMP, -- Time until which order is valid
    
    -- Order status and execution
    status order_status NOT NULL DEFAULT 'PENDING',
    filled_size DECIMAL(20, 8) DEFAULT 0 CHECK (filled_size >= 0),
    remaining_size DECIMAL(20, 8) DEFAULT 0 CHECK (remaining_size >= 0),
    average_fill_price DECIMAL(20, 8),
    
    -- Transaction information
    tx_hash VARCHAR(64), -- Transaction hash from blockchain
    block_height BIGINT, -- Block height where transaction was included
    
    -- Fees and costs
    maker_fee DECIMAL(20, 8) DEFAULT 0,
    taker_fee DECIMAL(20, 8) DEFAULT 0,
    gas_used BIGINT,
    gas_fee DECIMAL(20, 8),
    
    -- Error handling
    error_message TEXT,
    retry_count INTEGER DEFAULT 0,
    
    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    placed_at TIMESTAMP WITH TIME ZONE, -- When order was successfully placed on dYdX
    filled_at TIMESTAMP WITH TIME ZONE, -- When order was completely filled
    cancelled_at TIMESTAMP WITH TIME ZONE, -- When order was cancelled
    
    -- Constraints
    CONSTRAINT valid_filled_size CHECK (filled_size <= size),
    CONSTRAINT valid_remaining_size CHECK (remaining_size <= size),
    CONSTRAINT price_required_for_limit_orders CHECK (
        (type IN ('MARKET', 'STOP_MARKET', 'TAKE_PROFIT_MARKET') AND price IS NULL) OR
        (type IN ('LIMIT', 'STOP_LIMIT', 'TAKE_PROFIT_LIMIT') AND price IS NOT NULL)
    )
);

-- Order status history table - tracks all status changes
CREATE TABLE order_status_history (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    
    -- Status change details
    old_status order_status,
    new_status order_status NOT NULL,
    
    -- Additional context
    filled_size DECIMAL(20, 8),
    remaining_size DECIMAL(20, 8),
    fill_price DECIMAL(20, 8),
    
    -- Transaction details for this status change
    tx_hash VARCHAR(64),
    block_height BIGINT,
    
    -- Reason for status change
    reason TEXT,
    
    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    -- Ensure we don't have duplicate status changes
    UNIQUE(order_id, old_status, new_status, created_at)
);

-- Create indexes for better query performance
CREATE INDEX idx_orders_client_id ON orders(client_id);
CREATE INDEX idx_orders_order_id ON orders(order_id);
CREATE INDEX idx_orders_market ON orders(market);
CREATE INDEX idx_orders_status ON orders(status);
CREATE INDEX idx_orders_side ON orders(side);
CREATE INDEX idx_orders_type ON orders(type);
CREATE INDEX idx_orders_created_at ON orders(created_at);
CREATE INDEX idx_orders_updated_at ON orders(updated_at);
CREATE INDEX idx_orders_tx_hash ON orders(tx_hash);
CREATE INDEX idx_orders_market_status ON orders(market, status);
CREATE INDEX idx_orders_status_created_at ON orders(status, created_at);

CREATE INDEX idx_order_status_history_order_id ON order_status_history(order_id);
CREATE INDEX idx_order_status_history_new_status ON order_status_history(new_status);
CREATE INDEX idx_order_status_history_created_at ON order_status_history(created_at);

-- Create function to automatically update the updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create trigger to automatically update updated_at on orders table
CREATE TRIGGER update_orders_updated_at 
    BEFORE UPDATE ON orders 
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();

-- Create function to automatically log status changes
CREATE OR REPLACE FUNCTION log_order_status_change()
RETURNS TRIGGER AS $$
BEGIN
    -- Only log if status actually changed
    IF OLD.status IS DISTINCT FROM NEW.status THEN
        INSERT INTO order_status_history (
            order_id,
            old_status,
            new_status,
            filled_size,
            remaining_size,
            fill_price,
            tx_hash,
            block_height,
            reason
        ) VALUES (
            NEW.id,
            OLD.status,
            NEW.status,
            NEW.filled_size,
            NEW.remaining_size,
            NEW.average_fill_price,
            NEW.tx_hash,
            NEW.block_height,
            CASE 
                WHEN NEW.status = 'FILLED' THEN 'Order completely filled'
                WHEN NEW.status = 'PARTIALLY_FILLED' THEN 'Order partially filled'
                WHEN NEW.status = 'CANCELLED' THEN 'Order cancelled'
                WHEN NEW.status = 'REJECTED' THEN COALESCE(NEW.error_message, 'Order rejected')
                WHEN NEW.status = 'EXPIRED' THEN 'Order expired'
                ELSE 'Status changed'
            END
        );
    END IF;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create trigger to automatically log status changes
CREATE TRIGGER log_order_status_change_trigger
    AFTER UPDATE ON orders
    FOR EACH ROW
    EXECUTE FUNCTION log_order_status_change();

-- Insert initial data or configuration if needed
-- (This section can be used for reference data, configuration, etc.)

-- Create a view for active orders (commonly queried)
CREATE VIEW active_orders AS
SELECT 
    id,
    client_id,
    order_id,
    market,
    side,
    type,
    size,
    price,
    status,
    filled_size,
    remaining_size,
    average_fill_price,
    created_at,
    updated_at,
    placed_at
FROM orders 
WHERE status IN ('PENDING', 'OPEN', 'PARTIALLY_FILLED');

-- Create a view for order summary statistics
CREATE VIEW order_statistics AS
SELECT 
    market,
    side,
    type,
    status,
    COUNT(*) as order_count,
    SUM(size) as total_size,
    SUM(filled_size) as total_filled_size,
    AVG(price) as average_price,
    MIN(created_at) as first_order_at,
    MAX(created_at) as last_order_at
FROM orders 
GROUP BY market, side, type, status;

-- Grant permissions (adjust as needed for your application user)
-- GRANT SELECT, INSERT, UPDATE, DELETE ON orders TO cryptopulse_app;
-- GRANT SELECT, INSERT ON order_status_history TO cryptopulse_app;
-- GRANT SELECT ON active_orders TO cryptopulse_app;
-- GRANT SELECT ON order_statistics TO cryptopulse_app;
