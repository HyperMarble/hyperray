// External fixture extraction declares the standard default-feature build.
// Missing Cargo configuration must not silently select a tool.
use hyperray_rust::extract::{run, CompileRequest, Run};
use hyperray_rust::FeatureSelection;
use std::path::Path;

pub fn compile_crate(driver: &Path, source: &Path, output: &Path) -> Result<Run, String> {
    let cargo = std::env::var("CARGO").map_err(|error| error.to_string())?;
    let request = CompileRequest {
        driver: driver.into(),
        cargo: cargo.into(),
        crate_directory: source.into(),
        output_directory: output.into(),
        features: FeatureSelection {
            default_features: true,
            features: Vec::new(),
        },
    };
    Ok(run(&request))
}
