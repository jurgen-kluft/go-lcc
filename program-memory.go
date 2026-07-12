package lcc

type ProgramMemory struct {
	segment [segmentCount]MemorySegment
}

func NewProgramMemory(externCount, bssCount, frameCapacity, stackCapacity int) ProgramMemory {
	pm := ProgramMemory{
		segment: [segmentCount]MemorySegment{
			segmentExtern: NewMemorySegment(externCount, externCount),
			segmentBSS:    NewMemorySegment(bssCount, bssCount),
			segmentFrame:  NewMemorySegment(frameCapacity, frameCapacity),
			segmentStack:  NewMemorySegment(0, stackCapacity),
		},
	}
	return pm
}

func (memory *ProgramMemory) segmentForAddress(address Address) *MemorySegment {
	return &memory.segment[address.Segment()]
}

func (memory *ProgramMemory) ReadBits(address Address, kind ValueKind) (uint64, error) {
	segment := memory.segmentForAddress(address)
	return segment.ReadBits(address.Index(), kind)
}

func (memory *ProgramMemory) WriteBits(address Address, kind ValueKind, bits uint64) error {
	segment := memory.segmentForAddress(address)
	return segment.WriteBits(address.Index(), kind, bits)
}
