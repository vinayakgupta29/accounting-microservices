use crate::model::Invoice;
use std::collections::HashMap;
use std::sync::Arc;
use tokio::sync::RwLock;

/// InvoiceStore defines an abstract interface for storing and querying invoices.
#[derive(Clone, Default)]
pub struct InvoiceStore {
    // tenant_id -> list of invoices
    invoices: Arc<RwLock<HashMap<String, Vec<Invoice>>>>,
}

impl InvoiceStore {
    pub fn new() -> Self {
        Self {
            invoices: Arc::new(RwLock::new(HashMap::new())),
        }
    }

    pub async fn insert(&self, invoice: Invoice) -> Result<Invoice, String> {
        let mut map = self.invoices.write().await;
        let list = map.entry(invoice.tenant_id.clone()).or_insert_with(Vec::new);
        list.push(invoice.clone());
        Ok(invoice)
    }

    pub async fn find_by_txn(&self, tenant_id: &str, txn_id: &str) -> Option<Invoice> {
        let map = self.invoices.read().await;
        if let Some(list) = map.get(tenant_id) {
            for inv in list {
                if inv.transaction_id == txn_id {
                    return Some(inv.clone());
                }
            }
        }
        None
    }

    pub async fn list(&self, tenant_id: &str) -> Vec<Invoice> {
        let map = self.invoices.read().await;
        map.get(tenant_id).cloned().unwrap_or_default()
    }

    pub async fn count(&self, tenant_id: &str) -> usize {
        let map = self.invoices.read().await;
        map.get(tenant_id).map(|l| l.len()).unwrap_or(0)
    }
}
