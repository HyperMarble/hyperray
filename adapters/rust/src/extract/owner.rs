// The `impl` block a line sits inside, named by its self type. A free
// function has none. Nothing here reads past the given line.

pub fn owner_at(file_text: &str, line: u32) -> Option<String> {
    let index = usize::try_from(line).ok()?.checked_sub(1)?;
    let mut depth: i32 = 0;
    let mut open_impls: Vec<(i32, String)> = Vec::new();
    for text in file_text.lines().take(index) {
        if let Some(self_type) = impl_self_type(text) {
            open_impls.push((depth, self_type));
        }
        depth += delta(text, '{') - delta(text, '}');
        open_impls.retain(|(opened_at, _)| depth > *opened_at);
    }
    open_impls.pop().map(|(_, self_type)| self_type)
}

// `impl<T> Trait<X> for Type<T> {` names `Type`; `impl Type {` names `Type`.
fn impl_self_type(text: &str) -> Option<String> {
    let rest = text.trim_start().strip_prefix("impl")?;
    let rest = skip_generics(rest)?.trim_start();
    let self_part = match rest.split_once(" for ") {
        Some((_, after_for)) => after_for,
        None => rest,
    };
    let name: String = self_part
        .chars()
        .take_while(|c| c.is_alphanumeric() || *c == '_')
        .collect();
    (!name.is_empty()).then_some(name)
}

// The `<…>` after `impl`, if any, may nest; it is skipped by depth.
fn skip_generics(rest: &str) -> Option<&str> {
    let Some(after) = rest.strip_prefix('<') else {
        return rest.starts_with(' ').then_some(rest);
    };
    let mut depth = 1;
    for (offset, c) in after.char_indices() {
        depth += i32::from(c == '<') - i32::from(c == '>');
        if depth == 0 {
            return after.get(offset + 1..);
        }
    }
    None
}

fn delta(text: &str, brace: char) -> i32 {
    i32::try_from(text.matches(brace).count()).unwrap_or(0)
}
