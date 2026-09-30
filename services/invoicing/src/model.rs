use chrono::{DateTime, Utc};
use rust_decimal::Decimal;
use serde::{Deserialize, Serialize};

/// InvoiceLine represents an individual billed stock item with exact decimal price and totals.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct InvoiceLine {
    pub id: i64,
    pub invoice_id: String,
    pub product_id: i64,
    pub product_name: String,
    pub unit_price: Decimal,
    pub quantity: i32,
    pub amount: Decimal,
}

/// Invoice represents the master billing record for a customer transaction.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Invoice {
    pub id: i64,
    pub tenant_id: String,
    pub transaction_id: String,
    pub customer_id: String,
    pub customer_name: String,
    pub date_time: DateTime<Utc>,
    pub total: Decimal,
    pub total_discount: Decimal,
    pub packaging: Decimal,
    pub freight: Decimal,
    pub taxable_amount: Decimal,
    pub tax_collected_at_source: Decimal,
    pub round_off: Decimal,
    pub grand_total: Decimal,
    pub method_of_payment: String,
    pub lines: Vec<InvoiceLine>,
    pub created_at: DateTime<Utc>,
}

/// Generates a sequential, timestamped transaction identifier (e.g., txn_20260930120000001)
pub fn generate_transaction_id(counter: usize) -> String {
    let now = Utc::now();
    let date_str = now.format("%Y%m%d%H%M%S").to_string();
    format!("txn_{}_{:04}", date_str, counter)
}
