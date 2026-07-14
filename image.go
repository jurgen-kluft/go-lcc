package cova

import (
	"bytes"

	"github.com/jurgen-kluft/go-datastream/codestream"
)

const (
	ProgramImageMagic               = uint32('C') | uint32('O')<<8 | uint32('V')<<16 | uint32('A')<<24
	ProgramImageVersion      uint16 = 2
	ProgramImageEndianLittle uint8  = 1
	ProgramImageABI          uint8  = 1

	ProgramImageStringHeaderSize = 8
	ProgramImageArrayHeaderSize  = 8
	ProgramImageFunctionSize     = 20
)

type ProgramImageValueKind uint8

type ProgramImageStringHeader struct {
	ByteLen uint16
	RuneLen uint16
	DataOff uint32
}

type ProgramImageArrayHeader struct {
	Len     uint32
	DataOff uint32
}

type ProgramImageFunction struct {
	BodyAddress   uint32
	ParamStart    uint32
	ParamCount    uint32
	FrameByteSize uint32
	ReturnKind    ProgramImageValueKind
	Reserved0     uint8
	Reserved1     uint8
	Reserved2     uint8
}

type ProgramImage struct {
	Magic        uint32
	Version      uint16
	Endian       uint8
	ABI          uint8
	EntryPoint   uint32
	BSSByteSize  uint32
	Functions    []ProgramImageFunction
	ParamKinds   []ProgramImageValueKind
	ParamOffsets []uint32
	Text         []byte
	ConstData    []byte
	DataData     []byte
}

func BuildProgramImage(program *LinkedProgram) ([]byte, VMStatus) {
	image, status := linkedProgramToImage(program)
	if status != VMStatusOK {
		return nil, status
	}
	blob, err := encodeProgramImage(image)
	if err != nil {
		return nil, VMStatusInvalidImage
	}
	return blob, VMStatusOK
}

func OpenProgramImage(blob []byte) (*ProgramImage, VMStatus) {
	image, err := decodeProgramImage(blob)
	if err != nil {
		return nil, VMStatusInvalidImage
	}
	if status := validateProgramImage(image); status != VMStatusOK {
		return nil, status
	}
	return image, VMStatusOK
}

func imageUint32FromInt(value int) (uint32, bool) {
	if value < 0 || uint64(value) > uint64(^uint32(0)) {
		return 0, false
	}
	return uint32(value), true
}

func encodeProgramImage(image *ProgramImage) ([]byte, error) {
	var buffer bytes.Buffer
	if err := codestream.WriteToStream(&buffer, image); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func decodeProgramImage(blob []byte) (*ProgramImage, error) {
	image := &ProgramImage{}
	if err := codestream.ReadFromStream(bytes.NewReader(blob), image); err != nil {
		return nil, err
	}
	return image, nil
}

func validateProgramImage(image *ProgramImage) VMStatus {
	if image == nil {
		return VMStatusInvalidProgram
	}
	if image.Magic != ProgramImageMagic {
		return VMStatusInvalidImage
	}
	if image.Version != ProgramImageVersion {
		return VMStatusUnsupportedImage
	}
	if image.Endian != ProgramImageEndianLittle || image.ABI != ProgramImageABI {
		return VMStatusUnsupportedImage
	}
	if len(image.ParamKinds) != len(image.ParamOffsets) {
		return VMStatusInvalidParameter
	}
	if int(image.EntryPoint) >= len(image.Functions) {
		return VMStatusInvalidTarget
	}
	for index, kind := range image.ParamKinds {
		if ValueKind(kind) >= KindCount {
			return VMStatusInvalidParameter
		}
		_ = index
	}
	for index, function := range image.Functions {
		if ValueKind(function.ReturnKind) >= KindCount {
			return VMStatusInvalidDescriptor
		}
		if function.BodyAddress >= uint32(len(image.Text)) {
			return VMStatusInvalidDescriptor
		}
		if function.ParamStart > uint32(len(image.ParamKinds)) {
			return VMStatusInvalidParameter
		}
		if function.ParamCount > uint32(len(image.ParamKinds))-function.ParamStart {
			return VMStatusInvalidParameter
		}
		_ = index
	}
	return VMStatusOK
}

func linkedProgramToImage(program *LinkedProgram) (*ProgramImage, VMStatus) {
	if program == nil {
		return nil, VMStatusInvalidProgram
	}
	if program.ConstByteSize != uint32(len(program.ConstData)) {
		return nil, VMStatusInvalidImage
	}
	if program.DataByteSize != uint32(len(program.DataData)) {
		return nil, VMStatusInvalidImage
	}
	if program.EntryPoint >= uint32(len(program.Functions)) {
		return nil, VMStatusInvalidDescriptor
	}
	entryPoint, bssByteSize, status := imageRootScalarsFromProgram(program)
	if status != VMStatusOK {
		return nil, status
	}
	functions := make([]ProgramImageFunction, len(program.Functions))
	for index, function := range program.Functions {
		imageFunction, status := imageFunctionFromDescriptor(function)
		if status != VMStatusOK {
			return nil, status
		}
		functions[index] = imageFunction
	}
	paramKinds, status := imageParamKindsFromKinds(program.ParamKinds)
	if status != VMStatusOK {
		return nil, status
	}
	image := &ProgramImage{
		Magic:        ProgramImageMagic,
		Version:      ProgramImageVersion,
		Endian:       ProgramImageEndianLittle,
		ABI:          ProgramImageABI,
		EntryPoint:   entryPoint,
		BSSByteSize:  bssByteSize,
		Functions:    functions,
		ParamKinds:   paramKinds,
		ParamOffsets: append([]uint32(nil), program.ParamOffsets...),
		Text:         append([]byte(nil), program.Text...),
		ConstData:    append([]byte(nil), program.ConstData...),
		DataData:     append([]byte(nil), program.DataData...),
	}
	if status := validateProgramImage(image); status != VMStatusOK {
		return nil, status
	}
	return image, VMStatusOK
}

func imageRootScalarsFromProgram(program *LinkedProgram) (entryPoint uint32, bssByteSize uint32, status VMStatus) {
	return program.EntryPoint, program.BSSByteSize, VMStatusOK
}

func imageParamKindsFromKinds(kinds []ValueKind) ([]ProgramImageValueKind, VMStatus) {
	imageKinds := make([]ProgramImageValueKind, len(kinds))
	for index, kind := range kinds {
		if kind >= KindCount {
			return nil, VMStatusInvalidParameter
		}
		imageKinds[index] = ProgramImageValueKind(kind)
	}
	return imageKinds, VMStatusOK
}

func imageFunctionFromDescriptor(function ScriptFunctionDescriptor) (ProgramImageFunction, VMStatus) {
	if function.ReturnKind >= KindCount {
		return ProgramImageFunction{}, VMStatusInvalidDescriptor
	}
	return ProgramImageFunction{
		BodyAddress:   function.BodyAddress,
		ParamStart:    function.ParamStart,
		ParamCount:    function.ParamCount,
		FrameByteSize: function.FrameByteSize,
		ReturnKind:    ProgramImageValueKind(function.ReturnKind),
	}, VMStatusOK
}

func linkedProgramFromImage(image *ProgramImage) (*LinkedProgram, VMStatus) {
	if status := validateProgramImage(image); status != VMStatusOK {
		return nil, status
	}
	functions := make([]ScriptFunctionDescriptor, len(image.Functions))
	for index := range functions {
		function := image.Functions[index]
		functions[index] = ScriptFunctionDescriptor{
			BodyAddress:   function.BodyAddress,
			ParamStart:    function.ParamStart,
			ParamCount:    function.ParamCount,
			FrameByteSize: function.FrameByteSize,
			ReturnKind:    ValueKind(function.ReturnKind),
		}
	}
	paramKinds := make([]ValueKind, len(image.ParamKinds))
	for index := range paramKinds {
		paramKinds[index] = ValueKind(image.ParamKinds[index])
	}
	program := &LinkedProgram{
		Text:          CodeMemory(image.Text),
		EntryPoint:    image.EntryPoint,
		Functions:     functions,
		ParamKinds:    paramKinds,
		ParamOffsets:  append([]uint32(nil), image.ParamOffsets...),
		ConstByteSize: lenu32(image.ConstData),
		ConstData:     image.ConstData,
		DataByteSize:  lenu32(image.DataData),
		DataData:      image.DataData,
		BSSByteSize:   image.BSSByteSize,
	}
	for _, function := range functions {
		if end := function.ParamStart + function.ParamCount; end > program.FrameSize {
			program.FrameSize = end
		}
		if function.FrameByteSize > program.FrameByteSize {
			program.FrameByteSize = function.FrameByteSize
		}
	}
	return program, VMStatusOK
}
