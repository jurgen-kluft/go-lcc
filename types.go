package lcc

import "fmt"

type TypeKind int

const (
	TypeInvalid TypeKind = iota
	TypeInt
	TypeVoid
	TypePointer
)

type Type struct {
	Kind TypeKind
	Name string
	Size int
	Base *Type
}

var (
	IntType  = &Type{Kind: TypeInt, Name: "int", Size: 1}
	VoidType = &Type{Kind: TypeVoid, Name: "void", Size: 0}
)

func PointerTo(base *Type) *Type {
	if base == nil {
		return nil
	}
	return &Type{Kind: TypePointer, Name: base.Name + "*", Size: 1, Base: base}
}

type Opcode byte

const (
	OpPush Opcode = iota + 1
	OpAdd
	OpSub
	OpMul
	OpDiv
	OpAddrLocal
	OpAddrGlobalIdx
	OpOffset
	OpDereference
	OpAssign
	OpJumpIfFalse
	OpCallGlobalIdx
	OpRet
)

type memorySegment byte

const (
	segmentInvalid memorySegment = iota
	segmentLocal
	segmentGlobal
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

type MemorySlot struct {
	Bound *int
	Value int
}

func (slot *MemorySlot) Load() int {
	if slot == nil {
		return 0
	}
	if slot.Bound != nil {
		return *slot.Bound
	}
	return slot.Value
}

func (slot *MemorySlot) Store(value int) {
	if slot == nil {
		return
	}
	if slot.Bound != nil {
		*slot.Bound = value
		return
	}
	slot.Value = value
}

type FlatMemory struct {
	Globals []MemorySlot
	Locals  []MemorySlot
}

func NewFlatMemory(globalCount, localCount int) FlatMemory {
	return FlatMemory{
		Globals: make([]MemorySlot, globalCount),
		Locals:  make([]MemorySlot, localCount),
	}
}

func (memory *FlatMemory) Slot(address Address) (*MemorySlot, error) {
	if memory == nil {
		return nil, fmt.Errorf("memory is nil")
	}
	index := address.Index()
	switch address.Segment() {
	case segmentGlobal:
		if index < 0 || index >= len(memory.Globals) {
			return nil, fmt.Errorf("global slot %d out of range", index)
		}
		return &memory.Globals[index], nil
	case segmentLocal:
		if index < 0 || index >= len(memory.Locals) {
			return nil, fmt.Errorf("local slot %d out of range", index)
		}
		return &memory.Locals[index], nil
	default:
		return nil, fmt.Errorf("invalid address segment %d", address.Segment())
	}
}

type HostFunction func(vm *VM) error

type GlobalKind int

const (
	GlobalVariable GlobalKind = iota + 1
	GlobalFunction
)

type GlobalBinding struct {
	Name  string
	Index int
	Kind  GlobalKind
	Type  *Type
	Arity int
}

type LinkedProgram struct {
	Code            []byte
	Globals         map[string]GlobalBinding
	GlobalVars      []GlobalBinding
	GlobalFunctions []GlobalBinding
	Entry           string
	LocalSlotCount  int
}

func writeInt(code *[]byte, value int) {
	*code = append(*code,
		byte(value),
		byte(value>>8),
		byte(value>>16),
		byte(value>>24),
	)
}

func readInt(code []byte, ip *int) int {
	value := int(code[*ip]) |
		int(code[*ip+1])<<8 |
		int(code[*ip+2])<<16 |
		int(code[*ip+3])<<24
	*ip += 4
	return value
}
