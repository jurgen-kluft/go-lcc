package lcc

import "fmt"

func (core *parserCore) parseBlock() (*BlockStmt, error) {
	line := core.peek().Line
	if _, err := core.expectDelimiter("{"); err != nil {
		return nil, err
	}

	block := &BlockStmt{Line: line}
	for !(core.peek().Kind == TokDelimiter && core.peek().Value == "}") {
		if core.isEOF() {
			return nil, core.errorf(core.peek(), "expected closing brace")
		}
		stmt, err := core.parseStatement()
		if err != nil {
			return nil, err
		}
		block.Statements = append(block.Statements, stmt)
	}

	if _, err := core.expectDelimiter("}"); err != nil {
		return nil, err
	}
	return block, nil
}

func (core *parserCore) parseStatement() (StmtNode, error) {
	token := core.peek()
	if token.Kind == TokDelimiter && token.Value == "{" {
		return core.parseBlock()
	}
	if core.isTypeKeyword(token) {
		return core.parseLocalDeclStmt()
	}
	if token.Kind == TokKeyword && token.Value == "if" {
		return core.parseIfStmt()
	}
	if token.Kind == TokKeyword && token.Value == "while" {
		return core.parseWhileStmt()
	}
	if token.Kind == TokKeyword && token.Value == "for" {
		return core.parseForStmt()
	}
	if token.Kind == TokKeyword && token.Value == "switch" {
		return core.parseSwitchStmt()
	}
	if token.Kind == TokKeyword && token.Value == "return" {
		return core.parseReturnStmt()
	}
	if token.Kind == TokKeyword && token.Value == "break" {
		core.pos++
		if _, err := core.expectDelimiter(";"); err != nil {
			return nil, err
		}
		return &BreakStmt{Line: token.Line}, nil
	}
	if token.Kind == TokKeyword && token.Value == "continue" {
		core.pos++
		if _, err := core.expectDelimiter(";"); err != nil {
			return nil, err
		}
		return &ContinueStmt{Line: token.Line}, nil
	}

	line := token.Line
	expr, err := core.parseExpression()
	if err != nil {
		return nil, err
	}
	if core.matchOperator("=") {
		target, ok := expr.(LvalueNode)
		if !ok {
			return nil, core.errorf(token, "assignment target is not assignable")
		}
		value, err := core.parseExpression()
		if err != nil {
			return nil, err
		}
		if _, err := core.expectDelimiter(";"); err != nil {
			return nil, err
		}
		return &AssignStmt{Target: target, Value: value, Line: line}, nil
	}
	if _, err := core.expectDelimiter(";"); err != nil {
		return nil, err
	}
	return &ExprStmt{Expr: expr, Line: line}, nil
}

func (core *parserCore) parseLocalDeclStmt() (StmtNode, error) {
	line := core.peek().Line
	typ, err := core.parseType()
	if err != nil {
		return nil, err
	}
	nameToken, err := core.expect(TokIdent, "")
	if err != nil {
		return nil, err
	}
	if typ.Kind == TypeVoid {
		return nil, fmt.Errorf("syntax error on line %d: local variable %q cannot have type void", line, nameToken.Value)
	}
	var initializer ExprNode
	if core.matchOperator("=") {
		initializer, err = core.parseExpression()
		if err != nil {
			return nil, err
		}
	}
	if _, err := core.expectDelimiter(";"); err != nil {
		return nil, err
	}
	return &LocalDeclStmt{Type: typ, Name: nameToken.Value, Initializer: initializer, Line: line}, nil
}

func (core *parserCore) parseIfStmt() (StmtNode, error) {
	line := core.peek().Line
	if _, err := core.expectKeyword("if"); err != nil {
		return nil, err
	}
	if _, err := core.expectDelimiter("("); err != nil {
		return nil, err
	}
	condition, err := core.parseExpression()
	if err != nil {
		return nil, err
	}
	if _, err := core.expectDelimiter(")"); err != nil {
		return nil, err
	}
	thenStmt, err := core.parseStatement()
	if err != nil {
		return nil, err
	}
	var elseStmt StmtNode
	if core.peek().Kind == TokKeyword && core.peek().Value == "else" {
		core.pos++
		elseStmt, err = core.parseStatement()
		if err != nil {
			return nil, err
		}
	}
	return &IfStmt{Condition: condition, Then: thenStmt, Else: elseStmt, Line: line}, nil
}

func (core *parserCore) parseWhileStmt() (StmtNode, error) {
	line := core.peek().Line
	if _, err := core.expectKeyword("while"); err != nil {
		return nil, err
	}
	if _, err := core.expectDelimiter("("); err != nil {
		return nil, err
	}
	condition, err := core.parseExpression()
	if err != nil {
		return nil, err
	}
	if _, err := core.expectDelimiter(")"); err != nil {
		return nil, err
	}
	body, err := core.parseStatement()
	if err != nil {
		return nil, err
	}
	return &WhileStmt{Condition: condition, Body: body, Line: line}, nil
}

func (core *parserCore) parseForStmt() (StmtNode, error) {
	line := core.peek().Line
	if _, err := core.expectKeyword("for"); err != nil {
		return nil, err
	}
	if _, err := core.expectDelimiter("("); err != nil {
		return nil, err
	}
	var init StmtNode
	if !(core.peek().Kind == TokDelimiter && core.peek().Value == ";") {
		stmt, err := core.parseForClauseStatement()
		if err != nil {
			return nil, err
		}
		init = stmt
	}
	if _, err := core.expectDelimiter(";"); err != nil {
		return nil, err
	}
	var condition ExprNode
	if !(core.peek().Kind == TokDelimiter && core.peek().Value == ";") {
		expr, err := core.parseExpression()
		if err != nil {
			return nil, err
		}
		condition = expr
	}
	if _, err := core.expectDelimiter(";"); err != nil {
		return nil, err
	}
	var post StmtNode
	if !(core.peek().Kind == TokDelimiter && core.peek().Value == ")") {
		stmt, err := core.parseForClauseStatement()
		if err != nil {
			return nil, err
		}
		post = stmt
	}
	if _, err := core.expectDelimiter(")"); err != nil {
		return nil, err
	}
	body, err := core.parseStatement()
	if err != nil {
		return nil, err
	}
	return &ForStmt{Init: init, Condition: condition, Post: post, Body: body, Line: line}, nil
}

func (core *parserCore) parseSwitchStmt() (StmtNode, error) {
	line := core.peek().Line
	if _, err := core.expectKeyword("switch"); err != nil {
		return nil, err
	}
	if _, err := core.expectDelimiter("("); err != nil {
		return nil, err
	}
	value, err := core.parseExpression()
	if err != nil {
		return nil, err
	}
	if _, err := core.expectDelimiter(")"); err != nil {
		return nil, err
	}
	if _, err := core.expectDelimiter("{"); err != nil {
		return nil, err
	}
	stmt := &SwitchStmt{Value: value, Line: line}
	for !(core.peek().Kind == TokDelimiter && core.peek().Value == "}") {
		if core.isEOF() {
			return nil, core.errorf(core.peek(), "expected closing brace")
		}
		keyword := core.peek()
		if keyword.Kind == TokKeyword && keyword.Value == "case" {
			core.pos++
			caseValue, err := core.parseExpression()
			if err != nil {
				return nil, err
			}
			if _, err := core.expectDelimiter(":"); err != nil {
				return nil, err
			}
			caseBody, err := core.parseSwitchClauseBody()
			if err != nil {
				return nil, err
			}
			stmt.Cases = append(stmt.Cases, SwitchCase{Value: caseValue, Body: caseBody, Line: keyword.Line})
			continue
		}
		if keyword.Kind == TokKeyword && keyword.Value == "default" {
			core.pos++
			if _, err := core.expectDelimiter(":"); err != nil {
				return nil, err
			}
			defaultBody, err := core.parseSwitchClauseBody()
			if err != nil {
				return nil, err
			}
			stmt.Default = defaultBody
			continue
		}
		return nil, core.errorf(keyword, "expected case or default")
	}
	if _, err := core.expectDelimiter("}"); err != nil {
		return nil, err
	}
	return stmt, nil
}

func (core *parserCore) parseSwitchClauseBody() ([]StmtNode, error) {
	body := make([]StmtNode, 0, 4)
	for {
		token := core.peek()
		if token.Kind == TokDelimiter && token.Value == "}" {
			break
		}
		if token.Kind == TokKeyword && (token.Value == "case" || token.Value == "default") {
			break
		}
		stmt, err := core.parseStatement()
		if err != nil {
			return nil, err
		}
		body = append(body, stmt)
	}
	return body, nil
}

func (core *parserCore) parseForClauseStatement() (StmtNode, error) {
	token := core.peek()
	line := token.Line
	expr, err := core.parseExpression()
	if err != nil {
		return nil, err
	}
	if core.matchOperator("=") {
		target, ok := expr.(LvalueNode)
		if !ok {
			return nil, core.errorf(token, "assignment target is not assignable")
		}
		value, err := core.parseExpression()
		if err != nil {
			return nil, err
		}
		return &AssignStmt{Target: target, Value: value, Line: line}, nil
	}
	return &ExprStmt{Expr: expr, Line: line}, nil
}

func (core *parserCore) parseReturnStmt() (StmtNode, error) {
	line := core.peek().Line
	if _, err := core.expectKeyword("return"); err != nil {
		return nil, err
	}
	if core.matchDelimiter(";") {
		return &ReturnStmt{Line: line}, nil
	}
	value, err := core.parseExpression()
	if err != nil {
		return nil, err
	}
	if _, err := core.expectDelimiter(";"); err != nil {
		return nil, err
	}
	return &ReturnStmt{Value: value, Line: line}, nil
}