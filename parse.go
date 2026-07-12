package lcc

import (
	"fmt"
	"strconv"
)

type ProgramNode struct {
	Decls     []*TopLevelDeclNode
	Functions []*FunctionNode
}

type Parameter struct {
	Type *Type
	Name string
	Line int
}

type TopLevelDeclNode struct {
	Index  int
	Name   string
	Type   *Type
	Params []Parameter
	Kind   DeclKind
	Scope  ScopeKind
	Line   int
}

type FunctionNode struct {
	ReturnType *Type
	Name       string
	Params     []Parameter
	Body       *BlockStmt
	Line       int
}

type StmtNode interface {
	stmtNode()
}

type ExprNode interface {
	exprNode()
}

type LvalueNode interface {
	ExprNode
	EmitAddress(code *CodeMemory, c *Compiler)
}

type BlockStmt struct {
	Statements []StmtNode
	Line       int
}

type IfStmt struct {
	Condition ExprNode
	Then      StmtNode
	Line      int
}

type ReturnStmt struct {
	Value ExprNode
	Line  int
}

type ExprStmt struct {
	Expr ExprNode
	Line int
}

type AssignStmt struct {
	Target LvalueNode
	Value  ExprNode
	Line   int
}

type NumberLiteral struct {
	IntValue   int
	FloatValue float64
	IsFloat    bool
	Line       int
}

type IdentNode struct {
	Name string
	Line int
}

type BinaryExpr struct {
	Op    string
	Left  ExprNode
	Right ExprNode
	Line  int
}

type CallExpr struct {
	Callee string
	Args   []ExprNode
	Line   int
}

func (*BlockStmt) stmtNode()     {}
func (*IfStmt) stmtNode()        {}
func (*ReturnStmt) stmtNode()    {}
func (*ExprStmt) stmtNode()      {}
func (*AssignStmt) stmtNode()    {}
func (*NumberLiteral) exprNode() {}
func (*IdentNode) exprNode()     {}
func (*BinaryExpr) exprNode()    {}
func (*CallExpr) exprNode()      {}

func Parse(tokens []Token) (*ProgramNode, error) {
	parser := &parser{tokens: tokens}
	return parser.parseProgram()
}

type parser struct {
	tokens []Token
	pos    int
}

func (parser *parser) parseProgram() (*ProgramNode, error) {
	program := &ProgramNode{}
	for !parser.isEOF() {
		if parser.peek().Kind == TokKeyword && parser.peek().Value == "extern" {
			decl, err := parser.parseExternDecl()
			if err != nil {
				return nil, err
			}
			program.Decls = append(program.Decls, decl)
			continue
		}

		decl, function, err := parser.parseTopLevelDeclOrFunction()
		if err != nil {
			return nil, err
		}
		if decl != nil {
			program.Decls = append(program.Decls, decl)
			continue
		}
		program.Functions = append(program.Functions, function)
	}

	return program, nil
}

func (parser *parser) parseExternDecl() (*TopLevelDeclNode, error) {
	line := parser.peek().Line
	if _, err := parser.expectKeyword("extern"); err != nil {
		return nil, err
	}
	if _, err := parser.expectDelimiter("("); err != nil {
		return nil, err
	}
	indexToken, err := parser.expect(TokNum, "")
	if err != nil {
		return nil, err
	}
	if _, err := parser.expectDelimiter(")"); err != nil {
		return nil, err
	}

	index, err := strconv.Atoi(indexToken.Value)
	if err != nil {
		return nil, fmt.Errorf("syntax error on line %d: invalid extern index %q", indexToken.Line, indexToken.Value)
	}

	typ, err := parser.parseType()
	if err != nil {
		return nil, err
	}
	nameToken, err := parser.expect(TokIdent, "")
	if err != nil {
		return nil, err
	}

	decl := &TopLevelDeclNode{Index: index, Name: nameToken.Value, Type: typ, Scope: ScopeExtern, Line: line}
	if parser.matchDelimiter("(") {
		params, err := parser.parseParameters()
		if err != nil {
			return nil, err
		}
		decl.Params = params
		decl.Kind = DeclFunction
		if _, err := parser.expectDelimiter(")"); err != nil {
			return nil, err
		}
	} else {
		decl.Kind = DeclVariable
	}

	if _, err := parser.expectDelimiter(";"); err != nil {
		return nil, err
	}
	return decl, nil
}

func (parser *parser) parseTopLevelDeclOrFunction() (*TopLevelDeclNode, *FunctionNode, error) {
	line := parser.peek().Line
	returnType, err := parser.parseType()
	if err != nil {
		return nil, nil, err
	}
	nameToken, err := parser.expect(TokIdent, "")
	if err != nil {
		return nil, nil, err
	}
	if parser.matchDelimiter("(") {
		params, err := parser.parseParameters()
		if err != nil {
			return nil, nil, err
		}
		if _, err := parser.expectDelimiter(")"); err != nil {
			return nil, nil, err
		}
		body, err := parser.parseBlock()
		if err != nil {
			return nil, nil, err
		}
		return nil, &FunctionNode{ReturnType: returnType, Name: nameToken.Value, Params: params, Body: body, Line: line}, nil
	}
	if returnType.Kind == TypeVoid {
		return nil, nil, fmt.Errorf("syntax error on line %d: internal variable %q cannot have type void", line, nameToken.Value)
	}
	if _, err := parser.expectDelimiter(";"); err != nil {
		return nil, nil, err
	}
	decl := &TopLevelDeclNode{
		Index: -1,
		Name:  nameToken.Value,
		Type:  returnType,
		Kind:  DeclVariable,
		Scope: ScopeBSS,
		Line:  line,
	}
	return decl, nil, nil
}

func (parser *parser) parseParameters() ([]Parameter, error) {
	if parser.peek().Kind == TokDelimiter && parser.peek().Value == ")" {
		return nil, nil
	}

	params := make([]Parameter, 0, 4)
	for {
		typ, err := parser.parseType()
		if err != nil {
			return nil, err
		}
		nameToken, err := parser.expect(TokIdent, "")
		if err != nil {
			return nil, err
		}
		params = append(params, Parameter{Type: typ, Name: nameToken.Value, Line: nameToken.Line})

		if !parser.matchDelimiter(",") {
			break
		}
	}
	return params, nil
}

func (parser *parser) parseType() (*Type, error) {
	token := parser.peek()
	if token.Kind != TokKeyword {
		return nil, parser.errorf(token, "expected type")
	}
	typ := LookupNamedType(token.Value)
	if typ == nil {
		return nil, parser.errorf(token, "expected type")
	}
	parser.pos++

	for parser.peek().Kind == TokOp && parser.peek().Value == "*" {
		parser.pos++
		typ = PointerTo(typ)
	}

	return typ, nil
}

func (parser *parser) parseBlock() (*BlockStmt, error) {
	line := parser.peek().Line
	if _, err := parser.expectDelimiter("{"); err != nil {
		return nil, err
	}

	block := &BlockStmt{Line: line}
	for !(parser.peek().Kind == TokDelimiter && parser.peek().Value == "}") {
		if parser.isEOF() {
			return nil, parser.errorf(parser.peek(), "expected closing brace")
		}
		stmt, err := parser.parseStatement()
		if err != nil {
			return nil, err
		}
		block.Statements = append(block.Statements, stmt)
	}

	if _, err := parser.expectDelimiter("}"); err != nil {
		return nil, err
	}
	return block, nil
}

func (parser *parser) parseStatement() (StmtNode, error) {
	token := parser.peek()
	if token.Kind == TokDelimiter && token.Value == "{" {
		return parser.parseBlock()
	}
	if token.Kind == TokKeyword && token.Value == "if" {
		return parser.parseIfStmt()
	}
	if token.Kind == TokKeyword && token.Value == "return" {
		return parser.parseReturnStmt()
	}

	line := token.Line
	expr, err := parser.parseExpression()
	if err != nil {
		return nil, err
	}
	if parser.matchOperator("=") {
		target, ok := expr.(LvalueNode)
		if !ok {
			return nil, parser.errorf(token, "assignment target is not assignable")
		}
		value, err := parser.parseExpression()
		if err != nil {
			return nil, err
		}
		if _, err := parser.expectDelimiter(";"); err != nil {
			return nil, err
		}
		return &AssignStmt{Target: target, Value: value, Line: line}, nil
	}
	if _, err := parser.expectDelimiter(";"); err != nil {
		return nil, err
	}
	return &ExprStmt{Expr: expr, Line: line}, nil
}

func (parser *parser) parseIfStmt() (StmtNode, error) {
	line := parser.peek().Line
	if _, err := parser.expectKeyword("if"); err != nil {
		return nil, err
	}
	if _, err := parser.expectDelimiter("("); err != nil {
		return nil, err
	}
	condition, err := parser.parseExpression()
	if err != nil {
		return nil, err
	}
	if _, err := parser.expectDelimiter(")"); err != nil {
		return nil, err
	}
	thenStmt, err := parser.parseStatement()
	if err != nil {
		return nil, err
	}
	return &IfStmt{Condition: condition, Then: thenStmt, Line: line}, nil
}

func (parser *parser) parseReturnStmt() (StmtNode, error) {
	line := parser.peek().Line
	if _, err := parser.expectKeyword("return"); err != nil {
		return nil, err
	}
	if parser.matchDelimiter(";") {
		return &ReturnStmt{Line: line}, nil
	}
	value, err := parser.parseExpression()
	if err != nil {
		return nil, err
	}
	if _, err := parser.expectDelimiter(";"); err != nil {
		return nil, err
	}
	return &ReturnStmt{Value: value, Line: line}, nil
}

func (parser *parser) parseExpression() (ExprNode, error) {
	return parser.parseAdditive()
}

func (parser *parser) parseAdditive() (ExprNode, error) {
	left, err := parser.parseMultiplicative()
	if err != nil {
		return nil, err
	}

	for parser.peek().Kind == TokOp && (parser.peek().Value == "+" || parser.peek().Value == "-") {
		op := parser.peek()
		parser.pos++
		right, err := parser.parseMultiplicative()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Op: op.Value, Left: left, Right: right, Line: op.Line}
	}

	return left, nil
}

func (parser *parser) parseMultiplicative() (ExprNode, error) {
	left, err := parser.parsePrimary()
	if err != nil {
		return nil, err
	}

	for parser.peek().Kind == TokOp && (parser.peek().Value == "*" || parser.peek().Value == "/") {
		op := parser.peek()
		parser.pos++
		right, err := parser.parsePrimary()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Op: op.Value, Left: left, Right: right, Line: op.Line}
	}

	return left, nil
}

func (parser *parser) parsePrimary() (ExprNode, error) {
	token := parser.peek()
	switch token.Kind {
	case TokNum:
		parser.pos++
		if isFloatLiteral(token.Value) {
			value, err := strconv.ParseFloat(token.Value, 64)
			if err != nil {
				return nil, fmt.Errorf("syntax error on line %d: invalid float literal %q", token.Line, token.Value)
			}
			return &NumberLiteral{FloatValue: value, IsFloat: true, Line: token.Line}, nil
		}
		value, err := strconv.Atoi(token.Value)
		if err != nil {
			return nil, fmt.Errorf("syntax error on line %d: invalid integer literal %q", token.Line, token.Value)
		}
		return &NumberLiteral{IntValue: value, Line: token.Line}, nil
	case TokIdent:
		parser.pos++
		if parser.matchDelimiter("(") {
			args, err := parser.parseArguments()
			if err != nil {
				return nil, err
			}
			if _, err := parser.expectDelimiter(")"); err != nil {
				return nil, err
			}
			return &CallExpr{Callee: token.Value, Args: args, Line: token.Line}, nil
		}
		return &IdentNode{Name: token.Value, Line: token.Line}, nil
	case TokDelimiter:
		if token.Value != "(" {
			return nil, parser.errorf(token, "expected expression")
		}
		parser.pos++
		expr, err := parser.parseExpression()
		if err != nil {
			return nil, err
		}
		if _, err := parser.expectDelimiter(")"); err != nil {
			return nil, err
		}
		return expr, nil
	default:
		return nil, parser.errorf(token, "expected expression")
	}
}

func isFloatLiteral(value string) bool {
	for _, char := range value {
		if char == '.' || char == 'e' || char == 'E' {
			return true
		}
	}
	return false
}

func (parser *parser) parseArguments() ([]ExprNode, error) {
	if parser.peek().Kind == TokDelimiter && parser.peek().Value == ")" {
		return nil, nil
	}
	args := make([]ExprNode, 0, 4)
	for {
		expr, err := parser.parseExpression()
		if err != nil {
			return nil, err
		}
		args = append(args, expr)
		if !parser.matchDelimiter(",") {
			break
		}
	}
	return args, nil
}

func (parser *parser) expect(kind TokenKind, value string) (Token, error) {
	token := parser.peek()
	if token.Kind != kind {
		return Token{}, parser.errorf(token, parser.expectedLabel(kind, value))
	}
	if value != "" && token.Value != value {
		return Token{}, parser.errorf(token, fmt.Sprintf("expected %q", value))
	}
	parser.pos++
	return token, nil
}

func (parser *parser) expectKeyword(value string) (Token, error) {
	return parser.expect(TokKeyword, value)
}

func (parser *parser) expectDelimiter(value string) (Token, error) {
	return parser.expect(TokDelimiter, value)
}

func (parser *parser) matchDelimiter(value string) bool {
	if parser.peek().Kind == TokDelimiter && parser.peek().Value == value {
		parser.pos++
		return true
	}
	return false
}

func (parser *parser) matchOperator(value string) bool {
	if parser.peek().Kind == TokOp && parser.peek().Value == value {
		parser.pos++
		return true
	}
	return false
}

func (parser *parser) peek() Token {
	if parser.pos >= len(parser.tokens) {
		if len(parser.tokens) == 0 {
			return Token{Kind: TokEOF, Line: 1}
		}
		last := parser.tokens[len(parser.tokens)-1]
		return Token{Kind: TokEOF, Line: last.Line}
	}
	return parser.tokens[parser.pos]
}

func (parser *parser) isEOF() bool {
	return parser.peek().Kind == TokEOF
}

func (parser *parser) expectedLabel(kind TokenKind, value string) string {
	if value != "" {
		return fmt.Sprintf("expected %q", value)
	}
	return fmt.Sprintf("expected %s", tokenKindLabel(kind))
}

func (parser *parser) errorf(token Token, message string) error {
	return fmt.Errorf("syntax error on line %d: %s", token.Line, message)
}

func tokenKindLabel(kind TokenKind) string {
	switch kind {
	case TokKeyword:
		return "keyword"
	case TokIdent:
		return "identifier"
	case TokNum:
		return "number"
	case TokOp:
		return "operator"
	case TokDelimiter:
		return "delimiter"
	default:
		return "token"
	}
}
