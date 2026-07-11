@workspace We are building a custom high-performance, embedded C-style scripting language toolchain in pure Go. 

The architecture must explicitly separate execution into 4 sequential phases: Tokenize ➔ Parse ➔ Compile ➔ Link. It must also be future-proofed to support structs, delegates, and pointer references later by strictly decoupling memory addresses from values using an Address-Calculating Lvalue architecture.

Please generate the complete source code across our blank files following these strict structural milestones:

----------------------------------------------------
MILESTONE 1: CORE INFRASTRUCTURE (Write to #types.go)
----------------------------------------------------
1. Define a type system struct `Type` containing: `Kind int`, `Name string`, `Size int`, and `Base *Type` (to support recursive pointer wrappers like int* natively). Create static variables for `IntType` and `VoidType`.
2. Define a flat slice allocation model for our Virtual Machine Memory.
3. Define the following future-proofed VM Opcodes as bytes:
   - Arithmetic: `OpPush`, `OpAdd`, `OpSub`, `OpMul`, `OpDiv`
   - Address Calculation: `OpAddrLocal` (pushes a local variable slot index), `OpAddrGlobalIdx` (pushes an annotation-declared global index), `OpOffset` (pops address and offset, pushes computed address).
   - Memory Mutators: `OpDereference` (pops address, pushes value at address), `OpAssign` (pops address, pops value, writes value to address).
   - Control Flow: `OpJumpIfFalse`, `OpCallGlobalIdx`, `OpRet`.

----------------------------------------------------
MILESTONE 2: TOKENIZER & DIAGNOSTICS (Write to #lex.go)
----------------------------------------------------
1. Implement a line-aware sequential string text scanner `Tokenize(src string) ([]Token, error)`.
2. Tokens must capture `Kind TokenKind`, `Value string`, and `Line int` for robust error tracing.
3. It must cleanly categorize `TokKeyword`, `TokIdent`, `TokNum`, `TokOp`, and `TokDelimiter`.
4. Lexical Rule: If an unrecognized symbol (like '@' or '#') is matched, it must abort immediately and return a clean error detailing the exact line number.

----------------------------------------------------
MILESTONE 3: PARSER & AST SCHEMATICS (Write to #parse.go)
----------------------------------------------------
1. Implement a recursive descent parser that outputs a structured Abstract Syntax Tree (`*ProgramNode`).
2. It must support our elegant global contract annotation syntax:
   `global(0) void log_alert(int data);` -> Maps external functions
   `global(0) int player_health;`       -> Maps shared variables
3. The parser must enforce strict grammar structures. If a delimiter like a semicolon or closing parenthesis is missing, return a descriptive syntax error containing the line number.
4. Define an interface called `LvalueNode` containing an `EmitAddress(code *[]byte, c *Compiler)` method to handle assignable targets natively.

----------------------------------------------------
MILESTONE 4: ADDRESS-CALCULATING COMPILER (Write to #compile.go)
----------------------------------------------------
1. Implement the AST visitor or type switch loop that translates the verified AST directly into unlinked binary instructions.
2. It must enforce the decoupled address pipeline: To compile a variable load, emit its address (`OpAddrLocal` or `OpAddrGlobalIdx`) followed immediately by `OpDereference`. To compile an assignment (`x = 42`), evaluate the value expression, emit the destination target's `EmitAddress`, and cap it with `OpAssign`.

----------------------------------------------------
MILESTONE 5: SLOT-BOUNDARY LINKER FIREWALL (Write to #link.go)
----------------------------------------------------
1. Implement a `Linker` engine that acts as a security validation barrier between the compiled bytecode and the live host Go application.
2. It must iterate through the parsed script's contract annotations and verify that the requested numeric indices (`global(X)`) fall completely within the boundaries of the initialized Go VM array capacities, returning an explicit linker error before execution if bounds are breached.

----------------------------------------------------
MILESTONE 6: INTEGRATION ENGINE (Write to #vm.go and #cmd/main.go)
----------------------------------------------------
1. Implement a lean, loop-driven stack `VM` running a tight switch block matching our Address Opcodes.
2. In the `main()` function, wire a complete end-to-end integration demo:
   - Define a native Go int variable (`hostPlayerHealth := 45`).
   - Write a mock script using our annotation headers that reads `player_health`, evaluates an `if` condition using basic arithmetic, calls a host gateway function hook, and modifies the global memory index state.
   - Execute the 4 steps linearly in memory, bind the host memory address pointers directly into the VM array by slot index, and print the updated Go variable outcome.

Please generate the complete, production-ready Go source blocks for these files now.
