// Where does the fixed setup cost go: parsing, or initialising?
use isla_lib::bitvector::b129::B129;
use isla_lib::init::initialize_architecture;
use isla_lib::ir::{AssertionMode, IRTypeInfo, Symtab};
use isla_lib::ir_lexer::new_ir_lexer;
use isla_lib::ir_parser::IrParser;
use std::time::Instant;

fn main() {
    let Ok(path) = std::env::var("HYPERRAY_ARM64_SAIL_IR") else {
        println!("no model path");
        return;
    };
    let read = Instant::now();
    let Ok(text) = std::fs::read_to_string(&path) else {
        println!("cannot read {path}");
        return;
    };
    let read_once = read.elapsed().as_secs_f64();
    println!("read from disk   {read_once:.2}s  (isla does this 3x)");

    let mut symtab = Symtab::new();
    let started = Instant::now();
    let Ok(mut defs) = IrParser::new().parse::<B129, _, _>(&mut symtab, new_ir_lexer(&text)) else {
        println!("parse failed");
        return;
    };
    println!("parse            {:.2}s", started.elapsed().as_secs_f64());

    let typed = Instant::now();
    let type_info = IRTypeInfo::new(&defs);
    println!("type info        {:.2}s", typed.elapsed().as_secs_f64());

    let Ok(config_path) = std::env::var("HYPERRAY_ARM64_ISLA_CONFIG") else {
        println!("no config path");
        return;
    };
    let mut hasher = <sha2::Sha256 as sha2::Digest>::new();
    let config = isla_lib::config::ISAConfig::<B129>::from_file(
        &mut hasher,
        &config_path,
        None,
        &symtab,
        &type_info,
    );
    let Ok(config) = config else {
        println!("config failed");
        return;
    };

    let init = Instant::now();
    let _ready = initialize_architecture(
        &mut defs,
        symtab,
        type_info,
        &config,
        AssertionMode::Optimistic,
        true,
    );
    println!("initialize(reg-init) {:.2}s", init.elapsed().as_secs_f64());
    println!("setup total      {:.2}s", started.elapsed().as_secs_f64());

    let again = Instant::now();
    let mut symtab2 = Symtab::new();
    let _second = IrParser::new().parse::<B129, _, _>(&mut Symtab::new(), new_ir_lexer(&text));
    let mut symtab3 = Symtab::new();
    let _third = IrParser::new().parse::<B129, _, _>(&mut symtab3, new_ir_lexer(&text));
    println!("2 more parses    {:.2}s", again.elapsed().as_secs_f64());
    println!("--");
    let second_init = Instant::now();
    let mut defs2 = match IrParser::new().parse::<B129, _, _>(&mut symtab2, new_ir_lexer(&text)) {
        Ok(d) => d,
        Err(_) => return,
    };
    let type_info2 = IRTypeInfo::new(&defs2);
    let _ = initialize_architecture(
        &mut defs2,
        symtab2,
        type_info2,
        &config,
        AssertionMode::Optimistic,
        true,
    );
    println!(
        "2nd initialize   {:.2}s  (footprint copy)",
        second_init.elapsed().as_secs_f64()
    );
    println!("--");
    println!("accounted        {:.2}s", started.elapsed().as_secs_f64());
}
