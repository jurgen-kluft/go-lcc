package cova

type TypeKind uint8

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
	TypeString
)

type Type struct {
	Kind    TypeKind
	Name    string
	Size    int
	Base    *Type
	IsConst bool
}

func (typ *Type) String() string {
	if typ == nil {
		return "<nil>"
	}
	prefix := ""
	if typ.IsConst {
		prefix = "const "
	}
	if typ.Kind == TypePointer {
		if typ.Base == nil {
			return prefix + "<invalid>*"
		}
		return typ.Base.String() + "*" + suffixConst(typ.IsConst)
	}
	return prefix + typ.Name
}

func suffixConst(isConst bool) string {
	if isConst {
		return " const"
	}
	return ""
}

func (typ *Type) Alignment() int {
	if typ == nil {
		return 1
	}
	if typ.Kind == TypeVoid {
		return 1
	}
	if typ.Kind == TypePointer || typ.Kind == TypeString {
		return 4
	}
	if typ.Size <= 1 {
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
	StringType  = &Type{Kind: TypeString, Name: "string", Size: 4, Base: Uint8Type}
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

func QualifiedType(base *Type, isConst bool) *Type {
	if base == nil {
		return nil
	}
	if !isConst {
		return base
	}
	clone := *base
	clone.IsConst = true
	return &clone
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
	return PointerToQualified(base, false)
}

func PointerToQualified(base *Type, isConst bool) *Type {
	if base == nil {
		return nil
	}
	return &Type{Kind: TypePointer, Name: base.Name + "*", Size: 4, Base: base, IsConst: isConst}
}

func IsSameType(left *Type, right *Type) bool {
	if left == nil || right == nil {
		return left == right
	}
	if left.Kind != right.Kind || left.Name != right.Name || left.Size != right.Size || left.IsConst != right.IsConst {
		return false
	}
	return IsSameType(left.Base, right.Base)
}

func IsTopLevelConst(typ *Type) bool {
	return typ != nil && typ.IsConst
}

var typeToValueKind = map[TypeKind]ValueKind{
	TypeVoid: KindVoid,
	TypeBool: KindBool,
	TypeByte: KindByte,
	TypeInt8: KindInt8, TypeInt16: KindInt16, TypeInt32: KindInt32, TypeInt64: KindInt64,
	TypeUint8: KindUint8, TypeUint16: KindUint16, TypeUint32: KindUint32, TypeUint64: KindUint64,
	TypeFloat32: KindFloat32, TypeFloat64: KindFloat64,
	TypePointer: KindAddress,
	TypeString:  KindAddress,
}

func valueKindFromType(typ *Type) ValueKind {
	if typ == nil {
		return KindNone
	}
	if kind, ok := typeToValueKind[typ.Kind]; ok {
		return kind
	}
	return KindNone
}

type Opcode byte

// Note: Keep the number of opcodes below 32 to fit in 5 bits of the instruction encoding.
const (
	OpPush Opcode = iota + 1
	OpArithmetic
	OpConvert
	OpAddr
	OpOffset
	OpDereference
	OpAssign
	OpCompare
	OpJumpIfFalse
	OpJump
	OpCall
	OpCallExtern
	OpRet
)

type ArithmeticOp byte

// Note: Keep the number of arithmetic operations below 8 to fit in 3 bits of the instruction encoding.
const (
	ArithmeticInvalid ArithmeticOp = iota
	ArithmeticAdd
	ArithmeticSub
	ArithmeticMul
	ArithmeticDiv
)

type CompareOp byte

// Note: Keep the number of compare operations below 8 to fit in 3 bits of the instruction encoding.
const (
	CompareInvalid CompareOp = iota
	CompareEqual
	CompareNotEqual
	CompareLess
	CompareLessEqual
	CompareGreater
	CompareGreaterEqual
)

type ValueKind byte

const (
	KindNone ValueKind = iota
	KindVoid
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
	KindCount
)

var valueKindSize = [KindCount]uint32{
	KindNone: 0, KindVoid: 0,
	KindBool: 1, KindByte: 1,
	KindInt8: 1, KindInt16: 2, KindInt32: 4, KindInt64: 8,
	KindUint8: 1, KindUint16: 2, KindUint32: 4, KindUint64: 8,
	KindFloat32: 4, KindFloat64: 8,
	KindAddress: 4, // Assuming a 32-bit address space
}

func (kind ValueKind) Size() uint32 {
	if kind >= KindCount {
		return 0
	}
	size := valueKindSize[kind]
	return size
}

type ScopeKind int

const (
	ScopeInvalid ScopeKind = iota
	ScopeFrame
	ScopeBSS
	ScopeConst
	ScopeData
	ScopeExtern
)

type ExternDispatcher func(hostContext uintptr, vm *VM, importID uint32) VMStatus

type ExternDispatcherBinding struct {
	HostContext uintptr
	Dispatcher  ExternDispatcher
}

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
	SlotIndex      uint32
	ByteOffset     uint32
	ByteSize       uint32
	ByteAlignment  uint32
	ParamCount     uint32
	ParamTypes     []*Type
	ParamOffsets   []uint32
	FrameSlotCount uint32
	FrameByteSize  uint32
	TempFuncID     uint32
	ScriptAddress  uint32
}

type CallPatch struct {
	OperandPos int
	TempFuncID uint32
	Line       int
}

type ScriptFunctionDescriptor struct {
	BodyAddress   uint32
	ParamStart    uint32
	ParamCount    uint32
	FrameByteSize uint32
	ReturnKind    ValueKind
}

type ProgramSymbols struct {
	Symbols       map[string]SymbolBinding
	ExternSymbols []SymbolBinding
	BSSSymbols    []SymbolBinding
	DataSymbols   []SymbolBinding
	ConstSymbols  []SymbolBinding
}

func NewProgramSymbols() *ProgramSymbols {
	return &ProgramSymbols{
		Symbols:       make(map[string]SymbolBinding),
		ExternSymbols: make([]SymbolBinding, 0),
		BSSSymbols:    make([]SymbolBinding, 0),
		DataSymbols:   make([]SymbolBinding, 0),
		ConstSymbols:  make([]SymbolBinding, 0),
	}
}

func CopyProgramSymbols(src *ProgramSymbols) *ProgramSymbols {
	if src == nil {
		return nil
	}
	dst := &ProgramSymbols{
		Symbols:       cloneBindingsMap(src.Symbols),
		ExternSymbols: append([]SymbolBinding(nil), src.ExternSymbols...),
		BSSSymbols:    append([]SymbolBinding(nil), src.BSSSymbols...),
		DataSymbols:   append([]SymbolBinding(nil), src.DataSymbols...),
		ConstSymbols:  append([]SymbolBinding(nil), src.ConstSymbols...),
	}
	return dst
}

type LinkedProgram struct {
	Text          CodeMemory
	EntryPoint    uint32
	Functions     []ScriptFunctionDescriptor
	ParamKinds    []ValueKind
	ParamOffsets  []uint32
	FrameSize     uint32
	FrameByteSize uint32
	ConstByteSize uint32
	ConstData     []byte
	DataByteSize  uint32
	DataData      []byte
	BSSSize       uint32
	BSSByteSize   uint32
	DebugSymbols  *ProgramSymbols
}

type RelocatableProgram struct {
	Text           CodeMemory
	ProgramSymbols *ProgramSymbols
	Functions      []SymbolBinding
	CallPatches    []CallPatch
	EntryFunction  uint32
	FrameSize      uint32
	FrameByteSize  uint32
	ConstByteSize  uint32
	ConstData      []byte
	DataByteSize   uint32
	DataData       []byte
	BSSSize        uint32
	BSSByteSize    uint32
}

func alignUp(offset int, alignment int) int {
	if alignment <= 1 {
		return offset
	}
	mask := alignment - 1
	return (offset + mask) &^ mask
}

func alignUpU32(offset uint32, alignment uint32) uint32 {
	if alignment <= 1 {
		return offset
	}
	mask := alignment - 1
	return (offset + mask) &^ mask
}

func lenu32[S ~[]E, E any](values S) uint32 {
	return uint32(len(values))
}
