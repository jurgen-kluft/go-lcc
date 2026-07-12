package lcc

import (
	"fmt"
)

type ProgramMemory struct {
	Extern MemorySegment
	BSS    MemorySegment
	Frame  MemorySegment
	Stack  MemorySegment
}

func NewProgramMemory(externCount, bssCount, frameCapacity, stackCapacity int) ProgramMemory {
	return ProgramMemory{
		Extern: NewMemorySegment(externCount, externCount),
		BSS:    NewMemorySegment(bssCount, bssCount),
		Frame:  NewMemorySegment(0, frameCapacity),
		Stack:  NewMemorySegment(0, stackCapacity),
	}
}

func (memory *ProgramMemory) segmentForAddress(address Address) (*MemorySegment, error) {
	if memory == nil {
		return nil, fmt.Errorf("memory is nil")
	}
	switch address.Segment() {
	case segmentExtern:
		return &memory.Extern, nil
	case segmentBSS:
		return &memory.BSS, nil
	case segmentFrame:
		return &memory.Frame, nil
	default:
		return nil, fmt.Errorf("invalid address segment %d", address.Segment())
	}
}

func (memory *ProgramMemory) ReadBits(address Address, kind ValueKind) (uint64, error) {
	segment, err := memory.segmentForAddress(address)
	if err != nil {
		return 0, err
	}
	return segment.ReadBits(address.Index(), kind)
}

func (memory *ProgramMemory) WriteBits(address Address, kind ValueKind, bits uint64) error {
	segment, err := memory.segmentForAddress(address)
	if err != nil {
		return err
	}
	return segment.WriteBits(address.Index(), kind, bits)
}
