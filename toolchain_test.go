package lcc

import (
	"encoding/binary"
	"math"
	"strings"
	"testing"
)

func readCString(segment []byte, offset int) string {
	end := offset
	for end < len(segment) && segment[end] != 0 {
		end++
	}
	return string(segment[offset:end])
}

const testFrameCapacityBytes = 256

func TestCompileInternalGlobalUsesAddressPipeline(t *testing.T) {
	script := `
int threshold;

void script_main() {
	threshold = 7;
	return;
}
`

	linked := mustLinkProgram(t, script, 0, 0)
	code := linked.Text
	if len(code) < 14 {
		t.Fatalf("expected compiled code, got %d bytes", len(code))
	}
	header, err := code.ReadFunctionHeader(linked.EntryPoint)
	if err != nil {
		t.Fatalf("ReadFunctionHeader failed: %v", err)
	}
	ip := header.BodyAddress
	if op := code.ReadInstruction(&ip).Opcode(); op != OpPush {
		t.Fatalf("expected OpPush at function body start, got %d", op)
	}
	ip += 4
	addrInstruction := code.ReadInstruction(&ip)
	if op := addrInstruction.Opcode(); op != OpAddr {
		t.Fatalf("expected OpAddr after push operand, got %d", op)
	}
	if segment := addrInstruction.AddressSegment(); segment != segmentBSS {
		t.Fatalf("expected OpAddr to target bss, got %s", segment)
	}
	ip += 4
	if op := code.ReadInstruction(&ip).Opcode(); op != OpAssign {
		t.Fatalf("expected OpAssign after address operand, got %d", op)
	}
	if linked.BSSSize != 1 {
		t.Fatalf("expected one bss slot, got %d", linked.BSSSize)
	}
}

func TestInstructionEncodingUsesSixBitOpcodePayload(t *testing.T) {
	legacyInstruction := makeInstruction(OpPush, KindInt16, ModeExtend, FlagSigned)
	if op := legacyInstruction.Opcode(); op != OpPush {
		t.Fatalf("expected OpPush opcode, got %d", op)
	}
	if kind := legacyInstruction.Kind(); kind != KindInt16 {
		t.Fatalf("expected legacy kind int16, got %d", kind)
	}
	if mode := legacyInstruction.LegacyMode(); mode != ModeExtend {
		t.Fatalf("expected legacy mode extend, got %d", mode)
	}
	if flags := legacyInstruction.LegacyFlags(); flags != FlagSigned {
		t.Fatalf("expected legacy flags signed, got %d", flags)
	}

	addrInstruction := makeAddrInstruction(segmentConst)
	if op := addrInstruction.Opcode(); op != OpAddr {
		t.Fatalf("expected OpAddr opcode, got %d", op)
	}
	if segment := addrInstruction.AddressSegment(); segment != segmentConst {
		t.Fatalf("expected const segment payload, got %s", segment)
	}

	compareInstruction := makeCompareInstruction(KindFloat64, CompareGreaterEqual)
	if op := compareInstruction.Opcode(); op != OpCompare {
		t.Fatalf("expected OpCompare opcode, got %d", op)
	}
	if kind := compareInstruction.Kind(); kind != KindFloat64 {
		t.Fatalf("expected compare kind float64, got %d", kind)
	}
	if compareOp := compareInstruction.CompareOp(); compareOp != CompareGreaterEqual {
		t.Fatalf("expected compare subtype greater_equal, got %d", compareOp)
	}

	convertInstruction := makeConvertInstruction(KindInt32, KindFloat64)
	if op := convertInstruction.Opcode(); op != OpConvert {
		t.Fatalf("expected OpConvert opcode, got %d", op)
	}
	if kind := convertInstruction.Kind(); kind != KindFloat64 {
		t.Fatalf("expected convert target kind float64, got %d", kind)
	}
	if fromKind := convertInstruction.ConvertFromKind(); fromKind != KindInt32 {
		t.Fatalf("expected convert source kind int32, got %d", fromKind)
	}

	arithmeticInstruction := makeArithmeticInstruction(KindFloat64, ArithmeticMul)
	if op := arithmeticInstruction.Opcode(); op != OpArithmetic {
		t.Fatalf("expected OpArithmetic opcode, got %d", op)
	}
	if kind := arithmeticInstruction.Kind(); kind != KindFloat64 {
		t.Fatalf("expected arithmetic kind float64, got %d", kind)
	}
	if arithmeticOp := arithmeticInstruction.ArithmeticOp(); arithmeticOp != ArithmeticMul {
		t.Fatalf("expected arithmetic subtype mul, got %d", arithmeticOp)
	}
}

func TestRunSupportsInternalGlobalsAndScriptCalls(t *testing.T) {
	externMemory := make([]byte, 8)
	binary.LittleEndian.PutUint32(externMemory[4:], 45)
	logged := 0
	script := `
extern(0) void log_alert(int data);
extern(4) int player_health;
int health_drop;

void script_main() {
	health_drop = 5;
	if ((player_health - 40) + 1) {
		log_alert(player_health);
		reduce_health(health_drop);
	}
	return;
}

void reduce_health(int delta) {
	player_health = player_health - delta;
	return;
}
`

	linked := mustLinkProgram(t, script, len(externMemory), 1)
	vm := NewVM(testFrameCapacityBytes)
	vm.BindExternBlock(externMemory)
	vm.RegisterExternDispatcher(func(vm *VM, importID int) error {
		if importID != 0 {
			t.Fatalf("expected import id 0, got %d", importID)
		}
		value, err := vm.PopInt32()
		if err != nil {
			return err
		}
		logged = int(value)
		return nil
	})
	if err := vm.Run(linked); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if logged != 45 {
		t.Fatalf("expected logged value 45, got %d", logged)
	}
	if got := int(int32(binary.LittleEndian.Uint32(externMemory[4:]))); got != 40 {
		t.Fatalf("expected host health 40, got %d", got)
	}
}

func TestHostFunctionReceivesTypedArgs(t *testing.T) {
	byteValue := byte(0)
	readyValue := false
	totalValue := int64(0)
	script := `
extern(0) void inspect(byte status, bool ready, int64 total);

void script_main() {
	inspect(255, 1, 7);
	return;
}
`

	linked := mustLinkProgram(t, script, 0, 1)
	vm := NewVM(testFrameCapacityBytes)
	vm.RegisterExternDispatcher(func(vm *VM, importID int) error {
		if importID != 0 {
			t.Fatalf("expected import id 0, got %d", importID)
		}
		total, err := vm.PopInt64()
		if err != nil {
			return err
		}
		ready, err := vm.PopBool()
		if err != nil {
			return err
		}
		status, err := vm.PopByte()
		if err != nil {
			return err
		}
		byteValue = status
		readyValue = ready
		totalValue = total
		return nil
	})
	if err := vm.Run(linked); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if byteValue != 255 {
		t.Fatalf("expected byte arg 255, got %d", byteValue)
	}
	if !readyValue {
		t.Fatal("expected bool arg true")
	}
	if totalValue != 7 {
		t.Fatalf("expected int64 arg 7, got %d", totalValue)
	}
}

func TestRunPassesStringLiteralAsConstAddress(t *testing.T) {
	received := Address(0)
	script := `
extern(0) void inspect(const uint8* path);

void script_main() {
	inspect("asset/button_off");
	return;
}
`

	linked := mustLinkProgram(t, script, 0, 1)
	vm := NewVM(testFrameCapacityBytes)
	vm.RegisterExternDispatcher(func(vm *VM, importID int) error {
		if importID != 0 {
			t.Fatalf("expected import id 0, got %d", importID)
		}
		bits, err := vm.PopBits(KindAddress)
		if err != nil {
			return err
		}
		received = Address(uint32(bits))
		return nil
	})
	if err := vm.Run(linked); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if received.Segment() != segmentConst {
		t.Fatalf("expected const segment address, got %s", received.Segment())
	}
	if got := readCString(vm.memory.segment[segmentConst], received.Index()); got != "asset/button_off" {
		t.Fatalf("expected const string %q, got %q", "asset/button_off", got)
	}
}

func TestRunLoadsGlobalPointerInitializerFromData(t *testing.T) {
	received := Address(0)
	script := `
extern(0) void inspect(const uint8* path);
const uint8* asset_path = "asset/button_off";

void script_main() {
	inspect(asset_path);
	return;
}
`

	linked := mustLinkProgram(t, script, 0, 1)
	vm := NewVM(testFrameCapacityBytes)
	vm.RegisterExternDispatcher(func(vm *VM, importID int) error {
		if importID != 0 {
			t.Fatalf("expected import id 0, got %d", importID)
		}
		bits, err := vm.PopBits(KindAddress)
		if err != nil {
			return err
		}
		received = Address(uint32(bits))
		return nil
	})
	if err := vm.Run(linked); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if linked.DataByteSize != 4 {
		t.Fatalf("expected one pointer-sized data global, got %d bytes", linked.DataByteSize)
	}
	dataBinding := linked.DebugSymbols.Symbols["asset_path"]
	if dataBinding.Scope != ScopeData {
		t.Fatalf("expected data global scope, got %d", dataBinding.Scope)
	}
	storedBits := binary.LittleEndian.Uint32(vm.memory.segment[segmentData][dataBinding.ByteOffset : dataBinding.ByteOffset+4])
	storedAddress := Address(storedBits)
	if storedAddress.Segment() != segmentConst {
		t.Fatalf("expected data initializer to point into const segment, got %s", storedAddress.Segment())
	}
	if received != storedAddress {
		t.Fatalf("expected extern argument to equal stored global pointer %v, got %v", storedAddress, received)
	}
	if got := readCString(vm.memory.segment[segmentConst], received.Index()); got != "asset/button_off" {
		t.Fatalf("expected const string %q, got %q", "asset/button_off", got)
	}
}

func TestCompileRejectsStringLiteralForMutablePointer(t *testing.T) {
	script := `
uint8* asset_path = "asset/button_off";

void script_main() {
	return;
}
`
	tokens, err := Tokenize(script)
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}
	program, err := Parse(tokens)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if _, err := NewCompiler().Compile(program); err == nil {
		t.Fatal("expected compile failure for mutable pointer string initializer")
	}
}

func TestCompileRejectsAssignmentToConstLocal(t *testing.T) {
	script := `
int script_main() {
	const int answer = 42;
	answer = 7;
	return answer;
}
`
	tokens, err := Tokenize(script)
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}
	program, err := Parse(tokens)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if _, err := NewCompiler().Compile(program); err == nil {
		t.Fatal("expected compile failure for const local assignment")
	} else if !strings.Contains(err.Error(), "cannot assign to const variable") {
		t.Fatalf("expected const assignment error, got %v", err)
	}
}

func TestCompileRejectsAssignmentToConstGlobal(t *testing.T) {
	script := `
const int limit = 3;

int script_main() {
	limit = 4;
	return limit;
}
`
	tokens, err := Tokenize(script)
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}
	program, err := Parse(tokens)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if _, err := NewCompiler().Compile(program); err == nil {
		t.Fatal("expected compile failure for const global assignment")
	} else if !strings.Contains(err.Error(), "cannot assign to const variable") {
		t.Fatalf("expected const assignment error, got %v", err)
	}
}

func TestCompileAllowsReassigningPointerToConst(t *testing.T) {
	script := `
void script_main() {
	const uint8* path = "asset/button_off";
	path = "asset/button_on";
	return;
}
`
	tokens, err := Tokenize(script)
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}
	program, err := Parse(tokens)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if _, err := NewCompiler().Compile(program); err != nil {
		t.Fatalf("expected pointer-to-const reassignment to compile, got %v", err)
	}
}

func TestRunDeduplicatesStringLiteralsInConstSegment(t *testing.T) {
	received := make([]Address, 0, 2)
	script := `
extern(0) void inspect(const uint8* path);

void script_main() {
	inspect("asset/button_off");
	inspect("asset/button_off");
	return;
}
`

	linked := mustLinkProgram(t, script, 0, 1)
	vm := NewVM(testFrameCapacityBytes)
	vm.RegisterExternDispatcher(func(vm *VM, importID int) error {
		if importID != 0 {
			t.Fatalf("expected import id 0, got %d", importID)
		}
		bits, err := vm.PopBits(KindAddress)
		if err != nil {
			return err
		}
		received = append(received, Address(uint32(bits)))
		return nil
	})
	if err := vm.Run(linked); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if len(received) != 2 {
		t.Fatalf("expected 2 received addresses, got %d", len(received))
	}
	if received[0] != received[1] {
		t.Fatalf("expected duplicate literals to share one const address, got %v and %v", received[0], received[1])
	}
}

func TestRunSupportsInt64ByteAndBoolKinds(t *testing.T) {
	externMemory := make([]byte, 10)
	script := `
extern(0) int64 total;
extern(8) byte flag;
extern(9) bool ready;

void script_main() {
	total = bump(40);
	flag = 255;
	ready = 1;
	return;
}

int64 bump(int64 amount) {
	return amount + 2;
}
`

	linked := mustLinkProgram(t, script, len(externMemory), 0)
	vm := NewVM(testFrameCapacityBytes)
	vm.BindExternBlock(externMemory)
	if err := vm.Run(linked); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if got := int64(binary.LittleEndian.Uint64(externMemory[0:])); got != 42 {
		t.Fatalf("expected int64 total 42, got %d", got)
	}
	if externMemory[8] != 255 {
		t.Fatalf("expected byte flag 255, got %d", externMemory[8])
	}
	if externMemory[9] != 1 {
		t.Fatalf("expected bool ready 1, got %d", externMemory[9])
	}
}

func TestRunSupportsBooleanLiteralsAndLogicalShortCircuit(t *testing.T) {
	externMemory := make([]byte, 3)
	markCalls := 0
	script := `
extern(0) bool and_value;
extern(1) bool or_value;
extern(2) bool normalized;
extern(0) int mark_true();

void script_main() {
	and_value = false && mark_true();
	or_value = true || mark_true();
	normalized = 2 && 7;
	return;
}
`

	linked := mustLinkProgram(t, script, len(externMemory), 1)
	vm := NewVM(testFrameCapacityBytes)
	vm.BindExternBlock(externMemory)
	vm.RegisterExternDispatcher(func(vm *VM, importID int) error {
		if importID != 0 {
			t.Fatalf("expected import id 0, got %d", importID)
		}
		markCalls++
		return vm.PushInt32(1)
	})
	if err := vm.Run(linked); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if markCalls != 0 {
		t.Fatalf("expected short-circuit to skip host calls, got %d invocations", markCalls)
	}
	if externMemory[0] != 0 {
		t.Fatalf("expected false && mark_true() to store 0, got %d", externMemory[0])
	}
	if externMemory[1] != 1 {
		t.Fatalf("expected true || mark_true() to store 1, got %d", externMemory[1])
	}
	if externMemory[2] != 1 {
		t.Fatalf("expected 2 && 7 to normalize to 1, got %d", externMemory[2])
	}
}

func TestRunSupportsBooleanFunctionBoundariesAndEvaluatesLogicalRightHandWhenNeeded(t *testing.T) {
	externMemory := make([]byte, 5)
	markTrueCalls := 0
	script := `
extern(0) bool and_value;
extern(1) bool or_value;
extern(2) bool mixed_value;
extern(3) bool returned_value;
extern(4) bool param_value;
extern(0) int mark_true();

bool is_match(int value) {
	return value == 2;
}

void record(bool flag) {
	param_value = flag;
	return;
}

void script_main() {
	and_value = true && mark_true();
	or_value = false || mark_true();
	mixed_value = (10 > 5) && (2 == 2) || (1 < 0);
	returned_value = is_match(2);
	record(3);
	return;
}
`

	linked := mustLinkProgram(t, script, len(externMemory), 1)
	vm := NewVM(testFrameCapacityBytes)
	vm.BindExternBlock(externMemory)
	vm.RegisterExternDispatcher(func(vm *VM, importID int) error {
		if importID != 0 {
			t.Fatalf("expected import id 0, got %d", importID)
		}
		markTrueCalls++
		return vm.PushInt32(1)
	})
	if err := vm.Run(linked); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if markTrueCalls != 2 {
		t.Fatalf("expected right-hand logical evaluation twice, got %d calls", markTrueCalls)
	}
	if externMemory[0] != 1 {
		t.Fatalf("expected true && mark_true() to store 1, got %d", externMemory[0])
	}
	if externMemory[1] != 1 {
		t.Fatalf("expected false || mark_true() to store 1, got %d", externMemory[1])
	}
	if externMemory[2] != 1 {
		t.Fatalf("expected mixed logical/comparison expression to store 1, got %d", externMemory[2])
	}
	if externMemory[3] != 1 {
		t.Fatalf("expected bool-returning script function to store 1, got %d", externMemory[3])
	}
	if externMemory[4] != 1 {
		t.Fatalf("expected bool parameter conversion to normalize non-zero input to 1, got %d", externMemory[4])
	}
}

func TestRunSupportsZeroInitializedAndInitializedLocals(t *testing.T) {
	script := `
int script_main() {
	int count;
	int total = 5;
	count = count + 3;
	return count + total;
}
`

	linked := mustLinkProgram(t, script, 0, 0)
	vm := NewVM(testFrameCapacityBytes)
	if err := vm.Run(linked); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if got, err := vm.PopInt32(); err != nil || got != 8 {
		t.Fatalf("expected local variable result 8, got %d err=%v", got, err)
	}
}

func TestRunSupportsLocalShadowingAndBoolLocals(t *testing.T) {
	script := `
int script_main() {
	bool ready = false;
	int value = 1;
	{
		bool ready = true;
		int value = 2;
		if (ready) {
			value = value + 3;
		}
	}
	if (ready || false) {
		return 99;
	}
	return value;
}
`

	linked := mustLinkProgram(t, script, 0, 0)
	vm := NewVM(testFrameCapacityBytes)
	if err := vm.Run(linked); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if got, err := vm.PopInt32(); err != nil || got != 1 {
		t.Fatalf("expected outer local value 1 after inner shadowing, got %d err=%v", got, err)
	}
}

func TestCompileRejectsDuplicateLocalDeclarationsInSameScope(t *testing.T) {
	script := `
int script_main() {
	int value;
	int value;
	return 0;
}
`

	tokens, err := Tokenize(script)
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}
	program, err := Parse(tokens)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if _, err := NewCompiler().Compile(program); err == nil {
		t.Fatal("expected compile to reject duplicate local declarations in same scope")
	}
}

func TestHostInteropPreservesUint64Bits(t *testing.T) {
	externMemory := make([]byte, 16)
	binary.LittleEndian.PutUint64(externMemory[0:], uint64(1)<<63)
	script := `
extern(0) uint64 source;
extern(8) uint64 sink;
extern(0) void inspect(uint64 value);
extern(1) uint64 bounce(uint64 value);

void script_main() {
	inspect(source);
	sink = bounce(source);
	return;
}
`

	var seen uint64
	linked := mustLinkProgram(t, script, len(externMemory), 2)
	vm := NewVM(testFrameCapacityBytes)
	vm.BindExternBlock(externMemory)
	vm.RegisterExternDispatcher(func(vm *VM, importID int) error {
		value, err := vm.PopUint64()
		if err != nil {
			return err
		}
		switch importID {
		case 0:
			seen = value
			return nil
		case 1:
			return vm.PushUint64(value)
		default:
			t.Fatalf("unexpected import id %d", importID)
			return nil
		}
	})
	if err := vm.Run(linked); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if seen != uint64(1)<<63 {
		t.Fatalf("expected host arg uint64 0x%x, got 0x%x", uint64(1)<<63, seen)
	}
	if got := binary.LittleEndian.Uint64(externMemory[8:]); got != uint64(1)<<63 {
		t.Fatalf("expected bounced uint64 0x%x, got 0x%x", uint64(1)<<63, got)
	}
}

func TestRunLeavesFinalReturnOnStack(t *testing.T) {
	externMemory := make([]byte, 8)
	binary.LittleEndian.PutUint64(externMemory[0:], uint64(1)<<63)
	script := `
extern(0) uint64 source;

uint64 script_main() {
	return source;
}
`

	linked := mustLinkProgram(t, script, len(externMemory), 0)
	vm := NewVM(testFrameCapacityBytes)
	vm.BindExternBlock(externMemory)
	if err := vm.Run(linked); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if got, err := vm.PopUint64(); err != nil || got != uint64(1)<<63 {
		t.Fatalf("expected uint64 return 0x%x, got 0x%x err=%v", uint64(1)<<63, got, err)
	}
}

func TestVMTypedStackHelpersPreserveBits(t *testing.T) {
	vm := NewVM(8)
	if err := vm.PushFloat32(3.5); err != nil {
		t.Fatalf("PushFloat32 failed: %v", err)
	}
	if err := vm.PushFloat64(-7.25); err != nil {
		t.Fatalf("PushFloat64 failed: %v", err)
	}
	if got, err := vm.PopFloat64(); err != nil || got != -7.25 {
		t.Fatalf("expected float64 pop -7.25, got %v err=%v", got, err)
	}
	if got, err := vm.PopFloat32(); err != nil || got != 3.5 {
		t.Fatalf("expected float32 pop 3.5, got %v err=%v", got, err)
	}
	if err := vm.PushFloat32(1.25); err != nil {
		t.Fatalf("PushFloat32 failed: %v", err)
	}
	if bits, err := vm.PopBits(KindFloat32); err != nil || bits != uint64(math.Float32bits(1.25)) {
		t.Fatalf("expected float32 bits 0x%x, got 0x%x err=%v", math.Float32bits(1.25), bits, err)
	}
	if err := vm.PushFloat64(-9.5); err != nil {
		t.Fatalf("PushFloat64 failed: %v", err)
	}
	if bits, err := vm.PopBits(KindFloat64); err != nil || bits != math.Float64bits(-9.5) {
		t.Fatalf("expected float64 bits 0x%x, got 0x%x err=%v", math.Float64bits(-9.5), bits, err)
	}
}

func TestRunSupportsFloat64Arithmetic(t *testing.T) {
	var code CodeMemory
	entryPoint := code.AppendFunctionHeader(ScriptFunctionHeader{ReturnKind: KindFloat64})
	code.AppendInstruction(makeInstruction(OpPush, KindFloat64, ModeNone, FlagNone))
	code.AppendImmediate(KindFloat64, math.Float64bits(1.5))
	code.AppendInstruction(makeInstruction(OpPush, KindFloat64, ModeNone, FlagNone))
	code.AppendImmediate(KindFloat64, math.Float64bits(2.25))
	code.AppendInstruction(makeArithmeticInstruction(KindFloat64, ArithmeticAdd))
	code.AppendInstruction(makeInstruction(OpRet, KindNone, ModeNone, FlagNone))
	program := &LinkedProgram{Text: code, EntryPoint: entryPoint}

	vm := NewVM(8)
	if err := vm.Run(program); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if got, err := vm.PopFloat64(); err != nil || got != 3.75 {
		t.Fatalf("expected float64 result 3.75, got %v err=%v", got, err)
	}
}

func TestCompileAndRunFloat64Literals(t *testing.T) {
	script := `
float64 script_main() {
	return 1.5 + 2.25;
}
`

	linked := mustLinkProgram(t, script, 0, 0)
	vm := NewVM(testFrameCapacityBytes)
	if err := vm.Run(linked); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if got, err := vm.PopFloat64(); err != nil || got != 3.75 {
		t.Fatalf("expected compiled float64 result 3.75, got %v err=%v", got, err)
	}
}

func TestCompileAndRunDefaultFloatLiteralAsFloat32(t *testing.T) {
	script := `
float32 script_main() {
	return 0.5 + 0.25;
}
`

	linked := mustLinkProgram(t, script, 0, 0)
	vm := NewVM(testFrameCapacityBytes)
	if err := vm.Run(linked); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if got, err := vm.PopFloat32(); err != nil || got != 0.75 {
		t.Fatalf("expected compiled float32 result 0.75, got %v err=%v", got, err)
	}
}

func TestCompileAndRunExplicitFloat64LiteralSuffix(t *testing.T) {
	script := `
float64 script_main() {
	return 0.5d + 0.25d;
}
`

	linked := mustLinkProgram(t, script, 0, 0)
	vm := NewVM(testFrameCapacityBytes)
	if err := vm.Run(linked); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if got, err := vm.PopFloat64(); err != nil || got != 0.75 {
		t.Fatalf("expected compiled float64 result 0.75, got %v err=%v", got, err)
	}
}

func TestCompileAndRunFloatLiteralSuffixPromotion(t *testing.T) {
	script := `
float64 script_main() {
	return 1.0f + 2.5d;
}
`

	linked := mustLinkProgram(t, script, 0, 0)
	vm := NewVM(testFrameCapacityBytes)
	if err := vm.Run(linked); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if got, err := vm.PopFloat64(); err != nil || got != 3.5 {
		t.Fatalf("expected promoted float64 result 3.5, got %v err=%v", got, err)
	}
}

func TestCompileAndRunMixedIntAndFloat64Expression(t *testing.T) {
	script := `
int base;

float64 script_main() {
	base = 2;
	return base + 1.5;
}
`

	linked := mustLinkProgram(t, script, 0, 0)
	vm := NewVM(testFrameCapacityBytes)
	if err := vm.Run(linked); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if got, err := vm.PopFloat64(); err != nil || got != 3.5 {
		t.Fatalf("expected mixed result 3.5, got %v err=%v", got, err)
	}
}

func TestRunNestedCallsWithExplicitFrameCapacity(t *testing.T) {
	script := `
int64 level3(int64 value, int64 extra) {
	return value + extra;
}

int64 level2(int64 left, int64 right) {
	return level3(left + right, right);
}

int64 script_main() {
	return level2(2, 2);
}
`

	linked := mustLinkProgram(t, script, 0, 0)
	vm := NewVM(testFrameCapacityBytes)
	if err := vm.Run(linked); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if got, err := vm.PopInt64(); err != nil || got != 6 {
		t.Fatalf("expected nested call result 6, got %d err=%v", got, err)
	}
}

func TestCompileRejectsRecursiveScriptCallCycle(t *testing.T) {
	script := `
int recurse(int value) {
	return recurse(value);
}

int script_main() {
	return recurse(1);
}
`

	tokens, err := Tokenize(script)
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}
	program, err := Parse(tokens)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if _, err := NewCompiler().Compile(program); err == nil {
		t.Fatal("expected compile to reject recursive script call cycle")
	}
}

func TestRunFailsWhenFrameCapacityTooSmall(t *testing.T) {
	script := `
int64 level3(int64 value, int64 extra) {
	return value + extra;
}

int64 level2(int64 left, int64 right) {
	return level3(left + right, right);
}

int64 script_main() {
	return level2(2, 2);
}
`

	linked := mustLinkProgram(t, script, 0, 0)
	vm := NewVM(linked.FrameByteSize)
	if err := vm.Run(linked); err == nil {
		t.Fatal("expected runtime failure for insufficient frame capacity")
	} else if !strings.Contains(err.Error(), "frame capacity exceeded") {
		t.Fatalf("expected frame capacity error, got %v", err)
	}
}

func TestRunPreservesNestedUint64ReturnBits(t *testing.T) {
	var code CodeMemory
	entryPoint := code.AppendFunctionHeader(ScriptFunctionHeader{ReturnKind: KindUint64})
	code.AppendInstruction(makeInstruction(OpCall, KindNone, ModeNone, FlagNone))
	callOperandPos := len(code)
	code.AppendInt(0)
	code.AppendInstruction(makeInstruction(OpRet, KindNone, ModeNone, FlagNone))
	helperAddress := code.AppendFunctionHeader(ScriptFunctionHeader{ReturnKind: KindUint64})
	code.AppendInstruction(makeInstruction(OpPush, KindUint64, ModeNone, FlagNone))
	code.AppendImmediate(KindUint64, uint64(1)<<63)
	code.AppendInstruction(makeInstruction(OpRet, KindNone, ModeNone, FlagNone))
	code.PatchInt(callOperandPos, helperAddress)

	program := &LinkedProgram{Text: code, EntryPoint: entryPoint}

	vm := NewVM(8)
	if err := vm.Run(program); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if got, err := vm.PopUint64(); err != nil || got != uint64(1)<<63 {
		t.Fatalf("expected nested uint64 result 0x%x, got 0x%x err=%v", uint64(1)<<63, got, err)
	}
}

func TestOpCallErrorsOnOutOfRangeCodeAddress(t *testing.T) {
	var code CodeMemory
	entryPoint := code.AppendFunctionHeader(ScriptFunctionHeader{})
	code.AppendInstruction(makeInstruction(OpCall, KindNone, ModeNone, FlagNone))
	code.AppendInt(99)
	code.AppendInstruction(makeInstruction(OpRet, KindNone, ModeNone, FlagNone))
	program := &LinkedProgram{Text: code, EntryPoint: entryPoint}
	vm := NewVM(8)
	if err := vm.Run(program); err == nil {
		t.Fatal("expected error for out-of-range code address")
	}
}

func TestRunSupportsWhileAndElse(t *testing.T) {
	script := `
int counter;
int total;

void script_main() {
	counter = 0;
	total = 0;
	while (counter < 5) {
		if (counter == 3) {
			total = total + 10;
		} else {
			total = total + 1;
		}
		counter = counter + 1;
	}
	return;
}
`

	linked := mustLinkProgram(t, script, 0, 0)
	vm := NewVM(testFrameCapacityBytes)
	if err := vm.Run(linked); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	counterOffset := linked.DebugSymbols.Symbols["counter"].ByteOffset
	totalOffset := linked.DebugSymbols.Symbols["total"].ByteOffset
	counterBits := binary.LittleEndian.Uint32(vm.memory.segment[segmentBSS][counterOffset : counterOffset+4])
	totalBits := binary.LittleEndian.Uint32(vm.memory.segment[segmentBSS][totalOffset : totalOffset+4])
	if int32(counterBits) != 5 {
		t.Fatalf("expected counter 5, got %d", int32(counterBits))
	}
	if int32(totalBits) != 14 {
		t.Fatalf("expected total 14, got %d", int32(totalBits))
	}
}

func TestRunSupportsForContinueBreakAndSwitch(t *testing.T) {
	script := `
int counter;
int total;

void script_main() {
	total = 0;
	for (counter = 0; counter < 6; counter = counter + 1) {
		switch (counter) {
		case 1:
		case 2:
			total = total + 20;
			break;
		case 4:
			break;
		default:
			if (counter == 3) {
				continue;
			}
			total = total + counter;
		}
		if (counter >= 4) {
			break;
		}
	}
	return;
}
`

	linked := mustLinkProgram(t, script, 0, 0)
	vm := NewVM(testFrameCapacityBytes)
	if err := vm.Run(linked); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	counterOffset := linked.DebugSymbols.Symbols["counter"].ByteOffset
	totalOffset := linked.DebugSymbols.Symbols["total"].ByteOffset
	counterBits := binary.LittleEndian.Uint32(vm.memory.segment[segmentBSS][counterOffset : counterOffset+4])
	totalBits := binary.LittleEndian.Uint32(vm.memory.segment[segmentBSS][totalOffset : totalOffset+4])
	if int32(counterBits) != 4 {
		t.Fatalf("expected counter 4 after break, got %d", int32(counterBits))
	}
	if int32(totalBits) != 40 {
		t.Fatalf("expected total 40, got %d", int32(totalBits))
	}
}

func TestRunSupportsSwitchWithMixedNumericKinds(t *testing.T) {
	script := `
int whole;
float32 fraction;
int total;

void script_main() {
	whole = 2;
	fraction = 3.0f;
	total = 0;

	switch (whole) {
	case 1.0d:
		total = 10;
		break;
	case 2.0d:
		total = 20;
		break;
	default:
		total = 30;
	}

	switch (fraction) {
	case 2:
		total = total + 100;
		break;
	case 3:
		total = total + 3;
		break;
	default:
		total = total + 1000;
	}
	return;
}
`

	linked := mustLinkProgram(t, script, 0, 0)
	vm := NewVM(testFrameCapacityBytes)
	if err := vm.Run(linked); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	totalOffset := linked.DebugSymbols.Symbols["total"].ByteOffset
	totalBits := binary.LittleEndian.Uint32(vm.memory.segment[segmentBSS][totalOffset : totalOffset+4])
	if int32(totalBits) != 23 {
		t.Fatalf("expected total 23, got %d", int32(totalBits))
	}
}

func TestRunSupportsSwitchDefaultOnly(t *testing.T) {
	script := `
int code;
int total;

void script_main() {
	code = 7;
	total = 1;
	switch (code) {
	default:
		total = total + 9;
	}
	return;
}
`

	linked := mustLinkProgram(t, script, 0, 0)
	vm := NewVM(testFrameCapacityBytes)
	if err := vm.Run(linked); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	totalOffset := linked.DebugSymbols.Symbols["total"].ByteOffset
	totalBits := binary.LittleEndian.Uint32(vm.memory.segment[segmentBSS][totalOffset : totalOffset+4])
	if int32(totalBits) != 10 {
		t.Fatalf("expected total 10, got %d", int32(totalBits))
	}
}

func mustLinkProgram(t *testing.T, script string, variableCapacity, functionCapacity int) *LinkedProgram {
	t.Helper()
	tokens, err := Tokenize(script)
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}
	program, err := Parse(tokens)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	compiled, err := NewCompiler().Compile(program)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}
	linked, err := NewLinker(variableCapacity, functionCapacity).Link(program, compiled)
	if err != nil {
		t.Fatalf("Link failed: %v", err)
	}
	return linked
}

func TestRunRepresentativeScript(t *testing.T) {
	script := `
int helper(int left, int right) {
	return left + right;
}

int script_main() {
	int total = 1;
	int count = 0;
	while (count < 3) {
		total = total + helper(count, 2);
		count = count + 1;
	}
	if ((total > 0 && true) || false) {
		return total;
	}
	return 0;
}
`

	linked := mustLinkProgram(t, script, 0, 0)
	vm := NewVM(testFrameCapacityBytes)
	if err := vm.Run(linked); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	result, err := vm.PopInt32()
	if err != nil {
		t.Fatalf("PopInt32 failed: %v", err)
	}
	if result != 10 {
		t.Fatalf("expected result 10, got %d", result)
	}
}
