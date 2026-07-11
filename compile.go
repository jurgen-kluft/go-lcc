package lcc

import "fmt"

type CompiledProgram struct {
	Code            []byte
	Globals         map[string]GlobalBinding
	GlobalVars      []GlobalBinding
	GlobalFunctions []GlobalBinding
	Entry           string
	LocalSlotCount  int
}

type Compiler struct {
	program        *ProgramNode
	code           []byte
	globalBindings map[string]GlobalBinding
	globalVars     []GlobalBinding
	globalFuncs    []GlobalBinding
	localSlots     map[string]int
	localSlotCount int
	entry          string
	err            error
}

func NewCompiler() *Compiler {
	return &Compiler{}
}

func (compiler *Compiler) Compile(program *ProgramNode) (*CompiledProgram, error) {
	if program == nil {
		return nil, fmt.Errorf("compile error: program is nil")
	}
	if len(program.Functions) == 0 {
		return nil, fmt.Errorf("compile error: no function definitions found")
	}

	compiler.program = program
	compiler.code = compiler.code[:0]
	compiler.globalBindings = make(map[string]GlobalBinding, len(program.Globals))
	compiler.globalVars = compiler.globalVars[:0]
	compiler.globalFuncs = compiler.globalFuncs[:0]
	compiler.entry = program.Functions[0].Name
	compiler.err = nil

	for _, decl := range program.Globals {
		if _, exists := compiler.globalBindings[decl.Name]; exists {
			return nil, fmt.Errorf("compile error on line %d: duplicate global contract %q", decl.Line, decl.Name)
		}
		binding := GlobalBinding{Name: decl.Name, Index: decl.Index, Kind: decl.Kind, Type: decl.Type, Arity: len(decl.Params)}
		compiler.globalBindings[decl.Name] = binding
		if decl.Kind == GlobalVariable {
			compiler.globalVars = append(compiler.globalVars, binding)
		} else {
			compiler.globalFuncs = append(compiler.globalFuncs, binding)
		}
	}

	maxLocalSlots := 0
	for _, function := range program.Functions {
		compiler.compileFunction(function)
		if compiler.err != nil {
			return nil, compiler.err
		}
		if compiler.localSlotCount > maxLocalSlots {
			maxLocalSlots = compiler.localSlotCount
		}
	}

	compiled := &CompiledProgram{
		Code:            append([]byte(nil), compiler.code...),
		Globals:         compiler.globalBindings,
		GlobalVars:      append([]GlobalBinding(nil), compiler.globalVars...),
		GlobalFunctions: append([]GlobalBinding(nil), compiler.globalFuncs...),
		Entry:           compiler.entry,
		LocalSlotCount:  maxLocalSlots,
	}
	return compiled, nil
}

func (compiler *Compiler) compileFunction(function *FunctionNode) {
	if compiler.err != nil {
		return
	}
	compiler.localSlots = make(map[string]int, len(function.Params))
	compiler.localSlotCount = 0
	for _, param := range function.Params {
		if _, exists := compiler.localSlots[param.Name]; exists {
			compiler.fail(fmt.Errorf("compile error on line %d: duplicate parameter %q", param.Line, param.Name))
			return
		}
		compiler.localSlots[param.Name] = compiler.localSlotCount
		compiler.localSlotCount++
	}

	compiler.compileBlock(function.Body)
	if compiler.err != nil {
		return
	}
	if len(compiler.code) == 0 || Opcode(compiler.code[len(compiler.code)-1]) != OpRet {
		compiler.emit(OpRet)
	}
}

func (compiler *Compiler) compileBlock(block *BlockStmt) {
	for _, stmt := range block.Statements {
		compiler.compileStmt(stmt)
		if compiler.err != nil {
			return
		}
	}
}

func (compiler *Compiler) compileStmt(stmt StmtNode) {
	if compiler.err != nil {
		return
	}

	switch node := stmt.(type) {
	case *BlockStmt:
		compiler.compileBlock(node)
	case *IfStmt:
		compiler.compileExpr(node.Condition)
		jumpPos := compiler.emitOpWithOperand(OpJumpIfFalse, 0)
		compiler.compileStmt(node.Then)
		compiler.patchOperand(jumpPos, len(compiler.code))
	case *ReturnStmt:
		if node.Value != nil {
			compiler.compileExpr(node.Value)
		}
		compiler.emit(OpRet)
	case *ExprStmt:
		if _, ok := node.Expr.(*CallExpr); !ok {
			compiler.fail(fmt.Errorf("compile error on line %d: only function call expressions can be used as standalone statements", node.Line))
			return
		}
		compiler.compileExpr(node.Expr)
	case *AssignStmt:
		compiler.compileExpr(node.Value)
		node.Target.EmitAddress(&compiler.code, compiler)
		if compiler.err != nil {
			return
		}
		compiler.emit(OpAssign)
	default:
		compiler.fail(fmt.Errorf("compile error: unsupported statement type %T", stmt))
	}
}

func (compiler *Compiler) compileExpr(expr ExprNode) {
	if compiler.err != nil {
		return
	}

	switch node := expr.(type) {
	case *NumberLiteral:
		compiler.emitOpWithOperand(OpPush, node.Value)
	case *IdentNode:
		node.EmitAddress(&compiler.code, compiler)
		if compiler.err != nil {
			return
		}
		compiler.emit(OpDereference)
	case *BinaryExpr:
		compiler.compileExpr(node.Left)
		compiler.compileExpr(node.Right)
		if compiler.err != nil {
			return
		}
		switch node.Op {
		case "+":
			compiler.emit(OpAdd)
		case "-":
			compiler.emit(OpSub)
		case "*":
			compiler.emit(OpMul)
		case "/":
			compiler.emit(OpDiv)
		default:
			compiler.fail(fmt.Errorf("compile error on line %d: unsupported binary operator %q", node.Line, node.Op))
		}
	case *CallExpr:
		binding, ok := compiler.globalBindings[node.Callee]
		if !ok || binding.Kind != GlobalFunction {
			compiler.fail(fmt.Errorf("compile error on line %d: unknown global function %q", node.Line, node.Callee))
			return
		}
		if len(node.Args) != binding.Arity {
			compiler.fail(fmt.Errorf("compile error on line %d: global function %q expects %d arguments, got %d", node.Line, node.Callee, binding.Arity, len(node.Args)))
			return
		}
		for _, arg := range node.Args {
			compiler.compileExpr(arg)
			if compiler.err != nil {
				return
			}
		}
		compiler.emitOpWithOperand(OpCallGlobalIdx, binding.Index)
	default:
		compiler.fail(fmt.Errorf("compile error: unsupported expression type %T", expr))
	}
}

func (node *IdentNode) EmitAddress(code *[]byte, compiler *Compiler) {
	if slot, ok := compiler.localSlots[node.Name]; ok {
		*code = append(*code, byte(OpAddrLocal))
		writeInt(code, slot)
		return
	}
	binding, ok := compiler.globalBindings[node.Name]
	if ok && binding.Kind == GlobalVariable {
		*code = append(*code, byte(OpAddrGlobalIdx))
		writeInt(code, binding.Index)
		return
	}
	compiler.fail(fmt.Errorf("compile error on line %d: unknown variable %q", node.Line, node.Name))
}

func (compiler *Compiler) emit(op Opcode) {
	compiler.code = append(compiler.code, byte(op))
}

func (compiler *Compiler) emitOpWithOperand(op Opcode, operand int) int {
	compiler.emit(op)
	position := len(compiler.code)
	writeInt(&compiler.code, operand)
	return position
}

func (compiler *Compiler) patchOperand(position int, operand int) {
	compiler.code[position] = byte(operand)
	compiler.code[position+1] = byte(operand >> 8)
	compiler.code[position+2] = byte(operand >> 16)
	compiler.code[position+3] = byte(operand >> 24)
}

func (compiler *Compiler) fail(err error) {
	if compiler.err == nil {
		compiler.err = err
	}
}
