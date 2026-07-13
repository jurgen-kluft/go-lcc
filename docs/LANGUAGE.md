# Cova Script Language

This document describes the scripting language currently supported by `go-cova`.

The language is a small, typed, C-like scripting language intended to compile into bytecode and run inside the `go-cova` VM. It supports primitive numeric types, global variables, functions, control flow, and host interop through `extern` declarations.

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

## Overview

The language currently supports:

- Top-level global variables
- Local variables inside blocks
- `const` globals, locals, and parameters
- Host-linked `extern` variables
- Host-linked `extern` functions
- Script-defined functions
- Primitive numeric and boolean-like types
- String literals lowered to pointer values
- Numeric expressions
- Function calls
- `if`, `if/else`, `while`, `for`, `switch`, `break`, `continue`, and `return`

The language does not currently support:

- Local variable declarations inside `for` initializers
- Arrays
- Structs
- Field access
- Unary operators such as `-x`, `!x`, `*ptr`, `&x`
- Bitwise operators
- Modulo
- Usable source-level pointer operations

## Compact Grammar

This grammar is intentionally compact. It shows the syntax that is currently accepted by the parser, not a full semantic specification.

```ebnf
program        ::= top_level*

top_level      ::= extern_decl | global_decl | function_decl

extern_decl    ::= "extern" "(" number ")" type ident extern_tail ";"
extern_tail    ::= "(" param_list? ")" | ε
global_decl    ::= type ident ("=" expr)? ";"
function_decl  ::= type ident "(" param_list? ")" block

param_list     ::= param ("," param)*
param          ::= type ident
const_qualifier ::= "const"
type           ::= const_qualifier? named_type const_qualifier? ("*" const_qualifier?)*
named_type     ::= "void" | "bool" | "byte" | "int" |
                   "int8" | "int16" | "int32" | "int64" |
                   "uint8" | "uint16" | "uint32" | "uint64" |
                   "float32" | "float64"

block          ::= "{" stmt* "}"
stmt           ::= block
                 | local_decl
                 | if_stmt
                 | while_stmt
                 | for_stmt
                 | switch_stmt
                 | return_stmt
                 | break_stmt
                 | continue_stmt
                 | assign_stmt
                 | expr_stmt

if_stmt        ::= "if" "(" expr ")" stmt ("else" stmt)?
while_stmt     ::= "while" "(" expr ")" stmt
for_stmt       ::= "for" "(" for_init? ";" expr? ";" for_post? ")" stmt
for_init       ::= assign_stmt_no_semi | expr
for_post       ::= assign_stmt_no_semi | expr
switch_stmt    ::= "switch" "(" expr ")" "{" switch_case* default_case? "}"
switch_case    ::= "case" expr ":" stmt*
default_case   ::= "default" ":" stmt*
return_stmt    ::= "return" expr? ";"
break_stmt     ::= "break" ";"
continue_stmt  ::= "continue" ";"
assign_stmt    ::= lvalue "=" expr ";"
assign_stmt_no_semi ::= lvalue "=" expr
local_decl     ::= const_qualifier? type ident ("=" expr)? ";"
expr_stmt      ::= expr ";"

 expr           ::= logical_or
 logical_or     ::= logical_and ("||" logical_and)*
 logical_and    ::= equality (("&&") equality)*
 equality       ::= relational (("==" | "!=") relational)*
 relational     ::= additive (("<" | "<=" | ">" | ">=") additive)*
 additive       ::= multiplicative (("+" | "-") multiplicative)*
 multiplicative ::= primary (("*" | "/") primary)*
 primary        ::= boolean
                  | number
                  | string
                 | ident
                 | call
                 | "(" expr ")"
call           ::= ident "(" arg_list? ")"
arg_list       ::= expr ("," expr)*
lvalue         ::= ident

ident          ::= identifier
 boolean       ::= "true" | "false"
number         ::= integer_literal | float_literal
string         ::= string_literal
```

Notes:

- `expr_stmt` is broader in the grammar than in the compiler. In practice, only function calls are accepted as standalone expression statements.
- Pointer type syntax is accepted in declarations and signatures, but source-level pointer operators are not implemented.
- Boolean conditions are numeric at runtime: `0` is false and non-zero is true.

## Top-Level Declarations

There are two kinds of top-level declarations:

- Internal globals
- `extern` declarations

### Internal globals

Internal globals without an initializer are stored in VM-managed BSS memory and are zero-initialized before execution.

```c
int counter;
float32 ratio;
bool ready;
```

Internal globals with an initializer are stored in writable DATA memory.

```c
int retries = 3;
```

Globals whose top-level type is const are stored in the read-only CONST segment. A pointer-to-const type such as `const uint8*` is not itself a const global, because the pointer value remains mutable.

```c
const int32 threshold = 7;
const uint8* const asset_path = "asset/button_off";
```

### Extern variables

Extern variables map onto a host-provided memory block by byte offset.

```c
extern(0) int32 health;
extern(8) uint64 flags;
extern(16) float64 temperature;
```

`extern(N)` gives the byte offset inside the extern memory block.

`extern` variables cannot be declared with `const`.

### Extern functions

Extern functions are host callbacks identified by import slot.

```c
extern(0) void log_value(int value);
extern(1) uint64 bounce(uint64 value);
```

The VM calls the registered extern dispatcher with the given import ID.

Runtime dispatch uses a fixed-width `uint32` import ID, an opaque host context token, and a stable numeric `VMStatus` result. The host reads arguments and writes a return value through checked typed VM stack helpers. Detailed extern signature metadata and exact consumption/production enforcement are part of the portable host ABI work still in progress.

## VM Execution Capacities

Operand-stack, aggregate frame-buffer, and call-frame capacities are host policy supplied through `VMConfig`. They are not computed from script control flow and are not requirements embedded in a linked program. Linking and loading therefore remain independent of the selected workspace capacities. If a reached execution path exceeds one of the configured limits, the VM returns the corresponding stable overflow status at that operation.

`LinkedProgram.FrameByteSize` records only the largest individual script-function frame produced by the compiler. Nested calls can require more frame memory, so this value must not be interpreted as aggregate workspace sizing.

### Script functions

Script functions are defined directly in the script.

```c
int add_one(int value) {
    return value + 1;
}
```

The preferred entry point is `script_main`. If it is absent, the compiler currently falls back to the first script function it sees.

### Local variables

Local variables can be declared inside blocks with or without an initializer.

```c
int count;
bool ready = true;
```

Locals whose top-level type is const cannot be reassigned after their declaration-time initialization.

```c
const int answer = 42;
```

`const uint8*` means a mutable pointer to const bytes, so the pointer can still be reassigned. `uint8* const` means a const pointer to mutable bytes.

Pointer locals can be initialized from string literals only when the pointed-to type is `const uint8`.

```c
const uint8* asset_path = "asset/button_off";
```

Locals are block-scoped and are zero-initialized when no initializer is provided.

## Types

Supported type names:

- `void`
- `bool`
- `byte`
- `int`
- `int8`
- `int16`
- `int32`
- `int64`
- `uint8`
- `uint16`
- `uint32`
- `uint64`
- `float32`
- `float64`

### Type aliases

`int` is an alias for `int32`.

Use `float32` and `float64` explicitly. Although `float` is tokenized as a keyword, it is not currently resolved as a valid named type.

## String Literals

String literals use double quotes and support a small escape set: `\"`, `\\`, `\n`, `\r`, `\t`, and `\0`.

```c
extern(0) void inspect(const uint8* path);

void script_main() {
    inspect("asset/button_off");
    return;
}
```

Current behavior:

- String literals are stored in the CONST segment.
- Identical string literals are de-duplicated inside the CONST segment.
- Each literal is emitted as a NUL-terminated byte string for C-style `const char*` interop.
- Only globals whose top-level type is const are stored in CONST.
- Mutable globals with initializers are stored in DATA.
- Zero-initialized globals remain in BSS.
- String literals are assignable only to pointer targets whose pointee type is `const uint8`, such as `const uint8*` or `const uint8* const`.

## Literals

### Integer literals

```c
0
1
42
255
```

### Floating-point literals

```c
0.5
2.25
1e3
7.25e-2
```

### Float suffixes

- `f` or `F` means `float32`
- `d` or `D` means `float64`

```c
1.5f
2.5d
6e3F
3e1D
```

### Boolean literals

The language supports `true` and `false` as boolean literals.

At runtime, boolean truth is still numeric:

- `false` is `0`
- any non-zero value is true

Examples:

```c
ready = true;
ready = false;
ready = 1;
ready = 7;
```

Unary minus is not part of the current grammar, so write:

```c
0 - 1
```

instead of:

```c
-1
```

## Expressions

Supported expressions:

- Boolean literals
- Numeric literals
- Variable references
- Function calls
- Parenthesized expressions
- Binary arithmetic
- Comparisons
- Logical operators

### Arithmetic

Supported operators:

- `+`
- `-`
- `*`
- `/`

Examples:

```c
counter + 1
total - delta
base * 2
amount / 4
```

### Comparisons

Supported operators:

- `==`
- `!=`
- `<`
- `<=`
- `>`
- `>=`

Examples:

```c
counter < 10
value == 3
health >= limit
```

Comparisons produce an `int32` result using `0` for false and `1` for true.

### Logical operators

Supported operators:

- `&&`
- `||`

Examples:

```c
ready && enabled
counter > 0 || limit == 3
false && mark_true()
```

Logical operators use numeric truthiness, short-circuit evaluation, and produce a normalized boolean result:

- `0` when the expression is false
- `1` when the expression is true

`&&` binds tighter than `||`, so `bool1 && bool2 || bool3` parses as `(bool1 && bool2) || bool3`. Use parentheses to override that grouping, for example `bool1 && (bool2 || bool3)`.

### Numeric promotion

The compiler promotes mixed numeric expressions to a suitable common type.

Examples:

- `int32 + float64` becomes `float64`
- `float32 + float64` becomes `float64`

Example:

```c
int base;

float64 script_main() {
    base = 2;
    return base + 1.5;
}
```

## Statements

Supported statements:

- Block statements
- Assignment
- Function-call expression statements
- `if`
- `if/else`
- `while`
- `for`
- `switch`
- `break`
- `continue`
- `return`

### Blocks

```c
{
    counter = counter + 1;
    return;
}
```

### Assignment

```c
counter = 10;
total = total + 1;
player_health = player_health - delta;
```

Assignments require the left-hand side to be assignable. At the moment, that mainly means identifiers naming globals or parameters.

### Standalone expression statements

Only function calls are allowed as standalone expression statements.

Valid:

```c
log_value(counter);
```

Invalid:

```c
1 + 2;
counter;
```

### If / else

```c
if (counter == 3) {
    total = total + 10;
} else {
    total = total + 1;
}
```

### While

```c
while (counter < 5) {
    counter = counter + 1;
}
```

### For

```c
for (counter = 0; counter < 4; counter = counter + 1) {
    total = total + counter;
}
```

There are no local variable declarations in the `for` initializer.

### Switch

```c
switch (counter) {
case 1:
    total = total + 10;
    break;
case 2:
    total = total + 20;
    break;
default:
    total = total + 1;
}
```

Supported inside switch:

- `case`
- `default`
- `break`

`continue` is meaningful if the `switch` is inside a loop.

There is no explicit `fallthrough` keyword. If a case body does not `break`, execution naturally continues into later emitted case bodies.

### Return

```c
return;
return value + 1;
```

A non-void function can return a value. A void function can return without one.

## Truthiness

Conditions are numeric.

- `0` means false
- non-zero means true

Examples:

```c
if (counter) {
    return;
}

if (player_health - 40) {
    log_alert(player_health);
}
```

This is why code like this is valid today:

```c
if ((player_health - 40) + 1) {
    ...
}
```

## Functions

Functions take typed parameters and may return a typed value.

```c
int64 level3(int64 value, int64 extra) {
    return value + extra;
}

int64 level2(int64 left, int64 right) {
    return level3(left + right, right);
}
```

### Parameters

Parameters are supported and stored in the function frame.

```c
float64 blend(float32 a, float64 b) {
    return a + b;
}
```

### No local declarations yet

This is not currently supported:

```c
int script_main() {
    int x;
    x = 1;
    return x;
}
```

Only parameters and globals currently provide named storage.

### Recursion is rejected

Recursive script call cycles are currently rejected at compile time, including indirect recursion.

Invalid:

```c
int recurse(int value) {
    return recurse(value);
}
```

## Host Interop

Host interop happens through `extern` declarations.

### Extern memory

The host can bind a byte slice as extern memory. Script variables declared with `extern(offset)` read and write into that memory.

```c
extern(0) int64 total;
extern(8) byte flag;
extern(9) bool ready;
```

Script:

```c
void script_main() {
    total = 42;
    flag = 255;
    ready = 1;
    return;
}
```

### Extern functions

The host registers a dispatcher. Script calls into it by import slot.

```c
extern(0) void inspect(byte status, bool ready, int64 total);

void script_main() {
    inspect(255, 1, 7);
    return;
}
```

Typed host interop works for current primitive value kinds, including integers, floats, `byte`, `bool`, and `uint64`.

## Pointers

Pointer types exist in the type system and pointer syntax such as `int32*` parses as a type.

However, pointer usage is not really available at source level yet because the language currently lacks:

- address-of syntax
- dereference syntax
- pointer arithmetic syntax exposed through the parser

Treat pointers as reserved or incomplete rather than usable.

## Current Limits and Gotchas

### No unary operators

These are not currently supported:

- `-x`
- `!x`
- `*ptr`
- `&x`

### No bitwise operators

These are not currently supported:

- `&`
- `|`
- `^`
- `<<`
- `>>`

### No modulo

`%` is not supported.

### No strings or aggregates

These are not currently supported:

- strings
- arrays
- structs
- field access
- indexing

### No local declarations in `for` initializers

This is still not supported:

```c
for (int i = 0; i < 4; i = i + 1) {
}
```

### Float type spelling

Use `float32` or `float64`. Do not use `float`.

## Minimal Working Examples

### Minimal script

```c
void script_main() {
    return;
}
```

### Integer arithmetic

```c
int script_main() {
    return 40 + 2;
}
```

### Float arithmetic

```c
float64 script_main() {
    return 1.5 + 2.25;
}
```

### Mixed int and float

```c
int base;

float64 script_main() {
    base = 2;
    return base + 1.5;
}
```

### Globals and loops

```c
int counter;
int total;

void script_main() {
    counter = 0;
    total = 0;
    while (counter < 5) {
        if (counter == 3) {
            total = total + 10;
        } else {
            total = total + 1;
        }
        counter = counter + 1;
    }
    return;
}
```

### Extern memory

```c
extern(4) int value;

void script_main() {
    value = value + 9;
    return;
}
```

### Extern function

```c
extern(0) void log_value(int value);

void script_main() {
    log_value(42);
    return;
}
```

## Summary

The current language is a compact VM scripting language with:

- Typed primitive values
- Global state
- Functions
- Numeric expressions
- Structured control flow
- Host interop via extern variables and extern functions

It is best suited today for small rule scripts, control-flow-heavy logic, and host-integrated numeric scripting, rather than general-purpose programming.