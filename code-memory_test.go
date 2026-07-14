package cova

import "testing"

func TestValueKindSizeRejectsInvalidKind(t *testing.T) {
	if got := ValueKind(255).Size(); got != 0 {
		t.Fatalf("expected invalid kind size 0, got %d", got)
	}
}

func TestCheckedCodeReadersRejectTruncatedInput(t *testing.T) {
	tests := []struct {
		name string
		read func(CodeMemory) VMStatus
	}{
		{
			name: "instruction",
			read: func(code CodeMemory) VMStatus {
				var ip uint32
				_, status := code.ReadInstructionChecked(&ip)
				return status
			},
		},
		{
			name: "immediate",
			read: func(code CodeMemory) VMStatus {
				var ip uint32
				_, status := code.ReadImmediateChecked(&ip, KindUint32)
				return status
			},
		},
		{
			name: "int operand",
			read: func(code CodeMemory) VMStatus {
				var ip uint32
				_, status := code.ReadUint32Checked(&ip)
				return status
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if status := test.read(CodeMemory{0}); status != VMStatusMalformedBytecode {
				t.Fatal("expected truncated input error")
			}
		})
	}
}

func TestRunRejectsTruncatedInstructionWithoutPanic(t *testing.T) {
	code := CodeMemory{byte(OpRet)}
	program := &LinkedProgram{
		Text:       code,
		EntryPoint: 0,
		Functions:  []ScriptFunctionDescriptor{{BodyAddress: 0, ReturnKind: KindVoid}},
	}

	if status := NewVM(0).Run(program); status != VMStatusMalformedBytecode {
		t.Fatalf("expected malformed bytecode status, got %s", status)
	}
}

func TestRunRejectsTruncatedOperandWithoutPanic(t *testing.T) {
	code := CodeMemory{}
	code.AppendInstruction(makeInstruction(OpJump, KindNone, ModeNone, FlagNone))
	code = append(code, 0)
	program := &LinkedProgram{
		Text:       code,
		EntryPoint: 0,
		Functions:  []ScriptFunctionDescriptor{{BodyAddress: 0, ReturnKind: KindVoid}},
	}

	if status := NewVM(0).Run(program); status != VMStatusMalformedBytecode {
		t.Fatalf("expected malformed bytecode status, got %s", status)
	}
}
