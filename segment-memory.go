package lcc

import (
	"encoding/binary"
	"errors"
	"fmt"
)

var (
	ErrInvalidAddressSegment = errors.New("invalid address segment")
	ErrWriteToConstSegment   = errors.New("cannot write to const segment")
)

type Address int

func makeAddress(segment memorySegment, index int) Address {
	return Address(int(segment)<<24 | (index & 0x00ffffff))
}

func (address Address) Segment() memorySegment {
	return memorySegment((int(address) >> 24) & 0xff)
}

func (address Address) Index() int {
	return int(address) & 0x00ffffff
}

type memorySegment byte

const (
	segmentInvalid   memorySegment = 0
	segmentFrame     memorySegment = 1
	segmentBSS       memorySegment = 2
	segmentExtern    memorySegment = 3
	segmentConst     memorySegment = 4
	segmentData      memorySegment = 5
	segmentStack     memorySegment = 6
	segmentReserved0 memorySegment = 7
	segmentReserved1 memorySegment = 8
	segmentCount     memorySegment = 9
)

var segmentNames = [segmentCount]string{
	segmentInvalid:   "invalid",
	segmentFrame:     "frame",
	segmentBSS:       "bss",
	segmentExtern:    "extern",
	segmentConst:     "const",
	segmentData:      "data",
	segmentStack:     "stack",
	segmentReserved0: "reserved",
	segmentReserved1: "reserved",
}

func (segment memorySegment) String() string {
	if segment >= segmentCount {
		return "invalid"
	}
	return segmentNames[segment]
}

type MemorySegment []byte

func NewMemorySegment(size int, capacity int) MemorySegment {
	capacity = max(capacity, size)
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
