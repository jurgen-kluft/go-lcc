package lcc

import (
	"encoding/binary"
	"fmt"
)

type TypeKind int

const (
	TypeInvalid TypeKind = iota
	TypeVoid
	TypeBool
	TypeByte
	TypeInt8
	TypeInt16
	TypeInt32
	TypeInt64
	TypeUint8
	TypeUint16
	TypeUint32
	TypeUint64
	TypeFloat32
	TypeFloat64
	TypePointer
)

type Type struct {
	Kind TypeKind
	Name string
	Size int
	Base *Type
}

func (typ *Type) Alignment() int {
	if typ == nil {
		return 1
	}
	if typ.Kind == TypeVoid {
		return 1
	}
	if typ.Kind == TypePointer {
		return 4
	}
	if typ.Size <= 0 {
		return 1
	}
	if typ.Size >= 8 {
		return 8
	}
	return typ.Size
}

var (
	VoidType    = &Type{Kind: TypeVoid, Name: "void", Size: 0}
	BoolType    = &Type{Kind: TypeBool, Name: "bool", Size: 1}
	ByteType    = &Type{Kind: TypeByte, Name: "byte", Size: 1}
	Int8Type    = &Type{Kind: TypeInt8, Name: "int8", Size: 1}
	Int16Type   = &Type{Kind: TypeInt16, Name: "int16", Size: 2}
	Int32Type   = &Type{Kind: TypeInt32, Name: "int32", Size: 4}
	Int64Type   = &Type{Kind: TypeInt64, Name: "int64", Size: 8}
	Uint8Type   = &Type{Kind: TypeUint8, Name: "uint8", Size: 1}
	Uint16Type  = &Type{Kind: TypeUint16, Name: "uint16", Size: 2}
	Uint32Type  = &Type{Kind: TypeUint32, Name: "uint32", Size: 4}
	Uint64Type  = &Type{Kind: TypeUint64, Name: "uint64", Size: 8}
	Float32Type = &Type{Kind: TypeFloat32, Name: "float32", Size: 4}
	Float64Type = &Type{Kind: TypeFloat64, Name: "float64", Size: 8}
	IntType     = Int32Type
)

var namedTypes = map[string]*Type{
	"void":    VoidType,
	"bool":    BoolType,
	"byte":    ByteType,
	"int":     IntType,
	"int8":    Int8Type,
	"int16":   Int16Type,
	"int32":   Int32Type,
	"int64":   Int64Type,
	"uint8":   Uint8Type,
	"uint16":  Uint16Type,
	"uint32":  Uint32Type,
	"uint64":  Uint64Type,
	"float32": Float32Type,
	"float64": Float64Type,
}

func LookupNamedType(name string) *Type {
	return namedTypes[name]
}

func (typ *Type) IsSignedInteger() bool {
	if typ == nil {
		return false
	}
	switch typ.Kind {
	case TypeInt8, TypeInt16, TypeInt32, TypeInt64:
		return true
	default:
		return false
	}
}

func (typ *Type) IsUnsignedInteger() bool {
	if typ == nil {
		return false
	}
	switch typ.Kind {
	case TypeByte, TypeUint8, TypeUint16, TypeUint32, TypeUint64:
		return true
	default:
		return false
	}
}

func (typ *Type) IsFloat() bool {
	if typ == nil {
		return false
	}
	return typ.Kind == TypeFloat32 || typ.Kind == TypeFloat64
}

func (typ *Type) IsNumeric() bool {
	if typ == nil {
		return false
	}
	return typ.IsSignedInteger() || typ.IsUnsignedInteger() || typ.IsFloat()
}

func PointerTo(base *Type) *Type {
	if base == nil {
		return nil
	}
	return &Type{Kind: TypePointer, Name: base.Name + "*", Size: 1, Base: base}
}

func valueKindFromType(typ *Type) ValueKind {
	if typ == nil {
		return KindNone
	}
	switch typ.Kind {
	case TypeBool:
		return KindBool
	case TypeByte:
		return KindByte
	case TypeInt8:
		return KindInt8
	case TypeInt16:
		return KindInt16
	case TypeInt32:
		return KindInt32
	case TypeInt64:
		return KindInt64
	case TypeUint8:
		return KindUint8
	case TypeUint16:
		return KindUint16
	case TypeUint32:
		return KindUint32
	case TypeUint64:
		return KindUint64
	case TypeFloat32:
		return KindFloat32
	case TypeFloat64:
		return KindFloat64
	case TypePointer:
		return KindAddress
	default:
		return KindNone
	}
}

type Opcode byte

const (
	OpPush Opcode = iota + 1
	OpAdd
	OpSub
	OpMul
	OpDiv
	OpConvert
	OpAddrFrame
	OpAddrBSS
	OpAddrExtern
	OpOffset
	OpDereference
	OpAssign
	OpJumpIfFalse
	OpCall
	OpCallExtern
	OpRet
)

type ValueKind byte

const (
	KindNone ValueKind = iota
	KindBool
	KindByte
	KindInt8
	KindInt16
	KindInt32
	KindInt64
	KindUint8
	KindUint16
	KindUint32
	KindUint64
	KindFloat32
	KindFloat64
	KindAddress
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
	return Instruction(uint16(op) | uint16(kind&0x0f)<<8 | uint16(mode&0x03)<<12 | uint16(flags&0x03)<<14)
}

func (instruction Instruction) Opcode() Opcode {
	return Opcode(byte(instruction))
}

func (instruction Instruction) Kind() ValueKind {
	return ValueKind((instruction >> 8) & 0x0f)
}

func (instruction Instruction) Mode() InstructionMode {
	return InstructionMode((instruction >> 12) & 0x03)
}

func (instruction Instruction) Flags() InstructionFlag {
	return InstructionFlag((instruction >> 14) & 0x03)
}

type ScopeKind int

const (
	ScopeInvalid ScopeKind = iota
	ScopeFrame
	ScopeBSS
	ScopeExtern
)

type Address int

func makeAddress(segment memorySegment, index int) Address {
	return Address(int(segment)<<24 | (index & 0x00ffffff))
}

func (address Address) Segment() memorySegment {
	return memorySegment((int(address) >> 24) & 0xff)
}

func (address Address) Index() int {
	return int(address) & 0x00ffffff
}

func (kind ValueKind) Size() int {
	switch kind {
	case KindBool, KindByte, KindInt8, KindUint8:
		return 1
	case KindInt16, KindUint16:
		return 2
	case KindInt32, KindUint32, KindFloat32, KindAddress:
		return 4
	case KindInt64, KindUint64, KindFloat64:
		return 8
	default:
		return 0
	}
}

type ExternDispatcher func(vm *VM, importID int) error

type DeclKind int

const (
	DeclVariable DeclKind = iota + 1
	DeclFunction
)

type SymbolBinding struct {
	Name           string
	Kind           DeclKind
	Scope          ScopeKind
	Type           *Type
	SlotIndex      int
	ByteOffset     int
	ByteSize       int
	ByteAlignment  int
	ParamCount     int
	ParamTypes     []*Type
	ParamOffsets   []int
	FrameSlotCount int
	FrameByteSize  int
	TempFuncID     int
	ScriptAddress  int
}

type CallPatch struct {
	OperandPos int
	TempFuncID int
	Line       int
}

type DebugSymbols struct {
	Symbols       map[string]SymbolBinding
	ExternSymbols []SymbolBinding
	BSSSymbols    []SymbolBinding
}

const scriptFunctionHeaderMagic byte = 0xf1

type ScriptFunctionHeader struct {
	BodyAddress   int
	ParamCount    int
	ParamKinds    []ValueKind
	ParamOffsets  []int
	FrameByteSize int
	ReturnKind    ValueKind
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

type LinkedProgram struct {
	Text          CodeMemory
	EntryPoint    int
	FrameSize     int
	FrameByteSize int
	BSSSize       int
	BSSByteSize   int
	DebugSymbols  *DebugSymbols
}

type RelocatableProgram struct {
	Text          CodeMemory
	Symbols       map[string]SymbolBinding
	ExternSymbols []SymbolBinding
	BSSSymbols    []SymbolBinding
	Functions     []SymbolBinding
	CallPatches   []CallPatch
	EntryFunction int
	FrameSize     int
	FrameByteSize int
	BSSSize       int
	BSSByteSize   int
}

func alignUp(offset int, alignment int) int {
	if alignment <= 1 {
		return offset
	}
	mask := alignment - 1
	return (offset + mask) &^ mask
}
