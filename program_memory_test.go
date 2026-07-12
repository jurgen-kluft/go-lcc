package lcc

import "testing"

func TestProgramMemoryRoutesByAddressSegment(t *testing.T) {
	memory := NewProgramMemory(8, 8, 8, 8)

	if err := memory.WriteBits(makeAddress(segmentExtern, 0), KindByte, 11); err != nil {
		t.Fatalf("WriteBits extern failed: %v", err)
	}
	if err := memory.WriteBits(makeAddress(segmentBSS, 1), KindByte, 22); err != nil {
		t.Fatalf("WriteBits bss failed: %v", err)
	}
	memory.segment[segmentFrame] = append(memory.segment[segmentFrame], 0, 0, 0, 0)
	if err := memory.WriteBits(makeAddress(segmentFrame, 2), KindByte, 33); err != nil {
		t.Fatalf("WriteBits frame failed: %v", err)
	}

	if bits, err := memory.ReadBits(makeAddress(segmentExtern, 0), KindByte); err != nil || bits != 11 {
		t.Fatalf("expected extern byte 11, got %d, err=%v", bits, err)
	}
	if bits, err := memory.ReadBits(makeAddress(segmentBSS, 1), KindByte); err != nil || bits != 22 {
		t.Fatalf("expected bss byte 22, got %d, err=%v", bits, err)
	}
	if bits, err := memory.ReadBits(makeAddress(segmentFrame, 2), KindByte); err != nil || bits != 33 {
		t.Fatalf("expected frame byte 33, got %d, err=%v", bits, err)
	}
}

func TestVMAllocateExternMemoryCreatesOwnedExternSegment(t *testing.T) {
	vm := NewVM(8)
	vm.AllocateExternMemory(16)
	if len(vm.Memory.segment[segmentExtern]) != 16 {
		t.Fatalf("expected owned extern memory size 16, got %d", len(vm.Memory.segment[segmentExtern]))
	}
	if err := vm.Memory.WriteBits(makeAddress(segmentExtern, 4), KindInt32, uint64(uint32(99))); err != nil {
		t.Fatalf("expected write into allocated extern memory to succeed: %v", err)
	}
}

func TestProgramMemoryRejectsInvalidAddressSegment(t *testing.T) {
	memory := NewProgramMemory(0, 0, 0, 0)
	invalid := makeAddress(segmentInvalid, 0)

	if _, err := memory.ReadBits(invalid, KindByte); err == nil {
		t.Fatal("expected ReadBits to reject invalid segment")
	}
	if err := memory.WriteBits(invalid, KindByte, 1); err == nil {
		t.Fatal("expected WriteBits to reject invalid segment")
	}
}
