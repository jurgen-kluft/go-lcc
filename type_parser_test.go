package lcc

import "testing"

func TestParseExpandedPrimitiveTypes(t *testing.T) {
	script := `
extern(0) int8 small_value;
extern(8) uint64 flags;
bool ready;
float32 ratio;

float64 script_main(int input, byte tag) {
	return ratio;
}
`

	tokens, err := Tokenize(script)
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}
	program, err := Parse(tokens)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(program.Decls) != 4 {
		t.Fatalf("expected 4 top-level declarations, got %d", len(program.Decls))
	}
	if program.Decls[0].Type != Int8Type {
		t.Fatalf("expected first decl type int8, got %v", program.Decls[0].Type)
	}
	if program.Decls[1].Type != Uint64Type {
		t.Fatalf("expected second decl type uint64, got %v", program.Decls[1].Type)
	}
	if program.Decls[2].Type != BoolType {
		t.Fatalf("expected third decl type bool, got %v", program.Decls[2].Type)
	}
	if program.Decls[3].Type != Float32Type {
		t.Fatalf("expected fourth decl type float32, got %v", program.Decls[3].Type)
	}
	if len(program.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(program.Functions))
	}
	function := program.Functions[0]
	if function.ReturnType != Float64Type {
		t.Fatalf("expected return type float64, got %v", function.ReturnType)
	}
	if len(function.Params) != 2 {
		t.Fatalf("expected 2 params, got %d", len(function.Params))
	}
	if function.Params[0].Type != IntType {
		t.Fatalf("expected int param to alias int32, got %v", function.Params[0].Type)
	}
	if function.Params[1].Type != ByteType {
		t.Fatalf("expected second param type byte, got %v", function.Params[1].Type)
	}
}

func TestLookupNamedTypeIntAlias(t *testing.T) {
	if LookupNamedType("int") != Int32Type {
		t.Fatalf("expected int to alias int32")
	}
}

func TestParseScientificNotationLiteralAsFloat(t *testing.T) {
	script := `
float64 script_main() {
	return 1e3;
}
`

	tokens, err := Tokenize(script)
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}
	program, err := Parse(tokens)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	ret, ok := program.Functions[0].Body.Statements[0].(*ReturnStmt)
	if !ok {
		t.Fatalf("expected return statement, got %T", program.Functions[0].Body.Statements[0])
	}
	lit, ok := ret.Value.(*NumberLiteral)
	if !ok {
		t.Fatalf("expected numeric literal, got %T", ret.Value)
	}
	if !lit.IsFloat {
		t.Fatal("expected scientific notation literal to parse as float")
	}
	if lit.FloatValue != 1000 {
		t.Fatalf("expected float value 1000, got %v", lit.FloatValue)
	}
	if lit.FloatType != Float32Type {
		t.Fatalf("expected unsuffixed scientific notation literal to default to float32, got %v", lit.FloatType)
	}
}

func TestParseFloatLiteralSuffixes(t *testing.T) {
	script := `
float64 script_main() {
	0.5;
	1.5f;
	2.5d;
	return 3e1D;
}
`

	tokens, err := Tokenize(script)
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}
	program, err := Parse(tokens)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	statements := program.Functions[0].Body.Statements
	checks := []struct {
		index int
		want  *Type
		value float64
	}{
		{index: 0, want: Float32Type, value: 0.5},
		{index: 1, want: Float32Type, value: 1.5},
		{index: 2, want: Float64Type, value: 2.5},
		{index: 3, want: Float64Type, value: 30},
	}
	for _, check := range checks {
		var expr ExprNode
		switch node := statements[check.index].(type) {
		case *ExprStmt:
			expr = node.Expr
		case *ReturnStmt:
			expr = node.Value
		default:
			t.Fatalf("expected numeric expression statement, got %T", statements[check.index])
		}
		lit, ok := expr.(*NumberLiteral)
		if !ok {
			t.Fatalf("expected numeric literal at statement %d, got %T", check.index, expr)
		}
		if !lit.IsFloat {
			t.Fatalf("expected float literal at statement %d", check.index)
		}
		if lit.FloatType != check.want {
			t.Fatalf("expected float type %v at statement %d, got %v", check.want, check.index, lit.FloatType)
		}
		if lit.FloatValue != check.value {
			t.Fatalf("expected float value %v at statement %d, got %v", check.value, check.index, lit.FloatValue)
		}
	}
}

func TestParseControlFlowStatements(t *testing.T) {
	script := `
int counter;

void script_main() {
	while (counter < 10) {
		counter = counter + 1;
	}
	for (counter = 0; counter < 4; counter = counter + 1) {
		switch (counter) {
		case 1:
			break;
		default:
			continue;
		}
	}
	return;
}
`

	tokens, err := Tokenize(script)
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}
	program, err := Parse(tokens)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	statements := program.Functions[0].Body.Statements
	if _, ok := statements[0].(*WhileStmt); !ok {
		t.Fatalf("expected first statement to be while, got %T", statements[0])
	}
	forStmt, ok := statements[1].(*ForStmt)
	if !ok {
		t.Fatalf("expected second statement to be for, got %T", statements[1])
	}
	body, ok := forStmt.Body.(*BlockStmt)
	if !ok {
		t.Fatalf("expected for body block, got %T", forStmt.Body)
	}
	if _, ok := body.Statements[0].(*SwitchStmt); !ok {
		t.Fatalf("expected switch statement inside for body, got %T", body.Statements[0])
	}
}
