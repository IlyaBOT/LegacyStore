package macmeta

import (
	"bytes"
	"debug/macho"
	"encoding/binary"
	"errors"
	"os"
	"sort"
)

func inspectMachOPrefix(data []byte) ([]string, error) {
	if len(data) < 12 {
		return nil, errors.New("Mach-O executable is too small")
	}

	magicBE := binary.BigEndian.Uint32(data[:4])
	magicLE := binary.LittleEndian.Uint32(data[:4])

	switch magicBE {
	case 0xcafebabe, 0xcafebabf:
		if len(data) < 8 {
			return nil, errors.New("fat Mach-O header is truncated")
		}
		count := int(binary.BigEndian.Uint32(data[4:8]))
		entrySize := 20
		if magicBE == 0xcafebabf {
			entrySize = 32
		}
		if count < 1 || count > 128 || len(data) < 8+count*entrySize {
			return nil, errors.New("invalid fat Mach-O architecture table")
		}
		values := make([]string, 0, count)
		for i := 0; i < count; i++ {
			offset := 8 + i*entrySize
			cpu := macho.Cpu(binary.BigEndian.Uint32(data[offset : offset+4]))
			subCPU := binary.BigEndian.Uint32(data[offset+4 : offset+8])
			if value := machoArchitecture(cpu, subCPU); value != "" {
				values = append(values, value)
			}
		}
		return normalizeMachOArchitectures(values)
	case 0xbebafeca, 0xbfbafeca:
		if len(data) < 8 {
			return nil, errors.New("swapped fat Mach-O header is truncated")
		}
		count := int(binary.LittleEndian.Uint32(data[4:8]))
		entrySize := 20
		if magicBE == 0xbfbafeca {
			entrySize = 32
		}
		if count < 1 || count > 128 || len(data) < 8+count*entrySize {
			return nil, errors.New("invalid swapped fat Mach-O architecture table")
		}
		values := make([]string, 0, count)
		for i := 0; i < count; i++ {
			offset := 8 + i*entrySize
			cpu := macho.Cpu(binary.LittleEndian.Uint32(data[offset : offset+4]))
			subCPU := binary.LittleEndian.Uint32(data[offset+4 : offset+8])
			if value := machoArchitecture(cpu, subCPU); value != "" {
				values = append(values, value)
			}
		}
		return normalizeMachOArchitectures(values)
	}

	var order binary.ByteOrder
	switch {
	case magicBE == 0xfeedface || magicBE == 0xfeedfacf:
		order = binary.BigEndian
	case magicLE == 0xfeedface || magicLE == 0xfeedfacf:
		order = binary.LittleEndian
	default:
		return nil, errors.New("not a Mach-O executable")
	}
	cpu := macho.Cpu(order.Uint32(data[4:8]))
	subCPU := order.Uint32(data[8:12])
	value := machoArchitecture(cpu, subCPU)
	if value == "" {
		return nil, errors.New("unsupported Mach-O CPU type")
	}
	return []string{value}, nil
}

func inspectMachOBytes(data []byte) ([]string, error) {
	if len(data) < 28 {
		return nil, errors.New("Mach-O executable is too small")
	}
	reader := bytes.NewReader(data)
	if fat, err := macho.NewFatFile(reader); err == nil {
		defer fat.Close()
		values := make([]string, 0, len(fat.Arches))
		for _, arch := range fat.Arches {
			if value := machoArchitecture(arch.Cpu, uint32(arch.SubCpu)); value != "" {
				values = append(values, value)
			}
		}
		return normalizeMachOArchitectures(values)
	}
	file, err := macho.NewFile(reader)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	value := machoArchitecture(file.Cpu, uint32(file.SubCpu))
	if value == "" {
		return nil, errors.New("unsupported Mach-O CPU type")
	}
	return []string{value}, nil
}

func inspectMachOFile(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	if fat, err := macho.NewFatFile(file); err == nil {
		defer fat.Close()
		values := make([]string, 0, len(fat.Arches))
		for _, arch := range fat.Arches {
			if value := machoArchitecture(arch.Cpu, uint32(arch.SubCpu)); value != "" {
				values = append(values, value)
			}
		}
		return normalizeMachOArchitectures(values)
	}
	thin, err := macho.NewFile(file)
	if err != nil {
		return nil, err
	}
	defer thin.Close()
	value := machoArchitecture(thin.Cpu, uint32(thin.SubCpu))
	if value == "" {
		return nil, errors.New("unsupported Mach-O CPU type")
	}
	return []string{value}, nil
}

func machoArchitecture(cpu macho.Cpu, subCPU uint32) string {
	switch cpu {
	case macho.Cpu386:
		// Old Intel binaries are surfaced as i386. i686 remains an accepted
		// catalog code for manual metadata, but Mach-O itself does not provide
		// a reliable distinction worth exposing here.
		return "i386"
	case macho.CpuAmd64:
		return "x86_64"
	case macho.CpuPpc:
		switch subCPU & 0x00ffffff {
		case 9:
			return "ppc-g3"
		case 10, 11:
			return "ppc-g4"
		case 100:
			return "ppc-g5"
		default:
			return "ppc"
		}
	case macho.CpuPpc64:
		return "ppc64"
	default:
		return ""
	}
}

func normalizeMachOArchitectures(values []string) ([]string, error) {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	if len(out) == 0 {
		return nil, errors.New("no supported Mach-O architectures found")
	}
	order := map[string]int{
		"i386": 10, "i686": 11, "x86_64": 20,
		"ppc": 30, "ppc-g3": 31, "ppc-g4": 32, "ppc-g5": 33, "ppc64": 34,
	}
	sort.SliceStable(out, func(i, j int) bool { return order[out[i]] < order[out[j]] })
	return out, nil
}
