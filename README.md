# Cova

`cova` is a small compiler, linker, and VM for a typed, C-like scripting language.

The language is designed for small host-integrated scripts that work with primitive numeric types, control flow, global state, and `extern` bindings for memory and host functions.

## What It Supports

- Primitive types: `bool`, `byte`, `int`/`int32`, `int8`, `int16`, `int64`, `uint8`, `uint16`, `uint32`, `uint64`, `float32`, `float64`
- Top-level globals
- Typed, block-scoped local variables inside functions
- Script functions with typed parameters and returns
- String literals passed as pointer values
- Expressions using `true`, `false`, `+`, `-`, `*`, `/`, `==`, `!=`, `<`, `<=`, `>`, `>=`, `&&`, `||`
- Control flow: `if`, `if/else`, `while`, `for`, `switch`, `break`, `continue`, `return`
- Built-ins; 
  - `sin`, `cos`, `tan`, `asin`, `acos`, `atan` `sqrt`, `pow`
- `//` single-line comments
- `/* ... */` block comments
- `extern(offset)` variables backed by host memory
- `extern(slot)` functions dispatched by the host

## Current Limits

- No local declarations in `for` initializers
- No `/* ... */` block comments in expressions
- Standalone expression statements must be function calls
- No arrays, structs, or field access
- No unary operators such as `-x`, `!x`, `*ptr`, or `&x`
- No bitwise operators or modulo
- Pointer types can be declared in signatures and declarations, but source-level pointer operators are not implemented
- Recursive script call cycles are rejected at compile time

String literals are stored in a CONST segment as NUL-terminated byte strings. Zero-initialized globals remain in BSS, while initialized writable globals are placed in DATA.

Booleans use numeric truthiness at runtime: `false` is `0`, and any non-zero value is true. Logical operators short-circuit and produce normalized `0` or `1` results.

## VM Workspace Policy

The host owns execution workspace sizing. `VMConfig.FrameCapacity`, `StackCapacity`, and `CallFrameCapacity` are fixed limits selected from the application's memory budget; the compiler and linker do not infer aggregate frame use, maximum operand-stack height, or maximum call depth. `LoadProgram` validates the program image independently of these limits. A limit that is too small produces `VMStatusFrameOverflow`, `VMStatusStackOverflow`, or `VMStatusCallFrameOverflow` only if execution reaches the operation that needs more workspace.

`LinkedProgram.FrameByteSize` is compiler metadata for the largest individual function frame. It is not aggregate call-path memory and is not a complete VM sizing recommendation.

VM runtime and stack APIs return the stable numeric `VMStatus` enum directly. `VMStatusOK` is zero. `VMStatus.String()` provides optional diagnostics outside the execution hot path, while `VM.FaultInfo()` exposes the failing PC, target, required/available capacity, and host callback status without formatting strings.

```go
vm := cova.NewVMWithConfig(cova.VMConfig{
    FrameCapacity:     512,
    StackCapacity:     128,
    CallFrameCapacity: 8,
})
if status := vm.LoadProgram(linked); status != cova.VMStatusOK {
    return fmt.Errorf("load failed: %s", status)
}
if status := vm.RunLoaded(); status != cova.VMStatusOK {
    fault := vm.FaultInfo()
    return fmt.Errorf("VM failed at PC %d: %s", fault.PC, status)
}
```

## Optimization

Optimization is an explicit, optional stage between parsing and compilation:

```go
program, err := cova.Parse(tokens)
if err != nil {
    return err
}
if err := cova.Optimize(program); err != nil {
    return err
}
compiled, err := cova.NewCompiler().Compile(program)
```

`Optimize` mutates the AST in place. Its constant folding pass handles numeric arithmetic, comparisons, and logical expressions, including short-circuit branches. A reachable constant division by zero is reported as an optimization error; an unreachable short-circuit branch is not evaluated.

The optimizer is intentionally isolated from compiler and VM internals. It owns the small amount of type promotion, conversion, and evaluation logic required for folding, and optimized-versus-unoptimized tests guard that duplicated behavior against semantic drift.

## Example

```c
extern(0) void log_alert(int value);
extern(4) int player_health;

int health_drop;

void script_main() {
    health_drop = 5;
    if ((player_health - 40) + 1) {
        log_alert(player_health);
        reduce_health(health_drop);
    }
    return;
}

void reduce_health(int delta) {
    player_health = player_health - delta;
    return;
}
```

## Documentation

- Full language overview: [LANGUAGE.md](docs/LANGUAGE.md)

## Development

Run the test suite with:

```sh
cd cova
go test ./...
```

### Embedded native VM fixture

The native integration suite executes a program compiled and serialized by the Go toolchain. The binary fixture is tracked under `embedded/source/test/cpp`, and ccode converts it into an aligned C++ byte array under `source/test/cpp`.

Refresh the fixture after an intentional compiler, linker, or image-format change:

```sh
cd cova
CCOVA_UPDATE_EMBEDDED=1 go test -run TestEmbeddedProgramImageFixture -count=1
cd ..
go run ccova.go --dev=clay
```

Ordinary Go test runs only verify that the tracked fixture matches freshly generated bytes; they do not rewrite it. After regeneration, build and run the native integration test through Clay:

```sh
cd target/clay
./clay build --build debug-dev-test
./build/darwin-arm64-debug-dev-test/unittest_ccova/unittest_ccova
```