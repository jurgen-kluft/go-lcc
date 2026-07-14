package cova

import "testing"

func TestMemorySegmentExactWidthReadWrite(t *testing.T) {
	segment := NewMemorySegment(16, 16)
	if status := segment.WriteUint8(0, 255); status != VMStatusOK {
		t.Fatalf("WriteUint8 failed: %s", status)
	}
	if status := segment.WriteUint32(4, 0x12345678); status != VMStatusOK {
		t.Fatalf("WriteUint32 failed: %s", status)
	}
	if status := segment.WriteUint64(8, 0x0102030405060708); status != VMStatusOK {
		t.Fatalf("WriteUint64 failed: %s", status)
	}

	if value, status := segment.ReadUint8(0); status != VMStatusOK || value != 255 {
		t.Fatalf("expected byte 255, got %d, status=%s", value, status)
	}
	if value, status := segment.ReadUint32(4); status != VMStatusOK || value != 0x12345678 {
		t.Fatalf("expected uint32 0x12345678, got %#x, status=%s", value, status)
	}
	if value, status := segment.ReadUint64(8); status != VMStatusOK || value != 0x0102030405060708 {
		t.Fatalf("expected uint64 0x0102030405060708, got %#x, status=%s", value, status)
	}
}

func TestMemorySegmentAppendAndTruncateBits(t *testing.T) {
	segment := NewMemorySegment(0, 16)
	if status := segment.AppendBits(KindByte, 255); status != VMStatusOK {
		t.Fatalf("AppendBits byte failed: %s", status)
	}
	if status := segment.AppendBits(KindInt32, uint64(uint32(42))); status != VMStatusOK {
		t.Fatalf("AppendBits int32 failed: %s", status)
	}
	if status := segment.AppendBits(KindInt64, uint64(99)); status != VMStatusOK {
		t.Fatalf("AppendBits int64 failed: %s", status)
	}

	if bits, status := segment.TruncateBits(KindInt64); status != VMStatusOK || bits != 99 {
		t.Fatalf("expected int64 bits 99, got %d, status=%s", bits, status)
	}
	if bits, status := segment.TruncateBits(KindInt32); status != VMStatusOK || int32(bits) != 42 {
		t.Fatalf("expected int32 bits 42, got %d, status=%s", bits, status)
	}
	if bits, status := segment.TruncateBits(KindByte); status != VMStatusOK || bits != 255 {
		t.Fatalf("expected byte bits 255, got %d, status=%s", bits, status)
	}
	if _, status := segment.TruncateBits(KindByte); status != VMStatusStackUnderflow {
		t.Fatalf("expected stack underflow status, got %s", status)
	}
}

func TestMemorySegmentAppendRejectsCapacityOverflow(t *testing.T) {
	segment := NewMemorySegment(0, 4)
	if status := segment.AppendBits(KindInt32, 42); status != VMStatusOK {
		t.Fatalf("AppendBits within capacity failed: %s", status)
	}
	if status := segment.AppendBits(KindByte, 1); status != VMStatusStackOverflow {
		t.Fatalf("expected stack overflow status, got %s", status)
	}
	if len(segment) != 4 {
		t.Fatalf("expected failed append to preserve length 4, got %d", len(segment))
	}
}

func TestMemorySegmentExactWidthAppendAndTruncate(t *testing.T) {
	segment := NewMemorySegment(0, 15)
	if status := segment.AppendUint8(0x12); status != VMStatusOK || len(segment) != 1 {
		t.Fatalf("AppendUint8 failed: status=%s len=%d", status, len(segment))
	}
	if status := segment.AppendUint16(0x3456); status != VMStatusOK || len(segment) != 3 {
		t.Fatalf("AppendUint16 failed: status=%s len=%d", status, len(segment))
	}
	if status := segment.AppendUint32(0x789abcde); status != VMStatusOK || len(segment) != 7 {
		t.Fatalf("AppendUint32 failed: status=%s len=%d", status, len(segment))
	}
	if status := segment.AppendUint64(0x0123456789abcdef); status != VMStatusOK || len(segment) != 15 {
		t.Fatalf("AppendUint64 failed: status=%s len=%d", status, len(segment))
	}

	if value, status := segment.TruncateUint64(); status != VMStatusOK || value != 0x0123456789abcdef || len(segment) != 7 {
		t.Fatalf("TruncateUint64 failed: value=%#x status=%s len=%d", value, status, len(segment))
	}
	if value, status := segment.TruncateUint32(); status != VMStatusOK || value != 0x789abcde || len(segment) != 3 {
		t.Fatalf("TruncateUint32 failed: value=%#x status=%s len=%d", value, status, len(segment))
	}
	if value, status := segment.TruncateUint16(); status != VMStatusOK || value != 0x3456 || len(segment) != 1 {
		t.Fatalf("TruncateUint16 failed: value=%#x status=%s len=%d", value, status, len(segment))
	}
	if value, status := segment.TruncateUint8(); status != VMStatusOK || value != 0x12 || len(segment) != 0 {
		t.Fatalf("TruncateUint8 failed: value=%#x status=%s len=%d", value, status, len(segment))
	}
}
