package lcc

func Parse(tokens []Token) (*ProgramNode, error) {
	core := newParserCore(tokens)
	core.expr = newExpressionParser(&core)
	return core.parseProgram()
}
