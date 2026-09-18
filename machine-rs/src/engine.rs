// The engine, loaded once. Every stage of a proof borrows this one copy
// instead of starting a process that reads the model again.
use isla_lib::bitvector::b129::B129;
use isla_lib::config::ISAConfig;
use isla_lib::init::{initialize_architecture, Initialized};
use isla_lib::ir::{AssertionMode, Def, IRTypeInfo, Name, Symtab};
use isla_lib::ir_lexer::new_ir_lexer;
use isla_lib::ir_parser::IrParser;
use sha2::{Digest, Sha256};
use std::path::Path;

/// A model and its configuration, parsed and initialised.
///
/// The model text is held for the life of the process because the symbol
/// table, the definitions, and the register bindings all borrow from it.
pub struct Engine {
    ready: Initialized<'static, B129>,
    config: ISAConfig<B129>,
}

impl Engine {
    /// Reads and initialises the model at `model` under the configuration at
    /// `config`.
    ///
    /// Every failure names the file that caused it.
    pub fn load(model: &Path, config: &Path) -> Result<Engine, String> {
        let text = read_model_text(model)?;
        let mut symtab = Symtab::new();
        let defs = Box::leak(Box::new(parse_definitions(text, &mut symtab, model)?));
        let type_info = IRTypeInfo::new(defs);
        let mut hasher = Sha256::new();
        let config = ISAConfig::from_file(&mut hasher, config, None, &symtab, &type_info)
            .map_err(|error| format!("{}: {error}", config.display()))?;
        let ready = initialize_architecture(
            defs,
            symtab,
            type_info,
            &config,
            AssertionMode::Optimistic,
            true,
        );
        Ok(Engine { ready, config })
    }

    /// The initialised architecture, for one stage of a proof.
    pub fn ready(&self) -> &Initialized<'static, B129> {
        &self.ready
    }

    /// The configuration the model was initialised under.
    pub fn config(&self) -> &ISAConfig<B129> {
        &self.config
    }
}

fn read_model_text(path: &Path) -> Result<&'static str, String> {
    let text =
        std::fs::read_to_string(path).map_err(|error| format!("{}: {error}", path.display()))?;
    if text.is_empty() {
        return Err(format!("{}: model file is empty", path.display()));
    }
    Ok(Box::leak(text.into_boxed_str()))
}

fn parse_definitions(
    text: &'static str,
    symtab: &mut Symtab<'static>,
    path: &Path,
) -> Result<Vec<Def<Name, B129>>, String> {
    IrParser::new()
        .parse::<B129, _, _>(symtab, new_ir_lexer(text))
        .map_err(|error| format!("{}: {error}", path.display()))
}
