package cova

import (
	"bytes"
	"reflect"
	"testing"
)

func TestProgramImageSchemaUsesCastableTypes(t *testing.T) {
	assertMirrorableType(t, reflect.TypeOf(ProgramImage{}))
	assertMirrorableType(t, reflect.TypeOf(ProgramImageFunction{}))
	if reflect.TypeOf(ProgramImageStringHeader{}).Size() != ProgramImageStringHeaderSize {
		t.Fatalf("string header size = %d, want %d", reflect.TypeOf(ProgramImageStringHeader{}).Size(), ProgramImageStringHeaderSize)
	}
	if reflect.TypeOf(ProgramImageArrayHeader{}).Size() != ProgramImageArrayHeaderSize {
		t.Fatalf("array header size = %d, want %d", reflect.TypeOf(ProgramImageArrayHeader{}).Size(), ProgramImageArrayHeaderSize)
	}
	if reflect.TypeOf(ProgramImageFunction{}).Size() != ProgramImageFunctionSize {
		t.Fatalf("function record size = %d, want %d", reflect.TypeOf(ProgramImageFunction{}).Size(), ProgramImageFunctionSize)
	}
}

func TestBuildProgramImageIsDeterministicAndRoundTrips(t *testing.T) {
	script := `
int initialized = 3;
const uint8* asset = "asset/button_off";

int add(int left, int right) {
	return left + right;
}

int script_main() {
	return add(initialized, 2);
}
`
	linked := mustLinkProgram(t, script, 0, 0)
	imageA, status := BuildProgramImage(linked)
	if status != VMStatusOK {
		t.Fatalf("BuildProgramImage A failed: %s", status)
	}
	imageB, status := BuildProgramImage(linked)
	if status != VMStatusOK {
		t.Fatalf("BuildProgramImage B failed: %s", status)
	}
	if !bytes.Equal(imageA, imageB) {
		t.Fatal("expected deterministic program image bytes")
	}
	image, status := OpenProgramImage(imageA)
	if status != VMStatusOK {
		t.Fatalf("OpenProgramImage failed: %s", status)
	}
	if image.EntryPoint != linked.EntryPoint {
		t.Fatalf("entry point = %d, want %d", image.EntryPoint, linked.EntryPoint)
	}
	if image.BSSByteSize != linked.BSSByteSize {
		t.Fatalf("bss size = %d, want %d", image.BSSByteSize, linked.BSSByteSize)
	}
	if !bytes.Equal(image.Text, linked.Text) {
		t.Fatal("text view mismatch")
	}
	if !bytes.Equal(image.ConstData, linked.ConstData) {
		t.Fatal("const view mismatch")
	}
	if !bytes.Equal(image.DataData, linked.DataData) {
		t.Fatal("data initializer view mismatch")
	}
	program, status := linkedProgramFromImage(image)
	if status != VMStatusOK {
		t.Fatalf("linkedProgramFromImage failed: %s", status)
	}
	if !reflect.DeepEqual(program.Functions, linked.Functions) {
		t.Fatalf("functions mismatch: got %+v want %+v", program.Functions, linked.Functions)
	}
	if !reflect.DeepEqual(program.ParamKinds, linked.ParamKinds) {
		t.Fatalf("param kinds mismatch: got %+v want %+v", program.ParamKinds, linked.ParamKinds)
	}
	if !reflect.DeepEqual(program.ParamOffsets, linked.ParamOffsets) {
		t.Fatalf("param offsets mismatch: got %+v want %+v", program.ParamOffsets, linked.ParamOffsets)
	}
}

func TestOpenProgramImageRejectsCorruptionAndInvalidSchema(t *testing.T) {
	linked := mustLinkProgram(t, "int add(int left, int right) { return left + right; } int script_main() { return add(1, 2); }", 0, 0)
	blob, status := BuildProgramImage(linked)
	if status != VMStatusOK {
		t.Fatalf("BuildProgramImage failed: %s", status)
	}
	truncated := blob[:len(blob)-1]
	if status := OpenProgramImageStatus(truncated); status != VMStatusInvalidImage {
		t.Fatalf("truncated image status = %s, want invalid image", status)
	}
	image, status := linkedProgramToImage(linked)
	if status != VMStatusOK {
		t.Fatalf("linkedProgramToImage failed: %s", status)
	}
	image.Version++
	brokenVersion, err := encodeProgramImage(image)
	if err != nil {
		t.Fatalf("encodeProgramImage version failed: %v", err)
	}
	if status := OpenProgramImageStatus(brokenVersion); status != VMStatusUnsupportedImage {
		t.Fatalf("broken version status = %s, want unsupported image", status)
	}
	image, status = linkedProgramToImage(linked)
	if status != VMStatusOK {
		t.Fatalf("linkedProgramToImage failed: %s", status)
	}
	image.ParamOffsets = image.ParamOffsets[:0]
	brokenParams, err := encodeProgramImage(image)
	if err != nil {
		t.Fatalf("encodeProgramImage params failed: %v", err)
	}
	if status := OpenProgramImageStatus(brokenParams); status != VMStatusInvalidParameter {
		t.Fatalf("broken params status = %s, want invalid parameter", status)
	}
}

func TestVMLoadProgramImageRunsAndResetsMutableSections(t *testing.T) {
	script := `
int initialized = 3;
int zeroed;

int script_main() {
	initialized = initialized + 1;
	zeroed = zeroed + 2;
	return initialized + zeroed;
}
`
	linked := mustLinkProgram(t, script, 0, 0)
	image, status := BuildProgramImage(linked)
	if status != VMStatusOK {
		t.Fatalf("BuildProgramImage failed: %s", status)
	}
	vm := NewVM(testFrameCapacityBytes)
	if status := vm.LoadProgramImage(image); status != VMStatusOK {
		t.Fatalf("LoadProgramImage failed: %s", status)
	}
	for run := 0; run < 2; run++ {
		if status := vm.RunLoaded(); status != VMStatusOK {
			t.Fatalf("RunLoaded %d failed: %s", run, status)
		}
		if got, status := vm.PopInt32(); status != VMStatusOK || got != 6 {
			t.Fatalf("run %d expected reset result 6, got %d err=%s", run, got, status)
		}
	}
	if !bytes.Equal(vm.program.Text, linked.Text) {
		t.Fatal("vm text mismatch after image load")
	}
	if !bytes.Equal(vm.memory.segment[segmentConst], linked.ConstData) {
		t.Fatal("const segment mismatch after image load")
	}
}

func TestVMRunImageRejectsCorruptBlob(t *testing.T) {
	linked := mustLinkProgram(t, "int script_main() { return 1; }", 0, 0)
	image, status := BuildProgramImage(linked)
	if status != VMStatusOK {
		t.Fatalf("BuildProgramImage failed: %s", status)
	}
	vm := NewVM(testFrameCapacityBytes)
	if status := vm.RunImage(image[:len(image)-1]); status != VMStatusInvalidImage {
		t.Fatalf("RunImage status = %s, want invalid image", status)
	}
}

func TestProgramImageExcludesDebugSymbolsAndWorkspaceRecommendations(t *testing.T) {
	linked := mustLinkProgram(t, "int important_named_global; int script_main() { return 0; }", 0, 0)
	image, status := BuildProgramImage(linked)
	if status != VMStatusOK {
		t.Fatalf("BuildProgramImage failed: %s", status)
	}
	if bytes.Contains(image, []byte("important_named_global")) {
		t.Fatal("image unexpectedly contains debug symbol text")
	}
	if bytes.Contains(image, []byte("script_main")) {
		t.Fatal("image unexpectedly contains function symbol text")
	}
}

func OpenProgramImageStatus(blob []byte) VMStatus {
	_, status := OpenProgramImage(blob)
	return status
}

func assertMirrorableType(t *testing.T, typ reflect.Type) {
	t.Helper()
	switch typ.Kind() {
	case reflect.Bool,
		reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Float32, reflect.Float64:
		return
	case reflect.String:
		return
	case reflect.Array, reflect.Slice:
		assertMirrorableType(t, typ.Elem())
		return
	case reflect.Struct:
		for index := 0; index < typ.NumField(); index++ {
			assertMirrorableType(t, typ.Field(index).Type)
		}
		return
	default:
		t.Fatalf("non-mirrorable schema type: %s (%s)", typ.String(), typ.Kind())
	}
}
