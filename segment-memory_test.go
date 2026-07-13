package cova

import "testing"

func TestMemorySegmentReadWriteBits(t *testing.T) {
	segment := NewMemorySegment(16, 16)
	if status := segment.WriteBits(0, KindByte, 255); status != VMStatusOK {
		t.Fatalf("WriteBits byte failed: %s", status)
	}
	if status := segment.WriteBits(4, KindInt32, uint64(uint32(0x12345678))); status != VMStatusOK {
		t.Fatalf("WriteBits int32 failed: %s", status)
	}
	if status := segment.WriteBits(8, KindInt64, uint64(0x0102030405060708)); status != VMStatusOK {
		t.Fatalf("WriteBits int64 failed: %s", status)
	}

	if bits, status := segment.ReadBits(0, KindByte); status != VMStatusOK || bits != 255 {
		t.Fatalf("expected byte 255, got %d, status=%s", bits, status)
	}
	if bits, status := segment.ReadBits(4, KindInt32); status != VMStatusOK || uint32(bits) != 0x12345678 {
		t.Fatalf("expected int32 bits 0x12345678, got %#x, status=%s", uint32(bits), status)
	}
	if bits, status := segment.ReadBits(8, KindInt64); status != VMStatusOK || bits != 0x0102030405060708 {
		t.Fatalf("expected int64 bits 0x0102030405060708, got %#x, status=%s", bits, status)
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
