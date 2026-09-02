use hyperray_rust::handle_request;
use serde_json::json;

#[test]
fn version_request_returns_rust_adapter_identity() {
    let request = json!({
        "protocol_version": 1,
        "action": "version"
    });

    let response = handle_request(&request.to_string()).expect("version request must succeed");
    let response: serde_json::Value =
        serde_json::from_str(&response).expect("response must be JSON");

    assert_eq!(response["status"], "ok");
    assert_eq!(response["protocol_version"], 1);
    assert_eq!(response["adapter"], "rust");
    assert_eq!(response["adapter_version"], env!("CARGO_PKG_VERSION"));
}
