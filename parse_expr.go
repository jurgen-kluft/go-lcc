package lcc

type expressionParser struct {
	core *parserCore
}

func newExpressionParser(core *parserCore) *expressionParser {
	return &expressionParser{core: core}
}

func (parser *expressionParser) parseExpression() (ExprNode, error) {
	return parser.parseExpressionWithBindingPower(0)
}

func (parser *expressionParser) parseExpressionWithBindingPower(minBindingPower int) (ExprNode, error) {
	left, err := parser.parsePrefixExpression()
	if err != nil {
		return nil, err
	}

	for {
		token := parser.core.peek()
		if token.Kind == TokDelimiter && token.Value == "(" {
			const callBindingPower = 70
			if callBindingPower < minBindingPower {
				break
			}
			ident, ok := left.(*IdentNode)
			if !ok {
				return nil, parser.core.errorf(token, "expected expression")
			}
			parser.core.pos++
			args, err := parser.core.parseArguments()
			if err != nil {
				return nil, err
			}
			if _, err := parser.core.expectDelimiter(")"); err != nil {
				return nil, err
			}
			left = &CallExpr{Callee: ident.Name, Args: args, Line: ident.Line}
			continue
		}

		leftBP, rightBP, ok := infixBindingPower(token)
		if !ok || leftBP < minBindingPower {
			break
		}
		parser.core.pos++
		right, err := parser.parseExpressionWithBindingPower(rightBP)
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Op: token.Value, Left: left, Right: right, Line: token.Line}
	}

	return left, nil
}

func (parser *expressionParser) parsePrefixExpression() (ExprNode, error) {
	token := parser.core.peek()
	if literal, ok, err := parser.core.parseLiteral(); ok || err != nil {
		return literal, err
	}

	switch token.Kind {
	case TokKeyword:
		return nil, parser.core.errorf(token, "expected expression")
	case TokIdent:
		parser.core.pos++
		return &IdentNode{Name: token.Value, Line: token.Line}, nil
	case TokDelimiter:
		if token.Value != "(" {
			return nil, parser.core.errorf(token, "expected expression")
		}
		parser.core.pos++
		expr, err := parser.parseExpressionWithBindingPower(0)
		if err != nil {
			return nil, err
		}
		if _, err := parser.core.expectDelimiter(")"); err != nil {
			return nil, err
		}
		return expr, nil
	default:
		return nil, parser.core.errorf(token, "expected expression")
	}
}

func infixBindingPower(token Token) (int, int, bool) {
	if token.Kind != TokOp {
		return 0, 0, false
	}
	switch token.Value {
	case "||":
		return 10, 11, true
	case "&&":
		return 20, 21, true
	case "==", "!=":
		return 30, 31, true
	case "<", ">", "<=", ">=":
		return 40, 41, true
	case "+", "-":
		return 50, 51, true
	case "*", "/":
		return 60, 61, true
	default:
		return 0, 0, false
	}
}