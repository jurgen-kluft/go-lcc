package cova

import (
	"fmt"
	"math"
)

// Optimize applies isolated, in-place AST optimizations to program.
func Optimize(program *ProgramNode) error {
	if program == nil {
		return fmt.Errorf("optimization error: program is nil")
	}
	optimizer := newOptimizer(program)
	return optimizer.optimizeProgram(program)
}

type optimizer struct {
	globals         map[string]*Type
	functions       map[string][]*Type
	functionReturns map[string]*Type
	locals          []map[string]*Type
	returnType      *Type
}

func newOptimizer(program *ProgramNode) *optimizer {
	result := &optimizer{
		globals:         make(map[string]*Type, len(program.Decls)),
		functions:       make(map[string][]*Type, len(program.Decls)+len(program.Functions)),
		functionReturns: make(map[string]*Type, len(program.Decls)+len(program.Functions)),
	}
	for _, decl := range program.Decls {
		if decl == nil {
			continue
		}
		if decl.Kind == DeclFunction {
			result.functions[decl.Name] = optimizerParameterTypes(decl.Params)
			result.functionReturns[decl.Name] = decl.Type
		} else {
			result.globals[decl.Name] = decl.Type
		}
	}
	for _, function := range program.Functions {
		if function != nil {
			result.functions[function.Name] = optimizerParameterTypes(function.Params)
			result.functionReturns[function.Name] = function.ReturnType
		}
	}
	return result
}

func optimizerParameterTypes(params []Parameter) []*Type {
	types := make([]*Type, len(params))
	for index, param := range params {
		types[index] = param.Type
	}
	return types
}

func (optimizer *optimizer) optimizeProgram(program *ProgramNode) error {
	for _, decl := range program.Decls {
		if decl == nil || decl.Initializer == nil {
			continue
		}
		optimized, err := optimizer.optimizeExpr(decl.Initializer, decl.Type)
		if err != nil {
			return err
		}
		decl.Initializer = optimized
	}
	for _, function := range program.Functions {
		if function == nil {
			continue
		}
		optimizer.returnType = function.ReturnType
		optimizer.locals = []map[string]*Type{make(map[string]*Type, len(function.Params))}
		for _, param := range function.Params {
			optimizer.locals[0][param.Name] = param.Type
		}
		if err := optimizer.optimizeBlock(function.Body); err != nil {
			return err
		}
	}
	return nil
}

func (optimizer *optimizer) optimizeBlock(block *BlockStmt) error {
	if block == nil {
		return nil
	}
	optimizer.locals = append(optimizer.locals, make(map[string]*Type))
	defer func() { optimizer.locals = optimizer.locals[:len(optimizer.locals)-1] }()
	for _, statement := range block.Statements {
		if err := optimizer.optimizeStmt(statement); err != nil {
			return err
		}
	}
	return nil
}

func (optimizer *optimizer) optimizeStmt(statement StmtNode) error {
	var err error
	switch node := statement.(type) {
	case *BlockStmt:
		return optimizer.optimizeBlock(node)
	case *LocalDeclStmt:
		if node.Initializer != nil {
			node.Initializer, err = optimizer.optimizeExpr(node.Initializer, node.Type)
		}
		optimizer.locals[len(optimizer.locals)-1][node.Name] = node.Type
	case *IfStmt:
		node.Condition, err = optimizer.optimizeExpr(node.Condition, Int32Type)
		if err == nil {
			err = optimizer.optimizeStmt(node.Then)
		}
		if err == nil && node.Else != nil {
			err = optimizer.optimizeStmt(node.Else)
		}
	case *WhileStmt:
		node.Condition, err = optimizer.optimizeExpr(node.Condition, Int32Type)
		if err == nil {
			err = optimizer.optimizeStmt(node.Body)
		}
	case *ForStmt:
		if node.Init != nil {
			err = optimizer.optimizeStmt(node.Init)
		}
		if err == nil && node.Condition != nil {
			node.Condition, err = optimizer.optimizeExpr(node.Condition, Int32Type)
		}
		if err == nil && node.Post != nil {
			err = optimizer.optimizeStmt(node.Post)
		}
		if err == nil {
			err = optimizer.optimizeStmt(node.Body)
		}
	case *SwitchStmt:
		node.Value, err = optimizer.optimizeExpr(node.Value, optimizer.exprType(node.Value))
		for index := range node.Cases {
			if err != nil {
				break
			}
			switchCase := &node.Cases[index]
			switchCase.Value, err = optimizer.optimizeExpr(switchCase.Value, optimizer.exprType(switchCase.Value))
			for _, child := range switchCase.Body {
				if err == nil {
					err = optimizer.optimizeStmt(child)
				}
			}
		}
		for _, child := range node.Default {
			if err == nil {
				err = optimizer.optimizeStmt(child)
			}
		}
	case *ReturnStmt:
		if node.Value != nil {
			node.Value, err = optimizer.optimizeExpr(node.Value, optimizer.returnType)
		}
	case *ExprStmt:
		node.Expr, err = optimizer.optimizeExpr(node.Expr, optimizer.exprType(node.Expr))
	case *AssignStmt:
		node.Value, err = optimizer.optimizeExpr(node.Value, optimizer.exprType(node.Target))
	}
	return err
}

func (optimizer *optimizer) optimizeExpr(expression ExprNode, expected *Type) (ExprNode, error) {
	switch node := expression.(type) {
	case *BinaryExpr:
		return optimizer.optimizeBinary(node, expected)
	case *CallExpr:
		params := optimizer.functions[node.Callee]
		for index, argument := range node.Args {
			var argumentType *Type
			if index < len(params) {
				argumentType = params[index]
			}
			optimized, err := optimizer.optimizeExpr(argument, argumentType)
			if err != nil {
				return nil, err
			}
			node.Args[index] = optimized
		}
	}
	return expression, nil
}

func (optimizer *optimizer) optimizeBinary(node *BinaryExpr, expected *Type) (ExprNode, error) {
	if node.Op == "&&" || node.Op == "||" {
		return optimizer.optimizeLogical(node)
	}
	operationType := expected
	if optimizerIsComparison(node.Op) {
		operationType = optimizerPromoteNumericType(optimizer.exprType(node.Left), optimizer.exprType(node.Right))
	} else if operationType == nil {
		operationType = optimizer.exprType(node)
	}
	if operationType == nil {
		operationType = Int32Type
	}
	left, err := optimizer.optimizeExpr(node.Left, operationType)
	if err != nil {
		return nil, err
	}
	right, err := optimizer.optimizeExpr(node.Right, operationType)
	if err != nil {
		return nil, err
	}
	node.Left, node.Right = left, right
	leftLiteral, leftOK := left.(*NumberLiteral)
	rightLiteral, rightOK := right.(*NumberLiteral)
	if !leftOK || !rightOK {
		return node, nil
	}
	constantKind := optimizerKindFromType(operationType)
	leftConstant := optimizerLiteralBits(leftLiteral, constantKind)
	rightConstant := optimizerLiteralBits(rightLiteral, constantKind)
	if optimizerIsComparison(node.Op) {
		value := optimizerCompare(constantKind, node.Op, leftConstant, rightConstant)
		return &NumberLiteral{IntValue: value, Line: node.Line}, nil
	}
	bits, err := optimizerArithmetic(constantKind, node.Op, leftConstant, rightConstant)
	if err != nil {
		return nil, fmt.Errorf("optimization error on line %d: %v", node.Line, err)
	}
	return optimizerLiteralFromBits(bits, constantKind, node.Line), nil
}

func (optimizer *optimizer) optimizeLogical(node *BinaryExpr) (ExprNode, error) {
	left, err := optimizer.optimizeExpr(node.Left, Int32Type)
	if err != nil {
		return nil, err
	}
	node.Left = left
	if literal, ok := left.(*NumberLiteral); ok {
		truth := optimizerLiteralBits(literal, optimizerInt32) != 0
		if (node.Op == "&&" && !truth) || (node.Op == "||" && truth) {
			return &NumberLiteral{IntValue: optimizerBoolInt(truth), Line: node.Line}, nil
		}
	}
	right, err := optimizer.optimizeExpr(node.Right, Int32Type)
	if err != nil {
		return nil, err
	}
	node.Right = right
	leftLiteral, leftOK := left.(*NumberLiteral)
	rightLiteral, rightOK := right.(*NumberLiteral)
	if !leftOK || !rightOK {
		return node, nil
	}
	leftTruth := optimizerLiteralBits(leftLiteral, optimizerInt32) != 0
	rightTruth := optimizerLiteralBits(rightLiteral, optimizerInt32) != 0
	if node.Op == "&&" {
		return &NumberLiteral{IntValue: optimizerBoolInt(leftTruth && rightTruth), Line: node.Line}, nil
	}
	return &NumberLiteral{IntValue: optimizerBoolInt(leftTruth || rightTruth), Line: node.Line}, nil
}

func (optimizer *optimizer) exprType(expression ExprNode) *Type {
	switch node := expression.(type) {
	case *NumberLiteral:
		if node.IsFloat {
			if node.FloatType != nil {
				return node.FloatType
			}
			return Float32Type
		}
		return Int32Type
	case *IdentNode:
		for index := len(optimizer.locals) - 1; index >= 0; index-- {
			if typ, ok := optimizer.locals[index][node.Name]; ok {
				return typ
			}
		}
		return optimizer.globals[node.Name]
	case *BinaryExpr:
		if node.Op == "&&" || node.Op == "||" {
			return BoolType
		}
		if optimizerIsComparison(node.Op) {
			return Int32Type
		}
		return optimizerPromoteNumericType(optimizer.exprType(node.Left), optimizer.exprType(node.Right))
	case *CallExpr:
		return optimizer.functionReturns[node.Callee]
	}
	return nil
}

type optimizerNumericKind uint8

const (
	optimizerInt32 optimizerNumericKind = iota
	optimizerInt8
	optimizerInt16
	optimizerInt64
	optimizerUint8
	optimizerUint16
	optimizerUint32
	optimizerUint64
	optimizerFloat32
	optimizerFloat64
)

func optimizerKindFromType(typ *Type) optimizerNumericKind {
	if typ == nil {
		return optimizerInt32
	}
	switch typ.Kind {
	case TypeInt8:
		return optimizerInt8
	case TypeInt16:
		return optimizerInt16
	case TypeInt64:
		return optimizerInt64
	case TypeByte, TypeUint8, TypeBool:
		return optimizerUint8
	case TypeUint16:
		return optimizerUint16
	case TypeUint32:
		return optimizerUint32
	case TypeUint64:
		return optimizerUint64
	case TypeFloat32:
		return optimizerFloat32
	case TypeFloat64:
		return optimizerFloat64
	default:
		return optimizerInt32
	}
}

func optimizerLiteralBits(literal *NumberLiteral, kind optimizerNumericKind) uint64 {
	if literal.IsFloat {
		return optimizerFloatToBits(literal.FloatValue, kind)
	}
	return optimizerIntToBits(int64(literal.IntValue), kind)
}

func optimizerIntToBits(value int64, kind optimizerNumericKind) uint64 {
	switch kind {
	case optimizerFloat32:
		return uint64(math.Float32bits(float32(value)))
	case optimizerFloat64:
		return math.Float64bits(float64(value))
	case optimizerInt8, optimizerUint8:
		return uint64(uint8(value))
	case optimizerInt16, optimizerUint16:
		return uint64(uint16(value))
	case optimizerInt32, optimizerUint32:
		return uint64(uint32(value))
	default:
		return uint64(value)
	}
}

func optimizerFloatToBits(value float64, kind optimizerNumericKind) uint64 {
	switch kind {
	case optimizerFloat32:
		return uint64(math.Float32bits(float32(value)))
	case optimizerFloat64:
		return math.Float64bits(value)
	default:
		return optimizerIntToBits(int64(value), kind)
	}
}

func optimizerLiteralFromBits(bits uint64, kind optimizerNumericKind, line int) *NumberLiteral {
	switch kind {
	case optimizerFloat32:
		return &NumberLiteral{FloatValue: float64(math.Float32frombits(uint32(bits))), IsFloat: true, FloatType: Float32Type, Line: line}
	case optimizerFloat64:
		return &NumberLiteral{FloatValue: math.Float64frombits(bits), IsFloat: true, FloatType: Float64Type, Line: line}
	}
	return &NumberLiteral{IntValue: int(optimizerSignedOrUnsignedValue(bits, kind)), Line: line}
}

func optimizerSignedOrUnsignedValue(bits uint64, kind optimizerNumericKind) int64 {
	switch kind {
	case optimizerInt8:
		return int64(int8(bits))
	case optimizerInt16:
		return int64(int16(bits))
	case optimizerInt32:
		return int64(int32(bits))
	case optimizerInt64:
		return int64(bits)
	case optimizerUint8:
		return int64(uint8(bits))
	case optimizerUint16:
		return int64(uint16(bits))
	case optimizerUint32:
		return int64(uint32(bits))
	default:
		return int64(bits)
	}
}

func optimizerArithmetic(kind optimizerNumericKind, op string, left, right uint64) (uint64, error) {
	if op == "/" && optimizerIsZero(kind, right) {
		return 0, fmt.Errorf("division by zero")
	}
	switch kind {
	case optimizerFloat32:
		leftValue, rightValue := math.Float32frombits(uint32(left)), math.Float32frombits(uint32(right))
		return uint64(math.Float32bits(optimizerFloat32Arithmetic(op, leftValue, rightValue))), nil
	case optimizerFloat64:
		return math.Float64bits(optimizerFloatArithmetic(op, math.Float64frombits(left), math.Float64frombits(right))), nil
	case optimizerInt8:
		return optimizerSignedArithmetic(op, int64(int8(left)), int64(int8(right)), 8), nil
	case optimizerInt16:
		return optimizerSignedArithmetic(op, int64(int16(left)), int64(int16(right)), 16), nil
	case optimizerInt32:
		return optimizerSignedArithmetic(op, int64(int32(left)), int64(int32(right)), 32), nil
	case optimizerInt64:
		return optimizerSignedArithmetic(op, int64(left), int64(right), 64), nil
	default:
		return optimizerUnsignedArithmetic(op, optimizerUnsignedValue(left, kind), optimizerUnsignedValue(right, kind), kind), nil
	}
}

func optimizerSignedArithmetic(op string, left, right int64, width uint) uint64 {
	var result int64
	switch op {
	case "+":
		result = left + right
	case "-":
		result = left - right
	case "*":
		result = left * right
	case "/":
		result = left / right
	}
	if width == 64 {
		return uint64(result)
	}
	return uint64(result) & ((uint64(1) << width) - 1)
}

func optimizerUnsignedArithmetic(op string, left, right uint64, kind optimizerNumericKind) uint64 {
	var result uint64
	switch op {
	case "+":
		result = left + right
	case "-":
		result = left - right
	case "*":
		result = left * right
	case "/":
		result = left / right
	}
	switch kind {
	case optimizerUint8:
		return uint64(uint8(result))
	case optimizerUint16:
		return uint64(uint16(result))
	case optimizerUint32:
		return uint64(uint32(result))
	default:
		return result
	}
}

func optimizerFloat32Arithmetic(op string, left, right float32) float32 {
	switch op {
	case "+":
		return left + right
	case "-":
		return left - right
	case "*":
		return left * right
	case "/":
		return left / right
	default:
		return 0
	}
}

func optimizerFloatArithmetic(op string, left, right float64) float64 {
	switch op {
	case "+":
		return left + right
	case "-":
		return left - right
	case "*":
		return left * right
	case "/":
		return left / right
	default:
		return 0
	}
}

func optimizerCompare(kind optimizerNumericKind, op string, left, right uint64) int {
	switch kind {
	case optimizerFloat32:
		return optimizerCompareFloat(op, float64(math.Float32frombits(uint32(left))), float64(math.Float32frombits(uint32(right))))
	case optimizerFloat64:
		return optimizerCompareFloat(op, math.Float64frombits(left), math.Float64frombits(right))
	case optimizerInt8, optimizerInt16, optimizerInt32, optimizerInt64:
		return optimizerCompareInt(op, optimizerSignedOrUnsignedValue(left, kind), optimizerSignedOrUnsignedValue(right, kind))
	default:
		return optimizerCompareUint(op, optimizerUnsignedValue(left, kind), optimizerUnsignedValue(right, kind))
	}
}

func optimizerUnsignedValue(bits uint64, kind optimizerNumericKind) uint64 {
	switch kind {
	case optimizerUint8:
		return uint64(uint8(bits))
	case optimizerUint16:
		return uint64(uint16(bits))
	case optimizerUint32:
		return uint64(uint32(bits))
	default:
		return bits
	}
}

func optimizerCompareInt(op string, left, right int64) int {
	switch op {
	case "==":
		return optimizerBoolInt(left == right)
	case "!=":
		return optimizerBoolInt(left != right)
	case "<":
		return optimizerBoolInt(left < right)
	case "<=":
		return optimizerBoolInt(left <= right)
	case ">":
		return optimizerBoolInt(left > right)
	case ">=":
		return optimizerBoolInt(left >= right)
	default:
		return 0
	}
}

func optimizerCompareUint(op string, left, right uint64) int {
	switch op {
	case "==":
		return optimizerBoolInt(left == right)
	case "!=":
		return optimizerBoolInt(left != right)
	case "<":
		return optimizerBoolInt(left < right)
	case "<=":
		return optimizerBoolInt(left <= right)
	case ">":
		return optimizerBoolInt(left > right)
	case ">=":
		return optimizerBoolInt(left >= right)
	default:
		return 0
	}
}

func optimizerCompareFloat(op string, left, right float64) int {
	switch op {
	case "==":
		return optimizerBoolInt(left == right)
	case "!=":
		return optimizerBoolInt(left != right)
	case "<":
		return optimizerBoolInt(left < right)
	case "<=":
		return optimizerBoolInt(left <= right)
	case ">":
		return optimizerBoolInt(left > right)
	case ">=":
		return optimizerBoolInt(left >= right)
	default:
		return 0
	}
}

func optimizerIsZero(kind optimizerNumericKind, bits uint64) bool {
	switch kind {
	case optimizerFloat32:
		return math.Float32frombits(uint32(bits)) == 0
	case optimizerFloat64:
		return math.Float64frombits(bits) == 0
	default:
		return optimizerUnsignedValue(bits, kind) == 0
	}
}

func optimizerIsComparison(op string) bool {
	switch op {
	case "==", "!=", "<", "<=", ">", ">=":
		return true
	default:
		return false
	}
}

func optimizerPromoteNumericType(left, right *Type) *Type {
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

func optimizerBoolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
