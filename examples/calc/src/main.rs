use std::fmt;
use std::io::{self, Write};
use std::iter::Peekable;
use std::vec::IntoIter;

fn main() {
    println!("Welcome to the Calculator REPL!");
    print_help();

    let stdin = io::stdin();
    let mut stdout = io::stdout();
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

        match run_expression(trimmed) {
            Ok(result) => println!("{} = {}", trimmed, result),
            Err(e) => println!("{}", e),
        }
    }
}

#[derive(Debug)]
enum CalcError {
    DivisionByZero,
    InvalidOperator(char),
    ParseError(String),
    UnexpectedToken(Token),
    UnexpectedEof,
}

impl fmt::Display for CalcError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            CalcError::DivisionByZero => write!(f, "Error: Division by zero"),
            CalcError::InvalidOperator(c) => write!(f, "Error: Invalid operator '{}'", c),
            CalcError::ParseError(s) => write!(f, "Error: Parse error - {}", s),
            CalcError::UnexpectedToken(t) => write!(f, "Error: Unexpected token {:?}", t),
            CalcError::UnexpectedEof => write!(f, "Error: Unexpected end of expression"),
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

fn print_help() {
    println!("Usage: <expression>");
    println!("Example: 1 + 2 * (3 - 4) / 5");
    println!("Supported operators: +, -, *, /");
    println!("Supports parentheses and operator precedence.");
    println!("Special commands:");
    println!("  help       - Show this message");
    println!("  quit/exit  - End the session");
}

#[derive(Debug, Clone, PartialEq)]
enum Token {
    Number(f64),
    Plus,
    Minus,
    Star,
    Slash,
    LParen,
    RParen,
}

#[derive(Debug)]
enum Expr {
    Number(f64),
    BinaryOp {
        left: Box<Expr>,
        op: char,
        right: Box<Expr>,
    },
    UnaryMinus(Box<Expr>),
}

struct Lexer<'a> {
    chars: Peekable<std::str::Chars<'a>>,
}

impl<'a> Lexer<'a> {
    fn new(input: &'a str) -> Self {
        Lexer {
            chars: input.chars().peekable(),
        }
    }

    fn peek(&mut self) -> Option<char> {
        self.chars.peek().copied()
    }

    fn consume(&mut self) -> char {
        self.chars.next().unwrap()
    }

    /// Consume and return all tokens, or the first lex error.
    fn tokenize(&mut self) -> Result<Vec<Token>, CalcError> {
        let mut tokens = Vec::new();

        loop {
            self.skip_whitespace();

            match self.peek() {
                None => break,
                Some(ch) => {
                    match ch {
                        '0'..='9' | '.' => {
                            let c = self.consume();
                            tokens.push(self.read_number(c)?);
                        }
                        '+' => {
                            self.consume();
                            tokens.push(Token::Plus);
                        }
                        '-' => {
                            self.consume();
                            tokens.push(Token::Minus);
                        }
                        '*' => {
                            self.consume();
                            tokens.push(Token::Star);
                        }
                        '/' => {
                            self.consume();
                            tokens.push(Token::Slash);
                        }
                        '(' => {
                            self.consume();
                            tokens.push(Token::LParen);
                        }
                        ')' => {
                            self.consume();
                            tokens.push(Token::RParen);
                        }
                        c => {
                            return Err(CalcError::InvalidOperator(c));
                        }
                    }
                }
            }
        }

        Ok(tokens)
    }

    /// Skip ASCII whitespace characters.
    fn skip_whitespace(&mut self) {
        while let Some(c) = self.peek() {
            if c.is_ascii_whitespace() {
                self.chars.next();
            } else {
                break;
            }
        }
    }

    /// Read a full number literal (integer or decimal) from the char stream.
    fn read_number(&mut self, first: char) -> Result<Token, CalcError> {
        let mut s = String::new();
        s.push(first);

        while let Some(c) = self.peek() {
            if c.is_ascii_digit() || c == '.' {
                s.push(c);
                self.chars.next();
            } else {
                break;
            }
        }

        let n = s
            .parse::<f64>()
            .map_err(|_| CalcError::ParseError(s))?;

        Ok(Token::Number(n))
    }
}

struct Parser {
    tokens: Peekable<IntoIter<Token>>,
}

impl Parser {
    fn new(tokens: Vec<Token>) -> Self {
        Parser {
            tokens: tokens.into_iter().peekable(),
        }
    }

    /// Entry-point: parse a full expression and assert no trailing tokens remain.
    fn parse(&mut self) -> Result<Expr, CalcError> {
        let expr = self.parse_expr()?;
        match self.tokens.next() {
            Some(t) => Err(CalcError::UnexpectedToken(t)),
            None => Ok(expr),
        }
    }

    /// Parse addition / subtraction (lowest precedence).
    fn parse_expr(&mut self) -> Result<Expr, CalcError> {
        let mut left = self.parse_term()?;

        loop {
            match self.tokens.peek() {
                Some(Token::Plus) => {
                    self.tokens.next();
                    let op = '+';
                    let right = self.parse_term()?;
                    left = Expr::BinaryOp {
                        left: Box::new(left),
                        op,
                        right: Box::new(right),
                    };
                }
                Some(Token::Minus) => {
                    self.tokens.next();
                    let op = '-';
                    let right = self.parse_term()?;
                    left = Expr::BinaryOp {
                        left: Box::new(left),
                        op,
                        right: Box::new(right),
                    };
                }
                _ => break,
            }
        }

        Ok(left)
    }

    /// Parse multiplication / division (higher precedence).
    fn parse_term(&mut self) -> Result<Expr, CalcError> {
        let mut left = self.parse_factor()?;

        while let Some(token) = self.tokens.peek() {
            let op = match token {
                Token::Star => '*',
                Token::Slash => '/',
                _ => break,
            };

            self.tokens.next();
            let right = self.parse_factor()?;
            left = Expr::BinaryOp {
                left: Box::new(left),
                op,
                right: Box::new(right),
            };
        }

        Ok(left)
    }

    /// Parse a unary minus, a parenthesised sub-expression, or a number literal.
    fn parse_factor(&mut self) -> Result<Expr, CalcError> {
        match self.tokens.next() {
            Some(Token::Number(n)) => Ok(Expr::Number(n)),
            Some(Token::Minus) => {
                let inner = self.parse_factor()?;
                Ok(Expr::UnaryMinus(Box::new(inner)))
            }
            Some(Token::LParen) => {
                let inner = self.parse_expr()?;
                match self.tokens.next() {
                    Some(Token::RParen) => Ok(inner),
                    Some(_t) => Err(CalcError::ParseError("Expected ')'".to_string())),
                    None => Err(CalcError::UnexpectedEof),
                }
            }
            Some(t) => Err(CalcError::UnexpectedToken(t)),
            None => Err(CalcError::UnexpectedEof),
        }
    }
}

/// Recursively evaluate an AST node.
fn eval(expr: &Expr) -> Result<f64, CalcError> {
    match expr {
        Expr::Number(n) => Ok(*n),
        Expr::UnaryMinus(inner) => Ok(-eval(inner)?),
        Expr::BinaryOp { left, op, right } => {
            let a = eval(left)?;
            let b = eval(right)?;
            calculate(a, *op, b)
        }
    }
}

/// Lex → parse → evaluate a raw input string, returning the numeric result.
fn run_expression(input: &str) -> Result<f64, CalcError> {
    let mut lexer = Lexer::new(input);
    let tokens = lexer.tokenize()?;
    let mut parser = Parser::new(tokens);
    let expr = parser.parse()?;
    eval(&expr)
}
