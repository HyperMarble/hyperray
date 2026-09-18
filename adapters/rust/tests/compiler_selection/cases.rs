// Feature cases describe permitted builds and declared compilation errors.
// Fixture identity never selects production extraction behavior.
use hyperray_rust::FeatureSelection;

pub struct Case {
    pub name: &'static str,
    pub features: FeatureSelection,
    pub expected: Result<&'static str, &'static str>,
}

pub fn cases() -> Vec<Case> {
    vec![
        case("default", true, &[], Ok("left_only")),
        case("alternate", false, &["right"], Ok("right_only")),
        case("none", false, &[], Ok("neither")),
        case("explicit", false, &["left"], Ok("left_only")),
        case(
            "both",
            false,
            &["left", "right"],
            Err("left and right cannot be compiled together"),
        ),
        case(
            "default-conflict",
            true,
            &["right"],
            Err("left and right cannot be compiled together"),
        ),
        case(
            "unknown",
            false,
            &["missing"],
            Err("does not contain this feature: missing"),
        ),
        case("repeated-default", true, &[], Ok("left_only")),
    ]
}

fn case(
    name: &'static str,
    defaults: bool,
    names: &[&str],
    expected: Result<&'static str, &'static str>,
) -> Case {
    Case {
        name,
        features: FeatureSelection {
            default_features: defaults,
            features: names.iter().map(|name| name.to_string()).collect(),
        },
        expected,
    }
}
