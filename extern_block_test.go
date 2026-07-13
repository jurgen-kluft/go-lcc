package cova

import (
	"encoding/binary"
	"testing"
)

func TestExternAddressUsesByteOffset(t *testing.T) {
	externMemory := make([]byte, 8)
	binary.LittleEndian.PutUint32(externMemory[4:], 21)

	script := `
extern(4) int value;

void script_main() {
	value = value + 9;
	return;
}
`

	linked := mustLinkProgram(t, script, len(externMemory), 0)
	vm := NewVM(testFrameCapacityBytes)
	vm.BindExternBlock(externMemory)
	if status := vm.Run(linked); status != VMStatusOK {
		t.Fatalf("Run failed: %s", status)
	}
	if got := int(int32(binary.LittleEndian.Uint32(externMemory[4:]))); got != 30 {
		t.Fatalf("expected extern byte offset 4 to hold 30, got %d", got)
	}
	if got := int(int32(binary.LittleEndian.Uint32(externMemory[0:]))); got != 0 {
		t.Fatalf("expected extern byte offset 0 to remain 0, got %d", got)
	}
}
