package lcc

import "fmt"

type Linker struct {
	VariableCapacity int
	FunctionCapacity int
}

func NewLinker(variableCapacity, functionCapacity int) *Linker {
	return &Linker{VariableCapacity: variableCapacity, FunctionCapacity: functionCapacity}
}

func (linker *Linker) Link(program *ProgramNode, compiled *CompiledProgram) (*LinkedProgram, error) {
	if program == nil {
		return nil, fmt.Errorf("link error: program is nil")
	}
	if compiled == nil {
		return nil, fmt.Errorf("link error: compiled program is nil")
	}
	if linker == nil {
		return nil, fmt.Errorf("link error: linker is nil")
	}

	for _, decl := range program.Globals {
		switch decl.Kind {
		case GlobalVariable:
			if decl.Index < 0 || decl.Index >= linker.VariableCapacity {
				return nil, fmt.Errorf("link error: global variable %q requests slot %d, but variable capacity is %d", decl.Name, decl.Index, linker.VariableCapacity)
			}
		case GlobalFunction:
			if decl.Index < 0 || decl.Index >= linker.FunctionCapacity {
				return nil, fmt.Errorf("link error: global function %q requests slot %d, but function capacity is %d", decl.Name, decl.Index, linker.FunctionCapacity)
			}
		default:
			return nil, fmt.Errorf("link error: global contract %q has unknown kind %d", decl.Name, decl.Kind)
		}
	}

	linked := &LinkedProgram{
		Code:            append([]byte(nil), compiled.Code...),
		Globals:         compiled.Globals,
		GlobalVars:      append([]GlobalBinding(nil), compiled.GlobalVars...),
		GlobalFunctions: append([]GlobalBinding(nil), compiled.GlobalFunctions...),
		Entry:           compiled.Entry,
		LocalSlotCount:  compiled.LocalSlotCount,
	}
	return linked, nil
}
