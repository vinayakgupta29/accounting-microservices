-- Multi-Database Initialization Script for accounting-microservices

CREATE DATABASE auth_db;
CREATE DATABASE customer_db;
CREATE DATABASE inventory_db;
CREATE DATABASE invoice_db;

\c auth_db;

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    username VARCHAR(50) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    gstin VARCHAR(100) NOT NULL,
    pan_card VARCHAR(50) NOT NULL,
    aadhaar VARCHAR(50) NOT NULL,
    phone VARCHAR(15) NOT NULL,
    address TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);

\c customer_db;

CREATE TABLE IF NOT EXISTS customers (
    id BIGSERIAL PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL,
    cust_id VARCHAR(64) NOT NULL,
    name VARCHAR(255) NOT NULL,
    address TEXT NOT NULL,
    phone_number VARCHAR(15) NOT NULL,
    email VARCHAR(255) NOT NULL,
    gstin VARCHAR(50) NOT NULL,
    dealer_type VARCHAR(50) NOT NULL DEFAULT 'Regular',
    pan_card VARCHAR(50) NOT NULL,
    aadhaar VARCHAR(50) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_tenant_cust_id UNIQUE (tenant_id, cust_id),
    CONSTRAINT uq_tenant_gstin UNIQUE (tenant_id, gstin)
);
CREATE INDEX IF NOT EXISTS idx_customers_tenant ON customers(tenant_id);

\c inventory_db;

CREATE TABLE IF NOT EXISTS inventory_items (
    id BIGSERIAL PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL,
    product_name VARCHAR(255) NOT NULL,
    quantity INT NOT NULL DEFAULT 0,
    unit_price NUMERIC(12, 2) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_positive_qty CHECK (quantity >= 0)
);
CREATE INDEX IF NOT EXISTS idx_inventory_tenant ON inventory_items(tenant_id);

CREATE TABLE IF NOT EXISTS stock_reservations (
    reservation_id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL,
    product_id BIGINT NOT NULL,
    quantity INT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'RESERVED',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (product_id) REFERENCES inventory_items(id)
);

\c invoice_db;

CREATE TABLE IF NOT EXISTS invoices (
    id BIGSERIAL PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL,
    transaction_id VARCHAR(64) NOT NULL,
    customer_id VARCHAR(64) NOT NULL,
    customer_name VARCHAR(255) NOT NULL,
    date_time TIMESTAMP WITH TIME ZONE NOT NULL,
    total NUMERIC(12, 2) NOT NULL DEFAULT 0,
    total_discount NUMERIC(12, 2) NOT NULL DEFAULT 0,
    packaging NUMERIC(12, 2) NOT NULL DEFAULT 0,
    freight NUMERIC(12, 2) NOT NULL DEFAULT 0,
    taxable_amount NUMERIC(12, 2) NOT NULL DEFAULT 0,
    tax_collected_at_source NUMERIC(12, 2) NOT NULL DEFAULT 0,
    round_off NUMERIC(12, 2) NOT NULL DEFAULT 0,
    grand_total NUMERIC(12, 2) NOT NULL DEFAULT 0,
    method_of_payment VARCHAR(50) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_tenant_txn UNIQUE (tenant_id, transaction_id)
);
CREATE INDEX IF NOT EXISTS idx_invoices_tenant_date ON invoices(tenant_id, date_time);

CREATE TABLE IF NOT EXISTS invoice_lines (
    id BIGSERIAL PRIMARY KEY,
    invoice_id VARCHAR(64) NOT NULL,
    tenant_id VARCHAR(64) NOT NULL,
    product_id BIGINT NOT NULL,
    product_name VARCHAR(255) NOT NULL,
    unit_price NUMERIC(12, 2) NOT NULL,
    quantity INT NOT NULL,
    amount NUMERIC(12, 2) NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_invoice_lines_inv ON invoice_lines(invoice_id);
