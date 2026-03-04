# Vai Lang

Vai is a language that manages prompts and obligations to generate clear instructions for LLMs — without over-engineering and without requiring them to read thousands of lines of code.

## Goals

- Read `.vai` and `.plan` files written in natural prompt language
- Interface with the target language through specified files (e.g. `target "file.c"`) — tree-sitter extracts signatures and turns them into context the LLM can consume
- Compose clear prompts through structured declarations

## CLI Commands

| Command | Purpose |
|---|---|
| `vai build [file.vai]` | Parse + render (single file or package) |
| `vai config llm list\|add\|remove` | Manage LLM provider entries in vai.toml |
| `vai gen [skeleton\|plan\|code]` | Full generation pipeline via LLM |
| `vai init <name>` | Scaffold a `vai.toml` (interactive setup wizard) |
| `vai tree` | Show the package tree |

Global flag: `--json` — emit JSON output instead of terminal text.

`vai gen` flags:
- `--name <plan>` — restrict to a single named plan
- `--force` — ignore lock file and re-execute all requests

`vai gen` sub-commands:

| Sub-command | Steps executed |
|---|---|
| `vai gen` | architect + diff + save + executor + debug (full pipeline) |
| `vai gen skeleton` | architect + diff + flush (writes stubs, does NOT save impls to .vai) |
| `vai gen plan` | architect + diff + save + flush (everything before executor) |
| `vai gen code` | executor + debug (requires skeleton already saved in .vai file) |

## Architecture

### Compiler Pipeline

```
.vai / .plan file
       │
       ▼
    Reader          → CodeSource[]
       │
       ▼
    Lexer           → Token channel (goroutine)
       │
       ▼
    Parser          → AST (File, Declarations, BodySegments)
       │
       ▼
    Composer        → Validated Requests (semantic checks + dependency resolution)
       │
       ▼
    Runner          → LLM execution (5-step pipeline)
```

Source: [compiler.go](internal/compiler/compiler.go) — the `parseSources()` method orchestrates the full pipeline:

```go
cs := reader.NewVaiSource(source)
scanner := lexer.NewScanner(cs)
p := parser.New(scanner)
file, parseErrs := p.ParseFile()
// ... validate with composer, merge files, return program
```

### Key Interfaces

**Compiler** ([types.go](internal/compiler/types.go)):
```go
type Compiler interface {
    Parse(vaiPath string) (Program, []error)
    ParseSources(sources map[string]string) (Program, []error)
    SetBaseDir(dir string)
}
```

**Program** ([types.go](internal/compiler/types.go)) — compiled program ready for inspection and execution:
```go
type Program interface {
    // Execution
    Tasks() int
    Exec() (string, error)
    Eval(source string) (string, error)
    Render() string

    // Diagnostics
    Warnings() []error

    // Inspection
    File() *ast.File
    Requests() []composer.Request
    HasPrompt(name string) bool
    HasPlan(name string) bool
    ListPrompts() []string
    ListConstraints() []string
    GetPlanSpec(name string) string
    GetPlanImpl(name string) []string
}
```

**TargetInfo** ([render/types.go](internal/compiler/render/types.go)) — target file data for rendering:
```go
type TargetInfo interface {
    ResolveTarget(path string) (symbols, sigs, error)
    GetCode(path, name string) (string, bool)
    GetSkeleton(path string) (string, bool)
    GetDoc(path, name string) (string, bool)
    GetRawContent(path string) (string, bool)
}
```

**SymbolResolver** ([composer/types.go](internal/compiler/composer/types.go)) — bridges tree-sitter symbols into the composer:
```go
type SymbolResolver interface {
    Symbols() map[string]ast.SymbolKind
    Signatures() map[string]string
    GetCode(name string) (string, bool)
    GetDoc(name string) (string, bool)
    IsGenerated(name string) bool
}
```

**LangQuery** ([coder/api/query.go](internal/coder/api/query.go)) — tree-sitter abstraction per language:
```go
type LangQuery interface {
    ReadSymbols(source []byte) ([]Symbol, error)
    ReadImportZone(source []byte) (*ImportZone, error)
    ReadSkeleton(source []byte) (string, error)
    Close()
}
```

**Provider** ([runner/provider/types.go](internal/runner/provider/types.go)) — LLM API abstraction:
```go
type Provider interface {
    Call(ctx context.Context, req Request) (*Response, error)
}
```

**RequestLocker** ([runner/runner.go](internal/runner/runner.go)) — hash-based caching:
```go
type RequestLocker interface {
    IsLocked(key, hash string) bool
    Lock(key, hash string)
    LockWithTokens(key, hash string, tokensIn, tokensOut int)
    GetTokens(key string) (int, int)
    Save() error
}
```

## Vai Syntax

### Declarations

| Keyword      | Purpose                                          | Scope         |
|-------------|--------------------------------------------------|---------------|
| `prompt`    | Reusable prompt text block                       | Top-level     |
| `inject`    | Include a prompt's content                       | Top-level     |
| `plan`      | Structured scope with specs, constraints, code   | Top-level     |
| `constraint`| Rules the LLM must follow                        | Inside plan   |
| `spec`      | Detailed specification                           | Inside plan   |
| `reference` | Symbol source for `[use]` resolution (not emitted in status) | Top-level / plan / prompt / constraint |
| `impl`      | Implementation task for a named symbol            | Inside plan   |
| `target`    | Output file path                                  | Inside plan   |

### Body Directives

Inside any body block `{ ... }`, the following directives are available:

| Directive               | Purpose                                              |
|------------------------|------------------------------------------------------|
| `[use X]`              | Declare dependency on symbol `X`                     |
| `[use X+code]`         | Include full source code of `X`                      |
| `[use X+doc]`          | Include documentation of `X`                         |
| `[inject X]`           | Inline a prompt block                                |
| `[target "path"]`      | Reference a target file (forbidden in prompt/constraint) |
| `[reference "path"]`   | Load symbols for `[use]` resolution without status output |
| `[reference plan_name]`| Import a plan's targets as reference symbols         |
| `[match field]`        | Conditional block based on config value              |
| `[case "value"]`       | Case branch inside a match block                     |

### Inject Syntax

`inject` includes a prompt's content at the top level. It supports dotted names to inject a specific impl from a plan:

```vai
inject example           // inject a prompt
inject rust              // inject an entire plan
inject rust.add          // inject a specific impl from a plan
```

When `inject plan.impl` is used, the output includes:
1. The impl name as a heading
2. The function's full code from the target file
3. Body text instructions
4. A `## Reference` section with signatures of all `[use]` dependencies

### Restrictions

- `[target]` is **forbidden** in `prompt` and `constraint` declarations — use `[reference]` instead
- `impl` requires the plan to have at least one `target`
- `impl` takes an identifier name (not a string signature): `impl main { ... }` not `impl "fn main()" { ... }`

### Example

A `.vai` file:
```vai
prompt coding_style {
    [reference "src/lib.rs"]
    Follow Rust best practices
    [use TodoItem]
}

plan rust {
    target "src/main.rs"
    reference "src/lib.rs"

    spec {
        Build a todo list application
    }

    impl add {
        implement function to add a new todo item
        [use TodoItem]
    }

    impl main {
    }
}

inject rust
inject rust.add
```

The compiler reads the `.vai` file, uses tree-sitter to extract symbols from target and reference files, resolves `[use]` references, and composes the final prompt. Reference files provide symbols for resolution but are not included in the "Target File Status" output.

## Tree-Sitter Integration

The [coder](internal/coder/) package uses tree-sitter to extract symbols from target language files. Each language has its own query implementation:

| Language   | Package                                              |
|-----------|------------------------------------------------------|
| Go        | [coder/golang/](internal/coder/golang/query.go)      |
| C         | [coder/clang/](internal/coder/clang/query.go)        |
| Python    | [coder/python/](internal/coder/python/query.go)      |
| Rust      | [coder/rust/](internal/coder/rust/query.go)           |
| TypeScript| [coder/typescript/](internal/coder/typescript/query.go)|

The `targetResolverImpl` in [compiler.go](internal/compiler/compiler.go) lazily loads `coder.Coder` instances and satisfies both `composer.TargetResolver` and `render.TargetInfo` — this is the key integration point between tree-sitter symbol extraction and the compiler.

Symbols are cached in `~/.vai/headers/` as SHA256-named JSON files ([coder/api/cache.go](internal/coder/api/cache.go)).

## Execution Pipeline

Vai splits the generation process into a 5-step pipeline:

```
Plan (compiler) → Architect (LLM) → Diff (tree-sitter) → Executor (LLM) → Debug (loop)
```

### Step 1: Architect (PlannerAgent — Big Model)

A `plan` declaration is dispatched to the **PlannerAgent** — a large, capable model that creates the structural skeleton and assigns implementation tasks.

- Receives the full plan (specs, constraints, target file symbols) via `inject <planName>`
- Uses `inject std.vai_system` as system prompt
- Responds via `plan_skeleton` tool call
- Result: `PlanSkeletonInput{Imports, Declarations, Impls}`

The architect is responsible for:
- ALL import/include statements — the executor will NOT add any
- Complete type and function declarations with stub bodies (compilable stubs: `todo!()` in Rust, `panic("todo")` in Go, `pass` in Python)
- One impl instruction per function (2-5 lines, role/output style, no pseudocode)

From the standard library ([std/files/planner.vai](std/files/planner.vai)):
```vai
prompt vai_system {
    [inject std.vai_plan]
    [inject std.vai_impl]

    You are a software architect.
    You receive a plan with a spec as context.
    Your job: create the structural skeleton and assign atomic implementation tasks.
    Do not invent requirements beyond the spec.

    You MUST use the plan_skeleton tool to output your result.
    ...
}
```

### Step 2: Diff (tree-sitter)

`diff()` applies the skeleton to in-memory target files:
- **Imports**: inserted via `insertImportsInMemory()`
- **Declarations**: `add` appends, `modify` replaces via tree-sitter byte ranges, `remove` deletes

### Step 3: Save

`saveSkeleton()` rewrites the original `.vai`/`.plan` file on disk:
- Preserves: `target`, `spec`, `constraint`, `reference` blocks
- Adds: `impl` blocks from skeleton

### Step 4: Executor (ExecutorAgent — Small Model)

Each `impl` is rendered into a request and sent to the **ExecutorAgent** — a smaller, faster model that acts as a pure code generator:

- Executes all impls in **parallel** goroutines
- Per impl: `buildExecutorPrompt()` includes current skeleton stub code + instruction + dependency signatures
- Responds via `write_code` tool call
- `applyWriteCode()` replaces the symbol in-memory using tree-sitter byte ranges
- Does not know about Vai — receives the rendered prompt

The split is defined in [composer/types.go](internal/compiler/composer/types.go):
```go
const (
    PlannerAgent  RequestType = "planner"  // plan → big model
    ExecutorAgent RequestType = "executor" // impl → small model
)
```

### Step 5: Debug (compile-check loop)

`debug()` groups targets by language and runs the configured `compile_check` command:
- If errors: tries `fixTargeted()` first (structured diagnostics → symbol-level fix via `write_code`)
- Falls back to `fixLegacy()` (full file → `report_fix` tool)
- Repeats up to `max_attempts` times (default 3)

### LLM Tools

Three tools are defined in [runner/tools/](internal/runner/tools/):

| Tool | Agent | Purpose |
|---|---|---|
| `plan_skeleton` | Architect | Declare imports, declarations (with actions: add/modify/remove/keep), impls |
| `write_code` | Executor | Write one function body (code + doc) |
| `report_fix` | Debug | Report a fix attempt (code + fixed flag + reason) |

### Providers

Three first-class providers + custom via `base_url`:

| Provider | Wire format |
|---|---|
| `anthropic` | Anthropic Messages API |
| `openai` | OpenAI-compatible API |
| `gemini` | Google Gemini API |
| custom | Selected via `schema` field (`"openai"`, `"anthropic"`, `"gemini"`) |

All support configurable `max_retries` and `delay_retry_seconds`.

## Configuration (`vai.toml`)

LLM providers are configured as a `[[llm]]` array. Each entry has an optional `role` field (`"plan"` for the architect, `"code"` for the executor) and an optional `name` label.

```toml
[lib]
name = "myproject"
prompts = "./prompts"

[[llm]]
name = "architect"
role = "plan"
provider = "anthropic"
model = "claude-sonnet-4-6"
max_tokens = 8000

[[llm]]
name = "coder"
role = "code"
provider = "anthropic"
model = "claude-haiku-4-5"
max_tokens = 4096

[debug]
max_attempts = 3
[debug.languages.rust]
compile_check = "cargo check --message-format=json"
format = "json"
tools = ["cargo fmt"]

[vars]
target = "arm64"
```

Key fields:
- `LLMConfig.name` — optional human-readable label (used for `vai config llm remove --name`)
- `LLMConfig.role` — `"plan"` (architect), `"code"` (executor), or empty (available but not default)
- `LLMConfig.schema` — wire format for custom providers with `base_url`
- `LLMConfig.env_token_variable_name` — env var for API token
- `DebugLangConfig.compile_check` — shell command; `{target}` is replaced with file paths
- `DebugLangConfig.format` — `"json"` enables structured diagnostic parsing
- `DebugLangConfig.tools` — extra commands (e.g. formatters) run after successful compile
- `Config.vars` — variables for `[match]`/`[case]` resolution in .vai files

The executor (code role) is lazily validated — it's only required when running `vai gen` or `vai gen code`. Running `vai gen skeleton` or `vai gen plan` only requires a plan-role LLM.

Backward compatibility: legacy `[planner]`/`[executor]` sections are auto-migrated to `[[llm]]` entries on load.

### Config CLI

| Command | Purpose |
|---|---|
| `vai config llm list` | Show all configured LLM entries |
| `vai config llm add --provider anthropic --model claude-sonnet-4-6 --role plan` | Add an LLM entry |
| `vai config llm remove --name architect` | Remove an LLM entry by name |

## Lock File (`vai.lock`)

Hash-based caching to skip re-executing unchanged plans/impls.

- Keys: `plan:<name>`, `impl:<plan>.<impl>`
- Each entry stores: SHA-256 hash + token counts (in/out)
- `--force` flag bypasses the locker entirely

Hashing: `HashPlan` covers name + targets + spec texts + constraint texts (not impls — those are skeleton-generated). `HashImpl` covers name + body text + target path. Both normalize before SHA-256.

## AST Structure

All declarations implement the `Declaration` interface ([ast/ast.go](internal/compiler/ast/ast.go)):

```go
type Declaration interface {
    Node
    DeclName() string
    GetDirectives() []Directive
}
```

The `File` root node holds all top-level declarations:
```go
type File struct {
    Declarations  []Declaration
    TargetPath    string
    InjectPrompts []*InjectPromptDecl
}
```

Body blocks are composed of `BodySegment` types: `TextSegment`, `UseRefSegment`, `InjectRefSegment`, `TargetRefSegment`, `ReferenceRefSegment`, and `MatchSegment` (with `CaseClause`).

## Project Structure

```
cmd/vai/
  main.go                    CLI entry point (Cobra)
  build_cmd.go               "vai build" subcommand
  config_cmd.go              "vai config" subcommand (llm list/add/remove)
  gen_cmd.go                 "vai gen" subcommand (skeleton, plan, code)
  init_cmd.go                "vai init" subcommand (interactive wizard)
  tree_cmd.go                "vai tree" subcommand

internal/compiler/
  types.go                   Compiler + Program interfaces
  compiler.go                Pipeline: New(), Parse(), ParseSources(), targetResolver
  program.go                 Program struct, inspection methods, Eval()
  render/
    types.go                 TargetInfo interface
    program.go               Render(), Exec() — top-level entry points
    plan.go                  Plan(), Constraint() — plan rendering + Reference Files section
    impl.go                  ImplAtomic(), ImplInjected() — impl rendering
    body.go                  BodyText(), BodyResolved() — body segment rendering
    symbol.go                UseRef(), resolveAllTargets() — symbol resolution
    util.go                  LangTag(), ExtTag() — file extension utilities
  ast/
    ast.go                   Core AST: declarations, body segments
    ast_target.go            Target-language declarations, BodyKind, SymbolKind
  reader/
    reader.go                .vai/.plan file reader, vai:code block extraction
  lexer/
    token.go                 Token enum, keywords, IsBodyKeyword
    types.go                 CodeSource, Scanner interfaces
    lexer.go                 State-machine lexer (goroutine + channel)
    states.go                Lexer state functions
    body.go                  Body-mode lexing
  parser/
    types.go                 Parser struct, TokenStream interface
    declarations.go          Top-level parsing (prompt, plan, constraint, etc.)
    body.go                  Body segment parsing ([use], [inject], [match])
  composer/
    types.go                 SymbolResolver, TargetResolver, Request, Reference
    validate.go              Semantic validation of [use] and [inject] refs
    requests.go              Request building (assembleBody, buildTask)

internal/runner/
  runner.go                  Pipeline orchestration, RequestLocker interface, RunStats
  execute.go                 Parallel impl execution, buildExecutorPrompt()
  diff.go                    Skeleton application to in-memory files via tree-sitter
  save.go                    Rewrite .vai/.plan file with generated impls
  debug.go                   Compile-check loop with targeted/legacy LLM fix
  event.go                   Event types (step_start/complete/failed, impl_start, skeleton, summary, done)
  coderpool.go               Reusable tree-sitter Coder pool
  filemanager/
    filemanager.go           In-memory file management (Load/Read/Write/Flush/Rollback)
  provider/
    types.go                 Provider interface, Request, Response, ToolDefinition, ToolCall
    factory.go               Provider factory (anthropic, openai, gemini, custom)
    anthropic.go             Anthropic Messages API
    openai.go                OpenAI-compatible API
    gemini.go                Google Gemini API
    retry.go                 Retry with configurable max/delay
  tools/
    types.go                 PlanSkeletonInput, SkeletonDecl, SkeletonImpl, WriteCodeInput, ReportFixInput
    schema.go                plan_skeleton, write_code, report_fix tool definitions
    parse.go                 JSON parsing for tool call inputs
  diagnostic/
    diagnostic.go            Diagnostic type, Parser interface, ForLanguage() factory
    go.go                    Go compiler error parser
    rust.go                  Rust compiler error parser (JSON format)
    clang.go                 C compiler error parser
    python.go                Python error parser
    typescript.go            TypeScript error parser

internal/locker/
  locker.go                  vai.lock management, RequestLocker implementation
  hash.go                    SHA-256 hashing for plan/impl content

internal/config/
  vai_config.go              Config struct (TOML: lib, [[llm]], debug, vars), PlannerConfig(), ExecutorConfig()
  loader.go                  FindConfig(), LoadConfig() (with legacy migration), SaveConfig()
  package.go                 Package tree discovery, LoadPackageFiles()

internal/coder/
  coder.go                   ReaderCode/WriterCode interfaces, factory
  reader.go                  Load, Resolve, Symbols, Skeleton (tree-sitter)
  writer.go                  InsertImports, BuildImportBlock
  helpers.go                 Shared utilities
  api/
    types.go                 Symbol, Method, ImportZone, ResolvedSymbol
    symbols.go               SymbolKind constants
    lang.go                  Language detection + normalization
    query.go                 LangQuery interface
    cache.go                 Symbol caching (~/.vai/headers/)
    skeleton.go              BodyReplacement, ApplyReplacements
  golang/query.go            Go tree-sitter queries
  clang/query.go             C tree-sitter queries
  python/query.go            Python tree-sitter queries
  rust/query.go              Rust tree-sitter queries
  typescript/query.go        TypeScript tree-sitter queries

internal/ui/
  ui.go                      Terminal + JSON output modes, summary table

std/
  std.go                     Standard library (embedded .vai files)
  files/planner.vai          Architect system prompt (vai_plan, vai_impl, vai_system)
  files/skeleton.vai         Skeleton builder prompt
  files/developer.vai        Executor system prompt (write_code tool)
```
