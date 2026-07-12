package lcc

import (
	"encoding/binary"
	"fmt"
)

type memorySegment byte

const (
	segmentInvalid memorySegment = iota
	segmentFrame
	segmentBSS
	segmentExtern
)

func (segment memorySegment) String() string {
	switch segment {
	case segmentFrame:
		return "frame"
	case segmentBSS:
		return "bss"
	case segmentExtern:
		return "extern"
	default:
		return "invalid"
	}
}

type MemorySegment []byte

func NewMemorySegment(size int, capacity int) MemorySegment {
	if capacity < size {
		capacity = size
	}
	return make([]byte, size, capacity)
}

func (segment MemorySegment) ReadBits(offset int, kind ValueKind) (uint64, error) {
	size := kind.Size()
	if size == 0 {
		return 0, fmt.Errorf("unsupported value kind %d", kind)
	}
	if offset < 0 || offset+size > len(segment) {
		return 0, fmt.Errorf("memory segment offset %d out of range for kind %d", offset, kind)
	}
	switch size {
	case 1:
		return uint64(segment[offset]), nil
	case 2:
		return uint64(binary.LittleEndian.Uint16(segment[offset:])), nil
	case 4:
		return uint64(binary.LittleEndian.Uint32(segment[offset:])), nil
	case 8:
		return binary.LittleEndian.Uint64(segment[offset:]), nil
	default:
		return 0, fmt.Errorf("unsupported value size %d", size)
	}
}

func (segment *MemorySegment) WriteBits(offset int, kind ValueKind, bits uint64) error {
	if segment == nil {
		return fmt.Errorf("memory segment is nil")
	}
	size := kind.Size()
	if size == 0 {
		return fmt.Errorf("unsupported value kind %d", kind)
	}
	bytes := *segment
	if offset < 0 || offset+size > len(bytes) {
		return fmt.Errorf("memory segment offset %d out of range for kind %d", offset, kind)
	}
	switch size {
	case 1:
		bytes[offset] = byte(bits)
	case 2:
		binary.LittleEndian.PutUint16(bytes[offset:], uint16(bits))
	case 4:
		binary.LittleEndian.PutUint32(bytes[offset:], uint32(bits))
	case 8:
		binary.LittleEndian.PutUint64(bytes[offset:], bits)
	default:
		return fmt.Errorf("unsupported value size %d", size)
	}
	return nil
}

func (segment *MemorySegment) AppendBits(kind ValueKind, bits uint64) error {
	if segment == nil {
		return fmt.Errorf("memory segment is nil")
	}
	size := kind.Size()
	if size == 0 {
		return fmt.Errorf("unsupported stack value kind %d", kind)
	}
	base := len(*segment)
	*segment = append(*segment, make([]byte, size)...)
	return segment.WriteBits(base, kind, bits)
}

func (segment *MemorySegment) TruncateBits(kind ValueKind) (uint64, error) {
	if segment == nil {
		return 0, fmt.Errorf("memory segment is nil")
	}
	size := kind.Size()
	if size == 0 {
		return 0, fmt.Errorf("unsupported stack value kind %d", kind)
	}
	bytes := *segment
	if len(bytes) < size {
		return 0, fmt.Errorf("vm error: stack underflow")
	}
	offset := len(bytes) - size
	bits, err := MemorySegment(bytes).ReadBits(offset, kind)
	if err != nil {
		return 0, err
	}
	*segment = bytes[:offset]
	return bits, nil
}
