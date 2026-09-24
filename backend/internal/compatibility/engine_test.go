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
		Architectures: []string{"i386", "x86_64"},
	})
	if result.Status != "blocked" || result.Reasons[0].Code != "os_too_old" {
		t.Fatalf("expected os_too_old block, got %#v", result)
	}
}

func TestEvaluateCatalinaBlocks32BitOnly(t *testing.T) {
	result := Evaluate(Target{OSVersion: "10.15", Arch: "x86_64"}, Artifact{
		MinOS:         "10.6",
		Architectures: []string{"i386"},
	})
	if result.Status != "blocked" || result.Reasons[0].Code != "arch_mismatch" {
		t.Fatalf("expected architecture block for 32-bit-only build on Catalina, got %#v", result)
	}
}

func TestEvaluateUntestedNewerOS(t *testing.T) {
	result := Evaluate(Target{OSVersion: "10.14", Arch: "x86_64"}, Artifact{
		MinOS:          "10.8",
		MaxSupportedOS: "10.13",
		MaxTestedOS:    "10.13",
		Architectures:  []string{"x86_64"},
	})
	if result.Status != "untested" {
		t.Fatalf("expected untested, got %#v", result)
	}
}

func TestEvaluateOSSeriesAllowsPatchLevelMinimumWithinSameRelease(t *testing.T) {
	result := Evaluate(Target{OSVersion: "10.6", Arch: "x86_64", OSSeries: true}, Artifact{
		MinOS:         "10.6.8",
		Architectures: []string{"i386", "x86_64"},
	})
	if result.Status == "blocked" {
		t.Fatalf("10.6 catalog series must include artifacts requiring a later 10.6.x patch, got %#v", result)
	}
}

func TestEvaluateOSSeriesAllowsPatchLevelMaximumWithinSameRelease(t *testing.T) {
	result := Evaluate(Target{OSVersion: "10.6", Arch: "x86_64", OSSeries: true}, Artifact{
		MinOS:             "10.5",
		MaxSupportedOS:    "10.6.2",
		HardBlockAboveMax: true,
		Architectures:     []string{"i386", "x86_64"},
	})
	if result.Status == "blocked" {
		t.Fatalf("10.6 catalog series must include artifacts supporting only part of 10.6.x, got %#v", result)
	}
}

func TestEvaluateOSSeriesStillBlocksDifferentRelease(t *testing.T) {
	result := Evaluate(Target{OSVersion: "10.6", Arch: "x86_64", OSSeries: true}, Artifact{
		MinOS:         "10.7",
		Architectures: []string{"i386", "x86_64"},
	})
	if result.Status != "blocked" || len(result.Reasons) == 0 || result.Reasons[0].Code != "os_too_old" {
		t.Fatalf("10.6 catalog series must not include 10.7-only artifacts, got %#v", result)
	}
}

func TestEvaluateX8664RunsI386BeforeCatalina(t *testing.T) {
	result := Evaluate(Target{OSVersion: "10.14.6", Arch: "x86_64"}, Artifact{
		MinOS:         "10.6",
		Architectures: []string{"i386"},
	})
	if result.Status == "blocked" {
		t.Fatalf("Mojave x86_64 should accept i386 application, got %#v", result)
	}
}

func TestEvaluatePowerPCFutureTarget(t *testing.T) {
	result := Evaluate(Target{OSVersion: "10.5.8", Arch: "ppc-g4"}, Artifact{
		MinOS:         "10.4",
		Architectures: []string{"ppc-g3", "ppc-g4"},
	})
	if result.Status == "blocked" {
		t.Fatalf("PowerPC G4 target should accept G4 build, got %#v", result)
	}
}
