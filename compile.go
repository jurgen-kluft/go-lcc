package cova

import (
	"fmt"
	"math"
	"sort"
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

type Compiler struct {
	program               *ProgramNode
	code                  CodeMemory
	symbolBindings        map[string]SymbolBinding
	externSymbols         []SymbolBinding
	bssSymbols            []SymbolBinding
	dataSymbols           []SymbolBinding
	constSymbols          []SymbolBinding
	functions             []SymbolBinding
	functionBindingByTemp map[uint32]int
	localSlots            map[string]uint32
	localTypes            map[string]*Type
	currentReturnType     *Type
	currentFrameByteSize  uint32
	localSlotCount        uint32
	maxLocalSlots         uint32
	maxFrameByteSize      uint32
	bssByteSize           uint32
	dataByteSize          uint32
	entryFunction         uint32
	hasEntryFunction      bool
	nextTempFuncID        uint32
	callPatches           []CallPatch
	controlStack          []controlFrame
	localScopeStack       []map[string]struct{}
	constImage            []byte
	dataImage             []byte
	stringLiteralOffsets  map[string]uint32
	err                   error
}

func NewCompiler() *Compiler {
	return &Compiler{
		symbolBindings:        make(map[string]SymbolBinding),
		functionBindingByTemp: make(map[uint32]int),
		externSymbols:         make([]SymbolBinding, 256),
		bssSymbols:            make([]SymbolBinding, 256),
		dataSymbols:           make([]SymbolBinding, 256),
		constSymbols:          make([]SymbolBinding, 0, 16),
		functions:             make([]SymbolBinding, 256),
		callPatches:           make([]CallPatch, 256),
		controlStack:          make([]controlFrame, 32),
		localScopeStack:       make([]map[string]struct{}, 0, 16),
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

	// TODO is it possible to estimate how large the code memory will become?

	compiler.program = program
	compiler.code = make(CodeMemory, 0, 8192)
	compiler.symbolBindings = make(map[string]SymbolBinding, len(program.Decls)+len(program.Functions))
	compiler.externSymbols = compiler.externSymbols[:0]
	compiler.bssSymbols = compiler.bssSymbols[:0]
	compiler.dataSymbols = compiler.dataSymbols[:0]
	compiler.constSymbols = compiler.constSymbols[:0]
	compiler.functions = compiler.functions[:0]
	compiler.functionBindingByTemp = make(map[uint32]int, len(program.Functions))
	compiler.localSlots = nil
	compiler.localTypes = nil
	compiler.currentReturnType = nil
	compiler.currentFrameByteSize = 0
	compiler.localSlotCount = 0
	compiler.maxLocalSlots = 0
	compiler.maxFrameByteSize = 0
	compiler.bssByteSize = 0
	compiler.dataByteSize = 0
	compiler.entryFunction = 0
	compiler.hasEntryFunction = false
	compiler.nextTempFuncID = 0
	compiler.callPatches = compiler.callPatches[:0]
	compiler.controlStack = compiler.controlStack[:0]
	compiler.localScopeStack = compiler.localScopeStack[:0]
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
	for _, function := range program.Functions {
		compiler.compileFunction(function)
		if compiler.err != nil {
			return nil, compiler.err
		}
	}
	if err := compiler.validateScriptCallGraph(); err != nil {
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
		Text:           compiler.code.Clone(),
		ProgramSymbols: programSymbols,
		Functions:      append([]SymbolBinding(nil), compiler.functions...),
		CallPatches:    append([]CallPatch(nil), compiler.callPatches...),
		EntryFunction:  compiler.entryFunction,
		FrameSize:      compiler.maxLocalSlots,
		FrameByteSize:  compiler.maxFrameByteSize,
		ConstByteSize:  lenu32(compiler.constImage),
		ConstData:      append([]byte(nil), compiler.constImage...),
		DataByteSize:   compiler.dataByteSize,
		DataData:       append([]byte(nil), compiler.dataImage...),
		BSSSize:        lenu32(compiler.bssSymbols),
		BSSByteSize:    compiler.bssByteSize,
	}
	return compiled, nil
}

func (compiler *Compiler) validateScriptCallGraph() error {
	if !compiler.hasEntryFunction {
		return fmt.Errorf("compile error: entry function not set")
	}
	functionByTemp := make(map[uint32]SymbolBinding, len(compiler.functions))
	for _, binding := range compiler.functions {
		functionByTemp[binding.TempFuncID] = binding
	}
	callGraph, err := compiler.buildScriptCallGraph(functionByTemp)
	if err != nil {
		return err
	}
	states := make(map[uint32]callGraphState, len(callGraph))
	var visit func(tempFuncID uint32) error
	visit = func(tempFuncID uint32) error {
		switch states[tempFuncID] {
		case callGraphVisited:
			return nil
		case callGraphVisiting:
			binding := functionByTemp[tempFuncID]
			return fmt.Errorf("compile error: recursive script call cycle detected at function %q", binding.Name)
		}
		if _, ok := functionByTemp[tempFuncID]; !ok {
			return fmt.Errorf("compile error: unknown script function id %d", tempFuncID)
		}
		states[tempFuncID] = callGraphVisiting
		for _, calleeID := range callGraph[tempFuncID] {
			if err := visit(calleeID); err != nil {
				return err
			}
		}
		states[tempFuncID] = callGraphVisited
		return nil
	}
	return visit(compiler.entryFunction)
}

func (compiler *Compiler) buildScriptCallGraph(functionByTemp map[uint32]SymbolBinding) (map[uint32][]uint32, error) {
	ordered := make([]SymbolBinding, 0, len(compiler.functions))
	for _, binding := range compiler.functions {
		if binding.Scope == ScopeBSS {
			ordered = append(ordered, binding)
		}
	}
	sort.Slice(ordered, func(index int, other int) bool {
		return ordered[index].ScriptAddress < ordered[other].ScriptAddress
	})
	callGraph := make(map[uint32][]uint32, len(ordered))
	for _, binding := range ordered {
		callGraph[binding.TempFuncID] = nil
	}
	for _, patch := range compiler.callPatches {
		var callerID uint32
		callerFound := false
		for index, binding := range ordered {
			start := int(binding.ScriptAddress)
			end := len(compiler.code)
			if index+1 < len(ordered) {
				end = int(ordered[index+1].ScriptAddress)
			}
			if patch.OperandPos >= start && patch.OperandPos < end {
				callerID = binding.TempFuncID
				callerFound = true
				break
			}
		}
		if !callerFound {
			return nil, fmt.Errorf("compile error on line %d: unable to resolve caller for script call patch", patch.Line)
		}
		if _, ok := functionByTemp[patch.TempFuncID]; !ok {
			return nil, fmt.Errorf("compile error on line %d: unknown callee id %d", patch.Line, patch.TempFuncID)
		}
		callGraph[callerID] = append(callGraph[callerID], patch.TempFuncID)
	}
	return callGraph, nil
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
		ByteSize:      uint32(decl.Type.Size),
		ByteAlignment: uint32(decl.Type.Alignment()),
		ParamCount:    lenu32(decl.Params),
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
			binding.SlotIndex = lenu32(compiler.bssSymbols)
			binding.ByteOffset = byteOffsetU32
			compiler.bssByteSize = byteOffsetU32 + binding.ByteSize
			compiler.bssSymbols = append(compiler.bssSymbols, binding)
		case ScopeData:
			byteOffsetU32 := alignUpU32(compiler.dataByteSize, binding.ByteAlignment)
			binding.SlotIndex = lenu32(compiler.dataSymbols)
			binding.ByteOffset = byteOffsetU32
			compiler.dataByteSize = byteOffsetU32 + binding.ByteSize
			compiler.ensureDataSize(compiler.dataByteSize)
			compiler.dataSymbols = append(compiler.dataSymbols, binding)
		case ScopeConst:
			byteOffsetU32 := alignUpU32(lenu32(compiler.constImage), binding.ByteAlignment)
			binding.SlotIndex = lenu32(compiler.constSymbols)
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
		ByteSize:      uint32(function.ReturnType.Size),
		ByteAlignment: uint32(function.ReturnType.Alignment()),
		ParamCount:    lenu32(function.Params),
		ParamTypes:    parameterTypes(function.Params),
		TempFuncID:    compiler.allocateTempFuncID(),
	}
	compiler.trackFunctionBinding(binding)
	compiler.symbolBindings[function.Name] = binding
	if function.Name == "script_main" {
		compiler.entryFunction = binding.TempFuncID
		compiler.hasEntryFunction = true
	} else if !compiler.hasEntryFunction {
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
	scriptAddress, ok := imageUint32FromInt(len(compiler.code))
	if !ok {
		compiler.fail(fmt.Errorf("compile error on line %d: function %q code address %d exceeds uint32", function.Line, function.Name, len(compiler.code)))
		return
	}
	binding.ScriptAddress = scriptAddress
	compiler.localSlots = make(map[string]uint32, len(function.Params))
	compiler.localTypes = make(map[string]*Type, len(function.Params))
	compiler.currentReturnType = function.ReturnType
	compiler.localScopeStack = compiler.localScopeStack[:0]
	compiler.localSlotCount = 0
	frameByteSize := uint32(0)
	paramOffsets := make([]uint32, 0, len(function.Params))
	for _, param := range function.Params {
		if _, exists := compiler.localSlots[param.Name]; exists {
			compiler.fail(fmt.Errorf("compile error on line %d: duplicate parameter %q", param.Line, param.Name))
			return
		}
		frameByteSize = alignUpU32(frameByteSize, uint32(param.Type.Alignment()))
		compiler.localSlots[param.Name] = frameByteSize
		compiler.localTypes[param.Name] = param.Type
		paramOffsets = append(paramOffsets, frameByteSize)
		compiler.localSlotCount++
		frameByteSize += uint32(param.Type.Size)
	}
	binding.ParamOffsets = paramOffsets
	binding.FrameSlotCount = compiler.localSlotCount
	binding.FrameByteSize = frameByteSize
	compiler.currentFrameByteSize = frameByteSize
	binding.ScriptAddress = scriptAddress
	compiler.storeFunctionBinding(binding)
	compiler.compileBlock(function.Body)
	if compiler.err != nil {
		return
	}
	if len(compiler.code) == 0 || Opcode(compiler.code[len(compiler.code)-1]) != OpRet {
		compiler.emit(OpRet)
	}
	binding.FrameSlotCount = compiler.localSlotCount
	binding.FrameByteSize = compiler.currentFrameByteSize
	compiler.storeFunctionBinding(binding)
	if compiler.localSlotCount > compiler.maxLocalSlots {
		compiler.maxLocalSlots = compiler.localSlotCount
	}
	if compiler.currentFrameByteSize > compiler.maxFrameByteSize {
		compiler.maxFrameByteSize = compiler.currentFrameByteSize
	}
	compiler.localSlots = nil
	compiler.localTypes = nil
	compiler.currentReturnType = nil
	compiler.currentFrameByteSize = 0
	compiler.localScopeStack = compiler.localScopeStack[:0]
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
			binding.ParamOffsets = append([]uint32(nil), binding.ParamOffsets...)
		}
		clone[name] = binding
	}
	return clone
}

func (compiler *Compiler) exprType(expr ExprNode) *Type {
	switch node := expr.(type) {
	case *NumberLiteral:
		if node.IsFloat {
			if node.FloatType != nil {
				return node.FloatType
			}
			return Float32Type
		}
		return Int32Type
	case *StringLiteral:
		return PointerTo(QualifiedType(Uint8Type, true))
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
		switch node.Op {
		case "&&", "||":
			return BoolType
		case "==", "!=", "<", ">", "<=", ">=":
			return Int32Type
		}
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
	case *StringLiteral:
		if !compiler.canAssignStringLiteral(expected) {
			compiler.fail(fmt.Errorf("compile error on line %d: string literal is not assignable to %v", node.Line, expected))
			return
		}
		compiler.emitInstruction(makeAddrInstruction(segmentConst))
		compiler.code.AppendUint32(compiler.internStringLiteral(node.Value))
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
		if node.Op == "&&" || node.Op == "||" {
			compiler.compileLogicalExpr(node, kind)
			return
		}
		binaryType := expected
		comparisonOp := isComparisonOperator(node.Op)
		if comparisonOp {
			binaryType = promoteNumericType(compiler.exprType(node.Left), compiler.exprType(node.Right))
		}
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
		case "+", "-", "*", "/":
			compiler.emitArithmetic(node.Op, binaryKind)
		case "==", "!=", "<", ">", "<=", ">=":
			compiler.emitComparison(node.Op, binaryKind)
			compiler.emitConvertIfNeeded(KindBool, kind)
		default:
			compiler.fail(fmt.Errorf("compile error on line %d: unsupported binary operator %q", node.Line, node.Op))
		}
	case *CallExpr:
		binding, ok := compiler.symbolBindings[node.Callee]
		if !ok || binding.Kind != DeclFunction {
			compiler.fail(fmt.Errorf("compile error on line %d: unknown function %q", node.Line, node.Callee))
			return
		}
		if lenu32(node.Args) != binding.ParamCount {
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
	offset := lenu32(compiler.constImage)
	compiler.constImage = append(compiler.constImage, []byte(value)...)
	compiler.constImage = append(compiler.constImage, 0)
	compiler.stringLiteralOffsets[value] = offset
	return offset
}

func (compiler *Compiler) ensureDataSize(size uint32) {
	if size <= lenu32(compiler.dataImage) {
		return
	}
	compiler.dataImage = append(compiler.dataImage, make([]byte, int(size-lenu32(compiler.dataImage)))...)
}

func (compiler *Compiler) ensureConstSize(size uint32) {
	if size <= lenu32(compiler.constImage) {
		return
	}
	compiler.constImage = append(compiler.constImage, make([]byte, int(size-lenu32(compiler.constImage)))...)
}

func (compiler *Compiler) initializeGlobal(binding SymbolBinding, expr ExprNode, line int) {
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

func (compiler *Compiler) globalInitializerBits(target *Type, expr ExprNode, line int) (uint64, error) {
	if target == nil {
		return 0, fmt.Errorf("compile error on line %d: global initializer target has invalid type", line)
	}
	switch node := expr.(type) {
	case *NumberLiteral:
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
	case *StringLiteral:
		if !compiler.canAssignStringLiteral(target) {
			return 0, fmt.Errorf("compile error on line %d: string literal is not assignable to %v", line, target)
		}
		return uint64(uint32(makeAddress(segmentConst, compiler.internStringLiteral(node.Value)))), nil
	default:
		return 0, fmt.Errorf("compile error on line %d: unsupported global initializer %T", line, expr)
	}
}

func (compiler *Compiler) compileLogicalExpr(node *BinaryExpr, expectedKind ValueKind) {
	compiler.compileExprAs(node.Left, BoolType)
	if compiler.err != nil {
		return
	}

	switch node.Op {
	case "&&":
		leftFalsePos := compiler.emitOpWithOperand(OpJumpIfFalse, 0)
		compiler.compileExprAs(node.Right, BoolType)
		if compiler.err != nil {
			return
		}
		rightFalsePos := compiler.emitOpWithOperand(OpJumpIfFalse, 0)
		compiler.emitBooleanConstant(true)
		endPos := compiler.emitOpWithOperand(OpJump, 0)
		falsePos := len(compiler.code)
		compiler.patchOperand(leftFalsePos, falsePos)
		compiler.patchOperand(rightFalsePos, falsePos)
		compiler.emitBooleanConstant(false)
		compiler.patchOperand(endPos, len(compiler.code))
	case "||":
		leftFalsePos := compiler.emitOpWithOperand(OpJumpIfFalse, 0)
		compiler.emitBooleanConstant(true)
		leftEndPos := compiler.emitOpWithOperand(OpJump, 0)
		rightStart := len(compiler.code)
		compiler.patchOperand(leftFalsePos, rightStart)
		compiler.compileExprAs(node.Right, BoolType)
		if compiler.err != nil {
			return
		}
		rightFalsePos := compiler.emitOpWithOperand(OpJumpIfFalse, 0)
		compiler.emitBooleanConstant(true)
		rightEndPos := compiler.emitOpWithOperand(OpJump, 0)
		falsePos := len(compiler.code)
		compiler.patchOperand(rightFalsePos, falsePos)
		compiler.emitBooleanConstant(false)
		end := len(compiler.code)
		compiler.patchOperand(leftEndPos, end)
		compiler.patchOperand(rightEndPos, end)
	default:
		compiler.fail(fmt.Errorf("compile error on line %d: unsupported logical operator %q", node.Line, node.Op))
		return
	}

	compiler.emitConvertIfNeeded(KindBool, expectedKind)
}

func (compiler *Compiler) emitBooleanConstant(value bool) {
	compiler.emitTyped(OpPush, KindBool)
	if value {
		compiler.code.AppendImmediate(KindBool, 1)
		return
	}
	compiler.code.AppendImmediate(KindBool, 0)
}

func (compiler *Compiler) emitConvertIfNeeded(from ValueKind, to ValueKind) {
	if compiler.err != nil || from == to || to == KindNone || from == KindNone {
		return
	}
	if !isNumericKind(from) || !isNumericKind(to) {
		compiler.fail(fmt.Errorf("compile error: unsupported conversion from kind %d to kind %d", from, to))
		return
	}
	compiler.code.AppendInstruction(makeConvertInstruction(from, to))
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

func (compiler *Compiler) compileBlock(block *BlockStmt) {
	if block == nil {
		return
	}
	savedSlots := cloneUint32Map(compiler.localSlots)
	savedTypes := cloneTypeMap(compiler.localTypes)
	compiler.localScopeStack = append(compiler.localScopeStack, make(map[string]struct{}))
	defer func() {
		compiler.localSlots = savedSlots
		compiler.localTypes = savedTypes
		compiler.localScopeStack = compiler.localScopeStack[:len(compiler.localScopeStack)-1]
	}()
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
	case *LocalDeclStmt:
		compiler.compileLocalDecl(node)
	case *IfStmt:
		compiler.compileExprAs(node.Condition, BoolType)
		jumpPos := compiler.emitOpWithOperand(OpJumpIfFalse, 0)
		compiler.compileStmt(node.Then)
		if node.Else == nil {
			compiler.patchOperand(jumpPos, len(compiler.code))
			break
		}
		skipElsePos := compiler.emitOpWithOperand(OpJump, 0)
		compiler.patchOperand(jumpPos, len(compiler.code))
		compiler.compileStmt(node.Else)
		compiler.patchOperand(skipElsePos, len(compiler.code))
	case *WhileStmt:
		loopStart := len(compiler.code)
		compiler.compileExprAs(node.Condition, BoolType)
		exitPos := compiler.emitOpWithOperand(OpJumpIfFalse, 0)
		compiler.controlStack = append(compiler.controlStack, controlFrame{allowsContinue: true, continueTarget: loopStart})
		compiler.compileStmt(node.Body)
		compiler.patchCurrentContinues(loopStart)
		compiler.emitOpWithOperand(OpJump, uint32(loopStart))
		loopEnd := len(compiler.code)
		compiler.patchOperand(exitPos, loopEnd)
		compiler.patchCurrentBreaks(loopEnd)
		compiler.controlStack = compiler.controlStack[:len(compiler.controlStack)-1]
	case *ForStmt:
		if node.Init != nil {
			compiler.compileStmt(node.Init)
		}
		loopStart := len(compiler.code)
		exitPos := -1
		if node.Condition != nil {
			compiler.compileExprAs(node.Condition, BoolType)
			exitPos = compiler.emitOpWithOperand(OpJumpIfFalse, 0)
		}
		compiler.controlStack = append(compiler.controlStack, controlFrame{allowsContinue: true, continueTarget: -1})
		compiler.compileStmt(node.Body)
		postStart := len(compiler.code)
		compiler.controlStack[len(compiler.controlStack)-1].continueTarget = postStart
		compiler.patchCurrentContinues(postStart)
		if node.Post != nil {
			compiler.compileStmt(node.Post)
		}
		compiler.emitOpWithOperand(OpJump, uint32(loopStart))
		loopEnd := len(compiler.code)
		if exitPos >= 0 {
			compiler.patchOperand(exitPos, loopEnd)
		}
		compiler.patchCurrentBreaks(loopEnd)
		compiler.controlStack = compiler.controlStack[:len(compiler.controlStack)-1]
	case *SwitchStmt:
		compiler.compileSwitchStmt(node)
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
		if compiler.rejectConstAssignment(node.Target, node.Line) {
			return
		}
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
	case *BreakStmt:
		if len(compiler.controlStack) == 0 {
			compiler.fail(fmt.Errorf("compile error on line %d: break used outside loop or switch", node.Line))
			return
		}
		patchPos := compiler.emitOpWithOperand(OpJump, 0)
		frame := &compiler.controlStack[len(compiler.controlStack)-1]
		frame.breakPatches = append(frame.breakPatches, patchPos)
	case *ContinueStmt:
		controlIndex := compiler.findContinueControlIndex()
		if controlIndex < 0 {
			compiler.fail(fmt.Errorf("compile error on line %d: continue used outside loop", node.Line))
			return
		}
		patchPos := compiler.emitOpWithOperand(OpJump, 0)
		frame := &compiler.controlStack[controlIndex]
		frame.continuePatches = append(frame.continuePatches, patchPos)
	default:
		compiler.fail(fmt.Errorf("compile error: unsupported statement type %T", stmt))
	}
}

func (compiler *Compiler) compileLocalDecl(node *LocalDeclStmt) {
	if node == nil || compiler.err != nil {
		return
	}
	compiler.allocateLocal(node.Name, node.Type, node.Line)
	if compiler.err != nil || node.Initializer == nil {
		return
	}
	compiler.compileExprAs(node.Initializer, node.Type)
	ident := &IdentNode{Name: node.Name, Line: node.Line}
	ident.EmitAddress(&compiler.code, compiler)
	if compiler.err != nil {
		return
	}
	assignKind := valueKindFromType(node.Type)
	if assignKind == KindNone {
		assignKind = KindInt32
	}
	compiler.emitTyped(OpAssign, assignKind)
}

func (compiler *Compiler) allocateLocal(name string, typ *Type, line int) {
	if typ == nil {
		compiler.fail(fmt.Errorf("compile error on line %d: local variable %q has invalid type", line, name))
		return
	}
	if typ.Kind == TypeVoid {
		compiler.fail(fmt.Errorf("compile error on line %d: local variable %q cannot have type void", line, name))
		return
	}
	if len(compiler.localScopeStack) == 0 {
		compiler.localScopeStack = append(compiler.localScopeStack, make(map[string]struct{}))
	}
	scope := compiler.localScopeStack[len(compiler.localScopeStack)-1]
	if _, exists := scope[name]; exists {
		compiler.fail(fmt.Errorf("compile error on line %d: duplicate local declaration %q", line, name))
		return
	}
	offset := alignUpU32(compiler.currentFrameByteSize, uint32(typ.Alignment()))
	compiler.localSlots[name] = offset
	compiler.localTypes[name] = typ
	scope[name] = struct{}{}
	compiler.currentFrameByteSize = offset + uint32(typ.Size)
	compiler.localSlotCount++
}

func (compiler *Compiler) rejectConstAssignment(target LvalueNode, line int) bool {
	ident, ok := target.(*IdentNode)
	if !ok {
		return false
	}
	if localType, exists := compiler.localTypes[ident.Name]; exists {
		if IsTopLevelConst(localType) {
			compiler.fail(fmt.Errorf("compile error on line %d: cannot assign to const variable %q", line, ident.Name))
			return true
		}
		return false
	}
	binding, ok := compiler.symbolBindings[ident.Name]
	if ok && binding.Kind == DeclVariable && IsTopLevelConst(binding.Type) {
		compiler.fail(fmt.Errorf("compile error on line %d: cannot assign to const variable %q", line, ident.Name))
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

func (compiler *Compiler) emitArithmetic(op string, kind ValueKind) {
	if arithmeticOp, ok := arithmeticOperators[op]; ok {
		compiler.emitInstruction(makeArithmeticInstruction(kind, arithmeticOp))
	} else {
		compiler.fail(fmt.Errorf("compile error: arithmetic operator %q not yet fully supported", op))
	}
}

func (compiler *Compiler) emitComparison(op string, kind ValueKind) {
	if compareOp, ok := comparisonOperators[op]; ok {
		compiler.emitInstruction(makeCompareInstruction(kind, compareOp))
	} else {
		compiler.fail(fmt.Errorf("compile error: comparison operator %q not yet fully supported", op))
	}

}
func (compiler *Compiler) compileSwitchStmt(node *SwitchStmt) {
	if node == nil {
		return
	}
	compiler.controlStack = append(compiler.controlStack, controlFrame{})
	switchType := compiler.exprType(node.Value)
	if switchType == nil {
		switchType = Int32Type
	}
	compareFailPatches := make([]int, 0, len(node.Cases))
	caseEntryPatches := make([]int, 0, len(node.Cases))
	for _, switchCase := range node.Cases {
		compareStart := len(compiler.code)
		for _, patchPos := range compareFailPatches {
			compiler.patchOperand(patchPos, compareStart)
		}
		compareFailPatches = compareFailPatches[:0]
		caseType := promoteNumericType(switchType, compiler.exprType(switchCase.Value))
		if caseType == nil {
			caseType = switchType
		}
		caseKind := valueKindFromType(caseType)
		if caseKind == KindNone {
			caseType = Int32Type
			caseKind = KindInt32
		}
		compiler.compileExprAs(node.Value, caseType)
		compiler.compileExprAs(switchCase.Value, caseType)
		compiler.emitComparison("==", caseKind)
		compareFailPatches = append(compareFailPatches, compiler.emitOpWithOperand(OpJumpIfFalse, 0))
		caseEntryPatches = append(caseEntryPatches, compiler.emitOpWithOperand(OpJump, 0))
	}
	defaultJumpPos := compiler.emitOpWithOperand(OpJump, 0)
	for index, switchCase := range node.Cases {
		bodyStart := len(compiler.code)
		compiler.patchOperand(caseEntryPatches[index], bodyStart)
		for _, stmt := range switchCase.Body {
			compiler.compileStmt(stmt)
		}
	}
	defaultStart := len(compiler.code)
	compiler.patchOperand(defaultJumpPos, defaultStart)
	for _, patchPos := range compareFailPatches {
		compiler.patchOperand(patchPos, defaultStart)
	}
	for _, stmt := range node.Default {
		compiler.compileStmt(stmt)
	}
	endPos := len(compiler.code)
	compiler.patchCurrentBreaks(endPos)
	compiler.controlStack = compiler.controlStack[:len(compiler.controlStack)-1]
}

func (compiler *Compiler) findContinueControlIndex() int {
	for index := len(compiler.controlStack) - 1; index >= 0; index-- {
		if compiler.controlStack[index].allowsContinue {
			return index
		}
	}
	return -1
}

func (compiler *Compiler) patchCurrentBreaks(target int) {
	if len(compiler.controlStack) == 0 {
		return
	}
	for _, patchPos := range compiler.controlStack[len(compiler.controlStack)-1].breakPatches {
		compiler.patchOperand(patchPos, target)
	}
}

func (compiler *Compiler) patchCurrentContinues(target int) {
	if len(compiler.controlStack) == 0 {
		return
	}
	for _, patchPos := range compiler.controlStack[len(compiler.controlStack)-1].continuePatches {
		compiler.patchOperand(patchPos, target)
	}
}

func (compiler *Compiler) compileExpr(expr ExprNode) {
	compiler.compileExprAs(expr, compiler.exprType(expr))
}

func (node *IdentNode) EmitAddress(code *CodeMemory, compiler *Compiler) {
	if slot, ok := compiler.localSlots[node.Name]; ok {
		code.AppendInstruction(makeAddrInstruction(segmentFrame))
		code.AppendUint32(slot)
		return
	}
	binding, ok := compiler.symbolBindings[node.Name]
	if ok && binding.Kind == DeclVariable {
		switch binding.Scope {
		case ScopeBSS:
			code.AppendInstruction(makeAddrInstruction(segmentBSS))
			code.AppendUint32(binding.ByteOffset)
			return
		case ScopeData:
			code.AppendInstruction(makeAddrInstruction(segmentData))
			code.AppendUint32(binding.ByteOffset)
			return
		case ScopeConst:
			code.AppendInstruction(makeAddrInstruction(segmentConst))
			code.AppendUint32(binding.ByteOffset)
			return
		case ScopeExtern:
			code.AppendInstruction(makeAddrInstruction(segmentExtern))
			code.AppendUint32(binding.ByteOffset)
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

func (compiler *Compiler) emitOpWithOperand(op Opcode, operand uint32) int {
	compiler.emit(op)
	position := len(compiler.code)
	compiler.code.AppendUint32(operand)
	return position
}

func (compiler *Compiler) patchOperand(position int, operand int) {
	compiler.code.PatchUint32(position, uint32(operand))
}

func (compiler *Compiler) fail(err error) {
	if compiler.err == nil {
		compiler.err = err
	}
}
