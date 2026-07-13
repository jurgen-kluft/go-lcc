package lcc

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

func (memory *ProgramMemory) segmentForAddress(address Address) (*MemorySegment, error) {
	segment := address.Segment()
	if segment <= segmentInvalid || segment >= segmentCount {
		return nil, ErrInvalidAddressSegment
	}
	return &memory.segment[segment], nil
}

func (memory *ProgramMemory) ReadBits(address Address, kind ValueKind) (uint64, error) {
	segment, err := memory.segmentForAddress(address)
	if err != nil {
		return 0, err
	}
	return segment.ReadBits(address.Index(), kind)
}

func (memory *ProgramMemory) WriteBits(address Address, kind ValueKind, bits uint64) error {
	if address.Segment() == segmentConst {
		return ErrWriteToConstSegment
	}
	segment, err := memory.segmentForAddress(address)
	if err != nil {
		return err
	}
	return segment.WriteBits(address.Index(), kind, bits)
}
