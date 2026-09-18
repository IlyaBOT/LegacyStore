package architecture

import (
	"errors"
	"sort"
	"strings"
)

var ErrUnsupported = errors.New("unsupported architecture")

var supported = map[string]bool{
	"i386":    true,
	"i686":    true,
	"x86_64":  true,
	"ppc":     true,
	"ppc-g3":  true,
	"ppc-g4":  true,
	"ppc-g5":  true,
	"ppc64":   true,
}

func NormalizeOne(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "amd64", "x64", "x86-64":
		value = "x86_64"
	case "386", "x86", "ia32":
		value = "i386"
	case "686":
		value = "i686"
	case "powerpc":
		value = "ppc"
	case "powerpc64":
		value = "ppc64"
	}
	if !supported[value] {
		return "", ErrUnsupported
	}
	return value, nil
}

func Normalize(values []string) ([]string, error) {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		normalized, err := NormalizeOne(value)
		if err != nil {
			return nil, err
		}
		if seen[normalized] {
			continue
		}
		seen[normalized] = true
		out = append(out, normalized)
	}
	if len(out) == 0 {
		return nil, ErrUnsupported
	}
	sort.SliceStable(out, func(i, j int) bool {
		return order(out[i]) < order(out[j])
	})
	return out, nil
}

func HumanLabels(values []string) []string {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		if normalized, err := NormalizeOne(value); err == nil {
			set[normalized] = true
		}
	}
	labels := make([]string, 0, 6)
	if set["i386"] || set["i686"] {
		labels = append(labels, "Intel 32 Bit (i386 or i686)")
	}
	if set["x86_64"] {
		labels = append(labels, "Intel 64 Bit (x86_64)")
	}
	if set["ppc"] {
		labels = append(labels, "PowerPC 32 Bit (ppc)")
	}
	if set["ppc-g3"] {
		labels = append(labels, "PowerPC G3 (ppc-g3)")
	}
	if set["ppc-g4"] {
		labels = append(labels, "PowerPC G4 (ppc-g4)")
	}
	if set["ppc-g5"] {
		labels = append(labels, "PowerPC G5 (ppc-g5)")
	}
	if set["ppc64"] {
		labels = append(labels, "PowerPC 64 Bit (ppc64)")
	}
	return labels
}

func Compatible(target string, artifact []string, supports32BitApps bool) bool {
	target, err := NormalizeOne(target)
	if err != nil {
		return false
	}
	set := map[string]bool{}
	for _, value := range artifact {
		if normalized, normalizeErr := NormalizeOne(value); normalizeErr == nil {
			set[normalized] = true
		}
	}

	switch target {
	case "i386", "i686":
		return set["i386"] || set["i686"]
	case "x86_64":
		if set["x86_64"] {
			return true
		}
		return supports32BitApps && (set["i386"] || set["i686"])
	case "ppc":
		return set["ppc"]
	case "ppc-g3":
		return set["ppc"] || set["ppc-g3"]
	case "ppc-g4":
		return set["ppc"] || set["ppc-g3"] || set["ppc-g4"]
	case "ppc-g5":
		return set["ppc"] || set["ppc-g3"] || set["ppc-g4"] || set["ppc-g5"] || set["ppc64"]
	case "ppc64":
		return set["ppc64"]
	default:
		return false
	}
}

func Is32Bit(value string) bool {
	value, err := NormalizeOne(value)
	if err != nil {
		return false
	}
	switch value {
	case "i386", "i686", "ppc", "ppc-g3", "ppc-g4", "ppc-g5":
		return true
	default:
		return false
	}
}

func order(value string) int {
	switch value {
	case "i386":
		return 10
	case "i686":
		return 11
	case "x86_64":
		return 20
	case "ppc":
		return 30
	case "ppc-g3":
		return 31
	case "ppc-g4":
		return 32
	case "ppc-g5":
		return 33
	case "ppc64":
		return 34
	default:
		return 100
	}
}
