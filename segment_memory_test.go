package lcc

import "testing"

func TestMemorySegmentReadWriteBits(t *testing.T) {
	segment := NewMemorySegment(16, 16)
	if err := segment.WriteBits(0, KindByte, 255); err != nil {
		t.Fatalf("WriteBits byte failed: %v", err)
	}
	if err := segment.WriteBits(4, KindInt32, uint64(uint32(0x12345678))); err != nil {
		t.Fatalf("WriteBits int32 failed: %v", err)
	}
	if err := segment.WriteBits(8, KindInt64, uint64(0x0102030405060708)); err != nil {
		t.Fatalf("WriteBits int64 failed: %v", err)
	}

	if bits, err := segment.ReadBits(0, KindByte); err != nil || bits != 255 {
		t.Fatalf("expected byte 255, got %d, err=%v", bits, err)
	}
	if bits, err := segment.ReadBits(4, KindInt32); err != nil || uint32(bits) != 0x12345678 {
		t.Fatalf("expected int32 bits 0x12345678, got %#x, err=%v", uint32(bits), err)
	}
	if bits, err := segment.ReadBits(8, KindInt64); err != nil || bits != 0x0102030405060708 {
		t.Fatalf("expected int64 bits 0x0102030405060708, got %#x, err=%v", bits, err)
	}
}

func TestMemorySegmentAppendAndTruncateBits(t *testing.T) {
	segment := NewMemorySegment(0, 16)
	if err := segment.AppendBits(KindByte, 255); err != nil {
		t.Fatalf("AppendBits byte failed: %v", err)
	}
	if err := segment.AppendBits(KindInt32, uint64(uint32(42))); err != nil {
		t.Fatalf("AppendBits int32 failed: %v", err)
	}
	if err := segment.AppendBits(KindInt64, uint64(99)); err != nil {
		t.Fatalf("AppendBits int64 failed: %v", err)
	}

	if bits, err := segment.TruncateBits(KindInt64); err != nil || bits != 99 {
		t.Fatalf("expected int64 bits 99, got %d, err=%v", bits, err)
	}
	if bits, err := segment.TruncateBits(KindInt32); err != nil || int32(bits) != 42 {
		t.Fatalf("expected int32 bits 42, got %d, err=%v", bits, err)
	}
	if bits, err := segment.TruncateBits(KindByte); err != nil || bits != 255 {
		t.Fatalf("expected byte bits 255, got %d, err=%v", bits, err)
	}
	if _, err := segment.TruncateBits(KindByte); err == nil {
		t.Fatal("expected stack underflow error")
	}
}
