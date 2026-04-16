package browse

import "testing"

func TestBuildKiwixArgsBindsToLoopback(t *testing.T) {
	args := buildKiwixArgs(8083, []string{"/tmp/a.zim", "/tmp/b.zim"})

	want := []string{"--port", "8083", "--address", "127.0.0.1", "/tmp/a.zim", "/tmp/b.zim"}
	if len(args) != len(want) {
		t.Fatalf("len(args) = %d, want %d", len(args), len(want))
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("args[%d] = %q, want %q", i, args[i], want[i])
		}
	}
}
