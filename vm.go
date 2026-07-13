package cova

import "math"

type callFrame struct {
	returnPC   int
	localBase  int
	returnKind ValueKind
}

type VM struct {
	memory           ProgramMemory
	pc               int
	program          *LinkedProgram
	externDispatcher ExternDispatcherBinding
	callFrames       []callFrame
	callFrameTop     int
	frameTop         int
	fault            VMFaultInfo
	instructionPC    int
}

type VMConfig struct {
	FrameCapacity     int
	StackCapacity     int
	CallFrameCapacity int
}

func NewVM(frameCapacity int) *VM {
	return NewVMWithConfig(VMConfig{
		FrameCapacity:     frameCapacity,
		StackCapacity:     256,
		CallFrameCapacity: 8,
	})
}

func NewVMWithConfig(config VMConfig) *VM {
	if config.FrameCapacity < 0 {
		config.FrameCapacity = 0
	}
	if config.StackCapacity < 0 {
		config.StackCapacity = 0
	}
	if config.CallFrameCapacity < 1 {
		config.CallFrameCapacity = 1
	}
	return &VM{
		memory:        NewProgramMemory(0, 0, 0, 0, config.FrameCapacity, config.StackCapacity),
		callFrames:    make([]callFrame, config.CallFrameCapacity),
		fault:         noVMFault(),
		instructionPC: -1,
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
	bits, status := vm.memory.ReadBits(makeAddress(segmentExtern, offset), KindInt32)
	if status != VMStatusOK {
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

func (vm *VM) RegisterExternDispatcher(hostContext uintptr, dispatcher ExternDispatcher) {
	vm.externDispatcher = ExternDispatcherBinding{HostContext: hostContext, Dispatcher: dispatcher}
}

func (vm *VM) FaultInfo() VMFaultInfo {
	return vm.fault
}

func (vm *VM) PushBits(kind ValueKind, bits uint64) VMStatus {
	return vm.pushKind(kind, bits)
}

func (vm *VM) PushBool(value bool) VMStatus {
	if value {
		return vm.pushKind(KindBool, 1)
	}
	return vm.pushKind(KindBool, 0)
}

func (vm *VM) PushByte(value byte) VMStatus {
	return vm.pushKind(KindByte, uint64(value))
}

func (vm *VM) PushInt8(value int8) VMStatus {
	return vm.pushKind(KindInt8, uint64(uint8(value)))
}

func (vm *VM) PushInt16(value int16) VMStatus {
	return vm.pushKind(KindInt16, uint64(uint16(value)))
}

func (vm *VM) PushInt32(value int32) VMStatus {
	return vm.pushKind(KindInt32, uint64(uint32(value)))
}

func (vm *VM) PushInt64(value int64) VMStatus {
	return vm.pushKind(KindInt64, uint64(value))
}

func (vm *VM) PushUint8(value uint8) VMStatus {
	return vm.pushKind(KindUint8, uint64(value))
}

func (vm *VM) PushUint16(value uint16) VMStatus {
	return vm.pushKind(KindUint16, uint64(value))
}

func (vm *VM) PushUint32(value uint32) VMStatus {
	return vm.pushKind(KindUint32, uint64(value))
}

func (vm *VM) PushUint64(value uint64) VMStatus {
	return vm.pushKind(KindUint64, value)
}

func (vm *VM) PushFloat32(value float32) VMStatus {
	return vm.pushKind(KindFloat32, uint64(math.Float32bits(value)))
}

func (vm *VM) PushFloat64(value float64) VMStatus {
	return vm.pushKind(KindFloat64, math.Float64bits(value))
}

func (vm *VM) PopBits(kind ValueKind) (uint64, VMStatus) {
	return vm.popKind(kind)
}

func (vm *VM) PopBool() (bool, VMStatus) {
	bits, status := vm.popKind(KindBool)
	return bits != 0, status
}

func (vm *VM) PopByte() (byte, VMStatus) {
	bits, status := vm.popKind(KindByte)
	return byte(bits), status
}

func (vm *VM) PopInt8() (int8, VMStatus) {
	bits, status := vm.popKind(KindInt8)
	return int8(bits), status
}

func (vm *VM) PopInt16() (int16, VMStatus) {
	bits, status := vm.popKind(KindInt16)
	return int16(bits), status
}

func (vm *VM) PopInt32() (int32, VMStatus) {
	bits, status := vm.popKind(KindInt32)
	return int32(bits), status
}

func (vm *VM) PopInt64() (int64, VMStatus) {
	bits, status := vm.popKind(KindInt64)
	return int64(bits), status
}

func (vm *VM) PopUint8() (uint8, VMStatus) {
	bits, status := vm.popKind(KindUint8)
	return uint8(bits), status
}

func (vm *VM) PopUint16() (uint16, VMStatus) {
	bits, status := vm.popKind(KindUint16)
	return uint16(bits), status
}

func (vm *VM) PopUint32() (uint32, VMStatus) {
	bits, status := vm.popKind(KindUint32)
	return uint32(bits), status
}

func (vm *VM) PopUint64() (uint64, VMStatus) {
	return vm.popKind(KindUint64)
}

func (vm *VM) PopFloat32() (float32, VMStatus) {
	bits, status := vm.popKind(KindFloat32)
	return math.Float32frombits(uint32(bits)), status
}

func (vm *VM) PopFloat64() (float64, VMStatus) {
	bits, status := vm.popKind(KindFloat64)
	return math.Float64frombits(bits), status
}

func (vm *VM) Run(program *LinkedProgram) VMStatus {
	if status := vm.LoadProgram(program); status != VMStatusOK {
		return status
	}
	return vm.RunLoaded()
}

func (vm *VM) LoadProgram(program *LinkedProgram) VMStatus {
	vm.clearFault()
	if program == nil {
		return vm.fail(VMStatusInvalidProgram, -1, -1, -1)
	}
	if program.BSSByteSize < 0 {
		return vm.fail(VMStatusInvalidProgram, program.BSSByteSize, 0, -1)
	}
	if program.ConstByteSize < 0 || program.ConstByteSize != len(program.ConstData) {
		return vm.fail(VMStatusInvalidImage, -1, program.ConstByteSize, len(program.ConstData))
	}
	if program.DataByteSize < 0 || program.DataByteSize != len(program.DataData) {
		return vm.fail(VMStatusInvalidImage, -1, program.DataByteSize, len(program.DataData))
	}
	if len(vm.memory.segment[segmentBSS]) != program.BSSByteSize {
		vm.memory.segment[segmentBSS] = make([]byte, program.BSSByteSize)
	}
	if len(vm.memory.segment[segmentData]) != program.DataByteSize {
		vm.memory.segment[segmentData] = make([]byte, program.DataByteSize)
	}
	vm.memory.segment[segmentConst] = MemorySegment(program.ConstData)
	vm.program = program
	return VMStatusOK
}

func (vm *VM) Reset() VMStatus {
	vm.clearFault()
	if vm.program == nil {
		return vm.fail(VMStatusNoProgramLoaded, -1, -1, -1)
	}
	program := vm.program
	for index := range vm.memory.segment[segmentBSS] {
		vm.memory.segment[segmentBSS][index] = 0
	}
	copy(vm.memory.segment[segmentData], program.DataData)
	vm.pc = 0
	vm.memory.segment[segmentStack] = vm.memory.segment[segmentStack][:0]
	for index := range vm.memory.segment[segmentFrame] {
		vm.memory.segment[segmentFrame][index] = 0
	}
	vm.callFrameTop = 0
	vm.frameTop = 0

	if program.EntryPoint < 0 || program.EntryPoint >= len(program.Functions) {
		return vm.fail(VMStatusInvalidTarget, program.EntryPoint, len(program.Functions), len(program.Functions))
	}
	if program.Functions[program.EntryPoint].ParamCount != 0 {
		return vm.fail(VMStatusInvalidDescriptor, program.EntryPoint, 0, program.Functions[program.EntryPoint].ParamCount)
	}
	if status := vm.enterScriptFunction(program.EntryPoint, -1, false); status != VMStatusOK {
		return vm.recordStatus(status)
	}
	return VMStatusOK
}

func (vm *VM) RunLoaded() VMStatus {
	if status := vm.Reset(); status != VMStatusOK {
		return status
	}
	program := vm.program
	for vm.pc < len(program.Text) {
		vm.instructionPC = vm.pc
		instruction, status := program.Text.ReadInstructionChecked(&vm.pc)
		if status != VMStatusOK {
			return vm.recordStatus(status)
		}
		op := instruction.Opcode()

		switch op {
		case OpPush:
			kind := instruction.Kind()
			if kind == KindNone || kind == KindAddress {
				return vm.fail(VMStatusInvalidValueKind, int(kind), -1, -1)
			}
			immediate, status := program.Text.ReadImmediateChecked(&vm.pc, kind)
			if status != VMStatusOK {
				return vm.recordStatus(status)
			}
			if status = vm.pushKind(kind, immediate); status != VMStatusOK {
				return vm.recordStatus(status)
			}
		case OpArithmetic:
			kind := instruction.Kind()
			arithmeticOp := instruction.ArithmeticOp()
			right, left, status := vm.popBinary(kind)
			if status != VMStatusOK {
				return vm.recordStatus(status)
			}
			result, status := vm.executeArithmetic(kind, arithmeticOp, left, right)
			if status != VMStatusOK {
				return vm.recordStatus(status)
			}
			if status = vm.pushKind(kind, result); status != VMStatusOK {
				return vm.recordStatus(status)
			}
		case OpConvert:
			fromKind := instruction.ConvertFromKind()
			bits, status := vm.popKind(fromKind)
			if status != VMStatusOK {
				return vm.recordStatus(status)
			}
			kind := instruction.Kind()
			if status = vm.pushKind(kind, convertBits(fromKind, kind, bits)); status != VMStatusOK {
				return vm.recordStatus(status)
			}
		case OpAddr:
			segment := instruction.AddressSegment()
			offset, status := program.Text.ReadIntChecked(&vm.pc)
			if status != VMStatusOK {
				return vm.recordStatus(status)
			}
			if segment == segmentFrame {
				frame, frameStatus := vm.currentFrame()
				if frameStatus != VMStatusOK {
					return vm.recordStatus(frameStatus)
				}
				offset += frame.localBase
			}
			if status = vm.pushAddress(makeAddress(segment, offset)); status != VMStatusOK {
				return vm.recordStatus(status)
			}
		case OpOffset:
			offset, status := vm.popInt32()
			if status != VMStatusOK {
				return vm.recordStatus(status)
			}
			base, status := vm.popAddress()
			if status != VMStatusOK {
				return vm.recordStatus(status)
			}
			if status = vm.pushAddress(makeAddress(base.Segment(), base.Index()+offset)); status != VMStatusOK {
				return vm.recordStatus(status)
			}
		case OpDereference:
			kind := instruction.Kind()
			if kind == KindNone {
				return vm.fail(VMStatusInvalidValueKind, int(kind), -1, -1)
			}
			encodedAddress, status := vm.popAddress()
			if status != VMStatusOK {
				return vm.recordStatus(status)
			}
			value, status := vm.memory.ReadBits(encodedAddress, kind)
			if status != VMStatusOK {
				return vm.fail(status, int(encodedAddress), -1, -1)
			}
			if status = vm.pushKind(kind, value); status != VMStatusOK {
				return vm.recordStatus(status)
			}
		case OpAssign:
			kind := instruction.Kind()
			if kind == KindNone {
				return vm.fail(VMStatusInvalidValueKind, int(kind), -1, -1)
			}
			encodedAddress, status := vm.popAddress()
			if status != VMStatusOK {
				return vm.recordStatus(status)
			}
			value, status := vm.popKind(kind)
			if status != VMStatusOK {
				return vm.recordStatus(status)
			}
			if status = vm.memory.WriteBits(encodedAddress, kind, value); status != VMStatusOK {
				return vm.fail(status, int(encodedAddress), -1, -1)
			}
		case OpCompare:
			kind := instruction.Kind()
			compareOp := instruction.CompareOp()
			right, left, status := vm.popBinary(kind)
			if status != VMStatusOK {
				return vm.recordStatus(status)
			}
			result := vm.executeComparison(kind, compareOp, left, right)
			if status = vm.PushInt32(result); status != VMStatusOK {
				return vm.recordStatus(status)
			}
		case OpJumpIfFalse:
			target, status := program.Text.ReadIntChecked(&vm.pc)
			if status != VMStatusOK {
				return vm.recordStatus(status)
			}
			condition, status := vm.popInt32()
			if status != VMStatusOK {
				return vm.recordStatus(status)
			}
			if condition == 0 {
				if target < 0 || target >= len(program.Text) {
					return vm.fail(VMStatusInvalidTarget, target, len(program.Text), len(program.Text))
				}
				vm.pc = target
			}
		case OpJump:
			target, status := program.Text.ReadIntChecked(&vm.pc)
			if status != VMStatusOK {
				return vm.recordStatus(status)
			}
			if target < 0 || target >= len(program.Text) {
				return vm.fail(VMStatusInvalidTarget, target, len(program.Text), len(program.Text))
			}
			vm.pc = target
		case OpCall:
			target, status := program.Text.ReadIntChecked(&vm.pc)
			if status != VMStatusOK {
				return vm.recordStatus(status)
			}
			if status = vm.callScriptFunction(target); status != VMStatusOK {
				return vm.recordStatus(status)
			}
		case OpCallExtern:
			importID, status := program.Text.ReadIntChecked(&vm.pc)
			if status != VMStatusOK {
				return vm.recordStatus(status)
			}
			if status = vm.callExtern(importID); status != VMStatusOK {
				return vm.recordStatus(status)
			}
		case OpRet:
			done, status := vm.returnFromFunction()
			if status != VMStatusOK {
				return vm.recordStatus(status)
			}
			if done {
				return VMStatusOK
			}
		default:
			return vm.fail(VMStatusInvalidOpcode, int(op), -1, -1)
		}
	}

	return VMStatusOK
}

func (vm *VM) clearFault() {
	vm.fault = noVMFault()
	vm.instructionPC = -1
}

func (vm *VM) fail(status VMStatus, target, required, available int) VMStatus {
	vm.fault = VMFaultInfo{
		Status:     status,
		PC:         vm.instructionPC,
		Target:     target,
		Required:   required,
		Available:  available,
		HostStatus: VMStatusOK,
	}
	return status
}

func (vm *VM) recordStatus(status VMStatus) VMStatus {
	if status != VMStatusOK && vm.fault.Status == VMStatusOK {
		return vm.fail(status, -1, -1, -1)
	}
	return status
}

func (vm *VM) currentFrame() (*callFrame, VMStatus) {
	if vm.callFrameTop == 0 {
		return nil, VMStatusInvalidLifecycle
	}
	return &vm.callFrames[vm.callFrameTop-1], VMStatusOK
}

func (vm *VM) enterScriptFunction(functionIndex int, returnPC int, popArgs bool) VMStatus {
	if functionIndex < 0 || functionIndex >= len(vm.program.Functions) {
		return vm.fail(VMStatusInvalidTarget, functionIndex, len(vm.program.Functions), len(vm.program.Functions))
	}
	function := vm.program.Functions[functionIndex]
	if function.BodyAddress < 0 || function.BodyAddress >= len(vm.program.Text) {
		return vm.fail(VMStatusInvalidDescriptor, functionIndex, len(vm.program.Text), function.BodyAddress)
	}
	if function.ParamStart < 0 || function.ParamCount < 0 || function.ParamStart > len(vm.program.ParamKinds)-function.ParamCount || function.ParamStart > len(vm.program.ParamOffsets)-function.ParamCount {
		return vm.fail(VMStatusInvalidParameter, functionIndex, function.ParamCount, len(vm.program.ParamKinds))
	}
	if function.FrameByteSize < 0 {
		return vm.fail(VMStatusInvalidDescriptor, functionIndex, 0, function.FrameByteSize)
	}
	argumentBytes := 0
	for index := 0; index < function.ParamCount; index++ {
		kind := vm.program.ParamKinds[function.ParamStart+index]
		size := kind.Size()
		if size == 0 {
			return vm.fail(VMStatusInvalidParameter, index, -1, int(kind))
		}
		offset := vm.program.ParamOffsets[function.ParamStart+index]
		if offset < 0 || offset > function.FrameByteSize-size {
			return vm.fail(VMStatusInvalidParameter, index, function.FrameByteSize, offset+size)
		}
		argumentBytes += size
	}
	if popArgs && argumentBytes > len(vm.memory.segment[segmentStack]) {
		return vm.fail(VMStatusStackUnderflow, functionIndex, argumentBytes, len(vm.memory.segment[segmentStack]))
	}
	localBase := vm.frameTop
	if localBase+function.FrameByteSize > len(vm.memory.segment[segmentFrame]) {
		return vm.fail(VMStatusFrameOverflow, functionIndex, localBase+function.FrameByteSize, len(vm.memory.segment[segmentFrame]))
	}
	if vm.callFrameTop >= len(vm.callFrames) {
		return vm.fail(VMStatusCallFrameOverflow, functionIndex, vm.callFrameTop+1, len(vm.callFrames))
	}
	for offset := localBase; offset < localBase+function.FrameByteSize; offset++ {
		vm.memory.segment[segmentFrame][offset] = 0
	}
	if popArgs {
		for index := function.ParamCount - 1; index >= 0; index-- {
			paramIndex := function.ParamStart + index
			kind := vm.program.ParamKinds[paramIndex]
			value, status := vm.popKind(kind)
			if status != VMStatusOK {
				return status
			}
			if status = vm.memory.WriteBits(makeAddress(segmentFrame, localBase+vm.program.ParamOffsets[paramIndex]), kind, value); status != VMStatusOK {
				return status
			}
		}
	}
	vm.frameTop = localBase + function.FrameByteSize
	vm.callFrames[vm.callFrameTop] = callFrame{returnPC: returnPC, localBase: localBase, returnKind: function.ReturnKind}
	vm.callFrameTop++
	vm.pc = function.BodyAddress
	return VMStatusOK
}

func (vm *VM) callScriptFunction(functionIndex int) VMStatus {
	return vm.enterScriptFunction(functionIndex, vm.pc, true)
}

func (vm *VM) callExtern(importID int) VMStatus {
	if importID < 0 {
		return vm.fail(VMStatusInvalidTarget, importID, 0, importID)
	}
	if vm.externDispatcher.Dispatcher == nil {
		return vm.fail(VMStatusMissingExtern, importID, -1, -1)
	}
	hostStatus := vm.externDispatcher.Dispatcher(vm.externDispatcher.HostContext, vm, uint32(importID))
	if hostStatus != VMStatusOK {
		vm.fault = VMFaultInfo{
			Status:     VMStatusHostFailure,
			PC:         vm.instructionPC,
			Target:     importID,
			Required:   -1,
			Available:  -1,
			HostStatus: hostStatus,
		}
		return VMStatusHostFailure
	}
	return VMStatusOK
}

func (vm *VM) returnFromFunction() (bool, VMStatus) {
	if vm.callFrameTop == 0 {
		return false, VMStatusInvalidLifecycle
	}
	vm.callFrameTop--
	frame := vm.callFrames[vm.callFrameTop]
	var resultBits uint64
	if hasStackValueKind(frame.returnKind) {
		value, status := vm.popKind(frame.returnKind)
		if status != VMStatusOK {
			return false, status
		}
		resultBits = value
	}
	vm.frameTop = frame.localBase
	if vm.callFrameTop == 0 {
		if hasStackValueKind(frame.returnKind) {
			if status := vm.pushKind(frame.returnKind, resultBits); status != VMStatusOK {
				return false, status
			}
		}
		return true, VMStatusOK
	}
	vm.pc = frame.returnPC
	if hasStackValueKind(frame.returnKind) {
		if status := vm.pushKind(frame.returnKind, resultBits); status != VMStatusOK {
			return false, status
		}
	}
	return false, VMStatusOK
}

func hasStackValueKind(kind ValueKind) bool {
	return kind != KindNone && kind != KindVoid
}

func (vm *VM) pushKind(kind ValueKind, bits uint64) VMStatus {
	status := appendStackBits(&vm.memory.segment[segmentStack], kind, bits)
	if status == VMStatusStackOverflow {
		return vm.fail(status, -1, len(vm.memory.segment[segmentStack])+kind.Size(), cap(vm.memory.segment[segmentStack]))
	}
	return vm.recordStatus(status)
}

func (vm *VM) pushInt32(value int) {
	_ = appendStackBits(&vm.memory.segment[segmentStack], KindInt32, uint64(uint32(int32(value))))
}

func (vm *VM) pushAddress(address Address) VMStatus {
	return appendAddress(&vm.memory.segment[segmentStack], address)
}

func (vm *VM) popInt32() (int, VMStatus) {
	bits, status := truncateStackBits(&vm.memory.segment[segmentStack], KindInt32)
	if status != VMStatusOK {
		return 0, vm.recordStatus(status)
	}
	return int(int32(bits)), VMStatusOK
}

func (vm *VM) popAddress() (Address, VMStatus) {
	bits, status := truncateStackBits(&vm.memory.segment[segmentStack], KindAddress)
	if status != VMStatusOK {
		return 0, vm.recordStatus(status)
	}
	return Address(uint32(bits)), VMStatusOK
}

func (vm *VM) popKind(kind ValueKind) (uint64, VMStatus) {
	bits, status := truncateStackBits(&vm.memory.segment[segmentStack], kind)
	if status != VMStatusOK {
		return 0, vm.recordStatus(status)
	}
	return bits, VMStatusOK
}

func (vm *VM) popBinary(kind ValueKind) (right uint64, left uint64, status VMStatus) {
	right, left, status = popBinaryBits(&vm.memory.segment[segmentStack], kind)
	if status != VMStatusOK {
		status = vm.recordStatus(status)
	}
	return right, left, status
}

func appendStackBits(stack *MemorySegment, kind ValueKind, bits uint64) VMStatus {
	return stack.AppendBits(kind, bits)
}

func truncateStackBits(stack *MemorySegment, kind ValueKind) (uint64, VMStatus) {
	return stack.TruncateBits(kind)
}

func appendAddress(stack *MemorySegment, address Address) VMStatus {
	return appendStackBits(stack, KindAddress, uint64(uint32(address)))
}

func popBinaryBits(stack *MemorySegment, kind ValueKind) (right uint64, left uint64, status VMStatus) {
	right, status = truncateStackBits(stack, kind)
	if status != VMStatusOK {
		return 0, 0, status
	}
	left, status = truncateStackBits(stack, kind)
	if status != VMStatusOK {
		return 0, 0, status
	}
	return right, left, VMStatusOK
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

func (vm *VM) executeArithmetic(kind ValueKind, op ArithmeticOp, left uint64, right uint64) (uint64, VMStatus) {
	switch kind {
	case KindBool, KindByte, KindUint8, KindUint16, KindUint32, KindUint64:
		switch op {
		case ArithmeticAdd:
			return left + right, VMStatusOK
		case ArithmeticSub:
			return left - right, VMStatusOK
		case ArithmeticMul:
			return left * right, VMStatusOK
		case ArithmeticDiv:
			if vm.isZero(kind, right) {
				return 0, VMStatusDivisionByZero
			}
			return left / right, VMStatusOK
		}
	case KindInt8:
		leftValue, rightValue := int8(left), int8(right)
		switch op {
		case ArithmeticAdd:
			return uint64(uint8(leftValue + rightValue)), VMStatusOK
		case ArithmeticSub:
			return uint64(uint8(leftValue - rightValue)), VMStatusOK
		case ArithmeticMul:
			return uint64(uint8(leftValue * rightValue)), VMStatusOK
		case ArithmeticDiv:
			if vm.isZero(kind, right) {
				return 0, VMStatusDivisionByZero
			}
			return uint64(uint8(leftValue / rightValue)), VMStatusOK
		}
	case KindInt16:
		leftValue, rightValue := int16(left), int16(right)
		switch op {
		case ArithmeticAdd:
			return uint64(uint16(leftValue + rightValue)), VMStatusOK
		case ArithmeticSub:
			return uint64(uint16(leftValue - rightValue)), VMStatusOK
		case ArithmeticMul:
			return uint64(uint16(leftValue * rightValue)), VMStatusOK
		case ArithmeticDiv:
			if vm.isZero(kind, right) {
				return 0, VMStatusDivisionByZero
			}
			return uint64(uint16(leftValue / rightValue)), VMStatusOK
		}
	case KindInt64:
		leftValue, rightValue := int64(left), int64(right)
		switch op {
		case ArithmeticAdd:
			return uint64(leftValue + rightValue), VMStatusOK
		case ArithmeticSub:
			return uint64(leftValue - rightValue), VMStatusOK
		case ArithmeticMul:
			return uint64(leftValue * rightValue), VMStatusOK
		case ArithmeticDiv:
			if vm.isZero(kind, right) {
				return 0, VMStatusDivisionByZero
			}
			return uint64(leftValue / rightValue), VMStatusOK
		}
	case KindInt32:
		leftValue, rightValue := int32(left), int32(right)
		switch op {
		case ArithmeticAdd:
			return uint64(uint32(leftValue + rightValue)), VMStatusOK
		case ArithmeticSub:
			return uint64(uint32(leftValue - rightValue)), VMStatusOK
		case ArithmeticMul:
			return uint64(uint32(leftValue * rightValue)), VMStatusOK
		case ArithmeticDiv:
			if vm.isZero(kind, right) {
				return 0, VMStatusDivisionByZero
			}
			return uint64(uint32(leftValue / rightValue)), VMStatusOK
		}
	case KindFloat32:
		leftValue := math.Float32frombits(uint32(left))
		rightValue := math.Float32frombits(uint32(right))
		switch op {
		case ArithmeticAdd:
			return uint64(math.Float32bits(leftValue + rightValue)), VMStatusOK
		case ArithmeticSub:
			return uint64(math.Float32bits(leftValue - rightValue)), VMStatusOK
		case ArithmeticMul:
			return uint64(math.Float32bits(leftValue * rightValue)), VMStatusOK
		case ArithmeticDiv:
			if vm.isZero(kind, right) {
				return 0, VMStatusDivisionByZero
			}
			return uint64(math.Float32bits(leftValue / rightValue)), VMStatusOK
		}
	case KindFloat64:
		leftValue := math.Float64frombits(left)
		rightValue := math.Float64frombits(right)
		switch op {
		case ArithmeticAdd:
			return math.Float64bits(leftValue + rightValue), VMStatusOK
		case ArithmeticSub:
			return math.Float64bits(leftValue - rightValue), VMStatusOK
		case ArithmeticMul:
			return math.Float64bits(leftValue * rightValue), VMStatusOK
		case ArithmeticDiv:
			if vm.isZero(kind, right) {
				return 0, VMStatusDivisionByZero
			}
			return math.Float64bits(leftValue / rightValue), VMStatusOK
		}
	default:
		// unhandled kind, treat as error
	}

	return 0, VMStatusInvalidOpcode
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
