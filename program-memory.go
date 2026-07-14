package cova

import "math"

type ProgramMemory struct {
	segment [segmentCount]MemorySegment
}

func hostIntFromUint32(value uint32) (int, bool) {
	converted := int(value)
	return converted, converted >= 0 && uint32(converted) == value
}

func NewProgramMemory(externCount, constCount, dataCount, bssCount, frameCapacity, stackCapacity uint32) ProgramMemory {
	pm := ProgramMemory{
		segment: [segmentCount]MemorySegment{
			segmentExtern: NewMemorySegment(externCount, externCount),
			segmentConst:  NewMemorySegment(constCount, constCount),
			segmentData:   NewMemorySegment(dataCount, dataCount),
			segmentBSS:    NewMemorySegment(bssCount, bssCount),
			segmentFrame:  NewMemorySegment(frameCapacity, frameCapacity),
			segmentStack:  NewMemorySegment(0, stackCapacity),
		},
	}
	return pm
}

func (memory *ProgramMemory) segmentForAddress(address Address) (*MemorySegment, VMStatus) {
	segment := address.Segment()
	if segment <= segmentInvalid || segment >= segmentCount {
		return nil, VMStatusInvalidAddressSegment
	}
	return &memory.segment[segment], VMStatusOK
}

func (memory *ProgramMemory) writableSegmentForAddress(address Address) (*MemorySegment, VMStatus) {
	if address.Segment() == segmentConst {
		return nil, VMStatusReadOnlyMemory
	}
	return memory.segmentForAddress(address)
}

func (memory *ProgramMemory) ReadBool(address Address) (bool, VMStatus) {
	value, status := memory.ReadUint8(address)
	return value != 0, status
}

func (memory *ProgramMemory) ReadInt8(address Address) (int8, VMStatus) {
	value, status := memory.ReadUint8(address)
	return int8(value), status
}

func (memory *ProgramMemory) ReadInt16(address Address) (int16, VMStatus) {
	value, status := memory.ReadUint16(address)
	return int16(value), status
}

func (memory *ProgramMemory) ReadInt32(address Address) (int32, VMStatus) {
	value, status := memory.ReadUint32(address)
	return int32(value), status
}

func (memory *ProgramMemory) ReadInt64(address Address) (int64, VMStatus) {
	value, status := memory.ReadUint64(address)
	return int64(value), status
}

func (memory *ProgramMemory) ReadUint8(address Address) (uint8, VMStatus) {
	segment, status := memory.segmentForAddress(address)
	if status != VMStatusOK {
		return 0, status
	}
	return segment.ReadUint8(address.Index())
}

func (memory *ProgramMemory) ReadUint16(address Address) (uint16, VMStatus) {
	segment, status := memory.segmentForAddress(address)
	if status != VMStatusOK {
		return 0, status
	}
	return segment.ReadUint16(address.Index())
}

func (memory *ProgramMemory) ReadUint32(address Address) (uint32, VMStatus) {
	segment, status := memory.segmentForAddress(address)
	if status != VMStatusOK {
		return 0, status
	}
	return segment.ReadUint32(address.Index())
}

func (memory *ProgramMemory) ReadUint64(address Address) (uint64, VMStatus) {
	segment, status := memory.segmentForAddress(address)
	if status != VMStatusOK {
		return 0, status
	}
	return segment.ReadUint64(address.Index())
}

func (memory *ProgramMemory) ReadFloat32(address Address) (float32, VMStatus) {
	bits, status := memory.ReadUint32(address)
	return math.Float32frombits(bits), status
}

func (memory *ProgramMemory) ReadFloat64(address Address) (float64, VMStatus) {
	bits, status := memory.ReadUint64(address)
	return math.Float64frombits(bits), status
}

func (memory *ProgramMemory) ReadAddress(address Address) (Address, VMStatus) {
	value, status := memory.ReadUint32(address)
	return Address(value), status
}

func (memory *ProgramMemory) WriteBool(address Address, value bool) VMStatus {
	if value {
		return memory.WriteUint8(address, 1)
	}
	return memory.WriteUint8(address, 0)
}

func (memory *ProgramMemory) WriteInt8(address Address, value int8) VMStatus {
	return memory.WriteUint8(address, uint8(value))
}

func (memory *ProgramMemory) WriteInt16(address Address, value int16) VMStatus {
	return memory.WriteUint16(address, uint16(value))
}

func (memory *ProgramMemory) WriteInt32(address Address, value int32) VMStatus {
	return memory.WriteUint32(address, uint32(value))
}

func (memory *ProgramMemory) WriteInt64(address Address, value int64) VMStatus {
	return memory.WriteUint64(address, uint64(value))
}

func (memory *ProgramMemory) WriteUint8(address Address, value uint8) VMStatus {
	segment, status := memory.writableSegmentForAddress(address)
	if status != VMStatusOK {
		return status
	}
	return segment.WriteUint8(address.Index(), value)
}

func (memory *ProgramMemory) WriteUint16(address Address, value uint16) VMStatus {
	segment, status := memory.writableSegmentForAddress(address)
	if status != VMStatusOK {
		return status
	}
	return segment.WriteUint16(address.Index(), value)
}

func (memory *ProgramMemory) WriteUint32(address Address, value uint32) VMStatus {
	segment, status := memory.writableSegmentForAddress(address)
	if status != VMStatusOK {
		return status
	}
	return segment.WriteUint32(address.Index(), value)
}

func (memory *ProgramMemory) WriteUint64(address Address, value uint64) VMStatus {
	segment, status := memory.writableSegmentForAddress(address)
	if status != VMStatusOK {
		return status
	}
	return segment.WriteUint64(address.Index(), value)
}

func (memory *ProgramMemory) WriteFloat32(address Address, value float32) VMStatus {
	return memory.WriteUint32(address, math.Float32bits(value))
}

func (memory *ProgramMemory) WriteFloat64(address Address, value float64) VMStatus {
	return memory.WriteUint64(address, math.Float64bits(value))
}

func (memory *ProgramMemory) WriteAddress(destination Address, value Address) VMStatus {
	return memory.WriteUint32(destination, uint32(value))
}
