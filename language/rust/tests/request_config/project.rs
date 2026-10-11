// Makes distinct owner wrapper selections for real Cargo comparisons.
// Never uses a mock compiler or a fixture folder.
use super::{
    sample_project,
    wrappers, //
};
use std::path::{
    Path,
    PathBuf, //
};

pub fn create(name: &str) -> Result<PathBuf, String> {
    let root = sample_project::create(&sample_project::Sample {
        name,
        file: "main.rs",
        source: "fn main() {}\n",
        locked: true,
    })?;
    wrappers::configure(&root)?;
    for role in ["outer", "workspace"] {
        let requested = root.join(format!("requested-{role}.sh"));
        std::fs::copy(root.join(format!("{role}-wrapper.sh")), &requested)
            .map_err(|error| error.to_string())?;
        let source = std::fs::read_to_string(&requested).map_err(|error| error.to_string())?;
        let source = source.replace(&format!("'{role}\\n'"), &format!("'requested-{role}\\n'"));
        std::fs::write(requested, source).map_err(|error| error.to_string())?;
    }
    Ok(root)
}

pub fn file_configuration(root: &Path) -> Result<(), String> {
    let configuration = r#"include = ["request-values.toml"]
"#;
    let values = r#"[build]
rustc-wrapper = "./requested-outer.sh"
rustc-workspace-wrapper = "./requested-workspace.sh"
"#;
    std::fs::write(root.join(".cargo/request.toml"), configuration)
        .map_err(|error| error.to_string())?;
    std::fs::write(root.join(".cargo/request-values.toml"), values)
        .map_err(|error| error.to_string())
}

pub fn unavailable_defaults(root: &Path) -> Result<(), String> {
    let file = root.join(".cargo/config.toml");
    let contents = std::fs::read_to_string(&file).map_err(|error| error.to_string())?;
    let contents = contents.replace("./outer-wrapper.sh", "./missing-outer");
    let contents = contents.replace("./workspace-wrapper.sh", "./missing-workspace");
    std::fs::write(file, contents).map_err(|error| error.to_string())
}
