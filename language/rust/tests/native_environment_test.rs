// Purpose: preserves actual native environment names and hashes their bytes.
// Never: assumes that the operating system requires Unicode environment values.
use language_rust::environment::native_environment;
use sha2::{
    Digest,
    Sha256, //
};
use std::ffi::OsString;
use std::os::unix::ffi::OsStringExt;

#[test]
fn non_unicode_environment_is_accounted_for_without_plaintext() {
    let name = b"OWNER_\xff".to_vec();
    let value = b"value_\xfe".to_vec();
    let variables = [(
        OsString::from_vec(name.clone()),
        OsString::from_vec(value.clone()),
    )];
    let observed = native_environment(variables.into_iter());
    assert_eq!(observed.len(), 1);
    assert_eq!(observed[0].name, None);
    assert_eq!(observed[0].name_encoded, name);
    assert_eq!(observed[0].value, None);
    assert_eq!(
        observed[0].value_sha256,
        format!("{:x}", Sha256::digest(value))
    );
}
