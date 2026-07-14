package cova

import "testing"

func TestProgramMemoryRoutesByAddressSegment(t *testing.T) {
	memory := NewProgramMemory(8, 8, 8, 8, 8, 8)

	addresses := []Address{
		makeAddress(segmentExtern, 0),
		makeAddress(segmentBSS, 1),
		makeAddress(segmentFrame, 2),
	}
	for index, address := range addresses {
		if status := memory.WriteUint8(address, uint8(index+11)); status != VMStatusOK {
			t.Fatalf("WriteUint8 failed for %s: %s", address.Segment(), status)
		}
	}
	for index, address := range addresses {
		value, status := memory.ReadUint8(address)
		if status != VMStatusOK || value != uint8(index+11) {
			t.Fatalf("expected %s byte %d, got %d, status=%s", address.Segment(), index+11, value, status)
		}
	}
}

func TestProgramMemoryTypedReadWrite(t *testing.T) {
	memory := NewProgramMemory(64, 0, 0, 0, 0, 0)

	if status := memory.WriteBool(makeAddress(segmentExtern, 0), true); status != VMStatusOK {
		t.Fatalf("WriteBool failed: %s", status)
	}
	if value, status := memory.ReadBool(makeAddress(segmentExtern, 0)); status != VMStatusOK || !value {
		t.Fatalf("expected true, got %t, status=%s", value, status)
	}
	if stored, status := memory.ReadUint8(makeAddress(segmentExtern, 0)); status != VMStatusOK || stored != 1 {
		t.Fatalf("expected canonical bool byte 1, got %d, status=%s", stored, status)
	}
	if status := memory.WriteUint8(makeAddress(segmentExtern, 1), 2); status != VMStatusOK {
		t.Fatalf("WriteUint8 for bool failed: %s", status)
	}
	if value, status := memory.ReadBool(makeAddress(segmentExtern, 1)); status != VMStatusOK || !value {
		t.Fatalf("expected nonzero bool to read true, got %t, status=%s", value, status)
	}

	if status := memory.WriteUint8(makeAddress(segmentExtern, 2), 0xa5); status != VMStatusOK {
		t.Fatalf("WriteUint8 for byte failed: %s", status)
	}
	if value, status := memory.ReadUint8(makeAddress(segmentExtern, 2)); status != VMStatusOK || value != 0xa5 {
		t.Fatalf("expected byte 0xa5, got %#x, status=%s", value, status)
	}
	if status := memory.WriteInt8(makeAddress(segmentExtern, 3), -12); status != VMStatusOK {
		t.Fatalf("WriteInt8 failed: %s", status)
	}
	if value, status := memory.ReadInt8(makeAddress(segmentExtern, 3)); status != VMStatusOK || value != -12 {
		t.Fatalf("expected int8 -12, got %d, status=%s", value, status)
	}
	if status := memory.WriteUint8(makeAddress(segmentExtern, 4), 241); status != VMStatusOK {
		t.Fatalf("WriteUint8 failed: %s", status)
	}
	if value, status := memory.ReadUint8(makeAddress(segmentExtern, 4)); status != VMStatusOK || value != 241 {
		t.Fatalf("expected uint8 241, got %d, status=%s", value, status)
	}

	if status := memory.WriteInt16(makeAddress(segmentExtern, 6), -12345); status != VMStatusOK {
		t.Fatalf("WriteInt16 failed: %s", status)
	}
	if value, status := memory.ReadInt16(makeAddress(segmentExtern, 6)); status != VMStatusOK || value != -12345 {
		t.Fatalf("expected int16 -12345, got %d, status=%s", value, status)
	}
	if status := memory.WriteUint16(makeAddress(segmentExtern, 8), 54321); status != VMStatusOK {
		t.Fatalf("WriteUint16 failed: %s", status)
	}
	if value, status := memory.ReadUint16(makeAddress(segmentExtern, 8)); status != VMStatusOK || value != 54321 {
		t.Fatalf("expected uint16 54321, got %d, status=%s", value, status)
	}

	if status := memory.WriteInt32(makeAddress(segmentExtern, 12), -123456789); status != VMStatusOK {
		t.Fatalf("WriteInt32 failed: %s", status)
	}
	if value, status := memory.ReadInt32(makeAddress(segmentExtern, 12)); status != VMStatusOK || value != -123456789 {
		t.Fatalf("expected int32 -123456789, got %d, status=%s", value, status)
	}
	if status := memory.WriteUint32(makeAddress(segmentExtern, 16), 0xfedcba98); status != VMStatusOK {
		t.Fatalf("WriteUint32 failed: %s", status)
	}
	if value, status := memory.ReadUint32(makeAddress(segmentExtern, 16)); status != VMStatusOK || value != 0xfedcba98 {
		t.Fatalf("expected uint32 0xfedcba98, got %#x, status=%s", value, status)
	}
	if status := memory.WriteFloat32(makeAddress(segmentExtern, 20), -3.75); status != VMStatusOK {
		t.Fatalf("WriteFloat32 failed: %s", status)
	}
	if value, status := memory.ReadFloat32(makeAddress(segmentExtern, 20)); status != VMStatusOK || value != -3.75 {
		t.Fatalf("expected float32 -3.75, got %g, status=%s", value, status)
	}
	storedAddress := makeAddress(segmentData, 0x1234)
	if status := memory.WriteAddress(makeAddress(segmentExtern, 24), storedAddress); status != VMStatusOK {
		t.Fatalf("WriteAddress failed: %s", status)
	}
	if value, status := memory.ReadAddress(makeAddress(segmentExtern, 24)); status != VMStatusOK || value != storedAddress {
		t.Fatalf("expected address %#x, got %#x, status=%s", storedAddress, value, status)
	}

	if status := memory.WriteInt64(makeAddress(segmentExtern, 28), -0x0102030405060708); status != VMStatusOK {
		t.Fatalf("WriteInt64 failed: %s", status)
	}
	if value, status := memory.ReadInt64(makeAddress(segmentExtern, 28)); status != VMStatusOK || value != -0x0102030405060708 {
		t.Fatalf("expected signed 64-bit value, got %#x, status=%s", value, status)
	}
	if status := memory.WriteUint64(makeAddress(segmentExtern, 36), 0xfedcba9876543210); status != VMStatusOK {
		t.Fatalf("WriteUint64 failed: %s", status)
	}
	if value, status := memory.ReadUint64(makeAddress(segmentExtern, 36)); status != VMStatusOK || value != 0xfedcba9876543210 {
		t.Fatalf("expected uint64 0xfedcba9876543210, got %#x, status=%s", value, status)
	}
	if status := memory.WriteFloat64(makeAddress(segmentExtern, 44), 1.0/3.0); status != VMStatusOK {
		t.Fatalf("WriteFloat64 failed: %s", status)
	}
	if value, status := memory.ReadFloat64(makeAddress(segmentExtern, 44)); status != VMStatusOK || value != 1.0/3.0 {
		t.Fatalf("expected float64 1/3, got %.17g, status=%s", value, status)
	}
}

func TestVMAllocateExternMemoryCreatesOwnedExternSegment(t *testing.T) {
	vm := NewVM(8)
	vm.AllocateExternMemory(16)
	if len(vm.memory.segment[segmentExtern]) != 16 {
		t.Fatalf("expected owned extern memory size 16, got %d", len(vm.memory.segment[segmentExtern]))
	}
	if status := vm.memory.WriteInt32(makeAddress(segmentExtern, 4), 99); status != VMStatusOK {
		t.Fatalf("expected write into allocated extern memory to succeed: %s", status)
	}
}

func TestProgramMemoryTypedAccessRejectsInvalidAddresses(t *testing.T) {
	memory := NewProgramMemory(4, 0, 0, 0, 0, 0)
	invalidSegment := makeAddress(segmentInvalid, 0)
	outOfRange := makeAddress(segmentExtern, 1)

	if _, status := memory.ReadInt32(invalidSegment); status != VMStatusInvalidAddressSegment {
		t.Fatalf("expected invalid segment read status, got %s", status)
	}
	if status := memory.WriteInt32(invalidSegment, 1); status != VMStatusInvalidAddressSegment {
		t.Fatalf("expected invalid segment write status, got %s", status)
	}
	if _, status := memory.ReadUint32(outOfRange); status != VMStatusInvalidAddress {
		t.Fatalf("expected invalid address read status, got %s", status)
	}
	if status := memory.WriteUint32(outOfRange, 1); status != VMStatusInvalidAddress {
		t.Fatalf("expected invalid address write status, got %s", status)
	}
}

func TestProgramMemoryTypedWritesRejectConstSegment(t *testing.T) {
	memory := NewProgramMemory(0, 8, 0, 0, 0, 0)
	address := makeAddress(segmentConst, 0)
	writes := []struct {
		name  string
		write func() VMStatus
	}{
		{"bool", func() VMStatus { return memory.WriteBool(address, true) }},
		{"int8", func() VMStatus { return memory.WriteInt8(address, 1) }},
		{"int16", func() VMStatus { return memory.WriteInt16(address, 1) }},
		{"int32", func() VMStatus { return memory.WriteInt32(address, 1) }},
		{"int64", func() VMStatus { return memory.WriteInt64(address, 1) }},
		{"uint8", func() VMStatus { return memory.WriteUint8(address, 1) }},
		{"uint16", func() VMStatus { return memory.WriteUint16(address, 1) }},
		{"uint32", func() VMStatus { return memory.WriteUint32(address, 1) }},
		{"uint64", func() VMStatus { return memory.WriteUint64(address, 1) }},
		{"float32", func() VMStatus { return memory.WriteFloat32(address, 1) }},
		{"float64", func() VMStatus { return memory.WriteFloat64(address, 1) }},
		{"address", func() VMStatus { return memory.WriteAddress(address, makeAddress(segmentBSS, 0)) }},
	}
	for _, test := range writes {
		t.Run(test.name, func(t *testing.T) {
			if status := test.write(); status != VMStatusReadOnlyMemory {
				t.Fatalf("expected read-only memory status, got %s", status)
			}
		})
	}
}
