package lcc

import (
	"fmt"
	"math"
)

type callFrame struct {
	returnPC   int
	localBase  int
	returnKind ValueKind
}

type VM struct {
	memory           ProgramMemory
	pc               int
	program          *LinkedProgram
	externDispatcher ExternDispatcher
	callFrames       []callFrame
	callFrameTop     int
	frameTop         int
}

func NewVM(frameCapacity int) *VM {
	return NewVMWithCallFrameCapacity(frameCapacity, 8)
}

func NewVMWithCallFrameCapacity(frameCapacity int, callFrameCapacity int) *VM {
	if callFrameCapacity < 1 {
		callFrameCapacity = 1
	}
	return &VM{
		memory:     NewProgramMemory(0, 0, 0, 0, frameCapacity, 32),
		callFrames: make([]callFrame, callFrameCapacity),
	}
}

func (vm *VM) AllocateExternMemory(size int) {
	vm.memory.segment[segmentExtern] = NewMemorySegment(size, size)
}

func (vm *VM) BindExternBlock(block []byte) {
	vm.memory.segment[segmentExtern] = block
}

func (vm *VM) LoadExternInt32(offset int) int {
	if offset < 0 || offset+4 > len(vm.memory.segment[segmentExtern]) {
		return 0
	}
	bits, err := vm.memory.ReadBits(makeAddress(segmentExtern, offset), KindInt32)
	if err != nil {
		return 0
	}
	return int(int32(bits))
}

func (vm *VM) StoreExternInt32(offset int, value int) {
	if offset < 0 || offset+4 > len(vm.memory.segment[segmentExtern]) {
		return
	}
	_ = vm.memory.WriteBits(makeAddress(segmentExtern, offset), KindInt32, uint64(uint32(int32(value))))
}

func (vm *VM) RegisterExternDispatcher(dispatcher ExternDispatcher) {
	vm.externDispatcher = dispatcher
}

func (vm *VM) PushBits(kind ValueKind, bits uint64) error {
	return vm.pushKind(kind, bits)
}

func (vm *VM) PushBool(value bool) error {
	if value {
		return vm.pushKind(KindBool, 1)
	}
	return vm.pushKind(KindBool, 0)
}

func (vm *VM) PushByte(value byte) error {
	return vm.pushKind(KindByte, uint64(value))
}

func (vm *VM) PushInt8(value int8) error {
	return vm.pushKind(KindInt8, uint64(uint8(value)))
}

func (vm *VM) PushInt16(value int16) error {
	return vm.pushKind(KindInt16, uint64(uint16(value)))
}

func (vm *VM) PushInt32(value int32) error {
	return vm.pushKind(KindInt32, uint64(uint32(value)))
}

func (vm *VM) PushInt64(value int64) error {
	return vm.pushKind(KindInt64, uint64(value))
}

func (vm *VM) PushUint8(value uint8) error {
	return vm.pushKind(KindUint8, uint64(value))
}

func (vm *VM) PushUint16(value uint16) error {
	return vm.pushKind(KindUint16, uint64(value))
}

func (vm *VM) PushUint32(value uint32) error {
	return vm.pushKind(KindUint32, uint64(value))
}

func (vm *VM) PushUint64(value uint64) error {
	return vm.pushKind(KindUint64, value)
}

func (vm *VM) PushFloat32(value float32) error {
	return vm.pushKind(KindFloat32, uint64(math.Float32bits(value)))
}

func (vm *VM) PushFloat64(value float64) error {
	return vm.pushKind(KindFloat64, math.Float64bits(value))
}

func (vm *VM) PopBits(kind ValueKind) (uint64, error) {
	return vm.popKind(kind)
}

func (vm *VM) PopBool() (bool, error) {
	bits, err := vm.popKind(KindBool)
	return bits != 0, err
}

func (vm *VM) PopByte() (byte, error) {
	bits, err := vm.popKind(KindByte)
	return byte(bits), err
}

func (vm *VM) PopInt8() (int8, error) {
	bits, err := vm.popKind(KindInt8)
	return int8(bits), err
}

func (vm *VM) PopInt16() (int16, error) {
	bits, err := vm.popKind(KindInt16)
	return int16(bits), err
}

func (vm *VM) PopInt32() (int32, error) {
	bits, err := vm.popKind(KindInt32)
	return int32(bits), err
}

func (vm *VM) PopInt64() (int64, error) {
	bits, err := vm.popKind(KindInt64)
	return int64(bits), err
}

func (vm *VM) PopUint8() (uint8, error) {
	bits, err := vm.popKind(KindUint8)
	return uint8(bits), err
}

func (vm *VM) PopUint16() (uint16, error) {
	bits, err := vm.popKind(KindUint16)
	return uint16(bits), err
}

func (vm *VM) PopUint32() (uint32, error) {
	bits, err := vm.popKind(KindUint32)
	return uint32(bits), err
}

func (vm *VM) PopUint64() (uint64, error) {
	return vm.popKind(KindUint64)
}

func (vm *VM) PopFloat32() (float32, error) {
	bits, err := vm.popKind(KindFloat32)
	return math.Float32frombits(uint32(bits)), err
}

func (vm *VM) PopFloat64() (float64, error) {
	bits, err := vm.popKind(KindFloat64)
	return math.Float64frombits(bits), err
}

func (vm *VM) Run(program *LinkedProgram) error {
	if program == nil {
		return fmt.Errorf("vm error: linked program is nil")
	}
	if len(vm.memory.segment[segmentBSS]) != program.BSSByteSize {
		vm.memory.segment[segmentBSS] = make([]byte, program.BSSByteSize)
	} else {
		for index := range vm.memory.segment[segmentBSS] {
			vm.memory.segment[segmentBSS][index] = 0
		}
	}
	if len(vm.memory.segment[segmentConst]) != program.ConstByteSize {
		vm.memory.segment[segmentConst] = make([]byte, program.ConstByteSize)
	}
	copy(vm.memory.segment[segmentConst], program.ConstData)
	if len(vm.memory.segment[segmentData]) != program.DataByteSize {
		vm.memory.segment[segmentData] = make([]byte, program.DataByteSize)
	}
	copy(vm.memory.segment[segmentData], program.DataData)

	vm.program = program
	vm.pc = 0
	vm.memory.segment[segmentStack] = vm.memory.segment[segmentStack][:0]
	for index := range vm.memory.segment[segmentFrame] {
		vm.memory.segment[segmentFrame][index] = 0
	}
	vm.callFrameTop = 0
	vm.frameTop = 0

	if program.EntryPoint < 0 || program.EntryPoint >= len(program.Text) {
		return fmt.Errorf("vm error: entry point %d out of range", program.EntryPoint)
	}
	if err := vm.enterScriptFunction(program.EntryPoint, nil, -1); err != nil {
		return err
	}

	for vm.pc < len(program.Text) {
		instruction := program.Text.ReadInstruction(&vm.pc)
		op := instruction.Opcode()

		switch op {
		case OpPush:
			kind := instruction.Kind()
			if kind == KindNone || kind == KindAddress {
				return fmt.Errorf("vm error: unsupported push kind %d", kind)
			}
			if err := vm.pushKind(kind, program.Text.ReadImmediate(&vm.pc, kind)); err != nil {
				return err
			}
		case OpArithmetic:
			kind := instruction.Kind()
			arithmeticOp := instruction.ArithmeticOp()
			right, left, err := vm.popBinary(kind)
			if err != nil {
				return err
			}
			result, err := vm.executeArithmetic(kind, arithmeticOp, left, right)
			if err != nil {
				return err
			}
			if err := vm.pushKind(kind, result); err != nil {
				return err
			}
		case OpConvert:
			fromKind := instruction.ConvertFromKind()
			bits, err := vm.popKind(fromKind)
			if err != nil {
				return err
			}
			kind := instruction.Kind()
			if err := vm.pushKind(kind, convertBits(fromKind, kind, bits)); err != nil {
				return err
			}
		case OpAddr:
			segment := instruction.AddressSegment()
			offset := program.Text.ReadInt(&vm.pc)
			if segment == segmentFrame {
				frame, err := vm.currentFrame()
				if err != nil {
					return err
				}
				offset += frame.localBase
			}
			if err := vm.pushAddress(makeAddress(segment, offset)); err != nil {
				return err
			}
		case OpOffset:
			offset, err := vm.popInt32()
			if err != nil {
				return err
			}
			base, err := vm.popAddress()
			if err != nil {
				return err
			}
			if err := vm.pushAddress(makeAddress(base.Segment(), base.Index()+offset)); err != nil {
				return err
			}
		case OpDereference:
			kind := instruction.Kind()
			if kind == KindNone {
				return fmt.Errorf("vm error: unsupported dereference kind %d", kind)
			}
			encodedAddress, err := vm.popAddress()
			if err != nil {
				return err
			}
			value, err := vm.memory.ReadBits(encodedAddress, kind)
			if err != nil {
				return err
			}
			if err := vm.pushKind(kind, value); err != nil {
				return err
			}
		case OpAssign:
			kind := instruction.Kind()
			if kind == KindNone {
				return fmt.Errorf("vm error: unsupported assign kind %d", kind)
			}
			encodedAddress, err := vm.popAddress()
			if err != nil {
				return err
			}
			value, err := vm.popKind(kind)
			if err != nil {
				return err
			}
			if err := vm.memory.WriteBits(encodedAddress, kind, value); err != nil {
				return err
			}
		case OpCompare:
			kind := instruction.Kind()
			compareOp := instruction.CompareOp()
			right, left, err := vm.popBinary(kind)
			if err != nil {
				return err
			}
			result := vm.executeComparison(kind, compareOp, left, right)
			if err := vm.PushInt32(result); err != nil {
				return err
			}
		case OpJumpIfFalse:
			target := program.Text.ReadInt(&vm.pc)
			condition, err := vm.popInt32()
			if err != nil {
				return err
			}
			if condition == 0 {
				vm.pc = target
			}
		case OpJump:
			vm.pc = program.Text.ReadInt(&vm.pc)
		case OpCall:
			if err := vm.callScriptFunction(program.Text.ReadInt(&vm.pc)); err != nil {
				return err
			}
		case OpCallExtern:
			if err := vm.callExtern(program.Text.ReadInt(&vm.pc)); err != nil {
				return err
			}
		case OpRet:
			done, err := vm.returnFromFunction()
			if err != nil {
				return err
			}
			if done {
				return nil
			}
		default:
			return fmt.Errorf("vm error: unknown opcode %d at ip %d", op, vm.pc-2)
		}
	}

	return nil
}

func (vm *VM) currentFrame() (*callFrame, error) {
	if vm.callFrameTop == 0 {
		return nil, fmt.Errorf("vm error: no active call frame")
	}
	return &vm.callFrames[vm.callFrameTop-1], nil
}

func (vm *VM) enterScriptFunction(entryAddress int, args []uint64, returnPC int) error {
	header, err := vm.program.Text.ReadFunctionHeader(entryAddress)
	if err != nil {
		return err
	}
	if len(args) != header.ParamCount {
		return fmt.Errorf("vm error: function at %d expects %d args, got %d", entryAddress, header.ParamCount, len(args))
	}
	localBase := vm.frameTop
	if localBase+header.FrameByteSize > len(vm.memory.segment[segmentFrame]) {
		return fmt.Errorf("vm error: frame capacity exceeded: need %d bytes, have %d", localBase+header.FrameByteSize, len(vm.memory.segment[segmentFrame]))
	}
	for offset := localBase; offset < localBase+header.FrameByteSize; offset++ {
		vm.memory.segment[segmentFrame][offset] = 0
	}
	for index, value := range args {
		if index >= len(header.ParamOffsets) {
			return fmt.Errorf("vm error: function at %d missing byte offset for arg %d", entryAddress, index)
		}
		kind := KindInt32
		if index < len(header.ParamKinds) && header.ParamKinds[index] != KindNone {
			kind = header.ParamKinds[index]
		}
		if err := vm.memory.WriteBits(makeAddress(segmentFrame, localBase+header.ParamOffsets[index]), kind, value); err != nil {
			return err
		}
	}
	vm.frameTop = localBase + header.FrameByteSize
	if vm.callFrameTop >= len(vm.callFrames) {
		return fmt.Errorf("vm error: call frame capacity exceeded: need %d frames, have %d", vm.callFrameTop+1, len(vm.callFrames))
	}
	vm.callFrames[vm.callFrameTop] = callFrame{returnPC: returnPC, localBase: localBase, returnKind: header.ReturnKind}
	vm.callFrameTop++
	vm.pc = header.BodyAddress
	return nil
}

func (vm *VM) callScriptFunction(entryAddress int) error {
	header, err := vm.program.Text.ReadFunctionHeader(entryAddress)
	if err != nil {
		return err
	}
	args, err := vm.popArgBits(header.ParamKinds)
	if err != nil {
		return err
	}
	return vm.enterScriptFunction(entryAddress, args, vm.pc)
}

func (vm *VM) callExtern(importID int) error {
	if vm.externDispatcher == nil {
		return fmt.Errorf("vm error: no extern dispatcher registered for import %d", importID)
	}
	return vm.externDispatcher(vm, importID)
}

func (vm *VM) popArgBits(kinds []ValueKind) ([]uint64, error) {
	args := make([]uint64, len(kinds))
	for index := len(kinds) - 1; index >= 0; index-- {
		kind := kinds[index]
		if kind == KindNone {
			kind = KindInt32
		}
		bits, err := vm.popKind(kind)
		if err != nil {
			return nil, err
		}
		args[index] = bits
	}
	return args, nil
}

func (vm *VM) returnFromFunction() (bool, error) {
	if vm.callFrameTop == 0 {
		return false, fmt.Errorf("vm error: return without active frame")
	}
	vm.callFrameTop--
	frame := vm.callFrames[vm.callFrameTop]
	var resultBits uint64
	if hasStackValueKind(frame.returnKind) {
		value, err := vm.popKind(frame.returnKind)
		if err != nil {
			return false, err
		}
		resultBits = value
	}
	vm.frameTop = frame.localBase
	if vm.callFrameTop == 0 {
		if hasStackValueKind(frame.returnKind) {
			if err := vm.pushKind(frame.returnKind, resultBits); err != nil {
				return false, err
			}
		}
		return true, nil
	}
	vm.pc = frame.returnPC
	if hasStackValueKind(frame.returnKind) {
		if err := vm.pushKind(frame.returnKind, resultBits); err != nil {
			return false, err
		}
	}
	return false, nil
}

func hasStackValueKind(kind ValueKind) bool {
	return kind != KindNone && kind != KindVoid
}

func (vm *VM) pushKind(kind ValueKind, bits uint64) error {
	return appendStackBits(&vm.memory.segment[segmentStack], kind, bits)
}

func (vm *VM) pushInt32(value int) {
	_ = appendStackBits(&vm.memory.segment[segmentStack], KindInt32, uint64(uint32(int32(value))))
}

func (vm *VM) pushAddress(address Address) error {
	return appendAddress(&vm.memory.segment[segmentStack], address)
}

func (vm *VM) popInt32() (int, error) {
	bits, err := truncateStackBits(&vm.memory.segment[segmentStack], KindInt32)
	if err != nil {
		return 0, err
	}
	return int(int32(bits)), nil
}

func (vm *VM) popAddress() (Address, error) {
	bits, err := truncateStackBits(&vm.memory.segment[segmentStack], KindAddress)
	if err != nil {
		return 0, err
	}
	return Address(uint32(bits)), nil
}

func (vm *VM) popKind(kind ValueKind) (uint64, error) {
	return truncateStackBits(&vm.memory.segment[segmentStack], kind)
}

func (vm *VM) popBinary(kind ValueKind) (right uint64, left uint64, err error) {
	return popBinaryBits(&vm.memory.segment[segmentStack], kind)
}

func appendStackBits(stack *MemorySegment, kind ValueKind, bits uint64) error {
	return stack.AppendBits(kind, bits)
}

func truncateStackBits(stack *MemorySegment, kind ValueKind) (uint64, error) {
	return stack.TruncateBits(kind)
}

func appendAddress(stack *MemorySegment, address Address) error {
	return appendStackBits(stack, KindAddress, uint64(uint32(address)))
}

func popBinaryBits(stack *MemorySegment, kind ValueKind) (right uint64, left uint64, err error) {
	right, err = truncateStackBits(stack, kind)
	if err != nil {
		return 0, 0, err
	}
	left, err = truncateStackBits(stack, kind)
	if err != nil {
		return 0, 0, err
	}
	return right, left, nil
}

func (vm *VM) isZero(kind ValueKind, bits uint64) bool {
	switch kind {
	case KindFloat32:
		return math.Float32frombits(uint32(bits)) == 0
	case KindFloat64:
		return math.Float64frombits(bits) == 0
	default:
		return bits == 0
	}
}

func (vm *VM) executeArithmetic(kind ValueKind, op ArithmeticOp, left uint64, right uint64) (uint64, error) {
	switch kind {
	case KindBool, KindByte, KindUint8, KindUint16, KindUint32, KindUint64:
		switch op {
		case ArithmeticAdd:
			return left + right, nil
		case ArithmeticSub:
			return left - right, nil
		case ArithmeticMul:
			return left * right, nil
		case ArithmeticDiv:
			if vm.isZero(kind, right) {
				return 0, fmt.Errorf("vm error: division by zero")
			}
			return left / right, nil
		}
	case KindInt8:
		leftValue, rightValue := int8(left), int8(right)
		switch op {
		case ArithmeticAdd:
			return uint64(uint8(leftValue + rightValue)), nil
		case ArithmeticSub:
			return uint64(uint8(leftValue - rightValue)), nil
		case ArithmeticMul:
			return uint64(uint8(leftValue * rightValue)), nil
		case ArithmeticDiv:
			if vm.isZero(kind, right) {
				return 0, fmt.Errorf("vm error: division by zero")
			}
			return uint64(uint8(leftValue / rightValue)), nil
		}
	case KindInt16:
		leftValue, rightValue := int16(left), int16(right)
		switch op {
		case ArithmeticAdd:
			return uint64(uint16(leftValue + rightValue)), nil
		case ArithmeticSub:
			return uint64(uint16(leftValue - rightValue)), nil
		case ArithmeticMul:
			return uint64(uint16(leftValue * rightValue)), nil
		case ArithmeticDiv:
			if vm.isZero(kind, right) {
				return 0, fmt.Errorf("vm error: division by zero")
			}
			return uint64(uint16(leftValue / rightValue)), nil
		}
	case KindInt64:
		leftValue, rightValue := int64(left), int64(right)
		switch op {
		case ArithmeticAdd:
			return uint64(leftValue + rightValue), nil
		case ArithmeticSub:
			return uint64(leftValue - rightValue), nil
		case ArithmeticMul:
			return uint64(leftValue * rightValue), nil
		case ArithmeticDiv:
			if vm.isZero(kind, right) {
				return 0, fmt.Errorf("vm error: division by zero")
			}
			return uint64(leftValue / rightValue), nil
		}
	case KindInt32:
		leftValue, rightValue := int32(left), int32(right)
		switch op {
		case ArithmeticAdd:
			return uint64(uint32(leftValue + rightValue)), nil
		case ArithmeticSub:
			return uint64(uint32(leftValue - rightValue)), nil
		case ArithmeticMul:
			return uint64(uint32(leftValue * rightValue)), nil
		case ArithmeticDiv:
			if vm.isZero(kind, right) {
				return 0, fmt.Errorf("vm error: division by zero")
			}
			return uint64(uint32(leftValue / rightValue)), nil
		}
	case KindFloat32:
		leftValue := math.Float32frombits(uint32(left))
		rightValue := math.Float32frombits(uint32(right))
		switch op {
		case ArithmeticAdd:
			return uint64(math.Float32bits(leftValue + rightValue)), nil
		case ArithmeticSub:
			return uint64(math.Float32bits(leftValue - rightValue)), nil
		case ArithmeticMul:
			return uint64(math.Float32bits(leftValue * rightValue)), nil
		case ArithmeticDiv:
			if vm.isZero(kind, right) {
				return 0, fmt.Errorf("vm error: division by zero")
			}
			return uint64(math.Float32bits(leftValue / rightValue)), nil
		}
	case KindFloat64:
		leftValue := math.Float64frombits(left)
		rightValue := math.Float64frombits(right)
		switch op {
		case ArithmeticAdd:
			return math.Float64bits(leftValue + rightValue), nil
		case ArithmeticSub:
			return math.Float64bits(leftValue - rightValue), nil
		case ArithmeticMul:
			return math.Float64bits(leftValue * rightValue), nil
		case ArithmeticDiv:
			if vm.isZero(kind, right) {
				return 0, fmt.Errorf("vm error: division by zero")
			}
			return math.Float64bits(leftValue / rightValue), nil
		}
	default:
		// unhandled kind, treat as error
	}

	return 0, fmt.Errorf("vm error: unsupported arithmetic op %d", op)
}

func (vm *VM) executeComparison(kind ValueKind, op CompareOp, left uint64, right uint64) int32 {
	return compareBits(kind, left, right, op)
}

func compareBits(kind ValueKind, left uint64, right uint64, op CompareOp) int32 {
	switch kind {
	case KindFloat32:
		leftValue := float64(math.Float32frombits(uint32(left)))
		rightValue := float64(math.Float32frombits(uint32(right)))
		switch op {
		case CompareEqual:
			if leftValue == rightValue {
				return 1
			}
		case CompareNotEqual:
			if leftValue != rightValue {
				return 1
			}
		case CompareLess:
			if leftValue < rightValue {
				return 1
			}
		case CompareLessEqual:
			if leftValue <= rightValue {
				return 1
			}
		case CompareGreater:
			if leftValue > rightValue {
				return 1
			}
		case CompareGreaterEqual:
			if leftValue >= rightValue {
				return 1
			}
		}
	case KindFloat64:
		leftValue := math.Float64frombits(left)
		rightValue := math.Float64frombits(right)
		switch op {
		case CompareEqual:
			if leftValue == rightValue {
				return 1
			}
		case CompareNotEqual:
			if leftValue != rightValue {
				return 1
			}
		case CompareLess:
			if leftValue < rightValue {
				return 1
			}
		case CompareLessEqual:
			if leftValue <= rightValue {
				return 1
			}
		case CompareGreater:
			if leftValue > rightValue {
				return 1
			}
		case CompareGreaterEqual:
			if leftValue >= rightValue {
				return 1
			}
		}
	case KindInt8, KindInt16, KindInt32, KindInt64:
		leftValue := bitsToInt64(kind, left)
		rightValue := bitsToInt64(kind, right)
		switch op {
		case CompareEqual:
			if leftValue == rightValue {
				return 1
			}
		case CompareNotEqual:
			if leftValue != rightValue {
				return 1
			}
		case CompareLess:
			if leftValue < rightValue {
				return 1
			}
		case CompareLessEqual:
			if leftValue <= rightValue {
				return 1
			}
		case CompareGreater:
			if leftValue > rightValue {
				return 1
			}
		case CompareGreaterEqual:
			if leftValue >= rightValue {
				return 1
			}
		}
	case KindBool, KindByte, KindUint8, KindUint16, KindUint32, KindUint64, KindAddress:
		leftValue := bitsToUint64(kind, left)
		rightValue := bitsToUint64(kind, right)
		switch op {
		case CompareEqual:
			if leftValue == rightValue {
				return 1
			}
		case CompareNotEqual:
			if leftValue != rightValue {
				return 1
			}
		case CompareLess:
			if leftValue < rightValue {
				return 1
			}
		case CompareLessEqual:
			if leftValue <= rightValue {
				return 1
			}
		case CompareGreater:
			if leftValue > rightValue {
				return 1
			}
		case CompareGreaterEqual:
			if leftValue >= rightValue {
				return 1
			}
		}
	default:
		leftValue := bitsToInt64(kind, left)
		rightValue := bitsToInt64(kind, right)
		switch op {
		case CompareEqual:
			if leftValue == rightValue {
				return 1
			}
		case CompareNotEqual:
			if leftValue != rightValue {
				return 1
			}
		case CompareLess:
			if leftValue < rightValue {
				return 1
			}
		case CompareLessEqual:
			if leftValue <= rightValue {
				return 1
			}
		case CompareGreater:
			if leftValue > rightValue {
				return 1
			}
		case CompareGreaterEqual:
			if leftValue >= rightValue {
				return 1
			}
		}
	}
	return 0
}

func convertBits(from ValueKind, to ValueKind, bits uint64) uint64 {
	if from == to {
		return bits
	}
	switch to {
	case KindFloat32:
		return uint64(math.Float32bits(float32(bitsToFloat64(from, bits))))
	case KindFloat64:
		return math.Float64bits(bitsToFloat64(from, bits))
	case KindBool:
		if isZeroBits(from, bits) {
			return 0
		}
		return 1
	case KindByte, KindUint8:
		return uint64(uint8(bitsToUint64(from, bits)))
	case KindInt8:
		return uint64(uint8(int8(bitsToInt64(from, bits))))
	case KindInt16:
		return uint64(uint16(int16(bitsToInt64(from, bits))))
	case KindUint16:
		return uint64(uint16(bitsToUint64(from, bits)))
	case KindInt32:
		return uint64(uint32(int32(bitsToInt64(from, bits))))
	case KindUint32, KindAddress:
		return uint64(uint32(bitsToUint64(from, bits)))
	case KindInt64:
		return uint64(bitsToInt64(from, bits))
	case KindUint64:
		return bitsToUint64(from, bits)
	default:
		return bits
	}
}

func isZeroBits(kind ValueKind, bits uint64) bool {
	switch kind {
	case KindFloat32:
		return math.Float32frombits(uint32(bits)) == 0
	case KindFloat64:
		return math.Float64frombits(bits) == 0
	default:
		return bits == 0
	}
}

func bitsToFloat64(kind ValueKind, bits uint64) float64 {
	switch kind {
	case KindFloat32:
		return float64(math.Float32frombits(uint32(bits)))
	case KindFloat64:
		return math.Float64frombits(bits)
	case KindBool:
		if bits == 0 {
			return 0
		}
		return 1
	case KindByte, KindUint8, KindUint16, KindUint32, KindUint64, KindAddress:
		return float64(bitsToUint64(kind, bits))
	default:
		return float64(bitsToInt64(kind, bits))
	}
}

func bitsToInt64(kind ValueKind, bits uint64) int64 {
	switch kind {
	case KindBool:
		if bits == 0 {
			return 0
		}
		return 1
	case KindByte, KindUint8:
		return int64(uint8(bits))
	case KindInt8:
		return int64(int8(bits))
	case KindInt16:
		return int64(int16(bits))
	case KindUint16:
		return int64(uint16(bits))
	case KindInt32:
		return int64(int32(bits))
	case KindUint32, KindAddress:
		return int64(uint32(bits))
	case KindInt64:
		return int64(bits)
	case KindUint64:
		return int64(bits)
	case KindFloat32:
		return int64(math.Float32frombits(uint32(bits)))
	case KindFloat64:
		return int64(math.Float64frombits(bits))
	default:
		return int64(int32(bits))
	}
}

func bitsToUint64(kind ValueKind, bits uint64) uint64 {
	switch kind {
	case KindBool:
		if bits == 0 {
			return 0
		}
		return 1
	case KindByte, KindUint8:
		return uint64(uint8(bits))
	case KindInt8:
		return uint64(uint8(int8(bits)))
	case KindInt16:
		return uint64(uint16(int16(bits)))
	case KindUint16:
		return uint64(uint16(bits))
	case KindInt32:
		return uint64(uint32(int32(bits)))
	case KindUint32, KindAddress:
		return uint64(uint32(bits))
	case KindInt64, KindUint64:
		return uint64(bits)
	case KindFloat32:
		return uint64(math.Float32frombits(uint32(bits)))
	case KindFloat64:
		return uint64(math.Float64frombits(bits))
	default:
		return uint64(uint32(bits))
	}
}
