// Added-function tests require patch text to supply each extracted name.

use hyperray_rust::extract::change;

#[test]
fn pass_one_reads_only_the_patch_text() {
    let text = "+++ b/src/lib.rs\n@@ -1,2 +1,3 @@ mod x\n+pub fn added() {}\n-old\n";
    let files = change(text);
    assert_eq!(files[0].path, "src/lib.rs");
    assert_eq!(files[0].hunks[0].defines, vec!["added".to_string()]);
    assert_eq!(files[0].hunks[0].context, "mod x");
}
