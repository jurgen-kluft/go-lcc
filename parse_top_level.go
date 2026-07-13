package lcc

import (
	"fmt"
	"strconv"
)

func (core *parserCore) parseProgram() (*ProgramNode, error) {
	program := &ProgramNode{}
	for !core.isEOF() {
		if core.peek().Kind == TokKeyword && core.peek().Value == "extern" {
			decl, err := core.parseExternDecl()
			if err != nil {
				return nil, err
			}
			program.Decls = append(program.Decls, decl)
			continue
		}

		decl, function, err := core.parseTopLevelDeclOrFunction()
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

func (core *parserCore) parseExternDecl() (*TopLevelDeclNode, error) {
	line := core.peek().Line
	if _, err := core.expectKeyword("extern"); err != nil {
		return nil, err
	}
	if _, err := core.expectDelimiter("("); err != nil {
		return nil, err
	}
	indexToken, err := core.expect(TokNum, "")
	if err != nil {
		return nil, err
	}
	if _, err := core.expectDelimiter(")"); err != nil {
		return nil, err
	}

	index, err := strconv.Atoi(indexToken.Value)
	if err != nil {
		return nil, fmt.Errorf("syntax error on line %d: invalid extern index %q", indexToken.Line, indexToken.Value)
	}
	if core.peek().Kind == TokKeyword && core.peek().Value == "const" {
		return nil, fmt.Errorf("syntax error on line %d: extern declarations cannot be const", core.peek().Line)
	}

	typ, err := core.parseType()
	if err != nil {
		return nil, err
	}
	nameToken, err := core.expect(TokIdent, "")
	if err != nil {
		return nil, err
	}

	decl := &TopLevelDeclNode{Index: index, Name: nameToken.Value, Type: typ, Scope: ScopeExtern, Line: line}
	if core.matchDelimiter("(") {
		params, err := core.parseParameters()
		if err != nil {
			return nil, err
		}
		decl.Params = params
		decl.Kind = DeclFunction
		if _, err := core.expectDelimiter(")"); err != nil {
			return nil, err
		}
	} else {
		decl.Kind = DeclVariable
	}

	if _, err := core.expectDelimiter(";"); err != nil {
		return nil, err
	}
	return decl, nil
}

func (core *parserCore) parseTopLevelDeclOrFunction() (*TopLevelDeclNode, *FunctionNode, error) {
	line := core.peek().Line
	returnType, err := core.parseType()
	if err != nil {
		return nil, nil, err
	}
	nameToken, err := core.expect(TokIdent, "")
	if err != nil {
		return nil, nil, err
	}
	if core.matchDelimiter("(") {
		params, err := core.parseParameters()
		if err != nil {
			return nil, nil, err
		}
		if _, err := core.expectDelimiter(")"); err != nil {
			return nil, nil, err
		}
		body, err := core.parseBlock()
		if err != nil {
			return nil, nil, err
		}
		return nil, &FunctionNode{ReturnType: returnType, Name: nameToken.Value, Params: params, Body: body, Line: line}, nil
	}
	if returnType.Kind == TypeVoid {
		return nil, nil, fmt.Errorf("syntax error on line %d: internal variable %q cannot have type void", line, nameToken.Value)
	}
	var initializer ExprNode
	scope := ScopeBSS
	if core.matchOperator("=") {
		initializer, err = core.parseExpression()
		if err != nil {
			return nil, nil, err
		}
		scope = ScopeData
	}
	if IsTopLevelConst(returnType) {
		scope = ScopeConst
	}
	if _, err := core.expectDelimiter(";"); err != nil {
		return nil, nil, err
	}
	decl := &TopLevelDeclNode{
		Index:       -1,
		Name:        nameToken.Value,
		Type:        returnType,
		Kind:        DeclVariable,
		Scope:       scope,
		Initializer: initializer,
		Line:        line,
	}
	return decl, nil, nil
}

func (core *parserCore) parseParameters() ([]Parameter, error) {
	if core.peek().Kind == TokDelimiter && core.peek().Value == ")" {
		return nil, nil
	}

	params := make([]Parameter, 0, 4)
	for {
		typ, err := core.parseType()
		if err != nil {
			return nil, err
		}
		nameToken, err := core.expect(TokIdent, "")
		if err != nil {
			return nil, err
		}
		params = append(params, Parameter{Type: typ, Name: nameToken.Value, Line: nameToken.Line})

		if !core.matchDelimiter(",") {
			break
		}
	}
	return params, nil
}