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
	Memory           ProgramMemory
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
		Memory:     NewProgramMemory(0, 0, frameCapacity, 32),
		callFrames: make([]callFrame, callFrameCapacity),
	}
}

func (vm *VM) AllocateExternMemory(size int) {
	vm.Memory.segment[segmentExtern] = NewMemorySegment(size, size)
}

func (vm *VM) BindExternBlock(block []byte) {
	vm.Memory.segment[segmentExtern] = block
}

func (vm *VM) LoadExternInt32(offset int) int {
	if offset < 0 || offset+4 > len(vm.Memory.segment[segmentExtern]) {
		return 0
	}
	bits, err := vm.Memory.ReadBits(makeAddress(segmentExtern, offset), KindInt32)
	if err != nil {
		return 0
	}
	return int(int32(bits))
}

func (vm *VM) StoreExternInt32(offset int, value int) {
	if offset < 0 || offset+4 > len(vm.Memory.segment[segmentExtern]) {
		return
	}
	_ = vm.Memory.WriteBits(makeAddress(segmentExtern, offset), KindInt32, uint64(uint32(int32(value))))
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
	if len(vm.Memory.segment[segmentBSS]) != program.BSSByteSize {
		vm.Memory.segment[segmentBSS] = make([]byte, program.BSSByteSize)
	} else {
		for index := range vm.Memory.segment[segmentBSS] {
			vm.Memory.segment[segmentBSS][index] = 0
		}
	}

	vm.program = program
	vm.pc = 0
	vm.Memory.segment[segmentStack] = vm.Memory.segment[segmentStack][:0]
	for index := range vm.Memory.segment[segmentFrame] {
		vm.Memory.segment[segmentFrame][index] = 0
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
		kind := instruction.Kind()

		switch op {
		case OpPush:
			if kind == KindNone || kind == KindAddress {
				return fmt.Errorf("vm error: unsupported push kind %d", kind)
			}
			if err := vm.pushKind(kind, program.Text.ReadImmediate(&vm.pc, kind)); err != nil {
				return err
			}
		case OpAdd:
			right, left, err := vm.popBinary(kind)
			if err != nil {
				return err
			}
			if err := vm.pushKind(kind, addBits(kind, left, right)); err != nil {
				return err
			}
		case OpSub:
			right, left, err := vm.popBinary(kind)
			if err != nil {
				return err
			}
			if err := vm.pushKind(kind, subBits(kind, left, right)); err != nil {
				return err
			}
		case OpMul:
			right, left, err := vm.popBinary(kind)
			if err != nil {
				return err
			}
			if err := vm.pushKind(kind, mulBits(kind, left, right)); err != nil {
				return err
			}
		case OpDiv:
			right, left, err := vm.popBinary(kind)
			if err != nil {
				return err
			}
			if vm.isZero(kind, right) {
				return fmt.Errorf("vm error: division by zero")
			}
			if err := vm.pushKind(kind, divBits(kind, left, right)); err != nil {
				return err
			}
		case OpConvert:
			fromKind := ValueKind(program.Text.ReadImmediate(&vm.pc, KindUint8))
			bits, err := vm.popKind(fromKind)
			if err != nil {
				return err
			}
			if err := vm.pushKind(kind, convertBits(fromKind, kind, bits)); err != nil {
				return err
			}
		case OpAddrFrame:
			frame, err := vm.currentFrame()
			if err != nil {
				return err
			}
			offset := program.Text.ReadInt(&vm.pc)
			if err := vm.pushAddress(makeAddress(segmentFrame, frame.localBase+offset)); err != nil {
				return err
			}
		case OpAddrBSS:
			if err := vm.pushAddress(makeAddress(segmentBSS, program.Text.ReadInt(&vm.pc))); err != nil {
				return err
			}
		case OpAddrExtern:
			if err := vm.pushAddress(makeAddress(segmentExtern, program.Text.ReadInt(&vm.pc))); err != nil {
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
			if kind == KindNone || kind == KindAddress {
				return fmt.Errorf("vm error: unsupported dereference kind %d", kind)
			}
			encodedAddress, err := vm.popAddress()
			if err != nil {
				return err
			}
			value, err := vm.Memory.ReadBits(encodedAddress, kind)
			if err != nil {
				return err
			}
			if err := vm.pushKind(kind, value); err != nil {
				return err
			}
		case OpAssign:
			if kind == KindNone || kind == KindAddress {
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
			if err := vm.Memory.WriteBits(encodedAddress, kind, value); err != nil {
				return err
			}
		case OpEqual:
			right, left, err := vm.popBinary(kind)
			if err != nil {
				return err
			}
			if err := vm.PushInt32(compareBits(kind, left, right, OpEqual)); err != nil {
				return err
			}
		case OpNotEqual:
			right, left, err := vm.popBinary(kind)
			if err != nil {
				return err
			}
			if err := vm.PushInt32(compareBits(kind, left, right, OpNotEqual)); err != nil {
				return err
			}
		case OpLess:
			right, left, err := vm.popBinary(kind)
			if err != nil {
				return err
			}
			if err := vm.PushInt32(compareBits(kind, left, right, OpLess)); err != nil {
				return err
			}
		case OpLessEqual:
			right, left, err := vm.popBinary(kind)
			if err != nil {
				return err
			}
			if err := vm.PushInt32(compareBits(kind, left, right, OpLessEqual)); err != nil {
				return err
			}
		case OpGreater:
			right, left, err := vm.popBinary(kind)
			if err != nil {
				return err
			}
			if err := vm.PushInt32(compareBits(kind, left, right, OpGreater)); err != nil {
				return err
			}
		case OpGreaterEqual:
			right, left, err := vm.popBinary(kind)
			if err != nil {
				return err
			}
			if err := vm.PushInt32(compareBits(kind, left, right, OpGreaterEqual)); err != nil {
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
	if localBase+header.FrameByteSize > len(vm.Memory.segment[segmentFrame]) {
		return fmt.Errorf("vm error: frame capacity exceeded: need %d bytes, have %d", localBase+header.FrameByteSize, len(vm.Memory.segment[segmentFrame]))
	}
	for offset := localBase; offset < localBase+header.FrameByteSize; offset++ {
		vm.Memory.segment[segmentFrame][offset] = 0
	}
	for index, value := range args {
		if index >= len(header.ParamOffsets) {
			return fmt.Errorf("vm error: function at %d missing byte offset for arg %d", entryAddress, index)
		}
		kind := KindInt32
		if index < len(header.ParamKinds) && header.ParamKinds[index] != KindNone {
			kind = header.ParamKinds[index]
		}
		if err := vm.Memory.WriteBits(makeAddress(segmentFrame, localBase+header.ParamOffsets[index]), kind, value); err != nil {
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
	if frame.returnKind != KindNone {
		value, err := vm.popKind(frame.returnKind)
		if err != nil {
			return false, err
		}
		resultBits = value
	}
	vm.frameTop = frame.localBase
	if vm.callFrameTop == 0 {
		if frame.returnKind != KindNone {
			if err := vm.pushKind(frame.returnKind, resultBits); err != nil {
				return false, err
			}
		}
		return true, nil
	}
	vm.pc = frame.returnPC
	if frame.returnKind != KindNone {
		if err := vm.pushKind(frame.returnKind, resultBits); err != nil {
			return false, err
		}
	}
	return false, nil
}

func (vm *VM) pushKind(kind ValueKind, bits uint64) error {
	return appendStackBits(&vm.Memory.segment[segmentStack], kind, bits)
}

func (vm *VM) pushInt32(value int) {
	_ = appendStackBits(&vm.Memory.segment[segmentStack], KindInt32, uint64(uint32(int32(value))))
}

func (vm *VM) pushAddress(address Address) error {
	return appendAddress(&vm.Memory.segment[segmentStack], address)
}

func (vm *VM) popInt32() (int, error) {
	bits, err := truncateStackBits(&vm.Memory.segment[segmentStack], KindInt32)
	if err != nil {
		return 0, err
	}
	return int(int32(bits)), nil
}

func (vm *VM) popAddress() (Address, error) {
	bits, err := truncateStackBits(&vm.Memory.segment[segmentStack], KindAddress)
	if err != nil {
		return 0, err
	}
	return Address(uint32(bits)), nil
}

func (vm *VM) popKind(kind ValueKind) (uint64, error) {
	return truncateStackBits(&vm.Memory.segment[segmentStack], kind)
}

func (vm *VM) popBinary(kind ValueKind) (right uint64, left uint64, err error) {
	return popBinaryBits(&vm.Memory.segment[segmentStack], kind)
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

func addBits(kind ValueKind, left uint64, right uint64) uint64 {
	switch kind {
	case KindBool, KindByte, KindUint8, KindUint16, KindUint32, KindUint64:
		return left + right
	case KindInt8:
		return uint64(uint8(int8(left) + int8(right)))
	case KindInt16:
		return uint64(uint16(int16(left) + int16(right)))
	case KindInt64:
		return uint64(int64(left) + int64(right))
	case KindInt32:
		return uint64(uint32(int32(left) + int32(right)))
	case KindFloat32:
		return uint64(math.Float32bits(math.Float32frombits(uint32(left)) + math.Float32frombits(uint32(right))))
	case KindFloat64:
		return math.Float64bits(math.Float64frombits(left) + math.Float64frombits(right))
	default:
		return uint64(uint32(int32(left) + int32(right)))
	}
}

func subBits(kind ValueKind, left uint64, right uint64) uint64 {
	switch kind {
	case KindBool, KindByte, KindUint8, KindUint16, KindUint32, KindUint64:
		return left - right
	case KindInt8:
		return uint64(uint8(int8(left) - int8(right)))
	case KindInt16:
		return uint64(uint16(int16(left) - int16(right)))
	case KindInt64:
		return uint64(int64(left) - int64(right))
	case KindInt32:
		return uint64(uint32(int32(left) - int32(right)))
	case KindFloat32:
		return uint64(math.Float32bits(math.Float32frombits(uint32(left)) - math.Float32frombits(uint32(right))))
	case KindFloat64:
		return math.Float64bits(math.Float64frombits(left) - math.Float64frombits(right))
	default:
		return uint64(uint32(int32(left) - int32(right)))
	}
}

func mulBits(kind ValueKind, left uint64, right uint64) uint64 {
	switch kind {
	case KindBool, KindByte, KindUint8, KindUint16, KindUint32, KindUint64:
		return left * right
	case KindInt8:
		return uint64(uint8(int8(left) * int8(right)))
	case KindInt16:
		return uint64(uint16(int16(left) * int16(right)))
	case KindInt64:
		return uint64(int64(left) * int64(right))
	case KindInt32:
		return uint64(uint32(int32(left) * int32(right)))
	case KindFloat32:
		return uint64(math.Float32bits(math.Float32frombits(uint32(left)) * math.Float32frombits(uint32(right))))
	case KindFloat64:
		return math.Float64bits(math.Float64frombits(left) * math.Float64frombits(right))
	default:
		return uint64(uint32(int32(left) * int32(right)))
	}
}

func divBits(kind ValueKind, left uint64, right uint64) uint64 {
	switch kind {
	case KindBool, KindByte, KindUint8, KindUint16, KindUint32, KindUint64:
		return left / right
	case KindInt8:
		return uint64(uint8(int8(left) / int8(right)))
	case KindInt16:
		return uint64(uint16(int16(left) / int16(right)))
	case KindInt64:
		return uint64(int64(left) / int64(right))
	case KindInt32:
		return uint64(uint32(int32(left) / int32(right)))
	case KindFloat32:
		return uint64(math.Float32bits(math.Float32frombits(uint32(left)) / math.Float32frombits(uint32(right))))
	case KindFloat64:
		return math.Float64bits(math.Float64frombits(left) / math.Float64frombits(right))
	default:
		return uint64(uint32(int32(left) / int32(right)))
	}
}

func compareBits(kind ValueKind, left uint64, right uint64, op Opcode) int32 {
	switch kind {
	case KindFloat32:
		leftValue := float64(math.Float32frombits(uint32(left)))
		rightValue := float64(math.Float32frombits(uint32(right)))
		switch op {
		case OpEqual:
			if leftValue == rightValue {
				return 1
			}
		case OpNotEqual:
			if leftValue != rightValue {
				return 1
			}
		case OpLess:
			if leftValue < rightValue {
				return 1
			}
		case OpLessEqual:
			if leftValue <= rightValue {
				return 1
			}
		case OpGreater:
			if leftValue > rightValue {
				return 1
			}
		case OpGreaterEqual:
			if leftValue >= rightValue {
				return 1
			}
		}
	case KindFloat64:
		leftValue := math.Float64frombits(left)
		rightValue := math.Float64frombits(right)
		switch op {
		case OpEqual:
			if leftValue == rightValue {
				return 1
			}
		case OpNotEqual:
			if leftValue != rightValue {
				return 1
			}
		case OpLess:
			if leftValue < rightValue {
				return 1
			}
		case OpLessEqual:
			if leftValue <= rightValue {
				return 1
			}
		case OpGreater:
			if leftValue > rightValue {
				return 1
			}
		case OpGreaterEqual:
			if leftValue >= rightValue {
				return 1
			}
		}
	case KindInt8, KindInt16, KindInt32, KindInt64:
		leftValue := bitsToInt64(kind, left)
		rightValue := bitsToInt64(kind, right)
		switch op {
		case OpEqual:
			if leftValue == rightValue {
				return 1
			}
		case OpNotEqual:
			if leftValue != rightValue {
				return 1
			}
		case OpLess:
			if leftValue < rightValue {
				return 1
			}
		case OpLessEqual:
			if leftValue <= rightValue {
				return 1
			}
		case OpGreater:
			if leftValue > rightValue {
				return 1
			}
		case OpGreaterEqual:
			if leftValue >= rightValue {
				return 1
			}
		}
	case KindBool, KindByte, KindUint8, KindUint16, KindUint32, KindUint64, KindAddress:
		leftValue := bitsToUint64(kind, left)
		rightValue := bitsToUint64(kind, right)
		switch op {
		case OpEqual:
			if leftValue == rightValue {
				return 1
			}
		case OpNotEqual:
			if leftValue != rightValue {
				return 1
			}
		case OpLess:
			if leftValue < rightValue {
				return 1
			}
		case OpLessEqual:
			if leftValue <= rightValue {
				return 1
			}
		case OpGreater:
			if leftValue > rightValue {
				return 1
			}
		case OpGreaterEqual:
			if leftValue >= rightValue {
				return 1
			}
		}
	default:
		leftValue := bitsToInt64(kind, left)
		rightValue := bitsToInt64(kind, right)
		switch op {
		case OpEqual:
			if leftValue == rightValue {
				return 1
			}
		case OpNotEqual:
			if leftValue != rightValue {
				return 1
			}
		case OpLess:
			if leftValue < rightValue {
				return 1
			}
		case OpLessEqual:
			if leftValue <= rightValue {
				return 1
			}
		case OpGreater:
			if leftValue > rightValue {
				return 1
			}
		case OpGreaterEqual:
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
