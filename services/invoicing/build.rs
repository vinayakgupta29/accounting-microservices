fn main() -> Result<(), Box<dyn std::error::Error>> {
    tonic_build::configure()
        .compile_protos(
            &[
                "../../proto/invoice/v1/invoice.proto",
                "../../proto/customer/v1/customer.proto",
                "../../proto/inventory/v1/inventory.proto",
            ],
            &["../../proto"],
        )?;
    Ok(())
}
