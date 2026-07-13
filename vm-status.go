package cova

// VMStatus is the stable numeric status returned by the portable VM runtime.
// Values are part of the host ABI and must not be reordered or reused.
type VMStatus uint32

const (
	VMStatusOK VMStatus = 0

	VMStatusNoProgramLoaded       VMStatus = 1
	VMStatusInvalidLifecycle      VMStatus = 2
	VMStatusInvalidProgram        VMStatus = 3
	VMStatusInvalidImage          VMStatus = 4
	VMStatusUnsupportedImage      VMStatus = 5
	VMStatusMalformedBytecode     VMStatus = 6
	VMStatusInvalidDescriptor     VMStatus = 7
	VMStatusInvalidParameter      VMStatus = 8
	VMStatusInvalidValueKind      VMStatus = 9
	VMStatusInvalidAddress        VMStatus = 10
	VMStatusInvalidAddressSegment VMStatus = 11
	VMStatusReadOnlyMemory        VMStatus = 12
	VMStatusInvalidTarget         VMStatus = 13
	VMStatusInvalidOpcode         VMStatus = 14
	VMStatusStackUnderflow        VMStatus = 15
	VMStatusStackOverflow         VMStatus = 16
	VMStatusFrameOverflow         VMStatus = 17
	VMStatusCallFrameOverflow     VMStatus = 18
	VMStatusDivisionByZero        VMStatus = 19
	VMStatusMissingExtern         VMStatus = 20
	VMStatusExternABIViolation    VMStatus = 21
	VMStatusHostFailure           VMStatus = 22
)

func (status VMStatus) String() string {
	switch status {
	case VMStatusOK:
		return "ok"
	case VMStatusNoProgramLoaded:
		return "no program loaded"
	case VMStatusInvalidLifecycle:
		return "invalid lifecycle"
	case VMStatusInvalidProgram:
		return "invalid program"
	case VMStatusInvalidImage:
		return "invalid image"
	case VMStatusUnsupportedImage:
		return "unsupported image"
	case VMStatusMalformedBytecode:
		return "malformed bytecode"
	case VMStatusInvalidDescriptor:
		return "invalid descriptor"
	case VMStatusInvalidParameter:
		return "invalid parameter metadata"
	case VMStatusInvalidValueKind:
		return "invalid value kind"
	case VMStatusInvalidAddress:
		return "invalid address"
	case VMStatusInvalidAddressSegment:
		return "invalid address segment"
	case VMStatusReadOnlyMemory:
		return "read-only memory"
	case VMStatusInvalidTarget:
		return "invalid target"
	case VMStatusInvalidOpcode:
		return "invalid opcode"
	case VMStatusStackUnderflow:
		return "stack underflow"
	case VMStatusStackOverflow:
		return "stack overflow"
	case VMStatusFrameOverflow:
		return "frame overflow"
	case VMStatusCallFrameOverflow:
		return "call-frame overflow"
	case VMStatusDivisionByZero:
		return "division by zero"
	case VMStatusMissingExtern:
		return "missing extern dispatcher"
	case VMStatusExternABIViolation:
		return "extern ABI violation"
	case VMStatusHostFailure:
		return "host failure"
	default:
		return "unknown VM status"
	}
}

// VMFaultInfo provides allocation-free diagnostics for the most recent fault.
// A field is -1 when it is not applicable to the fault.
type VMFaultInfo struct {
	Status     VMStatus
	PC         int
	Target     int
	Required   int
	Available  int
	HostStatus VMStatus
}

func noVMFault() VMFaultInfo {
	return VMFaultInfo{
		Status:     VMStatusOK,
		PC:         -1,
		Target:     -1,
		Required:   -1,
		Available:  -1,
		HostStatus: VMStatusOK,
	}
}
