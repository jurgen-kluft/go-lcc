# go-lcc

`go-lcc` is a small compiler, linker, and VM for a typed, C-like scripting language.

The language is designed for small host-integrated scripts that work with primitive numeric types, control flow, global state, and `extern` bindings for memory and host functions.

## What It Supports

- Primitive types: `bool`, `byte`, `int`/`int32`, `int8`, `int16`, `int64`, `uint8`, `uint16`, `uint32`, `uint64`, `float32`, `float64`
- Top-level globals
- Local variables inside function blocks
- Script functions with typed parameters and returns
- `extern(offset)` variables backed by host memory
- `extern(slot)` functions dispatched by the host
- String literals passed as pointer values
- Expressions using `true`, `false`, `+`, `-`, `*`, `/`, `==`, `!=`, `<`, `<=`, `>`, `>=`, `&&`, `||`
- Control flow: `if`, `if/else`, `while`, `for`, `switch`, `break`, `continue`, `return`

## Current Limits

- No local declarations in `for` initializers
- Standalone expression statements must be function calls
- No arrays, structs, or field access
- No unary operators such as `-x`, `!x`, `*ptr`, or `&x`
- No bitwise operators or modulo
- Pointer types can be declared in signatures and declarations, but source-level pointer operators are not implemented
- Recursive script call cycles are rejected at compile time

String literals are stored in a CONST segment as NUL-terminated byte strings. Zero-initialized globals remain in BSS, while initialized writable globals are placed in DATA.

Booleans use numeric truthiness at runtime: `false` is `0`, and any non-zero value is true. Logical operators short-circuit and produce normalized `0` or `1` results.

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

- Full language overview: [LANGUAGE.md](LANGUAGE.md)

## Development

Run the test suite with:

```sh
go test ./...
```