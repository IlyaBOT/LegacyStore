package macmeta

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func syntheticMachO32(cpu uint32, subCPU uint32) []byte {
	var out bytes.Buffer
	_ = binary.Write(&out, binary.LittleEndian, uint32(0xfeedface))
	_ = binary.Write(&out, binary.LittleEndian, cpu)
	_ = binary.Write(&out, binary.LittleEndian, subCPU)
	_ = binary.Write(&out, binary.LittleEndian, uint32(2))
	_ = binary.Write(&out, binary.LittleEndian, uint32(0))
	_ = binary.Write(&out, binary.LittleEndian, uint32(0))
	_ = binary.Write(&out, binary.LittleEndian, uint32(0))
	return out.Bytes()
}

func syntheticMachO64(cpu uint32, subCPU uint32) []byte {
	var out bytes.Buffer
	_ = binary.Write(&out, binary.LittleEndian, uint32(0xfeedfacf))
	_ = binary.Write(&out, binary.LittleEndian, cpu)
	_ = binary.Write(&out, binary.LittleEndian, subCPU)
	_ = binary.Write(&out, binary.LittleEndian, uint32(2))
	_ = binary.Write(&out, binary.LittleEndian, uint32(0))
	_ = binary.Write(&out, binary.LittleEndian, uint32(0))
	_ = binary.Write(&out, binary.LittleEndian, uint32(0))
	_ = binary.Write(&out, binary.LittleEndian, uint32(0))
	return out.Bytes()
}

func TestInspectMachOIntelArchitectures(t *testing.T) {
	i386, err := inspectMachOBytes(syntheticMachO32(7, 3))
	if err != nil || len(i386) != 1 || i386[0] != "i386" {
		t.Fatalf("i386 = %#v, err=%v", i386, err)
	}
	x64, err := inspectMachOBytes(syntheticMachO64(0x01000007, 3))
	if err != nil || len(x64) != 1 || x64[0] != "x86_64" {
		t.Fatalf("x86_64 = %#v, err=%v", x64, err)
	}
}

func TestMachOPowerPCSubtypeLabels(t *testing.T) {
	cases := map[uint32]string{9: "ppc-g3", 10: "ppc-g4", 11: "ppc-g4", 100: "ppc-g5", 0: "ppc"}
	for subtype, want := range cases {
		if got := machoArchitecture(18, subtype); got != want {
			t.Fatalf("subtype %d = %q, want %q", subtype, got, want)
		}
	}
}
