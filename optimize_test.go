package cova

import (
	"math"
	"strings"
	"testing"
)

func TestOptimizeFoldsNestedArithmeticWithDestinationType(t *testing.T) {
	program := parseOptimizerTestProgram(t, `
int8 folded;
void script_main() {
	folded = (120 + 10) * 2;
}
`)
	if err := Optimize(program); err != nil {
		t.Fatalf("Optimize failed: %v", err)
	}
	assignment := program.Functions[0].Body.Statements[0].(*AssignStmt)
	literal, ok := assignment.Value.(*NumberLiteral)
	if !ok {
		t.Fatalf("expected folded number literal, got %T", assignment.Value)
	}
	if literal.IntValue != 4 {
		t.Fatalf("expected int8 wrapping result 4, got %d", literal.IntValue)
	}
}

func TestOptimizeFoldsComparisonAndLogicalExpressions(t *testing.T) {
	program := parseOptimizerTestProgram(t, `
void script_main() {
	int result = (2 + 3 == 5) && (9 > 4);
}
`)
	if err := Optimize(program); err != nil {
		t.Fatalf("Optimize failed: %v", err)
	}
	declaration := program.Functions[0].Body.Statements[0].(*LocalDeclStmt)
	literal, ok := declaration.Initializer.(*NumberLiteral)
	if !ok || literal.IntValue != 1 {
		t.Fatalf("expected folded true literal, got %#v", declaration.Initializer)
	}
}

func TestOptimizeSkipsUnreachableLogicalBranch(t *testing.T) {
	program := parseOptimizerTestProgram(t, `
void script_main() {
	int result = false && (1 / 0);
}
`)
	if err := Optimize(program); err != nil {
		t.Fatalf("Optimize evaluated unreachable branch: %v", err)
	}
}

func TestOptimizeReportsReachableDivisionByZero(t *testing.T) {
	program := parseOptimizerTestProgram(t, `
void script_main() {
	int result = 1 / 0;
}
`)
	err := Optimize(program)
	if err == nil || !strings.Contains(err.Error(), "optimization error on line 3: division by zero") {
		t.Fatalf("expected line-numbered division error, got %v", err)
	}
}

func TestOptimizeFoldsGlobalInitializerAndCallArgument(t *testing.T) {
	program := parseOptimizerTestProgram(t, `
int total = 2 + 3 * 4;
extern(0) void consume(float32 value);
void script_main() {
	consume(1.25f + 2.5f);
}
`)
	if err := Optimize(program); err != nil {
		t.Fatalf("Optimize failed: %v", err)
	}
	global, ok := program.Decls[0].Initializer.(*NumberLiteral)
	if !ok || global.IntValue != 14 {
		t.Fatalf("expected folded global initializer 14, got %#v", program.Decls[0].Initializer)
	}
	if _, err := NewCompiler().Compile(program); err != nil {
		t.Fatalf("Compile failed after folding global initializer: %v", err)
	}
	call := program.Functions[0].Body.Statements[0].(*ExprStmt).Expr.(*CallExpr)
	argument, ok := call.Args[0].(*NumberLiteral)
	if !ok || !argument.IsFloat || argument.FloatType != Float32Type || argument.FloatValue != 3.75 {
		t.Fatalf("expected folded float32 call argument, got %#v", call.Args[0])
	}
}

func TestOptimizePreservesFloatComparisonSemantics(t *testing.T) {
	nan := &NumberLiteral{FloatValue: math.NaN(), IsFloat: true, FloatType: Float64Type, Line: 1}
	program := &ProgramNode{Functions: []*FunctionNode{{
		Name:       "script_main",
		ReturnType: VoidType,
		Body: &BlockStmt{Statements: []StmtNode{&LocalDeclStmt{
			Name:        "result",
			Type:        Int32Type,
			Initializer: &BinaryExpr{Op: "!=", Left: nan, Right: nan, Line: 1},
		}}},
	}}}
	if err := Optimize(program); err != nil {
		t.Fatalf("Optimize failed: %v", err)
	}
	literal := program.Functions[0].Body.Statements[0].(*LocalDeclStmt).Initializer.(*NumberLiteral)
	if literal.IntValue != 1 {
		t.Fatalf("expected NaN != NaN to fold true, got %d", literal.IntValue)
	}
}

func TestOptimizeMatchesUnoptimizedRuntimeResult(t *testing.T) {
	source := `
int result;
void script_main() {
	int8 narrow = (120 + 10) * 2;
	float32 fraction = (7.0f / 3.0f) * 3.0f;
	result = narrow + (fraction > 6.9f);
}
`
	unoptimized := runOptimizerTestProgram(t, source, false)
	optimized := runOptimizerTestProgram(t, source, true)
	if optimized != unoptimized {
		t.Fatalf("optimized result %d differs from unoptimized result %d", optimized, unoptimized)
	}
}

func TestOptimizeRejectsNilProgram(t *testing.T) {
	if err := Optimize(nil); err == nil || err.Error() != "optimization error: program is nil" {
		t.Fatalf("expected nil program error, got %v", err)
	}
}

func runOptimizerTestProgram(t *testing.T, source string, optimize bool) int32 {
	t.Helper()
	program := parseOptimizerTestProgram(t, source)
	if optimize {
		if err := Optimize(program); err != nil {
			t.Fatalf("Optimize failed: %v", err)
		}
	}
	compiled, err := NewCompiler().Compile(program)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}
	linked, err := NewLinker(0, 0).Link(program, compiled)
	if err != nil {
		t.Fatalf("Link failed: %v", err)
	}
	vm := NewVM(testFrameCapacityBytes)
	if status := vm.Run(linked); status != VMStatusOK {
		t.Fatalf("Run failed: %s", status)
	}
	offset := linked.DebugSymbols.Symbols["result"].ByteOffset
	result, status := vm.memory.ReadInt32(makeAddress(segmentBSS, offset))
	if status != VMStatusOK {
		t.Fatalf("ReadInt32 result failed: %s", status)
	}
	return result
}

func parseOptimizerTestProgram(t *testing.T, source string) *ProgramNode {
	t.Helper()
	tokens, err := Tokenize(source)
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}
	program, err := Parse(tokens)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	return program
}
