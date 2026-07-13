package cova

import "testing"

func TestProgramMemoryRoutesByAddressSegment(t *testing.T) {
	memory := NewProgramMemory(8, 8, 8, 8, 8, 8)

	if status := memory.WriteBits(makeAddress(segmentExtern, 0), KindByte, 11); status != VMStatusOK {
		t.Fatalf("WriteBits extern failed: %s", status)
	}
	if status := memory.WriteBits(makeAddress(segmentBSS, 1), KindByte, 22); status != VMStatusOK {
		t.Fatalf("WriteBits bss failed: %s", status)
	}
	memory.segment[segmentFrame] = append(memory.segment[segmentFrame], 0, 0, 0, 0)
	if status := memory.WriteBits(makeAddress(segmentFrame, 2), KindByte, 33); status != VMStatusOK {
		t.Fatalf("WriteBits frame failed: %s", status)
	}

	if bits, status := memory.ReadBits(makeAddress(segmentExtern, 0), KindByte); status != VMStatusOK || bits != 11 {
		t.Fatalf("expected extern byte 11, got %d, status=%s", bits, status)
	}
	if bits, status := memory.ReadBits(makeAddress(segmentBSS, 1), KindByte); status != VMStatusOK || bits != 22 {
		t.Fatalf("expected bss byte 22, got %d, status=%s", bits, status)
	}
	if bits, status := memory.ReadBits(makeAddress(segmentFrame, 2), KindByte); status != VMStatusOK || bits != 33 {
		t.Fatalf("expected frame byte 33, got %d, status=%s", bits, status)
	}
}

func TestVMAllocateExternMemoryCreatesOwnedExternSegment(t *testing.T) {
	vm := NewVM(8)
	vm.AllocateExternMemory(16)
	if len(vm.memory.segment[segmentExtern]) != 16 {
		t.Fatalf("expected owned extern memory size 16, got %d", len(vm.memory.segment[segmentExtern]))
	}
	if status := vm.memory.WriteBits(makeAddress(segmentExtern, 4), KindInt32, uint64(uint32(99))); status != VMStatusOK {
		t.Fatalf("expected write into allocated extern memory to succeed: %s", status)
	}
}

func TestProgramMemoryRejectsInvalidAddressSegment(t *testing.T) {
	memory := NewProgramMemory(0, 0, 0, 0, 0, 0)
	invalid := makeAddress(segmentInvalid, 0)

	if _, status := memory.ReadBits(invalid, KindByte); status != VMStatusInvalidAddressSegment {
		t.Fatalf("expected invalid segment status, got %s", status)
	}
	if status := memory.WriteBits(invalid, KindByte, 1); status != VMStatusInvalidAddressSegment {
		t.Fatalf("expected invalid segment status, got %s", status)
	}
}

func TestProgramMemoryRejectsWritesToConstSegment(t *testing.T) {
	memory := NewProgramMemory(0, 4, 0, 0, 0, 0)
	if status := memory.WriteBits(makeAddress(segmentConst, 0), KindByte, 1); status != VMStatusReadOnlyMemory {
		t.Fatalf("expected read-only memory status, got %s", status)
	}
}
