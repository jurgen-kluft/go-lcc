package cova

import "testing"

func TestVMStatusNumericABIAndStrings(t *testing.T) {
	tests := []struct {
		status VMStatus
		value  uint32
		text   string
	}{
		{VMStatusOK, 0, "ok"},
		{VMStatusNoProgramLoaded, 1, "no program loaded"},
		{VMStatusInvalidLifecycle, 2, "invalid lifecycle"},
		{VMStatusInvalidProgram, 3, "invalid program"},
		{VMStatusInvalidImage, 4, "invalid image"},
		{VMStatusUnsupportedImage, 5, "unsupported image"},
		{VMStatusMalformedBytecode, 6, "malformed bytecode"},
		{VMStatusInvalidDescriptor, 7, "invalid descriptor"},
		{VMStatusInvalidParameter, 8, "invalid parameter metadata"},
		{VMStatusInvalidValueKind, 9, "invalid value kind"},
		{VMStatusInvalidAddress, 10, "invalid address"},
		{VMStatusInvalidAddressSegment, 11, "invalid address segment"},
		{VMStatusReadOnlyMemory, 12, "read-only memory"},
		{VMStatusInvalidTarget, 13, "invalid target"},
		{VMStatusInvalidOpcode, 14, "invalid opcode"},
		{VMStatusStackUnderflow, 15, "stack underflow"},
		{VMStatusStackOverflow, 16, "stack overflow"},
		{VMStatusFrameOverflow, 17, "frame overflow"},
		{VMStatusCallFrameOverflow, 18, "call-frame overflow"},
		{VMStatusDivisionByZero, 19, "division by zero"},
		{VMStatusMissingExtern, 20, "missing extern dispatcher"},
		{VMStatusExternABIViolation, 21, "extern ABI violation"},
		{VMStatusHostFailure, 22, "host failure"},
	}

	for _, test := range tests {
		if got := uint32(test.status); got != test.value {
			t.Fatalf("status %s changed numeric ABI: got %d, want %d", test.status, got, test.value)
		}
		if got := test.status.String(); got != test.text {
			t.Fatalf("status %d string = %q, want %q", test.value, got, test.text)
		}
	}
	if got := VMStatus(0xffffffff).String(); got != "unknown VM status" {
		t.Fatalf("unexpected unknown status string %q", got)
	}
}

func TestLoadDoesNotRejectHostExecutionCapacities(t *testing.T) {
	var code CodeMemory
	code.AppendInstruction(makeInstruction(OpPush, KindInt32, ModeNone, FlagNone))
	code.AppendImmediate(KindInt32, 1)
	code.AppendInstruction(makeInstruction(OpRet, KindNone, ModeNone, FlagNone))
	program := &LinkedProgram{
		Text:       code,
		EntryPoint: 0,
		Functions:  []ScriptFunctionDescriptor{{BodyAddress: 0, ReturnKind: KindInt32}},
	}
	vm := NewVMWithConfig(VMConfig{
		FrameCapacity:     0,
		StackCapacity:     0,
		CallFrameCapacity: 1,
	})

	if status := vm.LoadProgram(program); status != VMStatusOK {
		t.Fatalf("LoadProgram rejected host capacity policy: %s", status)
	}
	if status := vm.RunLoaded(); status != VMStatusStackOverflow {
		t.Fatalf("RunLoaded status = %s, want stack overflow", status)
	}
	fault := vm.FaultInfo()
	if fault.Status != VMStatusStackOverflow || fault.PC != 0 || fault.Required != 4 || fault.Available != 0 {
		t.Fatalf("unexpected stack fault details: %+v", fault)
	}
}

func TestExternDispatcherReceivesContextAndHostFailureIsPreserved(t *testing.T) {
	var code CodeMemory
	code.AppendInstruction(makeInstruction(OpCallExtern, KindNone, ModeNone, FlagNone))
	code.AppendUint32(7)
	code.AppendInstruction(makeInstruction(OpRet, KindNone, ModeNone, FlagNone))
	program := &LinkedProgram{
		Text:       code,
		EntryPoint: 0,
		Functions:  []ScriptFunctionDescriptor{{BodyAddress: 0, ReturnKind: KindVoid}},
	}
	vm := NewVM(0)
	const context uintptr = 0x1234
	vm.RegisterExternDispatcher(context, func(gotContext uintptr, _ *VM, importID uint32) VMStatus {
		if gotContext != context || importID != 7 {
			return VMStatusExternABIViolation
		}
		return VMStatusInvalidParameter
	})

	if status := vm.Run(program); status != VMStatusHostFailure {
		t.Fatalf("Run status = %s, want host failure", status)
	}
	fault := vm.FaultInfo()
	if fault.Status != VMStatusHostFailure || fault.HostStatus != VMStatusInvalidParameter || fault.Target != 7 || fault.PC != 0 {
		t.Fatalf("unexpected host fault details: %+v", fault)
	}
}

func TestOpOffsetRejectsAddressIndexUnderflowAndOverflow(t *testing.T) {
	tests := []struct {
		name   string
		base   uint32
		offset int32
	}{
		{name: "underflow", base: 0, offset: -1},
		{name: "overflow", base: addressIndexMask, offset: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var code CodeMemory
			code.AppendInstruction(makeAddrInstruction(segmentData))
			code.AppendUint32(test.base)
			code.AppendInstruction(makeInstruction(OpPush, KindInt32, ModeNone, FlagNone))
			code.AppendImmediate(KindInt32, uint64(uint32(test.offset)))
			code.AppendInstruction(makeInstruction(OpOffset, KindNone, ModeNone, FlagNone))
			code.AppendInstruction(makeInstruction(OpRet, KindNone, ModeNone, FlagNone))

			program := &LinkedProgram{
				Text:       code,
				EntryPoint: 0,
				Functions:  []ScriptFunctionDescriptor{{BodyAddress: 0, ReturnKind: KindAddress}},
			}
			if status := NewVM(0).Run(program); status != VMStatusInvalidAddress {
				t.Fatalf("Run status = %s, want invalid address", status)
			}
		})
	}
}
