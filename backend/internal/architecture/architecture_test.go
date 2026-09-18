package architecture

import "testing"

func TestNormalizeAndLabels(t *testing.T) {
	values, err := Normalize([]string{"x86-64", "i386", "i686", "i386"})
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 3 || values[0] != "i386" || values[1] != "i686" || values[2] != "x86_64" {
		t.Fatalf("unexpected normalized architectures: %#v", values)
	}
	labels := HumanLabels(values)
	if len(labels) != 2 || labels[0] != "Intel 32 Bit (i386 or i686)" || labels[1] != "Intel 64 Bit (x86_64)" {
		t.Fatalf("unexpected labels: %#v", labels)
	}
}

func TestIntelCompatibility(t *testing.T) {
	if !Compatible("x86_64", []string{"i386"}, true) {
		t.Fatal("x86_64 Mac before Catalina should accept i386 applications")
	}
	if Compatible("x86_64", []string{"i386"}, false) {
		t.Fatal("64-bit-only OS must reject i386-only application")
	}
	if Compatible("i386", []string{"x86_64"}, true) {
		t.Fatal("32-bit Intel target must reject x86_64-only application")
	}
	if !Compatible("x86_64", []string{"x86_64"}, false) {
		t.Fatal("x86_64 target must accept x86_64 application")
	}
}

func TestPowerPCCompatibility(t *testing.T) {
	if !Compatible("ppc-g5", []string{"ppc-g4"}, true) {
		t.Fatal("G5 should accept G4 32-bit code")
	}
	if Compatible("ppc-g3", []string{"ppc-g4"}, true) {
		t.Fatal("G3 must reject G4-specific code")
	}
}
