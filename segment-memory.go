package cova

import (
	"encoding/binary"
)

type Address uint32

const addressIndexMask uint32 = 0x00ffffff

func makeAddress(segment memorySegment, index uint32) Address {
	return Address(uint32(segment)<<24 | (index & addressIndexMask))
}

func (address Address) Segment() memorySegment {
	return memorySegment((uint32(address) >> 24) & 0xff)
}

func (address Address) Index() uint32 {
	return uint32(address) & addressIndexMask
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

func NewMemorySegment(size uint32, capacity uint32) MemorySegment {
	capacity = max(capacity, size)
	return make([]byte, size, capacity)
}

func (segment MemorySegment) ReadUint8(offset uint32) (uint8, VMStatus) {
	if uint64(offset)+1 > uint64(len(segment)) {
		return 0, VMStatusInvalidAddress
	}
	return segment[offset], VMStatusOK
}

func (segment MemorySegment) ReadUint16(offset uint32) (uint16, VMStatus) {
	if uint64(offset)+2 > uint64(len(segment)) {
		return 0, VMStatusInvalidAddress
	}
	return binary.LittleEndian.Uint16(segment[offset:]), VMStatusOK
}

func (segment MemorySegment) ReadUint32(offset uint32) (uint32, VMStatus) {
	if uint64(offset)+4 > uint64(len(segment)) {
		return 0, VMStatusInvalidAddress
	}
	return binary.LittleEndian.Uint32(segment[offset:]), VMStatusOK
}

func (segment MemorySegment) ReadUint64(offset uint32) (uint64, VMStatus) {
	if uint64(offset)+8 > uint64(len(segment)) {
		return 0, VMStatusInvalidAddress
	}
	return binary.LittleEndian.Uint64(segment[offset:]), VMStatusOK
}

func (segment *MemorySegment) WriteUint8(offset uint32, value uint8) VMStatus {
	if segment == nil {
		return VMStatusInvalidAddress
	}
	if uint64(offset)+1 > uint64(len(*segment)) {
		return VMStatusInvalidAddress
	}
	(*segment)[offset] = value
	return VMStatusOK
}

func (segment *MemorySegment) WriteUint16(offset uint32, value uint16) VMStatus {
	if segment == nil {
		return VMStatusInvalidAddress
	}
	if uint64(offset)+2 > uint64(len(*segment)) {
		return VMStatusInvalidAddress
	}
	binary.LittleEndian.PutUint16((*segment)[offset:], value)
	return VMStatusOK
}

func (segment *MemorySegment) WriteUint32(offset uint32, value uint32) VMStatus {
	if segment == nil {
		return VMStatusInvalidAddress
	}
	if uint64(offset)+4 > uint64(len(*segment)) {
		return VMStatusInvalidAddress
	}
	binary.LittleEndian.PutUint32((*segment)[offset:], value)
	return VMStatusOK
}

func (segment *MemorySegment) WriteUint64(offset uint32, value uint64) VMStatus {
	if segment == nil {
		return VMStatusInvalidAddress
	}
	if uint64(offset)+8 > uint64(len(*segment)) {
		return VMStatusInvalidAddress
	}
	binary.LittleEndian.PutUint64((*segment)[offset:], value)
	return VMStatusOK
}

func (segment *MemorySegment) growForAppend(size uint32) (uint32, VMStatus) {
	if segment == nil {
		return 0, VMStatusInvalidAddress
	}
	base := uint32(len(*segment))
	if uint64(base)+uint64(size) > uint64(cap(*segment)) {
		return 0, VMStatusStackOverflow
	}
	*segment = (*segment)[:base+size]
	return base, VMStatusOK
}

func (segment *MemorySegment) AppendUint8(value uint8) VMStatus {
	base, status := segment.growForAppend(1)
	if status != VMStatusOK {
		return status
	}
	return segment.WriteUint8(base, value)
}

func (segment *MemorySegment) AppendUint16(value uint16) VMStatus {
	base, status := segment.growForAppend(2)
	if status != VMStatusOK {
		return status
	}
	return segment.WriteUint16(base, value)
}

func (segment *MemorySegment) AppendUint32(value uint32) VMStatus {
	base, status := segment.growForAppend(4)
	if status != VMStatusOK {
		return status
	}
	return segment.WriteUint32(base, value)
}

func (segment *MemorySegment) AppendUint64(value uint64) VMStatus {
	base, status := segment.growForAppend(8)
	if status != VMStatusOK {
		return status
	}
	return segment.WriteUint64(base, value)
}

func (segment *MemorySegment) AppendFrom(source MemorySegment, offset uint32, size uint32) VMStatus {
	if size == 0 {
		return VMStatusInvalidValueKind
	}
	if uint64(offset)+uint64(size) > uint64(len(source)) {
		return VMStatusInvalidAddress
	}
	base, status := segment.growForAppend(size)
	if status != VMStatusOK {
		return status
	}
	copy((*segment)[base:], source[offset:offset+size])
	return VMStatusOK
}

func (segment *MemorySegment) TruncateTo(destination *MemorySegment, offset uint32, size uint32) VMStatus {
	if segment == nil || destination == nil {
		return VMStatusInvalidAddress
	}
	if size == 0 {
		return VMStatusInvalidValueKind
	}
	if uint64(offset)+uint64(size) > uint64(len(*destination)) {
		return VMStatusInvalidAddress
	}
	sourceOffset, status := segment.truncateOffset(size)
	if status != VMStatusOK {
		return status
	}
	copy((*destination)[offset:offset+size], (*segment)[sourceOffset:])
	*segment = (*segment)[:sourceOffset]
	return VMStatusOK
}

func (segment *MemorySegment) AppendBits(kind ValueKind, bits uint64) VMStatus {
	if segment == nil {
		return VMStatusInvalidAddress
	}
	switch kind.Size() {
	case 1:
		return segment.AppendUint8(uint8(bits))
	case 2:
		return segment.AppendUint16(uint16(bits))
	case 4:
		return segment.AppendUint32(uint32(bits))
	case 8:
		return segment.AppendUint64(bits)
	default:
		return VMStatusInvalidValueKind
	}
}

func (segment *MemorySegment) truncateOffset(size uint32) (uint32, VMStatus) {
	if segment == nil {
		return 0, VMStatusInvalidAddress
	}
	if uint64(len(*segment)) < uint64(size) {
		return 0, VMStatusStackUnderflow
	}
	return uint32(len(*segment)) - size, VMStatusOK
}

func (segment *MemorySegment) TruncateUint8() (uint8, VMStatus) {
	offset, status := segment.truncateOffset(1)
	if status != VMStatusOK {
		return 0, status
	}
	value, status := MemorySegment(*segment).ReadUint8(offset)
	if status == VMStatusOK {
		*segment = (*segment)[:offset]
	}
	return value, status
}

func (segment *MemorySegment) TruncateUint16() (uint16, VMStatus) {
	offset, status := segment.truncateOffset(2)
	if status != VMStatusOK {
		return 0, status
	}
	value, status := MemorySegment(*segment).ReadUint16(offset)
	if status == VMStatusOK {
		*segment = (*segment)[:offset]
	}
	return value, status
}

func (segment *MemorySegment) TruncateUint32() (uint32, VMStatus) {
	offset, status := segment.truncateOffset(4)
	if status != VMStatusOK {
		return 0, status
	}
	value, status := MemorySegment(*segment).ReadUint32(offset)
	if status == VMStatusOK {
		*segment = (*segment)[:offset]
	}
	return value, status
}

func (segment *MemorySegment) TruncateUint64() (uint64, VMStatus) {
	offset, status := segment.truncateOffset(8)
	if status != VMStatusOK {
		return 0, status
	}
	value, status := MemorySegment(*segment).ReadUint64(offset)
	if status == VMStatusOK {
		*segment = (*segment)[:offset]
	}
	return value, status
}

func (segment *MemorySegment) TruncateBits(kind ValueKind) (uint64, VMStatus) {
	if segment == nil {
		return 0, VMStatusInvalidAddress
	}
	switch kind.Size() {
	case 1:
		value, status := segment.TruncateUint8()
		return uint64(value), status
	case 2:
		value, status := segment.TruncateUint16()
		return uint64(value), status
	case 4:
		value, status := segment.TruncateUint32()
		return uint64(value), status
	case 8:
		return segment.TruncateUint64()
	default:
		return 0, VMStatusInvalidValueKind
	}
}
