package lcc

import "fmt"

type Linker struct {
	VariableCapacity int
	FunctionCapacity int
}

func NewLinker(variableCapacity, functionCapacity int) *Linker {
	return &Linker{VariableCapacity: variableCapacity, FunctionCapacity: functionCapacity}
}

func (linker *Linker) Link(program *ProgramNode, compiled *RelocatableProgram) (*LinkedProgram, error) {
	if program == nil {
		return nil, fmt.Errorf("link error: program is nil")
	}
	if compiled == nil {
		return nil, fmt.Errorf("link error: compiled program is nil")
	}
	if linker == nil {
		return nil, fmt.Errorf("link error: linker is nil")
	}

	for _, binding := range compiled.ProgramSymbols.ExternSymbols {
		if binding.ByteOffset < 0 || binding.ByteOffset+binding.ByteSize > linker.VariableCapacity {
			return nil, fmt.Errorf("link error: extern variable %q requests byte range [%d,%d), but extern memory capacity is %d", binding.Name, binding.ByteOffset, binding.ByteOffset+binding.ByteSize, linker.VariableCapacity)
		}
		if binding.ByteAlignment > 1 && binding.ByteOffset%binding.ByteAlignment != 0 {
			return nil, fmt.Errorf("link error: extern variable %q byte offset %d is not aligned to %d", binding.Name, binding.ByteOffset, binding.ByteAlignment)
		}
	}

	tempToAddress := make(map[int]int, len(compiled.Functions))
	for _, binding := range compiled.Functions {
		switch binding.Scope {
		case ScopeExtern:
			if binding.SlotIndex < 0 || binding.SlotIndex >= linker.FunctionCapacity {
				return nil, fmt.Errorf("link error: host-linked function %q requests slot %d, but function capacity is %d", binding.Name, binding.SlotIndex, linker.FunctionCapacity)
			}
		case ScopeBSS:
			tempToAddress[binding.TempFuncID] = binding.ScriptAddress
		default:
			return nil, fmt.Errorf("link error: function %q has invalid scope %d", binding.Name, binding.Scope)
		}
	}

	linkedText := compiled.Text.Clone()
	for _, patch := range compiled.CallPatches {
		address, ok := tempToAddress[patch.TempFuncID]
		if !ok {
			return nil, fmt.Errorf("link error on line %d: unresolved function id %d", patch.Line, patch.TempFuncID)
		}
		linkedText.PatchInt(patch.OperandPos, address)
	}

	entryPoint, ok := tempToAddress[compiled.EntryFunction]
	if !ok {
		return nil, fmt.Errorf("link error: entry function id %d was not finalized", compiled.EntryFunction)
	}

	linked := &LinkedProgram{
		Text:          linkedText,
		EntryPoint:    entryPoint,
		FrameSize:     compiled.FrameSize,
		FrameByteSize: compiled.FrameByteSize,
		ConstByteSize: compiled.ConstByteSize,
		ConstData:     append([]byte(nil), compiled.ConstData...),
		DataByteSize:  compiled.DataByteSize,
		DataData:      append([]byte(nil), compiled.DataData...),
		BSSSize:       compiled.BSSSize,
		BSSByteSize:   compiled.BSSByteSize,
		DebugSymbols:  CopyProgramSymbols(compiled.ProgramSymbols),
	}
	return linked, nil
}
