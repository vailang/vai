use std::fmt;
use std::io::{self, Write};


fn main() {
    println!("Welcome to the Calculator REPL!");
    print_help();
    
    let stdin = std::io::stdin();
    let mut stdout = std::io::stdout();
    let mut input = String::new();
    
    loop {
        print!("> ");
        stdout.flush().unwrap();
        
        input.clear();
        stdin.read_line(&mut input).unwrap();
        
        let trimmed = input.trim();
        
        if trimmed.is_empty() {
            continue;
        }
        
        let lowercased = trimmed.to_lowercase();
        
        if lowercased == "quit" || lowercased == "exit" {
            println!("Goodbye!");
            break;
        }
        
        if lowercased == "help" {
            print_help();
            continue;
        }
        
        match parse_input(trimmed) {
            Err(e) => {
                println!("{}", e);
                continue;
            }
            Ok((a, op, b)) => {
                match calculate(a, op, b) {
                    Err(e) => {
                        println!("{}", e);
                    }
                    Ok(result) => {
                        println!("{} {} {} = {}", a, op, b, result);
                    }
                }
            }
        }
    }
}

#[derive(Debug)]
enum CalcError {
    DivisionByZero,
    InvalidOperator(char),
    ParseError(String),
}

impl fmt::Display for CalcError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            CalcError::DivisionByZero => write!(f, "Error: division by zero"),
            CalcError::InvalidOperator(op) => write!(f, "Error: invalid operator '{}'", op),
            CalcError::ParseError(msg) => write!(f, "Error: {}", msg),
        }
    }
}

fn calculate(a: f64, op: char, b: f64) -> Result<f64, CalcError> {
    match op {
        '+' => Ok(a + b),
        '-' => Ok(a - b),
        '*' => Ok(a * b),
        '/' => {
            if b == 0.0 {
                Err(CalcError::DivisionByZero)
            } else {
                Ok(a / b)
            }
        }
        _ => Err(CalcError::InvalidOperator(op)),
    }
}

fn parse_input(input: &str) -> Result<(f64, char, f64), CalcError> {
    let tokens: Vec<&str> = input.trim().split_whitespace().collect();
    
    if tokens.len() != 3 {
        return Err(CalcError::ParseError("Expected format: <number> <op> <number>".to_string()));
    }
    
    let a: f64 = tokens[0].parse()
        .map_err(|_| CalcError::ParseError(tokens[0].to_string()))?;
    
    if tokens[1].is_empty() {
        return Err(CalcError::ParseError(tokens[1].to_string()));
    }
    let op = tokens[1].chars().next().unwrap();
    
    let b: f64 = tokens[2].parse()
        .map_err(|_| CalcError::ParseError(tokens[2].to_string()))?;
    
    Ok((a, op, b))
}

fn print_help() {
    println!("Usage: <number> <op> <number>");
    println!("Example: 3.5 + 2");
    println!("Supported operators: +, -, *, /");
    println!("Special commands:");
    println!("  help       - Show this message");
    println!("  quit/exit  - End the session");
}