// Exact compiler parent links decide which derived items belong to a root.
// Source-span overlap must not decide ownership.

use super::ItemAnalysis;
use std::collections::BTreeSet;

pub fn descendants<'a>(root: &str, items: &'a [ItemAnalysis]) -> Vec<&'a ItemAnalysis> {
    let mut family = BTreeSet::from([root.to_string()]);
    let mut found = Vec::new();
    loop {
        let added = children(&family, items);
        if added.is_empty() {
            return found;
        }
        for item in added {
            family.insert(item.name.clone());
            found.push(item);
        }
    }
}

fn children<'a>(family: &BTreeSet<String>, items: &'a [ItemAnalysis]) -> Vec<&'a ItemAnalysis> {
    items
        .iter()
        .filter(|item| !family.contains(&item.name))
        .filter(|item| {
            item.parent
                .as_ref()
                .is_some_and(|parent| family.contains(parent))
        })
        .collect()
}
