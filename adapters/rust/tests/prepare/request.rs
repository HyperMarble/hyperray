// Invalid paths, interpolation, and signatures remain compiler or request errors.
// Preparation must not overwrite an existing output directory.
use hyperray_rust::prepare::build;

#[test]
fn public_request() -> Result<(), Box<dyn std::error::Error>> {
    let request = super::fixture::request()?;
    request.validate()?;
    let result = build(&request);
    assert!(result.is_err());
    assert!(request.directory.is_dir());
    Ok(())
}

#[test]
fn invalid_function_paths() -> Result<(), Box<dyn std::error::Error>> {
    let mut request = super::fixture::request()?;
    for name in [
        "",
        "::solve",
        "solve::",
        "1solve",
        "solve(input);std::process::exit(0)",
    ] {
        request.subject.function = name.into();
        assert!(request.validate().is_err(), "{name}");
    }
    request.subject.function = "workflow::solve".into();
    request.tools.spin = "spin".into();
    assert!(request.validate().is_err());
    Ok(())
}

#[test]
fn unknown_request_fields() -> Result<(), Box<dyn std::error::Error>> {
    let request = super::fixture::request()?;
    let mut value = serde_json::to_value(request)?;
    value["invented_default"] = serde_json::json!(true);
    let parsed = serde_json::from_value::<hyperray_rust::prepare::Request>(value);
    assert!(parsed.is_err());
    Ok(())
}
