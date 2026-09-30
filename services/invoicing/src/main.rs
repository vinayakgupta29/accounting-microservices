mod db;
mod model;
mod service;

use db::InvoiceStore;
use service::invoice_v1::invoice_service_server::InvoiceServiceServer;
use service::InvoiceServiceImpl;
use std::env;
use std::net::SocketAddr;
use tonic::transport::Server;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let port = env::var("INVOICING_PORT").unwrap_or_else(|_| "50054".to_string());
    let addr: SocketAddr = format!("0.0.0.0:{}", port).parse()?;

    let customer_addr =
        env::var("CUSTOMER_SERVICE_URL").unwrap_or_else(|_| "http://127.0.0.1:50052".to_string());
    let inventory_addr =
        env::var("INVENTORY_SERVICE_URL").unwrap_or_else(|_| "http://127.0.0.1:50053".to_string());

    let store = InvoiceStore::new();
    let invoice_service = InvoiceServiceImpl::new(store, customer_addr, inventory_addr);

    println!(
        "Invoicing & Billing Engine (Rust) listening on gRPC port {}",
        port
    );

    Server::builder()
        .add_service(InvoiceServiceServer::new(invoice_service))
        .serve_with_shutdown(addr, async {
            tokio::signal::ctrl_c()
                .await
                .expect("failed to install CTRL+C signal handler");
            println!("Shutting down Invoicing gRPC engine...");
        })
        .await?;

    Ok(())
}
