package lcc

import (
	"encoding/binary"
	"fmt"
)

type InstructionMode byte

const (
	ModeNone InstructionMode = iota
	ModeImplicit
	ModeExtend
	ModeReserved
)

type InstructionFlag byte

const (
	FlagNone   InstructionFlag = 0
	FlagSigned InstructionFlag = 1 << iota
	FlagReserved
)

type Instruction uint16

func makeInstruction(op Opcode, kind ValueKind, mode InstructionMode, flags InstructionFlag) Instruction {
	return Instruction(uint16(op&0x3f) | uint16(kind&0x0f)<<6 | uint16(mode&0x03)<<10 | uint16(flags&0x0f)<<12)
}

func makeArithmeticInstruction(kind ValueKind, op ArithmeticOp) Instruction {
	return Instruction(uint16(OpArithmetic&0x3f) | uint16(kind&0x0f)<<6 | uint16(op&0x3f)<<10)
}

func makeAddrInstruction(segment memorySegment) Instruction {
	return Instruction(uint16(OpAddr&0x3f) | uint16(byte(segment))<<6)
}

func makeCompareInstruction(kind ValueKind, op CompareOp) Instruction {
	return Instruction(uint16(OpCompare&0x3f) | uint16(kind&0x0f)<<6 | uint16(op&0x3f)<<10)
}

func makeConvertInstruction(from ValueKind, to ValueKind) Instruction {
	return Instruction(uint16(OpConvert&0x3f) | uint16(to&0x0f)<<6 | uint16(from&0x0f)<<10)
}

func (instruction Instruction) Opcode() Opcode {
	return Opcode(byte(instruction & 0x3f))
}

func (instruction Instruction) Kind() ValueKind {
	return ValueKind((instruction >> 6) & 0x0f)
}

func (instruction Instruction) LegacyMode() InstructionMode {
	return InstructionMode((instruction >> 10) & 0x03)
}

func (instruction Instruction) LegacyFlags() InstructionFlag {
	return InstructionFlag((instruction >> 12) & 0x0f)
}

func (instruction Instruction) ArithmeticOp() ArithmeticOp {
	return ArithmeticOp((instruction >> 10) & 0x3f)
}

func (instruction Instruction) AddressSegment() memorySegment {
	return memorySegment(byte((instruction >> 6) & 0x03ff))
}

func (instruction Instruction) CompareOp() CompareOp {
	return CompareOp((instruction >> 10) & 0x3f)
}

func (instruction Instruction) ConvertFromKind() ValueKind {
	return ValueKind((instruction >> 10) & 0x0f)
}

type CodeMemory []byte

func (code CodeMemory) Clone() CodeMemory {
	if len(code) == 0 {
		return nil
	}
	return append(CodeMemory(nil), code...)
}

func (code *CodeMemory) AppendInstruction(instruction Instruction) {
	start := len(*code)
	*code = append(*code, 0, 0)
	binary.LittleEndian.PutUint16((*code)[start:start+2], uint16(instruction))
}

func (code CodeMemory) PatchInstruction(position int, instruction Instruction) {
	binary.LittleEndian.PutUint16(code[position:position+2], uint16(instruction))
}

func (code CodeMemory) ReadInstruction(ip *int) Instruction {
	instruction := Instruction(binary.LittleEndian.Uint16(code[*ip : *ip+2]))
	*ip += 2
	return instruction
}

func (code *CodeMemory) AppendImmediate(kind ValueKind, value uint64) {
	start := len(*code)
	size := kind.Size()
	switch size {
	case 1:
		*code = append(*code, 0)
	case 2:
		*code = append(*code, 0, 0)
	case 4:
		*code = append(*code, 0, 0, 0, 0)
	case 8:
		*code = append(*code, 0, 0, 0, 0, 0, 0, 0, 0)
	}
	switch size {
	case 1:
		(*code)[start] = byte(value)
	case 2:
		binary.LittleEndian.PutUint16((*code)[start:start+2], uint16(value))
	case 4:
		binary.LittleEndian.PutUint32((*code)[start:start+4], uint32(value))
	case 8:
		binary.LittleEndian.PutUint64((*code)[start:start+8], value)
	}
}

func (code CodeMemory) ReadImmediate(ip *int, kind ValueKind) (value uint64) {
	switch kind.Size() {
	case 1:
		value = uint64(code[*ip])
		*ip += 1
	case 2:
		value = uint64(binary.LittleEndian.Uint16(code[*ip : *ip+2]))
		*ip += 2
	case 4:
		value = uint64(binary.LittleEndian.Uint32(code[*ip : *ip+4]))
		*ip += 4
	case 8:
		value = binary.LittleEndian.Uint64(code[*ip : *ip+8])
		*ip += 8
	default:
		value = 0
	}
	return
}

func (code *CodeMemory) AppendInt(value int) {
	start := len(*code)
	*code = append(*code, 0, 0, 0, 0)
	binary.LittleEndian.PutUint32((*code)[start:start+4], uint32(value))
}

func (code CodeMemory) PatchInt(position int, value int) {
	binary.LittleEndian.PutUint32(code[position:position+4], uint32(value))
}

func (code CodeMemory) ReadInt(ip *int) int {
	value := int(int32(binary.LittleEndian.Uint32(code[*ip : *ip+4])))
	*ip += 4
	return value
}

func (code *CodeMemory) AppendFunctionHeader(header ScriptFunctionHeader) int {
	start := len(*code)
	code.AppendImmediate(KindUint8, uint64(scriptFunctionHeaderMagic))
	code.AppendImmediate(KindUint8, uint64(header.ReturnKind))
	code.AppendInt(header.ParamCount)
	code.AppendInt(header.FrameByteSize)
	for index := 0; index < header.ParamCount; index++ {
		kind := KindNone
		if index < len(header.ParamKinds) {
			kind = header.ParamKinds[index]
		}
		code.AppendImmediate(KindUint8, uint64(kind))
		offset := 0
		if index < len(header.ParamOffsets) {
			offset = header.ParamOffsets[index]
		}
		code.AppendInt(offset)
	}
	return start
}

func (code CodeMemory) ReadFunctionHeader(address int) (ScriptFunctionHeader, error) {
	ip := address
	if ip < 0 || ip >= len(code) {
		return ScriptFunctionHeader{}, fmt.Errorf("vm error: function header address %d out of range", address)
	}
	if got := byte(code.ReadImmediate(&ip, KindUint8)); got != scriptFunctionHeaderMagic {
		return ScriptFunctionHeader{}, fmt.Errorf("vm error: invalid function header magic 0x%x at %d", got, address)
	}
	header := ScriptFunctionHeader{
		ReturnKind:    ValueKind(code.ReadImmediate(&ip, KindUint8)),
		ParamCount:    code.ReadInt(&ip),
		FrameByteSize: code.ReadInt(&ip),
	}
	if header.ParamCount < 0 {
		return ScriptFunctionHeader{}, fmt.Errorf("vm error: invalid param count %d at %d", header.ParamCount, address)
	}
	if header.ParamCount > 0 {
		header.ParamKinds = make([]ValueKind, header.ParamCount)
		header.ParamOffsets = make([]int, header.ParamCount)
		for index := 0; index < header.ParamCount; index++ {
			header.ParamKinds[index] = ValueKind(code.ReadImmediate(&ip, KindUint8))
			header.ParamOffsets[index] = code.ReadInt(&ip)
		}
	}
	header.BodyAddress = ip
	return header, nil
}
