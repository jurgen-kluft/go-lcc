package lcc

import (
	"fmt"
	"strconv"
)

type exprParser interface {
	parseExpression() (ExprNode, error)
}

type parserCore struct {
	tokens []Token
	pos    int
	expr   exprParser
}

func newParserCore(tokens []Token) parserCore {
	return parserCore{tokens: tokens}
}

func (core *parserCore) parseExpression() (ExprNode, error) {
	if core.expr == nil {
		return nil, core.errorf(core.peek(), "expected expression")
	}
	return core.expr.parseExpression()
}

func (core *parserCore) expect(kind TokenKind, value string) (Token, error) {
	token := core.peek()
	if token.Kind != kind {
		return Token{}, core.errorf(token, core.expectedLabel(kind, value))
	}
	if value != "" && token.Value != value {
		return Token{}, core.errorf(token, fmt.Sprintf("expected %q", value))
	}
	core.pos++
	return token, nil
}

func (core *parserCore) expectKeyword(value string) (Token, error) {
	return core.expect(TokKeyword, value)
}

func (core *parserCore) expectDelimiter(value string) (Token, error) {
	return core.expect(TokDelimiter, value)
}

func (core *parserCore) matchDelimiter(value string) bool {
	if core.peek().Kind == TokDelimiter && core.peek().Value == value {
		core.pos++
		return true
	}
	return false
}

func (core *parserCore) matchOperator(value string) bool {
	if core.peek().Kind == TokOp && core.peek().Value == value {
		core.pos++
		return true
	}
	return false
}

func (core *parserCore) peek() Token {
	if core.pos >= len(core.tokens) {
		if len(core.tokens) == 0 {
			return Token{Kind: TokEOF, Line: 1}
		}
		last := core.tokens[len(core.tokens)-1]
		return Token{Kind: TokEOF, Line: last.Line}
	}
	return core.tokens[core.pos]
}

func (core *parserCore) isEOF() bool {
	return core.peek().Kind == TokEOF
}

func (core *parserCore) parseType() (*Type, error) {
	leadingConst := core.parseConstQualifier()
	token := core.peek()
	if token.Kind != TokKeyword {
		return nil, core.errorf(token, "expected type")
	}
	typ := LookupNamedType(token.Value)
	if typ == nil {
		return nil, core.errorf(token, "expected type")
	}
	core.pos++
	typ = QualifiedType(typ, leadingConst)
	if core.parseConstQualifier() {
		typ = QualifiedType(typ, true)
	}

	for core.peek().Kind == TokOp && core.peek().Value == "*" {
		core.pos++
		pointerConst := core.parseConstQualifier()
		typ = PointerToQualified(typ, pointerConst)
	}

	return typ, nil
}

func (core *parserCore) parseConstQualifier() bool {
	token := core.peek()
	if token.Kind == TokKeyword && token.Value == "const" {
		core.pos++
		return true
	}
	return false
}

func (core *parserCore) isTypeKeyword(token Token) bool {
	if token.Kind != TokKeyword {
		return false
	}
	return token.Value == "const" || LookupNamedType(token.Value) != nil
}

func (core *parserCore) parseArguments() ([]ExprNode, error) {
	if core.peek().Kind == TokDelimiter && core.peek().Value == ")" {
		return nil, nil
	}
	args := make([]ExprNode, 0, 4)
	for {
		expr, err := core.parseExpression()
		if err != nil {
			return nil, err
		}
		args = append(args, expr)
		if !core.matchDelimiter(",") {
			break
		}
	}
	return args, nil
}

func (core *parserCore) parseLiteral() (ExprNode, bool, error) {
	token := core.peek()
	switch token.Kind {
	case TokNum:
		core.pos++
		if isFloatLiteral(token.Value) {
			literalValue, floatType := parseFloatLiteralSpec(token.Value)
			value, err := strconv.ParseFloat(literalValue, 64)
			if err != nil {
				return nil, true, fmt.Errorf("syntax error on line %d: invalid float literal %q", token.Line, token.Value)
			}
			return &NumberLiteral{FloatValue: value, IsFloat: true, FloatType: floatType, Line: token.Line}, true, nil
		}
		value, err := strconv.Atoi(token.Value)
		if err != nil {
			return nil, true, fmt.Errorf("syntax error on line %d: invalid integer literal %q", token.Line, token.Value)
		}
		return &NumberLiteral{IntValue: value, Line: token.Line}, true, nil
	case TokString:
		core.pos++
		return &StringLiteral{Value: token.Value, Line: token.Line}, true, nil
	case TokKeyword:
		switch token.Value {
		case "true":
			core.pos++
			return &NumberLiteral{IntValue: 1, Line: token.Line}, true, nil
		case "false":
			core.pos++
			return &NumberLiteral{IntValue: 0, Line: token.Line}, true, nil
		}
	}
	return nil, false, nil
}

func isFloatLiteral(value string) bool {
	for _, char := range value {
		if char == '.' || char == 'e' || char == 'E' {
			return true
		}
	}
	if len(value) == 0 {
		return false
	}
	suffix := value[len(value)-1]
	return suffix == 'f' || suffix == 'F' || suffix == 'd' || suffix == 'D'
}

func parseFloatLiteralSpec(value string) (string, *Type) {
	if len(value) == 0 {
		return value, Float32Type
	}
	suffix := value[len(value)-1]
	switch suffix {
	case 'f', 'F':
		return value[:len(value)-1], Float32Type
	case 'd', 'D':
		return value[:len(value)-1], Float64Type
	default:
		return value, Float32Type
	}
}

func (core *parserCore) expectedLabel(kind TokenKind, value string) string {
	if value != "" {
		return fmt.Sprintf("expected %q", value)
	}
	return fmt.Sprintf("expected %s", tokenKindLabel(kind))
}

func (core *parserCore) errorf(token Token, message string) error {
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
	case TokString:
		return "string"
	case TokOp:
		return "operator"
	case TokDelimiter:
		return "delimiter"
	default:
		return "token"
	}
}