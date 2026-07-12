package lcc

import (
	"encoding/binary"
	"math"
	"strings"
	"testing"
)

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
	if op := code.ReadInstruction(&ip).Opcode(); op != OpAddrBSS {
		t.Fatalf("expected OpAddrBSS after push operand, got %d", op)
	}
	ip += 4
	if op := code.ReadInstruction(&ip).Opcode(); op != OpAssign {
		t.Fatalf("expected OpAssign after address operand, got %d", op)
	}
	if linked.BSSSize != 1 {
		t.Fatalf("expected one bss slot, got %d", linked.BSSSize)
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
	code.AppendInstruction(makeInstruction(OpAdd, KindFloat64, ModeNone, FlagNone))
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
	counterBits := binary.LittleEndian.Uint32(vm.Memory.segment[segmentBSS][counterOffset : counterOffset+4])
	totalBits := binary.LittleEndian.Uint32(vm.Memory.segment[segmentBSS][totalOffset : totalOffset+4])
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
	counterBits := binary.LittleEndian.Uint32(vm.Memory.segment[segmentBSS][counterOffset : counterOffset+4])
	totalBits := binary.LittleEndian.Uint32(vm.Memory.segment[segmentBSS][totalOffset : totalOffset+4])
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
	totalBits := binary.LittleEndian.Uint32(vm.Memory.segment[segmentBSS][totalOffset : totalOffset+4])
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
	totalBits := binary.LittleEndian.Uint32(vm.Memory.segment[segmentBSS][totalOffset : totalOffset+4])
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
