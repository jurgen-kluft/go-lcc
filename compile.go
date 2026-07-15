package cova

import (
	"fmt"
	"math"
)

type loopLabels struct {
	breakTarget    int
	continueTarget int
}

type controlFrame struct {
	allowsContinue  bool
	breakPatches    []int
	continuePatches []int
	continueTarget  int
}

type callGraphState int

const (
	callGraphUnvisited callGraphState = iota
	callGraphVisiting
	callGraphVisited
)

type compiledFunctionBlock struct {
	binding                 SymbolBinding
	code                    CodeMemory
	callPatches             []CallPatch
	jumpOperandPositions    []int
	usedExternalFunctionIDs []uint32
	callGraphState          callGraphState
}

type functionCompiler struct {
	symbolBindings          map[string]SymbolBinding
	constImage              *[]byte
	stringLiteralOffsets    map[string]uint32
	code                    CodeMemory
	callPatches             []CallPatch
	jumpOperandPositions    []int
	usedExternalFunctionIDs []uint32
	localSlots              map[string]uint32
	localTypes              map[string]*Type
	returnType              *Type
	frameByteSize           uint32
	localSlotCount          uint32
	controlStack            []controlFrame
	localScopeStack         []map[string]struct{}
	err                     error
}

type Compiler struct {
	code                    CodeMemory
	symbolBindings          map[string]SymbolBinding
	externSymbols           []SymbolBinding
	bssSymbols              []SymbolBinding
	dataSymbols             []SymbolBinding
	constSymbols            []SymbolBinding
	functions               []SymbolBinding
	maxLocalSlots           uint32
	maxFrameByteSize        uint32
	bssByteSize             uint32
	dataByteSize            uint32
	entryFunction           uint32
	hasEntryFunction        bool
	nextTempFuncID          uint32
	callPatches             []CallPatch
	usedExternalFunctionIDs []uint32
	constImage              []byte
	dataImage               []byte
	stringLiteralOffsets    map[string]uint32
	err                     error
}

func NewCompiler() *Compiler {
	return &Compiler{
		symbolBindings: make(map[string]SymbolBinding),
		externSymbols:  make([]SymbolBinding, 256),
		bssSymbols:     make([]SymbolBinding, 256),
		dataSymbols:    make([]SymbolBinding, 256),
		constSymbols:   make([]SymbolBinding, 0, 16),
		functions:      make([]SymbolBinding, 256),
		callPatches:    make([]CallPatch, 256),
	}
}

func cloneTypeSlice(types []*Type) []*Type {
	if len(types) == 0 {
		return nil
	}
	return append([]*Type(nil), types...)
}

func cloneUint32Map(values map[string]uint32) map[string]uint32 {
	if len(values) == 0 {
		return nil
	}
	clone := make(map[string]uint32, len(values))
	for name, value := range values {
		clone[name] = value
	}
	return clone
}

func cloneTypeMap(values map[string]*Type) map[string]*Type {
	if len(values) == 0 {
		return nil
	}
	clone := make(map[string]*Type, len(values))
	for name, value := range values {
		clone[name] = value
	}
	return clone
}

func parameterTypes(params []AstParameter) []*Type {
	if len(params) == 0 {
		return nil
	}
	types := make([]*Type, 0, len(params))
	for _, param := range params {
		types = append(types, param.Type)
	}
	return types
}

func (compiler *Compiler) Compile(program *AstProgramNode) (*RelocatableProgram, error) {
	if program == nil {
		return nil, fmt.Errorf("compile error: program is nil")
	}
	if len(program.Functions) == 0 {
		return nil, fmt.Errorf("compile error: no function definitions found")
	}

	compiler.code = make(CodeMemory, 0, 8192)
	compiler.symbolBindings = make(map[string]SymbolBinding, len(program.Decls)+len(program.Functions))
	compiler.externSymbols = compiler.externSymbols[:0]
	compiler.bssSymbols = compiler.bssSymbols[:0]
	compiler.dataSymbols = compiler.dataSymbols[:0]
	compiler.constSymbols = compiler.constSymbols[:0]
	compiler.functions = compiler.functions[:0]
	compiler.maxLocalSlots = 0
	compiler.maxFrameByteSize = 0
	compiler.bssByteSize = 0
	compiler.dataByteSize = 0
	compiler.entryFunction = 0
	compiler.hasEntryFunction = false
	compiler.nextTempFuncID = 0
	compiler.callPatches = compiler.callPatches[:0]
	compiler.usedExternalFunctionIDs = compiler.usedExternalFunctionIDs[:0]
	compiler.constImage = compiler.constImage[:0]
	compiler.dataImage = compiler.dataImage[:0]
	compiler.stringLiteralOffsets = make(map[string]uint32)
	compiler.err = nil

	for _, decl := range program.Decls {
		compiler.registerTopLevelDecl(decl)
		if compiler.err != nil {
			return nil, compiler.err
		}
	}
	for _, decl := range program.Decls {
		if decl == nil || decl.Initializer == nil {
			continue
		}
		binding, ok := compiler.symbolBindings[decl.Name]
		if !ok {
			return nil, fmt.Errorf("compile error on line %d: unknown top-level declaration %q", decl.Line, decl.Name)
		}
		compiler.initializeGlobal(binding, decl.Initializer, decl.Line)
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
	if !compiler.hasEntryFunction {
		return nil, fmt.Errorf("compile error: required entry function %q not found", "script_main")
	}
	blocks := make([]compiledFunctionBlock, 0, len(program.Functions))
	for _, function := range program.Functions {
		block, err := compiler.compileFunction(function)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, block)
	}
	if err := compiler.markReachableFunctions(blocks); err != nil {
		return nil, err
	}
	if err := compiler.assembleFunctionBlocks(blocks); err != nil {
		return nil, err
	}

	programSymbols := NewProgramSymbols()
	programSymbols.ExternSymbols = append(programSymbols.ExternSymbols, compiler.externSymbols...)
	programSymbols.BSSSymbols = append(programSymbols.BSSSymbols, compiler.bssSymbols...)
	programSymbols.DataSymbols = append(programSymbols.DataSymbols, compiler.dataSymbols...)
	programSymbols.ConstSymbols = append(programSymbols.ConstSymbols, compiler.constSymbols...)
	for _, binding := range compiler.externSymbols {
		programSymbols.Symbols[binding.Name] = binding
	}
	for _, binding := range compiler.bssSymbols {
		programSymbols.Symbols[binding.Name] = binding
	}
	for _, binding := range compiler.dataSymbols {
		programSymbols.Symbols[binding.Name] = binding
	}
	for _, binding := range compiler.constSymbols {
		programSymbols.Symbols[binding.Name] = binding
	}

	compiled := &RelocatableProgram{
		Text:                    compiler.code.Clone(),
		ProgramSymbols:          programSymbols,
		Functions:               append([]SymbolBinding(nil), compiler.functions...),
		CallPatches:             append([]CallPatch(nil), compiler.callPatches...),
		UsedExternalFunctionIDs: append([]uint32(nil), compiler.usedExternalFunctionIDs...),
		EntryFunction:           compiler.entryFunction,
		FrameSize:               compiler.maxLocalSlots,
		FrameByteSize:           compiler.maxFrameByteSize,
		ConstByteSize:           lenU32(compiler.constImage),
		ConstData:               append([]byte(nil), compiler.constImage...),
		DataByteSize:            compiler.dataByteSize,
		DataData:                append([]byte(nil), compiler.dataImage...),
		BSSSize:                 lenU32(compiler.bssSymbols),
		BSSByteSize:             compiler.bssByteSize,
	}
	return compiled, nil
}

func (compiler *Compiler) markReachableFunctions(blocks []compiledFunctionBlock) error {
	blocksByID := make(map[uint32]*compiledFunctionBlock, len(blocks))
	for index := range blocks {
		block := &blocks[index]
		blocksByID[block.binding.TempFuncID] = block
	}
	entry, ok := blocksByID[compiler.entryFunction]
	if !ok {
		return fmt.Errorf("compile error: unknown script function id %d", compiler.entryFunction)
	}
	return entry.markReachable(blocksByID)
}

// markReachable follows the direct local calls recorded while compiling this
// block. A block is retained only when this traversal reaches it from the entry.
func (block *compiledFunctionBlock) markReachable(blocksByID map[uint32]*compiledFunctionBlock) error {
	switch block.callGraphState {
	case callGraphVisited:
		return nil
	case callGraphVisiting:
		return fmt.Errorf("compile error: recursive script call cycle detected at function %q", block.binding.Name)
	}
	block.callGraphState = callGraphVisiting
	for _, patch := range block.callPatches {
		callee, ok := blocksByID[patch.TempFuncID]
		if !ok {
			return fmt.Errorf("compile error: unknown script function id %d", patch.TempFuncID)
		}
		if err := callee.markReachable(blocksByID); err != nil {
			return err
		}
	}
	block.callGraphState = callGraphVisited
	return nil
}

func (compiler *Compiler) assembleFunctionBlocks(blocks []compiledFunctionBlock) error {
	finalCode := make(CodeMemory, 0, 8192)
	finalPatches := make([]CallPatch, 0)
	usedExternalIDs := make([]uint32, 0)
	usedExternalSet := make(map[uint32]struct{})
	externalFunctions := make([]SymbolBinding, 0)
	for _, binding := range compiler.functions {
		if binding.Scope == ScopeExtern {
			externalFunctions = append(externalFunctions, binding)
		}
	}
	retainedFunctions := make([]SymbolBinding, 0, len(externalFunctions)+len(blocks))
	retainedFunctions = append(retainedFunctions, externalFunctions...)
	compiler.maxLocalSlots = 0
	compiler.maxFrameByteSize = 0

	for _, block := range blocks {
		if block.callGraphState != callGraphVisited {
			continue
		}
		base := len(finalCode)
		baseU32, ok := imageUint32FromInt(base)
		if !ok {
			return fmt.Errorf("compile error: function %q code address %d exceeds uint32", block.binding.Name, base)
		}
		code := block.code.Clone()
		for _, operandPos := range block.jumpOperandPositions {
			if operandPos < 0 || operandPos+4 > len(code) {
				return fmt.Errorf("compile error: function %q has invalid jump operand position %d", block.binding.Name, operandPos)
			}
			ip := uint32(operandPos)
			target := code.ReadUint32(&ip)
			if uint64(target)+uint64(baseU32) > uint64(^uint32(0)) {
				return fmt.Errorf("compile error: function %q jump target exceeds uint32", block.binding.Name)
			}
			code.PatchUint32(operandPos, target+baseU32)
		}
		for _, patch := range block.callPatches {
			patch.OperandPos += base
			finalPatches = append(finalPatches, patch)
		}
		finalCode = append(finalCode, code...)
		binding := block.binding
		binding.ScriptAddress = baseU32
		compiler.symbolBindings[binding.Name] = binding
		retainedFunctions = append(retainedFunctions, binding)
		if binding.FrameSlotCount > compiler.maxLocalSlots {
			compiler.maxLocalSlots = binding.FrameSlotCount
		}
		if binding.FrameByteSize > compiler.maxFrameByteSize {
			compiler.maxFrameByteSize = binding.FrameByteSize
		}
		for _, tempFuncID := range block.usedExternalFunctionIDs {
			if _, seen := usedExternalSet[tempFuncID]; seen {
				continue
			}
			usedExternalSet[tempFuncID] = struct{}{}
			usedExternalIDs = append(usedExternalIDs, tempFuncID)
		}
	}

	compiler.code = finalCode
	compiler.callPatches = finalPatches
	compiler.usedExternalFunctionIDs = usedExternalIDs
	compiler.functions = retainedFunctions
	return nil
}

func (compiler *Compiler) registerTopLevelDecl(decl *AstTopLevelDeclNode) {
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
		ByteSize:      uint32(decl.Type.Size),
		ByteAlignment: uint32(decl.Type.Alignment()),
		ParamCount:    lenU32(decl.Params),
		ParamTypes:    parameterTypes(decl.Params),
	}

	switch decl.Kind {
	case DeclVariable:
		switch decl.Scope {
		case ScopeExtern:
			indexU32, ok := imageUint32FromInt(decl.Index)
			if !ok {
				compiler.fail(fmt.Errorf("compile error on line %d: extern variable %q index %d exceeds uint32", decl.Line, decl.Name, decl.Index))
				return
			}
			binding.SlotIndex = indexU32
			binding.ByteOffset = indexU32
			compiler.externSymbols = append(compiler.externSymbols, binding)
		case ScopeBSS:
			byteOffsetU32 := alignUpU32(compiler.bssByteSize, binding.ByteAlignment)
			binding.SlotIndex = lenU32(compiler.bssSymbols)
			binding.ByteOffset = byteOffsetU32
			compiler.bssByteSize = byteOffsetU32 + binding.ByteSize
			compiler.bssSymbols = append(compiler.bssSymbols, binding)
		case ScopeData:
			byteOffsetU32 := alignUpU32(compiler.dataByteSize, binding.ByteAlignment)
			binding.SlotIndex = lenU32(compiler.dataSymbols)
			binding.ByteOffset = byteOffsetU32
			compiler.dataByteSize = byteOffsetU32 + binding.ByteSize
			compiler.ensureDataSize(compiler.dataByteSize)
			compiler.dataSymbols = append(compiler.dataSymbols, binding)
		case ScopeConst:
			byteOffsetU32 := alignUpU32(lenU32(compiler.constImage), binding.ByteAlignment)
			binding.SlotIndex = lenU32(compiler.constSymbols)
			binding.ByteOffset = byteOffsetU32
			compiler.ensureConstSize(byteOffsetU32 + binding.ByteSize)
			compiler.constSymbols = append(compiler.constSymbols, binding)
		default:
			compiler.fail(fmt.Errorf("compile error on line %d: variable %q has invalid scope %d", decl.Line, decl.Name, decl.Scope))
			return
		}
	case DeclFunction:
		if decl.Scope != ScopeExtern {
			compiler.fail(fmt.Errorf("compile error on line %d: function contract %q must be host-linked", decl.Line, decl.Name))
			return
		}
		indexU32, ok := imageUint32FromInt(decl.Index)
		if !ok {
			compiler.fail(fmt.Errorf("compile error on line %d: host-linked function %q slot %d exceeds uint32", decl.Line, decl.Name, decl.Index))
			return
		}
		binding.SlotIndex = indexU32
		binding.TempFuncID = compiler.allocateTempFuncID()
		compiler.trackFunctionBinding(binding)
	default:
		compiler.fail(fmt.Errorf("compile error on line %d: unsupported declaration kind %d", decl.Line, decl.Kind))
		return
	}

	compiler.symbolBindings[decl.Name] = binding
}

func (compiler *Compiler) registerScriptFunction(function *AstFunctionNode) {
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
		ByteSize:      uint32(function.ReturnType.Size),
		ByteAlignment: uint32(function.ReturnType.Alignment()),
		ParamCount:    lenU32(function.Params),
		ParamTypes:    parameterTypes(function.Params),
		TempFuncID:    compiler.allocateTempFuncID(),
	}
	compiler.trackFunctionBinding(binding)
	compiler.symbolBindings[function.Name] = binding
	if function.Name == "script_main" {
		compiler.entryFunction = binding.TempFuncID
		compiler.hasEntryFunction = true
	}
}

func (compiler *Compiler) allocateTempFuncID() uint32 {
	tempFuncID := compiler.nextTempFuncID
	compiler.nextTempFuncID++
	return tempFuncID
}

func (compiler *Compiler) trackFunctionBinding(binding SymbolBinding) {
	compiler.functions = append(compiler.functions, binding)
}

func (compiler *Compiler) compileFunction(function *AstFunctionNode) (compiledFunctionBlock, error) {
	if function == nil {
		return compiledFunctionBlock{}, fmt.Errorf("compile error: function is nil")
	}
	binding, ok := compiler.symbolBindings[function.Name]
	if !ok || binding.Kind != DeclFunction {
		return compiledFunctionBlock{}, fmt.Errorf("compile error on line %d: unknown function %q", function.Line, function.Name)
	}
	context := &functionCompiler{
		symbolBindings:       compiler.symbolBindings,
		constImage:           &compiler.constImage,
		stringLiteralOffsets: compiler.stringLiteralOffsets,
		code:                 make(CodeMemory, 0, 256),
		localSlots:           make(map[string]uint32, len(function.Params)),
		localTypes:           make(map[string]*Type, len(function.Params)),
		returnType:           function.ReturnType,
		controlStack:         make([]controlFrame, 0, 8),
		localScopeStack:      make([]map[string]struct{}, 0, 8),
	}
	frameByteSize := uint32(0)
	paramOffsets := make([]uint32, 0, len(function.Params))
	for _, param := range function.Params {
		if _, exists := context.localSlots[param.Name]; exists {
			return compiledFunctionBlock{}, fmt.Errorf("compile error on line %d: duplicate parameter %q", param.Line, param.Name)
		}
		frameByteSize = alignUpU32(frameByteSize, uint32(param.Type.Alignment()))
		context.localSlots[param.Name] = frameByteSize
		context.localTypes[param.Name] = param.Type
		paramOffsets = append(paramOffsets, frameByteSize)
		context.localSlotCount++
		frameByteSize += uint32(param.Type.Size)
	}
	binding.ParamOffsets = paramOffsets
	binding.FrameSlotCount = context.localSlotCount
	binding.FrameByteSize = frameByteSize
	context.frameByteSize = frameByteSize
	context.compileBlock(function.Body)
	if context.err != nil {
		return compiledFunctionBlock{}, context.err
	}
	if len(context.code) == 0 || Opcode(context.code[len(context.code)-1]) != OpRet {
		context.emit(OpRet)
	}
	binding.FrameSlotCount = context.localSlotCount
	binding.FrameByteSize = context.frameByteSize
	return compiledFunctionBlock{
		binding:                 binding,
		code:                    context.code,
		callPatches:             context.callPatches,
		jumpOperandPositions:    context.jumpOperandPositions,
		usedExternalFunctionIDs: context.usedExternalFunctionIDs,
	}, nil
}

func cloneBindingsMap(bindings map[string]SymbolBinding) map[string]SymbolBinding {
	if len(bindings) == 0 {
		return nil
	}
	clone := make(map[string]SymbolBinding, len(bindings))
	for name, binding := range bindings {
		binding.ParamTypes = cloneTypeSlice(binding.ParamTypes)
		if len(binding.ParamOffsets) != 0 {
			binding.ParamOffsets = append([]uint32(nil), binding.ParamOffsets...)
		}
		clone[name] = binding
	}
	return clone
}

func (fc *functionCompiler) exprType(expr AstExprNode) *Type {
	switch node := expr.(type) {
	case *AstNumberLiteral:
		if node.IsFloat {
			if node.FloatType != nil {
				return node.FloatType
			}
			return Float32Type
		}
		return Int32Type
	case *AstStringLiteral:
		return PointerTo(QualifiedType(Uint8Type, true))
	case *AstIdentNode:
		if localType, ok := fc.localTypes[node.Name]; ok {
			return localType
		}
		if binding, ok := fc.symbolBindings[node.Name]; ok {
			return binding.Type
		}
	case *AstBinaryExpr:
		left := fc.exprType(node.Left)
		right := fc.exprType(node.Right)
		switch node.Op {
		case "&&", "||":
			return BoolType
		case "==", "!=", "<", ">", "<=", ">=":
			return Int32Type
		}
		return promoteNumericType(left, right)
	case *AstCallExpr:
		if binding, ok := fc.symbolBindings[node.Callee]; ok {
			return binding.Type
		}
	}
	return nil
}

func (fc *functionCompiler) compileExprAs(expr AstExprNode, expected *Type) {
	if fc.err != nil {
		return
	}
	kind := valueKindFromType(expected)
	if kind == KindNone {
		kind = KindInt32
	}
	switch node := expr.(type) {
	case *AstNumberLiteral:
		fc.emitTyped(OpPush, kind)
		fc.code.AppendImmediate(kind, fc.numberLiteralBits(node, kind))
	case *AstStringLiteral:
		if !fc.canAssignStringLiteral(expected) {
			fc.fail(fmt.Errorf("compile error on line %d: string literal is not assignable to %v", node.Line, expected))
			return
		}
		fc.emitInstruction(makeAddrInstruction(segmentConst))
		fc.code.AppendUint32(fc.internStringLiteral(node.Value))
	case *AstIdentNode:
		actualKind := kind
		if actual := fc.exprType(expr); actual != nil {
			actualKind = valueKindFromType(actual)
		}
		if actualKind == KindNone {
			actualKind = KindInt32
		}
		node.astEmitAddress(fc)
		if fc.err != nil {
			return
		}
		fc.emitTyped(OpDereference, actualKind)
		fc.emitConvertIfNeeded(actualKind, kind)
	case *AstBinaryExpr:
		if node.Op == "&&" || node.Op == "||" {
			fc.compileLogicalExpr(node, kind)
			return
		}
		binaryType := expected
		comparisonOp := isComparisonOperator(node.Op)
		if comparisonOp {
			binaryType = promoteNumericType(fc.exprType(node.Left), fc.exprType(node.Right))
		}
		if binaryType == nil {
			binaryType = fc.exprType(expr)
		}
		binaryKind := valueKindFromType(binaryType)
		if binaryKind == KindNone {
			binaryKind = KindInt32
			binaryType = Int32Type
		}
		fc.compileExprAs(node.Left, binaryType)
		fc.compileExprAs(node.Right, binaryType)
		if fc.err != nil {
			return
		}
		switch node.Op {
		case "+", "-", "*", "/":
			fc.emitArithmetic(node.Op, binaryKind)
		case "==", "!=", "<", ">", "<=", ">=":
			fc.emitComparison(node.Op, binaryKind)
			fc.emitConvertIfNeeded(KindBool, kind)
		default:
			fc.fail(fmt.Errorf("compile error on line %d: unsupported binary operator %q", node.Line, node.Op))
		}
	case *AstCallExpr:
		binding, ok := fc.symbolBindings[node.Callee]
		if !ok || binding.Kind != DeclFunction {
			fc.fail(fmt.Errorf("compile error on line %d: unknown function %q", node.Line, node.Callee))
			return
		}
		if lenU32(node.Args) != binding.ParamCount {
			fc.fail(fmt.Errorf("compile error on line %d: function %q expects %d arguments, got %d", node.Line, node.Callee, binding.ParamCount, len(node.Args)))
			return
		}
		for index, arg := range node.Args {
			var paramType *Type
			if index < len(binding.ParamTypes) {
				paramType = binding.ParamTypes[index]
			}
			fc.compileExprAs(arg, paramType)
			if fc.err != nil {
				return
			}
		}
		if binding.Scope == ScopeExtern {
			fc.emitOpWithOperand(OpCallExtern, binding.SlotIndex)
			fc.usedExternalFunctionIDs = append(fc.usedExternalFunctionIDs, binding.TempFuncID)
		} else {
			operandPos := fc.emitOpWithOperand(OpCall, binding.TempFuncID)
			fc.callPatches = append(fc.callPatches, CallPatch{OperandPos: operandPos, TempFuncID: binding.TempFuncID, Line: node.Line})
		}
		returnKind := valueKindFromType(binding.Type)
		if returnKind != KindNone {
			fc.emitConvertIfNeeded(returnKind, kind)
		}
	default:
		fc.fail(fmt.Errorf("compile error: unsupported expression type %T", expr))
	}
}

func (compiler *Compiler) canAssignStringLiteral(target *Type) bool {
	if target == nil || target.Kind != TypePointer || target.Base == nil {
		return false
	}
	return target.Base.Kind == TypeUint8 && target.Base.IsConst
}

func (compiler *Compiler) internStringLiteral(value string) uint32 {
	if offset, ok := compiler.stringLiteralOffsets[value]; ok {
		return offset
	}
	offset := lenU32(compiler.constImage)
	compiler.constImage = append(compiler.constImage, []byte(value)...)
	compiler.constImage = append(compiler.constImage, 0)
	compiler.stringLiteralOffsets[value] = offset
	return offset
}

func (fc *functionCompiler) canAssignStringLiteral(target *Type) bool {
	return target != nil && target.Kind == TypePointer && target.Base != nil && target.Base.Kind == TypeUint8 && target.Base.IsConst
}

func (fc *functionCompiler) internStringLiteral(value string) uint32 {
	if offset, ok := fc.stringLiteralOffsets[value]; ok {
		return offset
	}
	offset := lenU32(*fc.constImage)
	*fc.constImage = append(*fc.constImage, []byte(value)...)
	*fc.constImage = append(*fc.constImage, 0)
	fc.stringLiteralOffsets[value] = offset
	return offset
}

func (fc *functionCompiler) numberLiteralBits(node *AstNumberLiteral, kind ValueKind) uint64 {
	return numberLiteralBits(node, kind)
}

func (compiler *Compiler) ensureDataSize(size uint32) {
	if size <= lenU32(compiler.dataImage) {
		return
	}
	compiler.dataImage = append(compiler.dataImage, make([]byte, int(size-lenU32(compiler.dataImage)))...)
}

func (compiler *Compiler) ensureConstSize(size uint32) {
	if size <= lenU32(compiler.constImage) {
		return
	}
	compiler.constImage = append(compiler.constImage, make([]byte, int(size-lenU32(compiler.constImage)))...)
}

func (compiler *Compiler) initializeGlobal(binding SymbolBinding, expr AstExprNode, line int) {
	if compiler.err != nil {
		return
	}
	if binding.Scope != ScopeData && binding.Scope != ScopeConst {
		compiler.fail(fmt.Errorf("compile error on line %d: initializer for %q requires static storage", line, binding.Name))
		return
	}
	bindingKind := valueKindFromType(binding.Type)
	if binding.Type != nil && binding.Type.Kind == TypePointer {
		bindingKind = KindAddress
	}
	bits, err := compiler.globalInitializerBits(binding.Type, expr, line)
	if err != nil {
		compiler.fail(err)
		return
	}
	var segment MemorySegment
	if binding.Scope == ScopeConst {
		compiler.ensureConstSize(binding.ByteOffset + binding.ByteSize)
		segment = MemorySegment(compiler.constImage)
	} else {
		segment = MemorySegment(compiler.dataImage)
	}
	if status := writeGlobalInitializer(&segment, binding.ByteOffset, bindingKind, bits); status != VMStatusOK {
		compiler.fail(fmt.Errorf("compile error on line %d: failed to encode initializer for %q: %s", line, binding.Name, status))
		return
	}
	if binding.Scope == ScopeConst {
		compiler.constImage = []byte(segment)
		return
	}
	compiler.dataImage = []byte(segment)
}

func writeGlobalInitializer(segment *MemorySegment, offset uint32, kind ValueKind, bits uint64) VMStatus {
	switch kind {
	case KindBool, KindByte, KindInt8, KindUint8:
		return segment.WriteUint8(offset, uint8(bits))
	case KindInt16, KindUint16:
		return segment.WriteUint16(offset, uint16(bits))
	case KindInt32, KindUint32, KindFloat32, KindAddress:
		return segment.WriteUint32(offset, uint32(bits))
	case KindInt64, KindUint64, KindFloat64:
		return segment.WriteUint64(offset, bits)
	default:
		return VMStatusInvalidValueKind
	}
}

func (compiler *Compiler) globalInitializerBits(target *Type, expr AstExprNode, line int) (uint64, error) {
	if target == nil {
		return 0, fmt.Errorf("compile error on line %d: global initializer target has invalid type", line)
	}
	switch node := expr.(type) {
	case *AstNumberLiteral:
		if target.Kind == TypePointer {
			if node.IsFloat || node.IntValue != 0 {
				return 0, fmt.Errorf("compile error on line %d: pointer global initializer must be a string literal or 0", line)
			}
			return 0, nil
		}
		kind := valueKindFromType(target)
		if kind == KindNone || kind == KindAddress {
			return 0, fmt.Errorf("compile error on line %d: unsupported global initializer type %v", line, target)
		}
		return compiler.numberLiteralBits(node, kind), nil
	case *AstStringLiteral:
		if !compiler.canAssignStringLiteral(target) {
			return 0, fmt.Errorf("compile error on line %d: string literal is not assignable to %v", line, target)
		}
		return uint64(uint32(makeAddress(segmentConst, compiler.internStringLiteral(node.Value)))), nil
	default:
		return 0, fmt.Errorf("compile error on line %d: unsupported global initializer %T", line, expr)
	}
}

func (fc *functionCompiler) compileLogicalExpr(node *AstBinaryExpr, expectedKind ValueKind) {
	fc.compileExprAs(node.Left, BoolType)
	if fc.err != nil {
		return
	}

	switch node.Op {
	case "&&":
		leftFalsePos := fc.emitOpWithOperand(OpJumpIfFalse, 0)
		fc.compileExprAs(node.Right, BoolType)
		if fc.err != nil {
			return
		}
		rightFalsePos := fc.emitOpWithOperand(OpJumpIfFalse, 0)
		fc.emitBooleanConstant(true)
		endPos := fc.emitOpWithOperand(OpJump, 0)
		falsePos := len(fc.code)
		fc.patchOperand(leftFalsePos, falsePos)
		fc.patchOperand(rightFalsePos, falsePos)
		fc.emitBooleanConstant(false)
		fc.patchOperand(endPos, len(fc.code))
	case "||":
		leftFalsePos := fc.emitOpWithOperand(OpJumpIfFalse, 0)
		fc.emitBooleanConstant(true)
		leftEndPos := fc.emitOpWithOperand(OpJump, 0)
		rightStart := len(fc.code)
		fc.patchOperand(leftFalsePos, rightStart)
		fc.compileExprAs(node.Right, BoolType)
		if fc.err != nil {
			return
		}
		rightFalsePos := fc.emitOpWithOperand(OpJumpIfFalse, 0)
		fc.emitBooleanConstant(true)
		rightEndPos := fc.emitOpWithOperand(OpJump, 0)
		falsePos := len(fc.code)
		fc.patchOperand(rightFalsePos, falsePos)
		fc.emitBooleanConstant(false)
		end := len(fc.code)
		fc.patchOperand(leftEndPos, end)
		fc.patchOperand(rightEndPos, end)
	default:
		fc.fail(fmt.Errorf("compile error on line %d: unsupported logical operator %q", node.Line, node.Op))
		return
	}

	fc.emitConvertIfNeeded(KindBool, expectedKind)
}

func (fc *functionCompiler) emitBooleanConstant(value bool) {
	fc.emitTyped(OpPush, KindBool)
	if value {
		fc.code.AppendImmediate(KindBool, 1)
		return
	}
	fc.code.AppendImmediate(KindBool, 0)
}

func (fc *functionCompiler) emitConvertIfNeeded(from ValueKind, to ValueKind) {
	if fc.err != nil || from == to || to == KindNone || from == KindNone {
		return
	}
	if !isNumericKind(from) || !isNumericKind(to) {
		fc.fail(fmt.Errorf("compile error: unsupported conversion from kind %d to kind %d", from, to))
		return
	}
	fc.code.AppendInstruction(makeConvertInstruction(from, to))
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

func (compiler *Compiler) numberLiteralBits(node *AstNumberLiteral, kind ValueKind) uint64 {
	return numberLiteralBits(node, kind)
}

func numberLiteralBits(node *AstNumberLiteral, kind ValueKind) uint64 {
	if node == nil {
		return 0
	}
	if node.IsFloat {
		switch kind {
		case KindBool:
			if node.FloatValue == 0 {
				return 0
			}
			return 1
		case KindFloat32:
			return uint64(math.Float32bits(float32(node.FloatValue)))
		case KindFloat64:
			return math.Float64bits(node.FloatValue)
		default:
			return uint64(int64(node.FloatValue))
		}
	}
	switch kind {
	case KindBool:
		if node.IntValue == 0 {
			return 0
		}
		return 1
	case KindFloat32:
		return uint64(math.Float32bits(float32(node.IntValue)))
	case KindFloat64:
		return math.Float64bits(float64(node.IntValue))
	default:
		return uint64(node.IntValue)
	}
}

func (fc *functionCompiler) compileBlock(block *AstBlockStmt) {
	if block == nil {
		return
	}
	savedSlots := cloneUint32Map(fc.localSlots)
	savedTypes := cloneTypeMap(fc.localTypes)
	fc.localScopeStack = append(fc.localScopeStack, make(map[string]struct{}))
	defer func() {
		fc.localSlots = savedSlots
		fc.localTypes = savedTypes
		fc.localScopeStack = fc.localScopeStack[:len(fc.localScopeStack)-1]
	}()
	for _, stmt := range block.Statements {
		fc.compileStmt(stmt)
		if fc.err != nil {
			return
		}
	}
}

func (fc *functionCompiler) compileStmt(stmt AstStmtNode) {
	if fc.err != nil {
		return
	}

	switch node := stmt.(type) {
	case *AstBlockStmt:
		fc.compileBlock(node)
	case *AstLocalDeclStmt:
		fc.compileLocalDecl(node)
	case *AstIfStmt:
		fc.compileExprAs(node.Condition, BoolType)
		jumpPos := fc.emitOpWithOperand(OpJumpIfFalse, 0)
		fc.compileStmt(node.Then)
		if node.Else == nil {
			fc.patchOperand(jumpPos, len(fc.code))
			break
		}
		skipElsePos := fc.emitOpWithOperand(OpJump, 0)
		fc.patchOperand(jumpPos, len(fc.code))
		fc.compileStmt(node.Else)
		fc.patchOperand(skipElsePos, len(fc.code))
	case *AstWhileStmt:
		loopStart := len(fc.code)
		fc.compileExprAs(node.Condition, BoolType)
		exitPos := fc.emitOpWithOperand(OpJumpIfFalse, 0)
		fc.controlStack = append(fc.controlStack, controlFrame{allowsContinue: true, continueTarget: loopStart})
		fc.compileStmt(node.Body)
		fc.patchCurrentContinues(loopStart)
		fc.emitOpWithOperand(OpJump, uint32(loopStart))
		loopEnd := len(fc.code)
		fc.patchOperand(exitPos, loopEnd)
		fc.patchCurrentBreaks(loopEnd)
		fc.controlStack = fc.controlStack[:len(fc.controlStack)-1]
	case *AstForStmt:
		if node.Init != nil {
			fc.compileStmt(node.Init)
		}
		loopStart := len(fc.code)
		exitPos := -1
		if node.Condition != nil {
			fc.compileExprAs(node.Condition, BoolType)
			exitPos = fc.emitOpWithOperand(OpJumpIfFalse, 0)
		}
		fc.controlStack = append(fc.controlStack, controlFrame{allowsContinue: true, continueTarget: -1})
		fc.compileStmt(node.Body)
		postStart := len(fc.code)
		fc.controlStack[len(fc.controlStack)-1].continueTarget = postStart
		fc.patchCurrentContinues(postStart)
		if node.Post != nil {
			fc.compileStmt(node.Post)
		}
		fc.emitOpWithOperand(OpJump, uint32(loopStart))
		loopEnd := len(fc.code)
		if exitPos >= 0 {
			fc.patchOperand(exitPos, loopEnd)
		}
		fc.patchCurrentBreaks(loopEnd)
		fc.controlStack = fc.controlStack[:len(fc.controlStack)-1]
	case *AstSwitchStmt:
		fc.compileSwitchStmt(node)
	case *AstReturnStmt:
		if node.Value != nil {
			fc.compileExprAs(node.Value, fc.returnType)
		}
		fc.emit(OpRet)
	case *AstExprStmt:
		if _, ok := node.Expr.(*AstCallExpr); !ok {
			fc.fail(fmt.Errorf("compile error on line %d: only function call expressions can be used as standalone statements", node.Line))
			return
		}
		fc.compileExpr(node.Expr)
	case *AstAssignStmt:
		if fc.rejectConstAssignment(node.Target, node.Line) {
			return
		}
		targetType := fc.exprType(node.Target)
		fc.compileExprAs(node.Value, targetType)
		node.Target.astEmitAddress(fc)
		if fc.err != nil {
			return
		}
		assignKind := valueKindFromType(targetType)
		if assignKind == KindNone {
			assignKind = KindInt32
		}
		fc.emitTyped(OpAssign, assignKind)
	case *AstBreakStmt:
		if len(fc.controlStack) == 0 {
			fc.fail(fmt.Errorf("compile error on line %d: break used outside loop or switch", node.Line))
			return
		}
		patchPos := fc.emitOpWithOperand(OpJump, 0)
		frame := &fc.controlStack[len(fc.controlStack)-1]
		frame.breakPatches = append(frame.breakPatches, patchPos)
	case *AstContinueStmt:
		controlIndex := fc.findContinueControlIndex()
		if controlIndex < 0 {
			fc.fail(fmt.Errorf("compile error on line %d: continue used outside loop", node.Line))
			return
		}
		patchPos := fc.emitOpWithOperand(OpJump, 0)
		frame := &fc.controlStack[controlIndex]
		frame.continuePatches = append(frame.continuePatches, patchPos)
	default:
		fc.fail(fmt.Errorf("compile error: unsupported statement type %T", stmt))
	}
}

func (fc *functionCompiler) compileLocalDecl(node *AstLocalDeclStmt) {
	if node == nil || fc.err != nil {
		return
	}
	fc.allocateLocal(node.Name, node.Type, node.Line)
	if fc.err != nil || node.Initializer == nil {
		return
	}
	fc.compileExprAs(node.Initializer, node.Type)
	ident := &AstIdentNode{Name: node.Name, Line: node.Line}
	ident.astEmitAddress(fc)
	if fc.err != nil {
		return
	}
	assignKind := valueKindFromType(node.Type)
	if assignKind == KindNone {
		assignKind = KindInt32
	}
	fc.emitTyped(OpAssign, assignKind)
}

func (fc *functionCompiler) allocateLocal(name string, typ *Type, line int) {
	if typ == nil {
		fc.fail(fmt.Errorf("compile error on line %d: local variable %q has invalid type", line, name))
		return
	}
	if typ.Kind == TypeVoid {
		fc.fail(fmt.Errorf("compile error on line %d: local variable %q cannot have type void", line, name))
		return
	}
	if len(fc.localScopeStack) == 0 {
		fc.localScopeStack = append(fc.localScopeStack, make(map[string]struct{}))
	}
	scope := fc.localScopeStack[len(fc.localScopeStack)-1]
	if _, exists := scope[name]; exists {
		fc.fail(fmt.Errorf("compile error on line %d: duplicate local declaration %q", line, name))
		return
	}
	offset := alignUpU32(fc.frameByteSize, uint32(typ.Alignment()))
	fc.localSlots[name] = offset
	fc.localTypes[name] = typ
	scope[name] = struct{}{}
	fc.frameByteSize = offset + uint32(typ.Size)
	fc.localSlotCount++
}

func (fc *functionCompiler) rejectConstAssignment(target AstLvalueNode, line int) bool {
	ident, ok := target.(*AstIdentNode)
	if !ok {
		return false
	}
	if localType, exists := fc.localTypes[ident.Name]; exists {
		if IsTopLevelConst(localType) {
			fc.fail(fmt.Errorf("compile error on line %d: cannot assign to const variable %q", line, ident.Name))
			return true
		}
		return false
	}
	binding, ok := fc.symbolBindings[ident.Name]
	if ok && binding.Kind == DeclVariable && IsTopLevelConst(binding.Type) {
		fc.fail(fmt.Errorf("compile error on line %d: cannot assign to const variable %q", line, ident.Name))
		return true
	}
	return false
}

func isComparisonOperator(op string) bool {
	switch op {
	case "==", "!=", "<", ">", "<=", ">=":
		return true
	default:
		return false
	}
}

var comparisonOperators = map[string]CompareOp{
	"==": CompareEqual,
	"!=": CompareNotEqual,
	"<":  CompareLess,
	"<=": CompareLessEqual,
	">":  CompareGreater,
	">=": CompareGreaterEqual,
}

var arithmeticOperators = map[string]ArithmeticOp{
	"+": ArithmeticAdd,
	"-": ArithmeticSub,
	"*": ArithmeticMul,
	"/": ArithmeticDiv,
}

func (fc *functionCompiler) emitArithmetic(op string, kind ValueKind) {
	if arithmeticOp, ok := arithmeticOperators[op]; ok {
		fc.emitInstruction(makeArithmeticInstruction(kind, arithmeticOp))
	} else {
		fc.fail(fmt.Errorf("compile error: arithmetic operator %q not yet fully supported", op))
	}
}

func (fc *functionCompiler) emitComparison(op string, kind ValueKind) {
	if compareOp, ok := comparisonOperators[op]; ok {
		fc.emitInstruction(makeCompareInstruction(kind, compareOp))
	} else {
		fc.fail(fmt.Errorf("compile error: comparison operator %q not yet fully supported", op))
	}

}
func (fc *functionCompiler) compileSwitchStmt(node *AstSwitchStmt) {
	if node == nil {
		return
	}
	fc.controlStack = append(fc.controlStack, controlFrame{})
	switchType := fc.exprType(node.Value)
	if switchType == nil {
		switchType = Int32Type
	}
	compareFailPatches := make([]int, 0, len(node.Cases))
	caseEntryPatches := make([]int, 0, len(node.Cases))
	for _, switchCase := range node.Cases {
		compareStart := len(fc.code)
		for _, patchPos := range compareFailPatches {
			fc.patchOperand(patchPos, compareStart)
		}
		compareFailPatches = compareFailPatches[:0]
		caseType := promoteNumericType(switchType, fc.exprType(switchCase.Value))
		if caseType == nil {
			caseType = switchType
		}
		caseKind := valueKindFromType(caseType)
		if caseKind == KindNone {
			caseType = Int32Type
			caseKind = KindInt32
		}
		fc.compileExprAs(node.Value, caseType)
		fc.compileExprAs(switchCase.Value, caseType)
		fc.emitComparison("==", caseKind)
		compareFailPatches = append(compareFailPatches, fc.emitOpWithOperand(OpJumpIfFalse, 0))
		caseEntryPatches = append(caseEntryPatches, fc.emitOpWithOperand(OpJump, 0))
	}
	defaultJumpPos := fc.emitOpWithOperand(OpJump, 0)
	for index, switchCase := range node.Cases {
		bodyStart := len(fc.code)
		fc.patchOperand(caseEntryPatches[index], bodyStart)
		for _, stmt := range switchCase.Body {
			fc.compileStmt(stmt)
		}
	}
	defaultStart := len(fc.code)
	fc.patchOperand(defaultJumpPos, defaultStart)
	for _, patchPos := range compareFailPatches {
		fc.patchOperand(patchPos, defaultStart)
	}
	for _, stmt := range node.Default {
		fc.compileStmt(stmt)
	}
	endPos := len(fc.code)
	fc.patchCurrentBreaks(endPos)
	fc.controlStack = fc.controlStack[:len(fc.controlStack)-1]
}

func (fc *functionCompiler) findContinueControlIndex() int {
	for index := len(fc.controlStack) - 1; index >= 0; index-- {
		if fc.controlStack[index].allowsContinue {
			return index
		}
	}
	return -1
}

func (fc *functionCompiler) patchCurrentBreaks(target int) {
	if len(fc.controlStack) == 0 {
		return
	}
	for _, patchPos := range fc.controlStack[len(fc.controlStack)-1].breakPatches {
		fc.patchOperand(patchPos, target)
	}
}

func (fc *functionCompiler) patchCurrentContinues(target int) {
	if len(fc.controlStack) == 0 {
		return
	}
	for _, patchPos := range fc.controlStack[len(fc.controlStack)-1].continuePatches {
		fc.patchOperand(patchPos, target)
	}
}

func (fc *functionCompiler) compileExpr(expr AstExprNode) {
	fc.compileExprAs(expr, fc.exprType(expr))
}

func (node *AstIdentNode) astEmitAddress(fc *functionCompiler) {
	if slot, ok := fc.localSlots[node.Name]; ok {
		fc.code.AppendInstruction(makeAddrInstruction(segmentFrame))
		fc.code.AppendUint32(slot)
		return
	}
	binding, ok := fc.symbolBindings[node.Name]
	if ok && binding.Kind == DeclVariable {
		switch binding.Scope {
		case ScopeBSS:
			fc.code.AppendInstruction(makeAddrInstruction(segmentBSS))
			fc.code.AppendUint32(binding.ByteOffset)
			return
		case ScopeData:
			fc.code.AppendInstruction(makeAddrInstruction(segmentData))
			fc.code.AppendUint32(binding.ByteOffset)
			return
		case ScopeConst:
			fc.code.AppendInstruction(makeAddrInstruction(segmentConst))
			fc.code.AppendUint32(binding.ByteOffset)
			return
		case ScopeExtern:
			fc.code.AppendInstruction(makeAddrInstruction(segmentExtern))
			fc.code.AppendUint32(binding.ByteOffset)
			return
		}
	}
	fc.fail(fmt.Errorf("compile error on line %d: unknown variable %q", node.Line, node.Name))
}

func (fc *functionCompiler) emit(op Opcode) {
	fc.emitInstruction(makeInstruction(op, KindNone, ModeNone, FlagNone))
}

func (fc *functionCompiler) emitTyped(op Opcode, kind ValueKind) {
	fc.emitInstruction(makeInstruction(op, kind, ModeNone, FlagNone))
}

func (fc *functionCompiler) emitInstruction(instruction Instruction) {
	fc.code.AppendInstruction(instruction)
}

func (fc *functionCompiler) emitOpWithOperand(op Opcode, operand uint32) int {
	fc.emit(op)
	position := len(fc.code)
	fc.code.AppendUint32(operand)
	if op == OpJump || op == OpJumpIfFalse {
		fc.jumpOperandPositions = append(fc.jumpOperandPositions, position)
	}
	return position
}

func (fc *functionCompiler) patchOperand(position int, operand int) {
	fc.code.PatchUint32(position, uint32(operand))
}

func (compiler *Compiler) fail(err error) {
	if compiler.err == nil {
		compiler.err = err
	}
}

func (fc *functionCompiler) fail(err error) {
	if fc.err == nil {
		fc.err = err
	}
}
