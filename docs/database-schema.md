# Database Schema Documentation

This document describes the database schema for the dYdX Order Routing Service.

## Overview

The service uses PostgreSQL as its primary database with the following design principles:

- **ACID Compliance**: All operations maintain data consistency
- **Audit Trail**: Complete history of order status changes
- **Performance**: Optimized indexes for common query patterns
- **Scalability**: Designed to handle high-frequency trading operations

## Database Configuration

### Connection Settings

```sql
-- Recommended PostgreSQL settings for production
shared_preload_libraries = 'pg_stat_statements'
max_connections = 100
shared_buffers = 256MB
effective_cache_size = 1GB
maintenance_work_mem = 64MB
checkpoint_completion_target = 0.9
wal_buffers = 16MB
default_statistics_target = 100
random_page_cost = 1.1
effective_io_concurrency = 200
```

### Extensions

```sql
-- Required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pg_stat_statements";
```

## Schema Definition

### Custom Types

```sql
-- Order side enumeration
CREATE TYPE order_side AS ENUM ('BUY', 'SELL');

-- Order type enumeration
CREATE TYPE order_type AS ENUM ('MARKET', 'LIMIT', 'STOP_MARKET', 'STOP_LIMIT', 'TAKE_PROFIT_MARKET', 'TAKE_PROFIT_LIMIT');

-- Order status enumeration
CREATE TYPE order_status AS ENUM ('PENDING', 'OPEN', 'FILLED', 'CANCELLED', 'REJECTED', 'EXPIRED', 'PARTIALLY_FILLED');

-- Time in force enumeration
CREATE TYPE time_in_force AS ENUM ('GTT', 'FOK', 'IOC');

-- Order status enumeration
CREATE TYPE order_status AS ENUM (
    'PENDING',           -- Order created but not yet submitted
    'OPEN',              -- Order submitted and active
    'FILLED',            -- Order completely filled
    'PARTIALLY_FILLED',  -- Order partially filled
    'CANCELLED',         -- Order cancelled by user
    'REJECTED',          -- Order rejected by exchange
    'EXPIRED'            -- Order expired (GTT orders)
);

-- Time in force enumeration
CREATE TYPE time_in_force AS ENUM ('GTT', 'FOK', 'IOC');
```

### Tables

#### 1. orders

Primary table for storing order information.

```sql
CREATE TABLE orders (
    -- Primary identifiers
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    client_id VARCHAR(255) NOT NULL UNIQUE,
    order_id VARCHAR(255) UNIQUE, -- dYdX order ID (set after placement)
    
    -- Order details
    market VARCHAR(50) NOT NULL,
    side order_side NOT NULL,
    type order_type NOT NULL,
    size DECIMAL(20, 8) NOT NULL CHECK (size > 0),
    price DECIMAL(20, 8) CHECK (price > 0 OR type = 'MARKET'),
    
    -- Order state
    status order_status NOT NULL DEFAULT 'PENDING',
    filled_size DECIMAL(20, 8) NOT NULL DEFAULT 0 CHECK (filled_size >= 0),
    remaining_size DECIMAL(20, 8) NOT NULL CHECK (remaining_size >= 0),
    
    -- Order parameters
    time_in_force time_in_force NOT NULL DEFAULT 'GTT',
    good_til_block INTEGER,
    reduce_only BOOLEAN NOT NULL DEFAULT FALSE,
    post_only BOOLEAN NOT NULL DEFAULT FALSE,
    
    -- Transaction details
    tx_hash VARCHAR(255),
    block_height BIGINT,
    gas_used BIGINT,
    gas_wanted BIGINT,
    
    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    placed_at TIMESTAMP WITH TIME ZONE,
    filled_at TIMESTAMP WITH TIME ZONE,
    cancelled_at TIMESTAMP WITH TIME ZONE,
    
    -- Error handling
    error_message TEXT,
    retry_count INTEGER NOT NULL DEFAULT 0,
    
    -- Metadata
    metadata JSONB,
    
    -- Constraints
    CONSTRAINT valid_filled_size CHECK (filled_size <= size),
    CONSTRAINT valid_remaining_size CHECK (remaining_size = size - filled_size),
    CONSTRAINT price_required_for_limit CHECK (
        (type = 'LIMIT' AND price IS NOT NULL) OR 
        (type = 'MARKET' AND price IS NULL)
    )
);
```

#### 2. order_status_history

Audit trail for order status changes.

```sql
CREATE TABLE order_status_history (
    -- Primary key
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    
    -- Foreign key to orders table
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    
    -- Status change details
    old_status order_status,
    new_status order_status NOT NULL,
    
    -- Change context
    reason VARCHAR(255),
    tx_hash VARCHAR(255),
    block_height BIGINT,
    
    -- Additional data
    filled_size DECIMAL(20, 8),
    remaining_size DECIMAL(20, 8),
    fill_price DECIMAL(20, 8),
    
    -- Timestamp
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    
    -- Metadata
    metadata JSONB
);
```

### Indexes

```sql
-- Primary indexes for orders table
CREATE INDEX idx_orders_client_id ON orders(client_id);
CREATE INDEX idx_orders_order_id ON orders(order_id) WHERE order_id IS NOT NULL;
CREATE INDEX idx_orders_status ON orders(status);
CREATE INDEX idx_orders_market ON orders(market);
CREATE INDEX idx_orders_created_at ON orders(created_at);
CREATE INDEX idx_orders_updated_at ON orders(updated_at);

-- Composite indexes for common queries
CREATE INDEX idx_orders_market_status ON orders(market, status);
CREATE INDEX idx_orders_status_created_at ON orders(status, created_at);
CREATE INDEX idx_orders_market_side_status ON orders(market, side, status);

-- Partial indexes for active orders
CREATE INDEX idx_orders_active ON orders(market, created_at) 
WHERE status IN ('PENDING', 'OPEN', 'PARTIALLY_FILLED');

-- Indexes for order_status_history table
CREATE INDEX idx_order_status_history_order_id ON order_status_history(order_id);
CREATE INDEX idx_order_status_history_created_at ON order_status_history(created_at);
CREATE INDEX idx_order_status_history_new_status ON order_status_history(new_status);
```

### Triggers

```sql
-- Trigger to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_orders_updated_at 
    BEFORE UPDATE ON orders 
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();

-- Trigger to create status history entries
CREATE OR REPLACE FUNCTION create_order_status_history()
RETURNS TRIGGER AS $$
BEGIN
    -- Only create history entry if status actually changed
    IF OLD.status IS DISTINCT FROM NEW.status THEN
        INSERT INTO order_status_history (
            order_id,
            old_status,
            new_status,
            filled_size,
            remaining_size,
            tx_hash,
            block_height,
            reason
        ) VALUES (
            NEW.id,
            OLD.status,
            NEW.status,
            NEW.filled_size,
            NEW.remaining_size,
            NEW.tx_hash,
            NEW.block_height,
            CASE 
                WHEN NEW.status = 'FILLED' THEN 'Order completely filled'
                WHEN NEW.status = 'PARTIALLY_FILLED' THEN 'Order partially filled'
                WHEN NEW.status = 'CANCELLED' THEN 'Order cancelled'
                WHEN NEW.status = 'REJECTED' THEN 'Order rejected'
                ELSE 'Status updated'
            END
        );
    END IF;
    
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER create_order_status_history_trigger
    AFTER UPDATE ON orders
    FOR EACH ROW
    EXECUTE FUNCTION create_order_status_history();
```

## Data Access Patterns

### Common Queries

#### 1. Get Active Orders for a Market

```sql
SELECT id, client_id, market, side, type, size, price, status, created_at
FROM orders 
WHERE market = $1 
  AND status IN ('PENDING', 'OPEN', 'PARTIALLY_FILLED')
ORDER BY created_at DESC;
```

#### 2. Get Order History with Pagination

```sql
SELECT id, client_id, market, side, type, size, price, status, 
       filled_size, remaining_size, created_at, updated_at
FROM orders 
WHERE ($1::VARCHAR IS NULL OR market = $1)
  AND ($2::order_status IS NULL OR status = $2)
ORDER BY created_at DESC 
LIMIT $3 OFFSET $4;
```

#### 3. Get Order Status with History

```sql
SELECT 
    o.*,
    COALESCE(
        json_agg(
            json_build_object(
                'old_status', h.old_status,
                'new_status', h.new_status,
                'timestamp', h.created_at,
                'reason', h.reason
            ) ORDER BY h.created_at
        ) FILTER (WHERE h.id IS NOT NULL),
        '[]'::json
    ) as history
FROM orders o
LEFT JOIN order_status_history h ON o.id = h.order_id
WHERE o.id = $1
GROUP BY o.id;
```

#### 4. Count Orders by Status

```sql
SELECT status, COUNT(*) as count
FROM orders 
WHERE created_at >= $1
GROUP BY status;
```

### Performance Considerations

#### Query Optimization

1. **Use Appropriate Indexes**: Ensure queries use the composite indexes
2. **Limit Result Sets**: Always use LIMIT for paginated queries
3. **Filter Early**: Apply WHERE clauses on indexed columns first
4. **Avoid SELECT \***: Only select needed columns

#### Connection Pooling

```go
// Recommended connection pool settings
db.SetMaxOpenConns(25)
db.SetMaxIdleConns(5)
db.SetConnMaxLifetime(5 * time.Minute)
```

## Maintenance

### Regular Maintenance Tasks

#### 1. Update Statistics

```sql
-- Run weekly
ANALYZE orders;
ANALYZE order_status_history;
```

#### 2. Vacuum Tables

```sql
-- Run daily during low-traffic periods
VACUUM ANALYZE orders;
VACUUM ANALYZE order_status_history;
```

#### 3. Archive Old Data

```sql
-- Archive orders older than 90 days
CREATE TABLE orders_archive (LIKE orders INCLUDING ALL);

WITH archived_orders AS (
    DELETE FROM orders 
    WHERE created_at < NOW() - INTERVAL '90 days'
      AND status IN ('FILLED', 'CANCELLED', 'REJECTED', 'EXPIRED')
    RETURNING *
)
INSERT INTO orders_archive SELECT * FROM archived_orders;
```

### Monitoring Queries

#### 1. Check Table Sizes

```sql
SELECT 
    schemaname,
    tablename,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) as size
FROM pg_tables 
WHERE schemaname = 'public'
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;
```

#### 2. Monitor Index Usage

```sql
SELECT 
    schemaname,
    tablename,
    indexname,
    idx_scan,
    idx_tup_read,
    idx_tup_fetch
FROM pg_stat_user_indexes 
ORDER BY idx_scan DESC;
```

#### 3. Check Slow Queries

```sql
SELECT 
    query,
    calls,
    total_time,
    mean_time,
    rows
FROM pg_stat_statements 
WHERE query LIKE '%orders%'
ORDER BY mean_time DESC 
LIMIT 10;
```

## Backup and Recovery

### Backup Strategy

```bash
# Daily full backup
pg_dump -h localhost -U cryptopulse -d cryptopulse \
    --format=custom --compress=9 \
    --file=cryptopulse_$(date +%Y%m%d).backup

# Continuous WAL archiving
archive_command = 'cp %p /backup/wal/%f'
```

### Point-in-Time Recovery

```bash
# Restore from backup
pg_restore -h localhost -U cryptopulse -d cryptopulse_restore \
    --clean --if-exists cryptopulse_20240115.backup

# Apply WAL files for point-in-time recovery
recovery_target_time = '2024-01-15 10:30:00'
```

## Security

### Access Control

```sql
-- Create application user with limited privileges
CREATE USER cryptopulse_app WITH PASSWORD 'secure_password';

-- Grant necessary permissions
GRANT CONNECT ON DATABASE cryptopulse TO cryptopulse_app;
GRANT USAGE ON SCHEMA public TO cryptopulse_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON orders TO cryptopulse_app;
GRANT SELECT, INSERT ON order_status_history TO cryptopulse_app;
GRANT USAGE ON ALL SEQUENCES IN SCHEMA public TO cryptopulse_app;
```

### Row Level Security (Optional)

```sql
-- Enable RLS for multi-tenant scenarios
ALTER TABLE orders ENABLE ROW LEVEL SECURITY;

-- Create policy (example for multi-tenant setup)
CREATE POLICY orders_tenant_policy ON orders
    FOR ALL TO cryptopulse_app
    USING (tenant_id = current_setting('app.tenant_id'));
```
