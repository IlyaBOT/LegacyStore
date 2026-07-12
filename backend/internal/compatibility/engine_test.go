package compatibility

import "testing"

func TestCompareMacOSVersionsNumerically(t *testing.T) {
	if CompareVersions("10.15", "10.9") <= 0 {
		t.Fatalf("10.15 must compare greater than 10.9")
	}
	if CompareVersions("10.5.2", "10.5") <= 0 {
		t.Fatalf("10.5.2 must compare greater than 10.5")
	}
}

func TestEvaluateOSTooOld(t *testing.T) {
	result := Evaluate(Target{OSVersion: "10.5.8", Arch: "i386"}, Artifact{
		MinOS:         "10.8",
		ArchI386:      true,
		ArchX8664:     true,
		Supports32Bit: true,
		Supports64Bit: true,
	})
	if result.Status != "blocked" || result.Reasons[0].Code != "os_too_old" {
		t.Fatalf("expected os_too_old block, got %#v", result)
	}
}

func TestEvaluateCatalinaBlocks32BitOnly(t *testing.T) {
	result := Evaluate(Target{OSVersion: "10.15", Arch: "x86_64"}, Artifact{
		MinOS:         "10.6",
		ArchI386:      true,
		ArchX8664:     true,
		Supports32Bit: true,
		Supports64Bit: false,
	})
	if result.Status != "blocked" || result.Reasons[0].Code != "requires_32bit" {
		t.Fatalf("expected requires_32bit block, got %#v", result)
	}
}

func TestEvaluateUntestedNewerOS(t *testing.T) {
	result := Evaluate(Target{OSVersion: "10.14", Arch: "x86_64"}, Artifact{
		MinOS:          "10.8",
		MaxSupportedOS: "10.13",
		MaxTestedOS:    "10.13",
		ArchX8664:      true,
		Supports64Bit:  true,
	})
	if result.Status != "untested" {
		t.Fatalf("expected untested, got %#v", result)
	}
}
