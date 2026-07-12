package compatibility

import (
	"fmt"
	"strconv"
	"strings"
)

type Version struct {
	Major int
	Minor int
	Patch int
}

func ParseVersion(raw string) (Version, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Version{}, fmt.Errorf("empty version")
	}

	parts := strings.Split(raw, ".")
	if len(parts) > 3 {
		parts = parts[:3]
	}

	nums := [3]int{}
	for i, part := range parts {
		if part == "" {
			return Version{}, fmt.Errorf("invalid version %q", raw)
		}
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return Version{}, fmt.Errorf("invalid version %q", raw)
		}
		nums[i] = n
	}

	return Version{Major: nums[0], Minor: nums[1], Patch: nums[2]}, nil
}

func CompareVersions(a, b string) int {
	av, aerr := ParseVersion(a)
	bv, berr := ParseVersion(b)
	if aerr != nil && berr != nil {
		return strings.Compare(a, b)
	}
	if aerr != nil {
		return -1
	}
	if berr != nil {
		return 1
	}
	return av.Compare(bv)
}

func (v Version) Compare(other Version) int {
	if v.Major != other.Major {
		return compareInt(v.Major, other.Major)
	}
	if v.Minor != other.Minor {
		return compareInt(v.Minor, other.Minor)
	}
	return compareInt(v.Patch, other.Patch)
}

func compareInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func (v Version) Supports32BitApps() bool {
	catalina, _ := ParseVersion("10.15")
	return v.Compare(catalina) < 0
}
