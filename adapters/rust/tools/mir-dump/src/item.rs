// One schema item for each local compiler item. The compiler supplies every
// name, parent, span, type, and body.

use crate::{block, input, instance, read, value};
use hyperray_rust::mir::{Body, Dump, Item, Kind, SCHEMA_VERSION};
use rustc_middle::ty::TyCtxt;
use rustc_public::crate_def::CrateDef;
use rustc_public::{CrateItem, ItemKind};

pub fn dump(tcx: TyCtxt<'_>) -> Result<Dump, String> {
    let items = rustc_public::all_local_items()
        .iter()
        .map(item)
        .collect::<Result<Vec<_>, _>>()?;
    Ok(Dump {
        schema_version: SCHEMA_VERSION,
        crate_name: rustc_public::local_crate().name,
        items,
        instances: instance::of(tcx)?,
    })
}

fn item(item: &CrateItem) -> Result<Item, String> {
    let compiler_body = read::body_of(item);
    let span = compiler_body
        .as_ref()
        .map_or_else(|| item.span(), |body| body.span);
    let lines = span.get_lines();
    Ok(Item {
        name: item.name(),
        parent: item
            .def_id()
            .parent()
            .map(|parent| parent.name().to_string()),
        kind: kind_of(item.kind()),
        file: span.get_filename(),
        start_line: line(lines.start_line)?,
        end_line: line(lines.end_line)?,
        value: value::of(item),
        inputs: match compiler_body.as_ref() {
            Some(body) => input::of(body)?,
            None => Vec::new(),
        },
        body: compiler_body.as_ref().map(body).transpose()?,
    })
}

fn body(body: &rustc_public::mir::Body) -> Result<Body, String> {
    Ok(Body {
        local_count: body.locals().len(),
        blocks: block::of(body)?,
    })
}

fn line(value: usize) -> Result<u32, String> {
    u32::try_from(value).map_err(|_| format!("source line {value} does not fit in u32"))
}

fn kind_of(kind: ItemKind) -> Kind {
    match kind {
        ItemKind::Fn => Kind::Function,
        ItemKind::Static => Kind::Static,
        ItemKind::Const => Kind::Constant,
        ItemKind::Ctor(_) => Kind::Constructor,
    }
}
