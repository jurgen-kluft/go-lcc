package cova

import (
	"math"
	"testing"
)

func TestOpcodeConvertAllRuntimeKindPairs(t *testing.T) {
	kinds := append(append([]ValueKind(nil), opcodeNumericKinds...), KindAddress)
	caseCount := 0
	for _, from := range kinds {
		for _, to := range kinds {
			caseCount++
			name := valueKindTestName(from) + "/to_" + valueKindTestName(to)
			t.Run(name, func(t *testing.T) {
				var code CodeMemory
				appendOpcodeValue(&code, from, opcodeValueBits(from, 1))
				code.AppendInstruction(makeConvertInstruction(from, to))
				if got, want := runOpcodeResult(t, code, to), opcodeValueBits(to, 1); got != want {
					t.Fatalf("converted bits = %#x, want %#x", got, want)
				}
			})
		}
	}
	if caseCount != 169 {
		t.Fatalf("conversion matrix has %d entries, want 169", caseCount)
	}
}

func TestOpcodeConvertFocusedSemantics(t *testing.T) {
	tests := []struct {
		name     string
		from     ValueKind
		to       ValueKind
		input    uint64
		expected uint64
	}{
		{"negative_int32_to_int64", KindInt32, KindInt64, uint64(uint32(0xffffffff)), uint64(0xffffffffffffffff)},
		{"negative_int16_to_uint8", KindInt16, KindUint8, uint64(uint16(0xffff)), 0xff},
		{"narrow_uint64_to_uint16", KindUint64, KindUint16, 0x123456789abcdef0, 0xdef0},
		{"float32_to_int32_truncates", KindFloat32, KindInt32, uint64(math.Float32bits(3.75)), 3},
		{"float64_to_int32_negative", KindFloat64, KindInt32, math.Float64bits(-3.75), uint64(uint32(0xfffffffd))},
		{"int32_to_float32", KindInt32, KindFloat32, 7, uint64(math.Float32bits(7))},
		{"uint64_to_bool_false", KindUint64, KindBool, 0, 0},
		{"uint64_to_bool_true", KindUint64, KindBool, 256, 1},
		{"address_to_uint32", KindAddress, KindUint32, uint64(makeAddress(segmentData, 7)), uint64(makeAddress(segmentData, 7))},
		{"uint32_to_address", KindUint32, KindAddress, uint64(makeAddress(segmentBSS, 9)), uint64(makeAddress(segmentBSS, 9))},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var code CodeMemory
			appendOpcodeValue(&code, test.from, test.input)
			code.AppendInstruction(makeConvertInstruction(test.from, test.to))
			if got := runOpcodeResult(t, code, test.to); got != test.expected {
				t.Fatalf("converted bits = %#x, want %#x", got, test.expected)
			}
		})
	}
}

func TestOpcodeConvertRejectsInvalidKindsAndUnderflow(t *testing.T) {
	tests := []struct {
		name string
		from ValueKind
		to   ValueKind
		push bool
	}{
		{"none_source", KindNone, KindInt32, false},
		{"void_source", KindVoid, KindInt32, false},
		{"none_destination", KindInt32, KindNone, true},
		{"void_destination", KindInt32, KindVoid, true},
		{"underflow", KindInt32, KindUint32, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var code CodeMemory
			if test.push {
				appendOpcodeValue(&code, test.from, 1)
			}
			code.AppendInstruction(makeConvertInstruction(test.from, test.to))
			_, status := runOpcodeProgram(t, code, KindVoid)
			want := VMStatusInvalidValueKind
			if test.name == "underflow" {
				want = VMStatusStackUnderflow
			}
			if status != want {
				t.Fatalf("Run status = %s, want %s", status, want)
			}
		})
	}
}

func TestCompilerNumericConversionMatrix(t *testing.T) {
	caseCount := 0
	for _, from := range opcodeNumericKinds {
		for _, to := range opcodeNumericKinds {
			caseCount++
			compiler := &functionCompiler{}
			compiler.emitConvertIfNeeded(from, to)
			if compiler.err != nil {
				t.Fatalf("numeric conversion %d -> %d failed: %v", from, to, compiler.err)
			}
			if from == to {
				if len(compiler.code) != 0 {
					t.Fatalf("identity conversion %d emitted %d bytes", from, len(compiler.code))
				}
				continue
			}
			var ip uint32
			instruction, status := compiler.code.ReadInstructionChecked(&ip)
			if status != VMStatusOK || instruction.Opcode() != OpConvert || instruction.ConvertFromKind() != from || instruction.Kind() != to {
				t.Fatalf("conversion %d -> %d encoded as %#x, status=%s", from, to, instruction, status)
			}
		}
	}
	if caseCount != 144 {
		t.Fatalf("compiler conversion matrix has %d entries, want 144", caseCount)
	}
}

func TestCompilerRejectsAddressNumericConversions(t *testing.T) {
	for _, test := range []struct {
		from ValueKind
		to   ValueKind
	}{{KindAddress, KindInt32}, {KindInt32, KindAddress}} {
		compiler := &functionCompiler{}
		compiler.emitConvertIfNeeded(test.from, test.to)
		if compiler.err == nil {
			t.Fatalf("compiler accepted conversion %d -> %d", test.from, test.to)
		}
	}
}
