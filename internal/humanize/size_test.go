package humanize

import "testing"

func TestSize(t *testing.T) {
	tests := []struct {
		in   int64
		want string
	}{
		{0, "0B"},
		{1, "1B"},
		{999, "999B"},
		{1023, "1023B"},
		{1024, "1.0K"},
		{1536, "1.5K"},
		{1024*1024 - 1, "1024.0K"},
		{1024 * 1024, "1.0M"},
		{1024 * 1024 * 3 / 2, "1.5M"},
		{1024 * 1024 * 1024, "1.0G"},
		{1024 * 1024 * 1024 * 1024, "1.0T"},
		{1024 * 1024 * 1024 * 1024 * 1024, "1.0P"},
	}

	for _, tt := range tests {
		if got := Size(tt.in); got != tt.want {
			t.Errorf("Size(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
