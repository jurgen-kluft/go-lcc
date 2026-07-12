package lcc

import (
	"fmt"
	"math"
)

type Compiler struct {
	program               *ProgramNode
	code                  CodeMemory
	symbolBindings        map[string]SymbolBinding
	externSymbols         []SymbolBinding
	bssSymbols            []SymbolBinding
	functions             []SymbolBinding
	functionBindingByTemp map[int]int
	localSlots            map[string]int
	localTypes            map[string]*Type
	currentReturnType     *Type
	localSlotCount        int
	maxLocalSlots         int
	maxFrameByteSize      int
	bssByteSize           int
	entryFunction         int
	nextTempFuncID        int
	callPatches           []CallPatch
	err                   error
}

func NewCompiler() *Compiler {
	return &Compiler{}
}

func cloneTypeSlice(types []*Type) []*Type {
	if len(types) == 0 {
		return nil
	}
	return append([]*Type(nil), types...)
}

func parameterTypes(params []Parameter) []*Type {
	if len(params) == 0 {
		return nil
	}
	types := make([]*Type, 0, len(params))
	for _, param := range params {
		types = append(types, param.Type)
	}
	return types
}

func valueKindsFromTypes(types []*Type) []ValueKind {
	if len(types) == 0 {
		return nil
	}
	kinds := make([]ValueKind, 0, len(types))
	for _, typ := range types {
		kinds = append(kinds, valueKindFromType(typ))
	}
	return kinds
}

func (compiler *Compiler) Compile(program *ProgramNode) (*RelocatableProgram, error) {
	if program == nil {
		return nil, fmt.Errorf("compile error: program is nil")
	}
	if len(program.Functions) == 0 {
		return nil, fmt.Errorf("compile error: no function definitions found")
	}

	compiler.program = program
	compiler.code = compiler.code[:0]
	compiler.symbolBindings = make(map[string]SymbolBinding, len(program.Decls)+len(program.Functions))
	compiler.externSymbols = compiler.externSymbols[:0]
	compiler.bssSymbols = compiler.bssSymbols[:0]
	compiler.functions = compiler.functions[:0]
	compiler.functionBindingByTemp = make(map[int]int, len(program.Functions))
	compiler.localSlots = nil
	compiler.localTypes = nil
	compiler.currentReturnType = nil
	compiler.localSlotCount = 0
	compiler.maxLocalSlots = 0
	compiler.maxFrameByteSize = 0
	compiler.bssByteSize = 0
	compiler.entryFunction = -1
	compiler.nextTempFuncID = 0
	compiler.callPatches = compiler.callPatches[:0]
	compiler.err = nil

	for _, decl := range program.Decls {
		compiler.registerTopLevelDecl(decl)
		if compiler.err != nil {
			return nil, compiler.err
		}
	}
	for _, function := range program.Functions {
		compiler.registerScriptFunction(function)
		if compiler.err != nil {
			return nil, compiler.err
		}
	}
	for _, function := range program.Functions {
		compiler.compileFunction(function)
		if compiler.err != nil {
			return nil, compiler.err
		}
	}

	compiled := &RelocatableProgram{
		Text:          compiler.code.Clone(),
		Symbols:       cloneBindingsMap(compiler.symbolBindings),
		ExternSymbols: append([]SymbolBinding(nil), compiler.externSymbols...),
		BSSSymbols:    append([]SymbolBinding(nil), compiler.bssSymbols...),
		Functions:     append([]SymbolBinding(nil), compiler.functions...),
		CallPatches:   append([]CallPatch(nil), compiler.callPatches...),
		EntryFunction: compiler.entryFunction,
		FrameSize:     compiler.maxLocalSlots,
		FrameByteSize: compiler.maxFrameByteSize,
		BSSSize:       len(compiler.bssSymbols),
		BSSByteSize:   compiler.bssByteSize,
	}
	return compiled, nil
}

func (compiler *Compiler) registerTopLevelDecl(decl *TopLevelDeclNode) {
	if decl == nil || compiler.err != nil {
		return
	}
	if _, exists := compiler.symbolBindings[decl.Name]; exists {
		compiler.fail(fmt.Errorf("compile error on line %d: duplicate top-level declaration %q", decl.Line, decl.Name))
		return
	}

	binding := SymbolBinding{
		Name:          decl.Name,
		Kind:          decl.Kind,
		Scope:         decl.Scope,
		Type:          decl.Type,
		ByteSize:      decl.Type.Size,
		ByteAlignment: decl.Type.Alignment(),
		ParamCount:    len(decl.Params),
		ParamTypes:    parameterTypes(decl.Params),
	}

	switch decl.Kind {
	case DeclVariable:
		switch decl.Scope {
		case ScopeExtern:
			binding.SlotIndex = decl.Index
			binding.ByteOffset = decl.Index
			compiler.externSymbols = append(compiler.externSymbols, binding)
		case ScopeBSS:
			binding.SlotIndex = len(compiler.bssSymbols)
			binding.ByteOffset = alignUp(compiler.bssByteSize, binding.ByteAlignment)
			compiler.bssByteSize = binding.ByteOffset + binding.ByteSize
			compiler.bssSymbols = append(compiler.bssSymbols, binding)
		default:
			compiler.fail(fmt.Errorf("compile error on line %d: variable %q has invalid scope %d", decl.Line, decl.Name, decl.Scope))
			return
		}
	case DeclFunction:
		if decl.Scope != ScopeExtern {
			compiler.fail(fmt.Errorf("compile error on line %d: function contract %q must be host-linked", decl.Line, decl.Name))
			return
		}
		binding.SlotIndex = decl.Index
		binding.TempFuncID = compiler.allocateTempFuncID()
		compiler.trackFunctionBinding(binding)
	default:
		compiler.fail(fmt.Errorf("compile error on line %d: unsupported declaration kind %d", decl.Line, decl.Kind))
		return
	}

	compiler.symbolBindings[decl.Name] = binding
}

func (compiler *Compiler) registerScriptFunction(function *FunctionNode) {
	if function == nil || compiler.err != nil {
		return
	}
	if _, exists := compiler.symbolBindings[function.Name]; exists {
		compiler.fail(fmt.Errorf("compile error on line %d: duplicate top-level declaration %q", function.Line, function.Name))
		return
	}
	binding := SymbolBinding{
		Name:          function.Name,
		Kind:          DeclFunction,
		Scope:         ScopeBSS,
		Type:          function.ReturnType,
		ByteSize:      function.ReturnType.Size,
		ByteAlignment: function.ReturnType.Alignment(),
		ParamCount:    len(function.Params),
		ParamTypes:    parameterTypes(function.Params),
		TempFuncID:    compiler.allocateTempFuncID(),
	}
	compiler.trackFunctionBinding(binding)
	compiler.symbolBindings[function.Name] = binding
	if compiler.entryFunction < 0 {
		compiler.entryFunction = binding.TempFuncID
	}
}

func (compiler *Compiler) allocateTempFuncID() int {
	tempFuncID := compiler.nextTempFuncID
	compiler.nextTempFuncID++
	return tempFuncID
}

func (compiler *Compiler) trackFunctionBinding(binding SymbolBinding) {
	compiler.functionBindingByTemp[binding.TempFuncID] = len(compiler.functions)
	compiler.functions = append(compiler.functions, binding)
}

func (compiler *Compiler) compileFunction(function *FunctionNode) {
	if compiler.err != nil || function == nil {
		return
	}
	binding, ok := compiler.symbolBindings[function.Name]
	if !ok || binding.Kind != DeclFunction {
		compiler.fail(fmt.Errorf("compile error on line %d: unknown function %q", function.Line, function.Name))
		return
	}
	binding.ScriptAddress = len(compiler.code)
	compiler.localSlots = make(map[string]int, len(function.Params))
	compiler.localTypes = make(map[string]*Type, len(function.Params))
	compiler.currentReturnType = function.ReturnType
	compiler.localSlotCount = 0
	frameByteSize := 0
	paramOffsets := make([]int, 0, len(function.Params))
	for _, param := range function.Params {
		if _, exists := compiler.localSlots[param.Name]; exists {
			compiler.fail(fmt.Errorf("compile error on line %d: duplicate parameter %q", param.Line, param.Name))
			return
		}
		frameByteSize = alignUp(frameByteSize, param.Type.Alignment())
		compiler.localSlots[param.Name] = frameByteSize
		compiler.localTypes[param.Name] = param.Type
		paramOffsets = append(paramOffsets, frameByteSize)
		compiler.localSlotCount++
		frameByteSize += param.Type.Size
	}
	binding.ParamOffsets = paramOffsets
	binding.FrameSlotCount = compiler.localSlotCount
	binding.FrameByteSize = frameByteSize
	binding.ScriptAddress = len(compiler.code)
	compiler.storeFunctionBinding(binding)
	compiler.code.AppendFunctionHeader(ScriptFunctionHeader{
		ParamCount:    binding.ParamCount,
		ParamKinds:    valueKindsFromTypes(binding.ParamTypes),
		ParamOffsets:  binding.ParamOffsets,
		FrameByteSize: binding.FrameByteSize,
		ReturnKind:    valueKindFromType(binding.Type),
	})

	compiler.compileBlock(function.Body)
	if compiler.err != nil {
		return
	}
	if len(compiler.code) == 0 || Opcode(compiler.code[len(compiler.code)-1]) != OpRet {
		compiler.emit(OpRet)
	}
	if compiler.localSlotCount > compiler.maxLocalSlots {
		compiler.maxLocalSlots = compiler.localSlotCount
	}
	if frameByteSize > compiler.maxFrameByteSize {
		compiler.maxFrameByteSize = frameByteSize
	}
	compiler.localTypes = nil
	compiler.currentReturnType = nil
}

func (compiler *Compiler) storeFunctionBinding(binding SymbolBinding) {
	compiler.symbolBindings[binding.Name] = binding
	if functionIndex, ok := compiler.functionBindingByTemp[binding.TempFuncID]; ok {
		compiler.functions[functionIndex] = binding
	}
}

func cloneBindingsMap(bindings map[string]SymbolBinding) map[string]SymbolBinding {
	if len(bindings) == 0 {
		return nil
	}
	clone := make(map[string]SymbolBinding, len(bindings))
	for name, binding := range bindings {
		binding.ParamTypes = cloneTypeSlice(binding.ParamTypes)
		if len(binding.ParamOffsets) != 0 {
			binding.ParamOffsets = append([]int(nil), binding.ParamOffsets...)
		}
		clone[name] = binding
	}
	return clone
}

func (compiler *Compiler) exprType(expr ExprNode) *Type {
	switch node := expr.(type) {
	case *NumberLiteral:
		if node.IsFloat {
			return Float64Type
		}
		return Int32Type
	case *IdentNode:
		if localType, ok := compiler.localTypes[node.Name]; ok {
			return localType
		}
		if binding, ok := compiler.symbolBindings[node.Name]; ok {
			return binding.Type
		}
	case *BinaryExpr:
		left := compiler.exprType(node.Left)
		right := compiler.exprType(node.Right)
		return promoteNumericType(left, right)
	case *CallExpr:
		if binding, ok := compiler.symbolBindings[node.Callee]; ok {
			return binding.Type
		}
	}
	return nil
}

func (compiler *Compiler) compileExprAs(expr ExprNode, expected *Type) {
	if compiler.err != nil {
		return
	}
	kind := valueKindFromType(expected)
	if kind == KindNone {
		kind = KindInt32
	}
	switch node := expr.(type) {
	case *NumberLiteral:
		compiler.emitTyped(OpPush, kind)
		compiler.code.AppendImmediate(kind, compiler.numberLiteralBits(node, kind))
	case *IdentNode:
		actualKind := kind
		if actual := compiler.exprType(expr); actual != nil {
			actualKind = valueKindFromType(actual)
		}
		if actualKind == KindNone {
			actualKind = KindInt32
		}
		node.EmitAddress(&compiler.code, compiler)
		if compiler.err != nil {
			return
		}
		compiler.emitTyped(OpDereference, actualKind)
		compiler.emitConvertIfNeeded(actualKind, kind)
	case *BinaryExpr:
		binaryType := expected
		if binaryType == nil {
			binaryType = compiler.exprType(expr)
		}
		binaryKind := valueKindFromType(binaryType)
		if binaryKind == KindNone {
			binaryKind = KindInt32
			binaryType = Int32Type
		}
		compiler.compileExprAs(node.Left, binaryType)
		compiler.compileExprAs(node.Right, binaryType)
		if compiler.err != nil {
			return
		}
		switch node.Op {
		case "+":
			compiler.emitTyped(OpAdd, binaryKind)
		case "-":
			compiler.emitTyped(OpSub, binaryKind)
		case "*":
			compiler.emitTyped(OpMul, binaryKind)
		case "/":
			compiler.emitTyped(OpDiv, binaryKind)
		default:
			compiler.fail(fmt.Errorf("compile error on line %d: unsupported binary operator %q", node.Line, node.Op))
		}
	case *CallExpr:
		binding, ok := compiler.symbolBindings[node.Callee]
		if !ok || binding.Kind != DeclFunction {
			compiler.fail(fmt.Errorf("compile error on line %d: unknown function %q", node.Line, node.Callee))
			return
		}
		if len(node.Args) != binding.ParamCount {
			compiler.fail(fmt.Errorf("compile error on line %d: function %q expects %d arguments, got %d", node.Line, node.Callee, binding.ParamCount, len(node.Args)))
			return
		}
		for index, arg := range node.Args {
			var paramType *Type
			if index < len(binding.ParamTypes) {
				paramType = binding.ParamTypes[index]
			}
			compiler.compileExprAs(arg, paramType)
			if compiler.err != nil {
				return
			}
		}
		if binding.Scope == ScopeExtern {
			compiler.emitOpWithOperand(OpCallExtern, binding.SlotIndex)
		} else {
			operandPos := compiler.emitOpWithOperand(OpCall, binding.TempFuncID)
			compiler.callPatches = append(compiler.callPatches, CallPatch{OperandPos: operandPos, TempFuncID: binding.TempFuncID, Line: node.Line})
		}
		returnKind := valueKindFromType(binding.Type)
		if returnKind != KindNone {
			compiler.emitConvertIfNeeded(returnKind, kind)
		}
	default:
		compiler.fail(fmt.Errorf("compile error: unsupported expression type %T", expr))
	}
}

func (compiler *Compiler) emitConvertIfNeeded(from ValueKind, to ValueKind) {
	if compiler.err != nil || from == to || to == KindNone || from == KindNone {
		return
	}
	if !isNumericKind(from) || !isNumericKind(to) {
		compiler.fail(fmt.Errorf("compile error: unsupported conversion from kind %d to kind %d", from, to))
		return
	}
	compiler.emitTyped(OpConvert, to)
	compiler.code.AppendImmediate(KindUint8, uint64(from))
}

func isNumericKind(kind ValueKind) bool {
	switch kind {
	case KindBool, KindByte, KindInt8, KindInt16, KindInt32, KindInt64, KindUint8, KindUint16, KindUint32, KindUint64, KindFloat32, KindFloat64:
		return true
	default:
		return false
	}
}

func promoteNumericType(left *Type, right *Type) *Type {
	if left == nil {
		return right
	}
	if right == nil {
		return left
	}
	if left == right {
		return left
	}
	if left.Kind == TypeFloat64 || right.Kind == TypeFloat64 {
		return Float64Type
	}
	if left.Kind == TypeFloat32 || right.Kind == TypeFloat32 {
		return Float32Type
	}
	if left.Kind == TypeUint64 || right.Kind == TypeUint64 {
		return Uint64Type
	}
	if left.Kind == TypeInt64 || right.Kind == TypeInt64 {
		return Int64Type
	}
	if left.Kind == TypeUint32 || right.Kind == TypeUint32 {
		return Uint32Type
	}
	if left.Kind == TypeUint16 || right.Kind == TypeUint16 {
		return Uint16Type
	}
	if left.Kind == TypeUint8 || right.Kind == TypeUint8 || left.Kind == TypeByte || right.Kind == TypeByte {
		return Uint8Type
	}
	return Int32Type
}

func (compiler *Compiler) numberLiteralBits(node *NumberLiteral, kind ValueKind) uint64 {
	if node == nil {
		return 0
	}
	if node.IsFloat {
		switch kind {
		case KindFloat32:
			return uint64(math.Float32bits(float32(node.FloatValue)))
		case KindFloat64:
			return math.Float64bits(node.FloatValue)
		default:
			return uint64(int64(node.FloatValue))
		}
	}
	switch kind {
	case KindFloat32:
		return uint64(math.Float32bits(float32(node.IntValue)))
	case KindFloat64:
		return math.Float64bits(float64(node.IntValue))
	default:
		return uint64(node.IntValue)
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
		compiler.compileExprAs(node.Condition, Int32Type)
		jumpPos := compiler.emitOpWithOperand(OpJumpIfFalse, 0)
		compiler.compileStmt(node.Then)
		compiler.patchOperand(jumpPos, len(compiler.code))
	case *ReturnStmt:
		if node.Value != nil {
			compiler.compileExprAs(node.Value, compiler.currentReturnType)
		}
		compiler.emit(OpRet)
	case *ExprStmt:
		if _, ok := node.Expr.(*CallExpr); !ok {
			compiler.fail(fmt.Errorf("compile error on line %d: only function call expressions can be used as standalone statements", node.Line))
			return
		}
		compiler.compileExpr(node.Expr)
	case *AssignStmt:
		targetType := compiler.exprType(node.Target)
		compiler.compileExprAs(node.Value, targetType)
		node.Target.EmitAddress(&compiler.code, compiler)
		if compiler.err != nil {
			return
		}
		assignKind := valueKindFromType(targetType)
		if assignKind == KindNone {
			assignKind = KindInt32
		}
		compiler.emitTyped(OpAssign, assignKind)
	default:
		compiler.fail(fmt.Errorf("compile error: unsupported statement type %T", stmt))
	}
}

func (compiler *Compiler) compileExpr(expr ExprNode) {
	compiler.compileExprAs(expr, compiler.exprType(expr))
}

func (node *IdentNode) EmitAddress(code *CodeMemory, compiler *Compiler) {
	if slot, ok := compiler.localSlots[node.Name]; ok {
		code.AppendInstruction(makeInstruction(OpAddrFrame, KindAddress, ModeNone, FlagNone))
		code.AppendInt(slot)
		return
	}
	binding, ok := compiler.symbolBindings[node.Name]
	if ok && binding.Kind == DeclVariable {
		switch binding.Scope {
		case ScopeBSS:
			code.AppendInstruction(makeInstruction(OpAddrBSS, KindAddress, ModeNone, FlagNone))
			code.AppendInt(binding.ByteOffset)
			return
		case ScopeExtern:
			code.AppendInstruction(makeInstruction(OpAddrExtern, KindAddress, ModeNone, FlagNone))
			code.AppendInt(binding.ByteOffset)
			return
		}
	}
	compiler.fail(fmt.Errorf("compile error on line %d: unknown variable %q", node.Line, node.Name))
}

func (compiler *Compiler) emit(op Opcode) {
	compiler.emitInstruction(makeInstruction(op, KindNone, ModeNone, FlagNone))
}

func (compiler *Compiler) emitTyped(op Opcode, kind ValueKind) {
	compiler.emitInstruction(makeInstruction(op, kind, ModeNone, FlagNone))
}

func (compiler *Compiler) emitInstruction(instruction Instruction) {
	compiler.code.AppendInstruction(instruction)
}

func (compiler *Compiler) emitOpWithOperand(op Opcode, operand int) int {
	compiler.emit(op)
	position := len(compiler.code)
	compiler.code.AppendInt(operand)
	return position
}

func (compiler *Compiler) patchOperand(position int, operand int) {
	compiler.code.PatchInt(position, operand)
}

func (compiler *Compiler) fail(err error) {
	if compiler.err == nil {
		compiler.err = err
	}
}
