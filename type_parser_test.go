package lcc

import (
	"testing"
)

func parseProgram(t *testing.T, script string) *ProgramNode {
	t.Helper()
	tokens, err := Tokenize(script)
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}
	program, err := Parse(tokens)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	return program
}

func TestParseExpressions(t *testing.T) {
	script := `
bool bool1;
bool bool2;
bool bool3;
int counter;

int script_main() {
	int total = (1 + 2) * 3;
	if (bool1 && (bool2 || bool3)) {
		total = total + counter;
	}
	return total + helper(counter, 4);
}

int helper(int left, int right) {
	return left + right;
}
`

	program := parseProgram(t, script)
	if len(program.Decls) != 4 {
		t.Fatalf("expected 4 top-level declarations, got %d", len(program.Decls))
	}
	if len(program.Functions) != 2 {
		t.Fatalf("expected 2 functions, got %d", len(program.Functions))
	}
	mainFn := program.Functions[0]
	if len(mainFn.Body.Statements) != 3 {
		t.Fatalf("expected 3 statements in script_main, got %d", len(mainFn.Body.Statements))
	}
	if _, ok := mainFn.Body.Statements[1].(*IfStmt); !ok {
		t.Fatalf("expected second statement to be if, got %T", mainFn.Body.Statements[1])
	}
}

func TestParseRejectsLocalDeclarationInForInitializer(t *testing.T) {
	script := `
int script_main() {
	for (int i = 0; i < 4; i = i + 1) {
	}
	return 0;
}
`

	tokens, err := Tokenize(script)
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}
	if _, err := Parse(tokens); err == nil {
		t.Fatal("expected parser to reject local declaration in for initializer")
	}
}

func TestParseControlFlow(t *testing.T) {
	script := `
int counter;
bool cond;

int helper(int left, int right) {
	return left + right;
}

int script_main() {
	int total = helper(counter, 1 + 2);
	while (counter < 3 && cond || total == 0) {
		total = total + helper(counter, 2);
		counter = counter + 1;
	}
	for (counter = 0; counter < 2; counter = counter + 1) {
		switch (helper(counter, total)) {
		case 1:
			total = total + 10;
			break;
		default:
			total = total + 1;
			continue;
		}
	}
	return total;
}
`

	program := parseProgram(t, script)
	mainFn := program.Functions[1]
	if len(mainFn.Body.Statements) != 4 {
		t.Fatalf("expected 4 top-level statements in script_main, got %d", len(mainFn.Body.Statements))
	}
	if _, ok := mainFn.Body.Statements[1].(*WhileStmt); !ok {
		t.Fatalf("expected second statement to be while, got %T", mainFn.Body.Statements[1])
	}
	if _, ok := mainFn.Body.Statements[2].(*ForStmt); !ok {
		t.Fatalf("expected third statement to be for, got %T", mainFn.Body.Statements[2])
	}
	if _, ok := mainFn.Body.Statements[3].(*ReturnStmt); !ok {
		t.Fatalf("expected fourth statement to be return, got %T", mainFn.Body.Statements[3])
	}
}

func TestParseLocalInitializersAndReturns(t *testing.T) {
	script := `
float32 ratio;
bool ready;

float64 helper(float32 value) {
	return value + 2.5d;
}

float64 script_main() {
	float32 base = 1.5f;
	float64 total = helper((base + ratio) * 2.0f);
	if (ready) {
		return total + 3e1D;
	}
	return total;
}
`

	program := parseProgram(t, script)
	mainFn := program.Functions[1]
	if len(mainFn.Body.Statements) != 4 {
		t.Fatalf("expected 4 statements in script_main, got %d", len(mainFn.Body.Statements))
	}
	if _, ok := mainFn.Body.Statements[0].(*LocalDeclStmt); !ok {
		t.Fatalf("expected first statement to be local declaration, got %T", mainFn.Body.Statements[0])
	}
	if _, ok := mainFn.Body.Statements[1].(*LocalDeclStmt); !ok {
		t.Fatalf("expected second statement to be local declaration, got %T", mainFn.Body.Statements[1])
	}
	if _, ok := mainFn.Body.Statements[2].(*IfStmt); !ok {
		t.Fatalf("expected third statement to be if, got %T", mainFn.Body.Statements[2])
	}
	if _, ok := mainFn.Body.Statements[3].(*ReturnStmt); !ok {
		t.Fatalf("expected fourth statement to be return, got %T", mainFn.Body.Statements[3])
	}
}

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

func TestParseStringLiteralAsExpression(t *testing.T) {
	script := `
void script_main() {
	"asset/button_off";
	return;
}
`

	program := parseProgram(t, script)
	stmt, ok := program.Functions[0].Body.Statements[0].(*ExprStmt)
	if !ok {
		t.Fatalf("expected expression statement, got %T", program.Functions[0].Body.Statements[0])
	}
	literal, ok := stmt.Expr.(*StringLiteral)
	if !ok {
		t.Fatalf("expected string literal, got %T", stmt.Expr)
	}
	if literal.Value != "asset/button_off" {
		t.Fatalf("expected string literal value %q, got %q", "asset/button_off", literal.Value)
	}
}

func TestParseGlobalPointerStringInitializer(t *testing.T) {
	script := `
const uint8* asset_path = "asset/button_off";

void script_main() {
	return;
}
`

	program := parseProgram(t, script)
	decl := program.Decls[0]
	if decl.Scope != ScopeData {
		t.Fatalf("expected initialized pointer global to use data scope, got %d", decl.Scope)
	}
	if decl.Type == nil || !decl.Type.Base.IsConst || decl.Type.IsConst {
		t.Fatalf("expected parsed type const uint8*, got %v", decl.Type)
	}
	literal, ok := decl.Initializer.(*StringLiteral)
	if !ok {
		t.Fatalf("expected string literal initializer, got %T", decl.Initializer)
	}
	if literal.Value != "asset/button_off" {
		t.Fatalf("expected initializer value %q, got %q", "asset/button_off", literal.Value)
	}
}

func TestParseBooleanLiteralsAndLogicalPrecedence(t *testing.T) {
	script := `
int cond;

int script_main() {
	return true || false && cond == 1;
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
	orExpr, ok := ret.Value.(*BinaryExpr)
	if !ok {
		t.Fatalf("expected top-level binary expression, got %T", ret.Value)
	}
	if orExpr.Op != "||" {
		t.Fatalf("expected top-level operator ||, got %q", orExpr.Op)
	}
	leftLiteral, ok := orExpr.Left.(*NumberLiteral)
	if !ok {
		t.Fatalf("expected left operand to be numeric literal, got %T", orExpr.Left)
	}
	if leftLiteral.IntValue != 1 {
		t.Fatalf("expected true to lower to 1, got %d", leftLiteral.IntValue)
	}
	andExpr, ok := orExpr.Right.(*BinaryExpr)
	if !ok {
		t.Fatalf("expected right operand to be binary expression, got %T", orExpr.Right)
	}
	if andExpr.Op != "&&" {
		t.Fatalf("expected right operand operator &&, got %q", andExpr.Op)
	}
	falseLiteral, ok := andExpr.Left.(*NumberLiteral)
	if !ok {
		t.Fatalf("expected false literal to lower to numeric literal, got %T", andExpr.Left)
	}
	if falseLiteral.IntValue != 0 {
		t.Fatalf("expected false to lower to 0, got %d", falseLiteral.IntValue)
	}
	compareExpr, ok := andExpr.Right.(*BinaryExpr)
	if !ok {
		t.Fatalf("expected comparison expression on && right operand, got %T", andExpr.Right)
	}
	if compareExpr.Op != "==" {
		t.Fatalf("expected comparison operator ==, got %q", compareExpr.Op)
	}
}

func TestParseLogicalPrecedenceWithExplicitGrouping(t *testing.T) {
	script := `
bool bool1;
bool bool2;
bool bool3;

int script_main() {
	return bool1 && (bool2 || bool3);
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
	andExpr, ok := ret.Value.(*BinaryExpr)
	if !ok {
		t.Fatalf("expected top-level binary expression, got %T", ret.Value)
	}
	if andExpr.Op != "&&" {
		t.Fatalf("expected top-level operator &&, got %q", andExpr.Op)
	}
	leftIdent, ok := andExpr.Left.(*IdentNode)
	if !ok {
		t.Fatalf("expected left operand identifier, got %T", andExpr.Left)
	}
	if leftIdent.Name != "bool1" {
		t.Fatalf("expected left operand bool1, got %q", leftIdent.Name)
	}
	orExpr, ok := andExpr.Right.(*BinaryExpr)
	if !ok {
		t.Fatalf("expected grouped right operand to be binary expression, got %T", andExpr.Right)
	}
	if orExpr.Op != "||" {
		t.Fatalf("expected grouped right operand operator ||, got %q", orExpr.Op)
	}
	leftGrouped, ok := orExpr.Left.(*IdentNode)
	if !ok {
		t.Fatalf("expected grouped left operand identifier, got %T", orExpr.Left)
	}
	if leftGrouped.Name != "bool2" {
		t.Fatalf("expected grouped left operand bool2, got %q", leftGrouped.Name)
	}
	rightGrouped, ok := orExpr.Right.(*IdentNode)
	if !ok {
		t.Fatalf("expected grouped right operand identifier, got %T", orExpr.Right)
	}
	if rightGrouped.Name != "bool3" {
		t.Fatalf("expected grouped right operand bool3, got %q", rightGrouped.Name)
	}
}

func TestParseLocalDeclarations(t *testing.T) {
	script := `
int script_main() {
	int count;
	const bool ready = true;
	{
		int count = 3;
		count;
	}
	return count;
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
	countDecl, ok := statements[0].(*LocalDeclStmt)
	if !ok {
		t.Fatalf("expected first statement to be local declaration, got %T", statements[0])
	}
	if countDecl.Type != IntType || countDecl.Name != "count" || countDecl.Initializer != nil {
		t.Fatalf("unexpected first local declaration: type=%v name=%q initializer=%T", countDecl.Type, countDecl.Name, countDecl.Initializer)
	}
	readyDecl, ok := statements[1].(*LocalDeclStmt)
	if !ok {
		t.Fatalf("expected second statement to be local declaration, got %T", statements[1])
	}
	if readyDecl.Type == nil || readyDecl.Type.Kind != TypeBool || !readyDecl.Type.IsConst || readyDecl.Name != "ready" {
		t.Fatalf("unexpected second local declaration: type=%v name=%q", readyDecl.Type, readyDecl.Name)
	}
	readyInit, ok := readyDecl.Initializer.(*NumberLiteral)
	if !ok || readyInit.IntValue != 1 {
		t.Fatalf("expected bool initializer to lower to numeric literal 1, got %T value=%v", readyDecl.Initializer, readyDecl.Initializer)
	}
	innerBlock, ok := statements[2].(*BlockStmt)
	if !ok {
		t.Fatalf("expected third statement to be inner block, got %T", statements[2])
	}
	innerDecl, ok := innerBlock.Statements[0].(*LocalDeclStmt)
	if !ok {
		t.Fatalf("expected first inner statement to be local declaration, got %T", innerBlock.Statements[0])
	}
	if innerDecl.Name != "count" {
		t.Fatalf("expected inner declaration to shadow count, got %q", innerDecl.Name)
	}
}

func TestParseConstGlobalDeclaration(t *testing.T) {
	script := `
const uint8* asset_path = "asset/button_off";

void script_main() {
	return;
}
`

	program := parseProgram(t, script)
	decl := program.Decls[0]
	if decl.Type == nil || decl.Type.Kind != TypePointer || decl.Type.Base == nil {
		t.Fatalf("expected const global pointer type, got %v", decl.Type)
	}
	if decl.Type.IsConst {
		t.Fatalf("expected pointer itself to remain mutable for const uint8*, got %v", decl.Type)
	}
	if decl.Type.Base.Kind != TypeUint8 || !decl.Type.Base.IsConst {
		t.Fatalf("expected const global type uint8*, got %v", decl.Type)
	}
}

func TestParseExternConstDeclarationRejected(t *testing.T) {
	script := `extern(0) const uint8* asset_path;`
	tokens, err := Tokenize(script)
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}
	if _, err := Parse(tokens); err == nil {
		t.Fatal("expected const extern declaration to fail")
	}
}

func TestParseConstFunctionReturnType(t *testing.T) {
	script := `
const int script_main() {
	return 0;
}
`
	program := parseProgram(t, script)
	if len(program.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(program.Functions))
	}
	if program.Functions[0].ReturnType == nil || !program.Functions[0].ReturnType.IsConst || program.Functions[0].ReturnType.Kind != TypeInt32 {
		t.Fatalf("expected const int return type, got %v", program.Functions[0].ReturnType)
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
