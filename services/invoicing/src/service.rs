pub mod invoice_v1 {
    tonic::include_proto!("invoice.v1");
}

pub mod customer_v1 {
    tonic::include_proto!("customer.v1");
}

pub mod inventory_v1 {
    tonic::include_proto!("inventory.v1");
}

use chrono::Utc;
use invoice_v1::invoice_service_server::InvoiceService;
use invoice_v1::*;
use rust_decimal::prelude::*;
use rust_decimal::Decimal;
use std::sync::atomic::{AtomicUsize, Ordering};
use std::sync::Arc;
use tokio_stream::wrappers::ReceiverStream;
use tonic::{Request, Response, Status};

use crate::db::InvoiceStore;
use crate::model;

pub struct InvoiceServiceImpl {
    store: InvoiceStore,
    customer_client_addr: String,
    inventory_client_addr: String,
    counter: Arc<AtomicUsize>,
}

impl InvoiceServiceImpl {
    pub fn new(
        store: InvoiceStore,
        customer_client_addr: String,
        inventory_client_addr: String,
    ) -> Self {
        Self {
            store,
            customer_client_addr,
            inventory_client_addr,
            counter: Arc::new(AtomicUsize::new(1)),
        }
    }
}

fn model_to_proto(inv: &model::Invoice) -> invoice_v1::Invoice {
    invoice_v1::Invoice {
        id: inv.id,
        tenant_id: inv.tenant_id.clone(),
        transaction_id: inv.transaction_id.clone(),
        customer_id: inv.customer_id.clone(),
        customer_name: inv.customer_name.clone(),
        date_time: inv.date_time.to_rfc3339(),
        total: inv.total.to_f64().unwrap_or(0.0),
        total_discount: inv.total_discount.to_f64().unwrap_or(0.0),
        packaging: inv.packaging.to_f64().unwrap_or(0.0),
        freight: inv.freight.to_f64().unwrap_or(0.0),
        taxable_amount: inv.taxable_amount.to_f64().unwrap_or(0.0),
        tax_collected_at_source: inv.tax_collected_at_source.to_f64().unwrap_or(0.0),
        round_off: inv.round_off.to_f64().unwrap_or(0.0),
        grand_total: inv.grand_total.to_f64().unwrap_or(0.0),
        method_of_payment: inv.method_of_payment.clone(),
        lines: inv
            .lines
            .iter()
            .map(|l| invoice_v1::InvoiceLine {
                id: l.id,
                invoice_id: l.invoice_id.clone(),
                product_id: l.product_id,
                product_name: l.product_name.clone(),
                unit_price: l.unit_price.to_f64().unwrap_or(0.0),
                quantity: l.quantity,
                amount: l.amount.to_f64().unwrap_or(0.0),
            })
            .collect(),
        created_at: inv.created_at.to_rfc3339(),
    }
}

#[tonic::async_trait]
impl InvoiceService for InvoiceServiceImpl {
    async fn create_invoice(
        &self,
        request: Request<CreateInvoiceRequest>,
    ) -> Result<Response<CreateInvoiceResponse>, Status> {
        let req = request.into_inner();
        let tenant_id = req.tenant_id.trim();
        let customer_id = req.customer_id.trim();

        if tenant_id.is_empty() || customer_id.is_empty() {
            return Err(Status::invalid_argument(
                "tenant_id and customer_id are required",
            ));
        }

        if req.lines.is_empty() {
            return Err(Status::invalid_argument(
                "at least one invoice line item is required",
            ));
        }

        // Step 1: Verify Customer exists via CustomerService gRPC client (with graceful fallback if not running)
        let mut customer_name = format!("Customer {}", customer_id);
        if let Ok(mut cust_client) =
            customer_v1::customer_service_client::CustomerServiceClient::connect(
                self.customer_client_addr.clone(),
            )
            .await
        {
            let verify_req = Request::new(customer_v1::VerifyCustomerRequest {
                tenant_id: tenant_id.to_string(),
                cust_id: customer_id.to_string(),
            });
            if let Ok(res) = cust_client.verify_customer(verify_req).await {
                let v = res.into_inner();
                if !v.exists {
                    return Err(Status::not_found(format!(
                        "Customer '{}' not found",
                        customer_id
                    )));
                }
                if let Some(c) = v.customer {
                    customer_name = c.name;
                }
            }
        }

        // Step 2: Reserve Stock & Fetch Product Prices via InventoryService gRPC
        let reservation_id = format!("res_{}_{}", tenant_id, uuid::Uuid::new_v4());
        let mut line_models: Vec<model::InvoiceLine> = Vec::new();
        let mut taxable_total = Decimal::ZERO;

        let seq = self.counter.fetch_add(1, Ordering::SeqCst);
        let txn_id = model::generate_transaction_id(seq);

        let stock_items: Vec<inventory_v1::StockItem> = req
            .lines
            .iter()
            .map(|l| inventory_v1::StockItem {
                product_id: l.product_id,
                quantity: l.quantity,
            })
            .collect();

        // Attempt gRPC reservation
        let mut inventory_connected = false;
        if let Ok(mut inv_client) =
            inventory_v1::inventory_service_client::InventoryServiceClient::connect(
                self.inventory_client_addr.clone(),
            )
            .await
        {
            let res_req = Request::new(inventory_v1::ReserveStockRequest {
                tenant_id: tenant_id.to_string(),
                reservation_id: reservation_id.clone(),
                items: stock_items,
            });

            match inv_client.reserve_stock(res_req).await {
                Ok(resp) => {
                    let r = resp.into_inner();
                    if !r.success {
                        return Err(Status::failed_precondition(r.message));
                    }
                    inventory_connected = true;

                    // Compute line items with prices from inventory
                    for (i, p) in r.products.iter().enumerate() {
                        let qty = req.lines.get(i).map(|l| l.quantity).unwrap_or(1);
                        let unit_price = Decimal::from_f64_retain(p.unit_price)
                            .unwrap_or(Decimal::new(100, 0));
                        let amount = unit_price * Decimal::from(qty);
                        taxable_total += amount;

                        line_models.push(model::InvoiceLine {
                            id: (i + 1) as i64,
                            invoice_id: txn_id.clone(),
                            product_id: p.id,
                            product_name: p.product_name.clone(),
                            unit_price,
                            quantity: qty,
                            amount,
                        });
                    }
                }
                Err(e) => {
                    return Err(Status::internal(format!("Inventory reservation failed: {}", e)));
                }
            }
        }

        // Fallback default calculation if standalone inventory service is offline
        if !inventory_connected {
            for (i, l) in req.lines.iter().enumerate() {
                let unit_price = Decimal::from_i32(100).unwrap(); // default base unit price
                let amount = unit_price * Decimal::from(l.quantity);
                taxable_total += amount;

                line_models.push(model::InvoiceLine {
                    id: (i + 1) as i64,
                    invoice_id: txn_id.clone(),
                    product_id: l.product_id,
                    product_name: format!("Item #{}", l.product_id),
                    unit_price,
                    quantity: l.quantity,
                    amount,
                });
            }
        }

        // Step 3: Exact financial calculation with rust_decimal
        let discount = Decimal::from_f64_retain(req.total_discount).unwrap_or(Decimal::ZERO);
        let packaging = Decimal::from_f64_retain(req.packaging).unwrap_or(Decimal::ZERO);
        let freight = Decimal::from_f64_retain(req.freight).unwrap_or(Decimal::ZERO);
        let tcs = Decimal::from_f64_retain(req.tax_collected_at_source).unwrap_or(Decimal::ZERO);
        let round_off = Decimal::from_f64_retain(req.round_off).unwrap_or(Decimal::ZERO);

        let grand_total = taxable_total + packaging + freight + tcs + round_off - discount;

        let inv_model = model::Invoice {
            id: seq as i64,
            tenant_id: tenant_id.to_string(),
            transaction_id: txn_id.clone(),
            customer_id: customer_id.to_string(),
            customer_name,
            date_time: Utc::now(),
            total: taxable_total,
            total_discount: discount,
            packaging,
            freight,
            taxable_amount: taxable_total,
            tax_collected_at_source: tcs,
            round_off,
            grand_total,
            method_of_payment: if req.method_of_payment.is_empty() {
                "Bank Transfer".to_string()
            } else {
                req.method_of_payment
            },
            lines: line_models,
            created_at: Utc::now(),
        };

        // Step 4: Persist Invoice in Ledger
        let saved = self
            .store
            .insert(inv_model)
            .await
            .map_err(|e| Status::internal(e))?;

        // Step 5: Commit Stock Reservation in Inventory Service
        if inventory_connected {
            if let Ok(mut inv_client) =
                inventory_v1::inventory_service_client::InventoryServiceClient::connect(
                    self.inventory_client_addr.clone(),
                )
                .await
            {
                let commit_req = Request::new(inventory_v1::CommitStockRequest {
                    tenant_id: tenant_id.to_string(),
                    reservation_id,
                });
                let _ = inv_client.commit_stock(commit_req).await;
            }
        }

        Ok(Response::new(CreateInvoiceResponse {
            invoice: Some(model_to_proto(&saved)),
            message: "Invoice created successfully".to_string(),
        }))
    }

    async fn get_invoice(
        &self,
        request: Request<GetInvoiceRequest>,
    ) -> Result<Response<invoice_v1::Invoice>, Status> {
        let req = request.into_inner();
        let inv = self
            .store
            .find_by_txn(&req.tenant_id, &req.transaction_id)
            .await
            .ok_or_else(|| Status::not_found("invoice not found"))?;

        Ok(Response::new(model_to_proto(&inv)))
    }

    async fn list_invoices(
        &self,
        request: Request<ListInvoicesRequest>,
    ) -> Result<Response<ListInvoicesResponse>, Status> {
        let req = request.into_inner();
        let invoices = self.store.list(&req.tenant_id).await;
        let total_count = invoices.len() as i64;

        let proto_invoices: Vec<invoice_v1::Invoice> =
            invoices.iter().map(model_to_proto).collect();

        Ok(Response::new(ListInvoicesResponse {
            invoices: proto_invoices,
            total_count,
        }))
    }

    type StreamInvoicesStream = ReceiverStream<Result<invoice_v1::Invoice, Status>>;

    async fn stream_invoices(
        &self,
        request: Request<ListInvoicesRequest>,
    ) -> Result<Response<Self::StreamInvoicesStream>, Status> {
        let req = request.into_inner();
        let invoices = self.store.list(&req.tenant_id).await;

        let (tx, rx) = tokio::sync::mpsc::channel(128);

        tokio::spawn(async move {
            for inv in invoices {
                if tx.send(Ok(model_to_proto(&inv))).await.is_err() {
                    break;
                }
            }
        });

        Ok(Response::new(ReceiverStream::new(rx)))
    }
}
