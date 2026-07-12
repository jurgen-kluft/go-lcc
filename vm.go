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
	LastResultKind   ValueKind
	LastResultBits   uint64
	LastResult       int
}

func NewVM(globalCount, localCount int) *VM {
	return &VM{
		Memory:     NewProgramMemory(globalCount, 0, localCount, 32),
		callFrames: make([]callFrame, 0, 8),
	}
}

func (vm *VM) BindExtern(index int, target *int) error {
	if index < 0 || index+4 > len(vm.Memory.Extern) {
		return fmt.Errorf("vm error: extern byte offset %d out of range", index)
	}
	vm.StoreExternInt32(index, *target)
	return nil
}

func (vm *VM) BindExternBlock(block []byte) {
	vm.Memory.Extern = block
}

func (vm *VM) LoadExternInt32(offset int) int {
	if offset < 0 || offset+4 > len(vm.Memory.Extern) {
		return 0
	}
	bits, err := vm.Memory.ReadBits(makeAddress(segmentExtern, offset), KindInt32)
	if err != nil {
		return 0
	}
	return int(int32(bits))
}

func (vm *VM) StoreExternInt32(offset int, value int) {
	if offset < 0 || offset+4 > len(vm.Memory.Extern) {
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
	if len(vm.Memory.BSS) != program.BSSByteSize {
		vm.Memory.BSS = make([]byte, program.BSSByteSize)
	} else {
		for index := range vm.Memory.BSS {
			vm.Memory.BSS[index] = 0
		}
	}

	vm.program = program
	vm.pc = 0
	vm.Memory.Stack = vm.Memory.Stack[:0]
	vm.Memory.Frame = vm.Memory.Frame[:0]
	vm.callFrames = vm.callFrames[:0]
	vm.LastResultKind = KindNone
	vm.LastResultBits = 0
	vm.LastResult = 0

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
			if err := vm.pushKind(kind, vm.binaryOp(kind, left, right, func(a int64, b int64) int64 { return a + b }, func(a uint64, b uint64) uint64 { return a + b }, func(a float32, b float32) float32 { return a + b }, func(a float64, b float64) float64 { return a + b })); err != nil {
				return err
			}
		case OpSub:
			right, left, err := vm.popBinary(kind)
			if err != nil {
				return err
			}
			if err := vm.pushKind(kind, vm.binaryOp(kind, left, right, func(a int64, b int64) int64 { return a - b }, func(a uint64, b uint64) uint64 { return a - b }, func(a float32, b float32) float32 { return a - b }, func(a float64, b float64) float64 { return a - b })); err != nil {
				return err
			}
		case OpMul:
			right, left, err := vm.popBinary(kind)
			if err != nil {
				return err
			}
			if err := vm.pushKind(kind, vm.binaryOp(kind, left, right, func(a int64, b int64) int64 { return a * b }, func(a uint64, b uint64) uint64 { return a * b }, func(a float32, b float32) float32 { return a * b }, func(a float64, b float64) float64 { return a * b })); err != nil {
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
			if err := vm.pushKind(kind, vm.binaryOp(kind, left, right, func(a int64, b int64) int64 { return a / b }, func(a uint64, b uint64) uint64 { return a / b }, func(a float32, b float32) float32 { return a / b }, func(a float64, b float64) float64 { return a / b })); err != nil {
				return err
			}
		case OpConvert:
			fromKind := ValueKind(program.Text.ReadImmediate(&vm.pc, KindUint8))
			bits, err := vm.popKind(fromKind)
			if err != nil {
				return err
			}
			if err := vm.pushKind(kind, vm.convertBits(fromKind, kind, bits)); err != nil {
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
		case OpJumpIfFalse:
			target := program.Text.ReadInt(&vm.pc)
			condition, err := vm.popInt32()
			if err != nil {
				return err
			}
			if condition == 0 {
				vm.pc = target
			}
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
	if len(vm.callFrames) == 0 {
		return nil, fmt.Errorf("vm error: no active call frame")
	}
	return &vm.callFrames[len(vm.callFrames)-1], nil
}

func (vm *VM) enterScriptFunction(entryAddress int, args []uint64, returnPC int) error {
	header, err := vm.program.Text.ReadFunctionHeader(entryAddress)
	if err != nil {
		return err
	}
	if len(args) != header.ParamCount {
		return fmt.Errorf("vm error: function at %d expects %d args, got %d", entryAddress, header.ParamCount, len(args))
	}
	localBase := len(vm.Memory.Frame)
	for offset := 0; offset < header.FrameByteSize; offset++ {
		vm.Memory.Frame = append(vm.Memory.Frame, 0)
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
	vm.callFrames = append(vm.callFrames, callFrame{returnPC: returnPC, localBase: localBase, returnKind: header.ReturnKind})
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
	if len(vm.callFrames) == 0 {
		return false, fmt.Errorf("vm error: return without active frame")
	}
	frame := vm.callFrames[len(vm.callFrames)-1]
	vm.callFrames = vm.callFrames[:len(vm.callFrames)-1]
	result := 0
	if frame.returnKind != KindNone {
		value, err := vm.popKind(frame.returnKind)
		if err != nil {
			return false, err
		}
		vm.LastResultBits = value
		result = vm.bitsToInt(frame.returnKind, value)
	}
	vm.Memory.Frame = vm.Memory.Frame[:frame.localBase]
	if len(vm.callFrames) == 0 {
		vm.LastResultKind = frame.returnKind
		vm.LastResult = result
		return true, nil
	}
	vm.pc = frame.returnPC
	if frame.returnKind != KindNone {
		if err := vm.pushKind(frame.returnKind, vm.LastResultBits); err != nil {
			return false, err
		}
	}
	return false, nil
}

func (vm *VM) pushKind(kind ValueKind, bits uint64) error {
	return vm.Memory.Stack.AppendBits(kind, bits)
}

func (vm *VM) pushInt32(value int) {
	_ = vm.Memory.Stack.AppendBits(KindInt32, uint64(uint32(int32(value))))
}

func (vm *VM) pushAddress(address Address) error {
	return vm.Memory.Stack.AppendBits(KindAddress, uint64(uint32(address)))
}

func (vm *VM) popInt32() (int, error) {
	bits, err := vm.Memory.Stack.TruncateBits(KindInt32)
	if err != nil {
		return 0, err
	}
	return int(int32(bits)), nil
}

func (vm *VM) popAddress() (Address, error) {
	bits, err := vm.Memory.Stack.TruncateBits(KindAddress)
	if err != nil {
		return 0, err
	}
	return Address(uint32(bits)), nil
}

func (vm *VM) popKind(kind ValueKind) (uint64, error) {
	return vm.Memory.Stack.TruncateBits(kind)
}

func (vm *VM) popBinaryInt32(kind ValueKind) (right int, left int, err error) {
	if kind != KindInt32 {
		return 0, 0, fmt.Errorf("vm error: unsupported arithmetic kind %d", kind)
	}
	right, err = vm.popInt32()
	if err != nil {
		return 0, 0, err
	}
	left, err = vm.popInt32()
	if err != nil {
		return 0, 0, err
	}
	return right, left, nil
}

func (vm *VM) popBinary(kind ValueKind) (right uint64, left uint64, err error) {
	right, err = vm.popKind(kind)
	if err != nil {
		return 0, 0, err
	}
	left, err = vm.popKind(kind)
	if err != nil {
		return 0, 0, err
	}
	return right, left, nil
}

func (vm *VM) intToBits(kind ValueKind, value int) uint64 {
	switch kind {
	case KindBool:
		if value != 0 {
			return 1
		}
		return 0
	case KindByte, KindUint8:
		return uint64(uint8(value))
	case KindInt8:
		return uint64(uint8(int8(value)))
	case KindInt16:
		return uint64(uint16(int16(value)))
	case KindUint16:
		return uint64(uint16(value))
	case KindInt32:
		return uint64(uint32(int32(value)))
	case KindUint32, KindAddress:
		return uint64(uint32(value))
	case KindInt64, KindUint64:
		return uint64(value)
	default:
		return uint64(uint32(int32(value)))
	}
}

func (vm *VM) bitsToInt(kind ValueKind, bits uint64) int {
	switch kind {
	case KindBool:
		if bits != 0 {
			return 1
		}
		return 0
	case KindByte, KindUint8:
		return int(uint8(bits))
	case KindInt8:
		return int(int8(bits))
	case KindInt16:
		return int(int16(bits))
	case KindUint16:
		return int(uint16(bits))
	case KindInt32:
		return int(int32(bits))
	case KindUint32, KindAddress:
		return int(uint32(bits))
	case KindInt64:
		return int(int64(bits))
	case KindUint64:
		return int(bits)
	default:
		return int(int32(bits))
	}
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

func (vm *VM) binaryOp(kind ValueKind, left uint64, right uint64, signed func(int64, int64) int64, unsigned func(uint64, uint64) uint64, float32op func(float32, float32) float32, float64op func(float64, float64) float64) uint64 {
	switch kind {
	case KindBool, KindByte, KindUint8, KindUint16, KindUint32, KindUint64:
		return unsigned(left, right)
	case KindInt8:
		return uint64(uint8(int8(signed(int64(int8(left)), int64(int8(right))))))
	case KindInt16:
		return uint64(uint16(int16(signed(int64(int16(left)), int64(int16(right))))))
	case KindInt64:
		return uint64(signed(int64(left), int64(right)))
	case KindInt32:
		return uint64(uint32(int32(signed(int64(int32(left)), int64(int32(right))))))
	case KindFloat32:
		return uint64(math.Float32bits(float32op(math.Float32frombits(uint32(left)), math.Float32frombits(uint32(right)))))
	case KindFloat64:
		return math.Float64bits(float64op(math.Float64frombits(left), math.Float64frombits(right)))
	default:
		return uint64(uint32(int32(signed(int64(int32(left)), int64(int32(right))))))
	}
}

func (vm *VM) convertBits(from ValueKind, to ValueKind, bits uint64) uint64 {
	if from == to {
		return bits
	}
	switch to {
	case KindFloat32:
		return uint64(math.Float32bits(float32(vm.numericToFloat64(from, bits))))
	case KindFloat64:
		return math.Float64bits(vm.numericToFloat64(from, bits))
	case KindBool:
		if vm.isZero(from, bits) {
			return 0
		}
		return 1
	case KindByte, KindUint8:
		return uint64(uint8(vm.numericToUint64(from, bits)))
	case KindInt8:
		return uint64(uint8(int8(vm.numericToInt64(from, bits))))
	case KindInt16:
		return uint64(uint16(int16(vm.numericToInt64(from, bits))))
	case KindUint16:
		return uint64(uint16(vm.numericToUint64(from, bits)))
	case KindInt32:
		return uint64(uint32(int32(vm.numericToInt64(from, bits))))
	case KindUint32, KindAddress:
		return uint64(uint32(vm.numericToUint64(from, bits)))
	case KindInt64:
		return uint64(vm.numericToInt64(from, bits))
	case KindUint64:
		return vm.numericToUint64(from, bits)
	default:
		return bits
	}
}

func (vm *VM) numericToFloat64(kind ValueKind, bits uint64) float64 {
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
		return float64(vm.numericToUint64(kind, bits))
	default:
		return float64(vm.numericToInt64(kind, bits))
	}
}

func (vm *VM) numericToInt64(kind ValueKind, bits uint64) int64 {
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

func (vm *VM) numericToUint64(kind ValueKind, bits uint64) uint64 {
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
