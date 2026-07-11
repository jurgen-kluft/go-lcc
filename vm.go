package lcc

import "fmt"

type VM struct {
	Memory        FlatMemory
	stack         []int
	ip            int
	program       *LinkedProgram
	functions     map[int]HostFunction
	functionTable map[int]GlobalBinding
	activeArgs    []int
	returnValue   int
	LastResult    int
}

func NewVM(globalCount, localCount int) *VM {
	return &VM{
		Memory:        NewFlatMemory(globalCount, localCount),
		stack:         make([]int, 0, 32),
		functions:     make(map[int]HostFunction),
		functionTable: make(map[int]GlobalBinding),
	}
}

func (vm *VM) BindGlobal(index int, target *int) error {
	if index < 0 || index >= len(vm.Memory.Globals) {
		return fmt.Errorf("vm error: global slot %d out of range", index)
	}
	vm.Memory.Globals[index].Bound = target
	return nil
}

func (vm *VM) RegisterFunction(index int, fn HostFunction) {
	vm.functions[index] = fn
}

func (vm *VM) Arg(index int) int {
	if index < 0 || index >= len(vm.activeArgs) {
		return 0
	}
	return vm.activeArgs[index]
}

func (vm *VM) ArgCount() int {
	return len(vm.activeArgs)
}

func (vm *VM) SetReturnValue(value int) {
	vm.returnValue = value
}

func (vm *VM) Run(program *LinkedProgram) error {
	if program == nil {
		return fmt.Errorf("vm error: linked program is nil")
	}
	if len(vm.Memory.Locals) < program.LocalSlotCount {
		vm.Memory.Locals = make([]MemorySlot, program.LocalSlotCount)
	}

	vm.program = program
	vm.ip = 0
	vm.stack = vm.stack[:0]
	vm.LastResult = 0
	vm.functionTable = make(map[int]GlobalBinding, len(program.GlobalFunctions))
	for _, binding := range program.GlobalFunctions {
		vm.functionTable[binding.Index] = binding
	}

	for vm.ip < len(program.Code) {
		op := Opcode(program.Code[vm.ip])
		vm.ip++

		switch op {
		case OpPush:
			vm.push(readInt(program.Code, &vm.ip))
		case OpAdd:
			right, left, err := vm.popBinary()
			if err != nil {
				return err
			}
			vm.push(left + right)
		case OpSub:
			right, left, err := vm.popBinary()
			if err != nil {
				return err
			}
			vm.push(left - right)
		case OpMul:
			right, left, err := vm.popBinary()
			if err != nil {
				return err
			}
			vm.push(left * right)
		case OpDiv:
			right, left, err := vm.popBinary()
			if err != nil {
				return err
			}
			if right == 0 {
				return fmt.Errorf("vm error: division by zero")
			}
			vm.push(left / right)
		case OpAddrLocal:
			vm.push(int(makeAddress(segmentLocal, readInt(program.Code, &vm.ip))))
		case OpAddrGlobalIdx:
			vm.push(int(makeAddress(segmentGlobal, readInt(program.Code, &vm.ip))))
		case OpOffset:
			offset, err := vm.pop()
			if err != nil {
				return err
			}
			base, err := vm.pop()
			if err != nil {
				return err
			}
			address := Address(base)
			vm.push(int(makeAddress(address.Segment(), address.Index()+offset)))
		case OpDereference:
			encodedAddress, err := vm.pop()
			if err != nil {
				return err
			}
			slot, err := vm.Memory.Slot(Address(encodedAddress))
			if err != nil {
				return err
			}
			vm.push(slot.Load())
		case OpAssign:
			encodedAddress, err := vm.pop()
			if err != nil {
				return err
			}
			value, err := vm.pop()
			if err != nil {
				return err
			}
			slot, err := vm.Memory.Slot(Address(encodedAddress))
			if err != nil {
				return err
			}
			slot.Store(value)
		case OpJumpIfFalse:
			target := readInt(program.Code, &vm.ip)
			condition, err := vm.pop()
			if err != nil {
				return err
			}
			if condition == 0 {
				vm.ip = target
			}
		case OpCallGlobalIdx:
			index := readInt(program.Code, &vm.ip)
			binding, ok := vm.functionTable[index]
			if !ok {
				return fmt.Errorf("vm error: no linked function binding at index %d", index)
			}
			fn, ok := vm.functions[index]
			if !ok {
				return fmt.Errorf("vm error: no host function registered at index %d", index)
			}
			args := make([]int, binding.Arity)
			for i := binding.Arity - 1; i >= 0; i-- {
				value, err := vm.pop()
				if err != nil {
					return err
				}
				args[i] = value
			}
			vm.activeArgs = args
			vm.returnValue = 0
			if err := fn(vm); err != nil {
				return err
			}
			vm.activeArgs = nil
			if binding.Type != nil && binding.Type.Kind != TypeVoid {
				vm.push(vm.returnValue)
			}
		case OpRet:
			if len(vm.stack) > 0 {
				value, _ := vm.pop()
				vm.LastResult = value
			}
			return nil
		default:
			return fmt.Errorf("vm error: unknown opcode %d at ip %d", op, vm.ip-1)
		}
	}

	return nil
}

func (vm *VM) push(value int) {
	vm.stack = append(vm.stack, value)
}

func (vm *VM) pop() (int, error) {
	if len(vm.stack) == 0 {
		return 0, fmt.Errorf("vm error: stack underflow")
	}
	index := len(vm.stack) - 1
	value := vm.stack[index]
	vm.stack = vm.stack[:index]
	return value, nil
}

func (vm *VM) popBinary() (right int, left int, err error) {
	right, err = vm.pop()
	if err != nil {
		return 0, 0, err
	}
	left, err = vm.pop()
	if err != nil {
		return 0, 0, err
	}
	return right, left, nil
}
