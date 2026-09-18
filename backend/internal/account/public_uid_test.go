package account

import "testing"

func TestNewPublicUIDLengthAndUniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		uid, err := newPublicUID()
		if err != nil {
			t.Fatal(err)
		}
		if len(uid) < 16 || len(uid) > 32 {
			t.Fatalf("uid length = %d, want 16..32: %q", len(uid), uid)
		}
		if seen[uid] {
			t.Fatalf("duplicate uid generated: %q", uid)
		}
		seen[uid] = true
	}
}
