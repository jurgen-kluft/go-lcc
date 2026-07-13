package cova

type ProgramMemory struct {
	segment [segmentCount]MemorySegment
}

func NewProgramMemory(externCount, constCount, dataCount, bssCount, frameCapacity, stackCapacity int) ProgramMemory {
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

func (memory *ProgramMemory) ReadBits(address Address, kind ValueKind) (uint64, VMStatus) {
	segment, status := memory.segmentForAddress(address)
	if status != VMStatusOK {
		return 0, status
	}
	return segment.ReadBits(address.Index(), kind)
}

func (memory *ProgramMemory) WriteBits(address Address, kind ValueKind, bits uint64) VMStatus {
	if address.Segment() == segmentConst {
		return VMStatusReadOnlyMemory
	}
	segment, status := memory.segmentForAddress(address)
	if status != VMStatusOK {
		return status
	}
	return segment.WriteBits(address.Index(), kind, bits)
}
