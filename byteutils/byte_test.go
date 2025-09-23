package byteutils

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestFormatBytes_SI(t *testing.T) {
	t.Parallel()
	tests := []struct {
		b   int64
		out string
	}{
		{0, "0 B"},
		{1, "1 B"},
		{999, "999 B"},
		{1000, "1 KB"},
		{1500, "1.5 KB"},
		{2*GB + 345*MB, "2.3 GB"},
		{-10, "-10 B"},
		{-1500, "-1.5 KB"},
	}
	for _, tc := range tests {
		got := FormatBytes(tc.b, false)
		if got != tc.out {
			t.Fatalf("FormatBytes(%d,false) = %q; want %q", tc.b, got, tc.out)
		}
	}
}

func TestFormatBytes_IEC(t *testing.T) {
	t.Parallel()
	tests := []struct {
		b   int64
		out string
	}{
		{0, "0 B"},
		{1, "1 B"},
		{1023, "1023 B"},
		{1024, "1 KiB"},
		{1536, "1.5 KiB"},
		{(2 * GiB) + (300 * MiB), "2.3 GiB"},
		{-1536, "-1.5 KiB"},
	}
	for _, tc := range tests {
		got := FormatBytes(tc.b, true)
		if got != tc.out {
			t.Fatalf("FormatBytes(%d,true) = %q; want %q", tc.b, got, tc.out)
		}
	}
}

func TestHumanizeBytes_DefaultsToSI(t *testing.T) {
	t.Parallel()
	if got := HumanizeBytes(1500); got != "1.5 KB" {
		t.Fatalf("HumanizeBytes(1500) = %q; want %q", got, "1.5 KB")
	}
	if got := HumanizeBytes(1024); got != "1.0 KB" && got != "1 KB" {
		t.Fatalf("HumanizeBytes(1024) = %q; want %q or %q", got, "1 KB", "1.0 KB")
	}
}

func almostEqual(a, b, eps float64) bool { return math.Abs(a-b) <= eps }

func TestToFrom_SI(t *testing.T) {
	t.Parallel()
	n := int64(3 * GB)
	if got := ToGB(n); !almostEqual(got, 3.0, 1e-12) {
		t.Fatalf("ToGB(%d)=%v; want 3", n, got)
	}
	if got := FromGB(3); got != n {
		t.Fatalf("FromGB(3)=%d; want %d", got, n)
	}

	if ToKB(5000) != 5.0 {
		t.Fatalf("ToKB(5000)=%.1f; want 5.0", ToKB(5000))
	}
	if FromKB(5.0) != 5000 {
		t.Fatalf("FromKB(5.0)=%d; want 5000", FromKB(5.0))
	}
}

func TestToFrom_IEC(t *testing.T) {
	t.Parallel()
	n := 4 * MiB
	if got := ToMiB(n); !almostEqual(got, 4.0, 1e-12) {
		t.Fatalf("ToMiB(%d)=%v; want 4", n, got)
	}
	if got := FromMiB(4); got != n {
		t.Fatalf("FromMiB(4)=%d; want %d", got, n)
	}

	if ToKiB(2048) != 2.0 {
		t.Fatalf("ToKiB(2048)=%.1f; want 2.0", ToKiB(2048))
	}
	if FromKiB(2.0) != 2048 {
		t.Fatalf("FromKiB(2.0)=%d; want 2048", FromKiB(2.0))
	}
}

func TestParseBytes_ValidInputs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in  string
		out int64
	}{
		{"0", 0},
		{"1024", 1024},
		{"  512B ", 512},
		{"-1", -1},
		{"1.9", 1},
		{"-1.9", -1},

		{"1k", 1 * KB},
		{"1K", 1 * KB},
		{"2m", 2 * MB},
		{"3g", 3 * GB},
		{"200MB", 200 * MB},
		{"1.5 GB", int64(1.5 * float64(GB))},
		{"-1.5KB", -int64(1.5 * float64(KB))},
		{"42 kb", 42 * KB},

		{"1KiB", 1 * KiB},
		{"7miB", 7 * MiB},
		{"1.5GiB", int64(1.5 * float64(GiB))},
		{"2Ti", 2 * TiB},
		{"3pi", 3 * PiB},
		{"4EiB", 4 * EiB},
	}
	for _, tc := range tests {
		got, err := ParseBytes(tc.in)
		if err != nil {
			t.Fatalf("ParseBytes(%q) unexpected error: %v", tc.in, err)
		}
		if got != tc.out {
			t.Fatalf("ParseBytes(%q) = %d; want %d", tc.in, got, tc.out)
		}
	}
}

func TestParseBytes_InvalidInputs(t *testing.T) {
	t.Parallel()
	bad := []string{
		"", " ", "abc", "1 XB", "1 B B", "1..2", "--1", "+-2", "1.2.3MB",
	}
	for _, in := range bad {
		if _, err := ParseBytes(in); err == nil {
			t.Fatalf("ParseBytes(%q) expected error, got nil", in)
		}
	}
}

func TestParseBytes_OverflowProtection(t *testing.T) {
	t.Parallel()
	s := fmt.Sprintf("%dEB", math.MaxInt64)
	if _, err := ParseBytes(s); err == nil {
		t.Fatalf("ParseBytes(%q) expected overflow error, got nil", s)
	}
}

func TestMustParseBytes(t *testing.T) {
	t.Parallel()
	if got := MustParseBytes("1.5GiB"); got != int64(1.5*float64(GiB)) {
		t.Fatalf("MustParseBytes good path = %d; want %d", got, int64(1.5*float64(GiB)))
	}

	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("MustParseBytes did not panic on invalid input")
		} else {
			if msg := fmt.Sprint(r); !strings.Contains(strings.ToLower(msg), "unknown unit") {
			}
		}
	}()
	_ = MustParseBytes("12XB")
}

func TestFormat_TrimsTrailingZero(t *testing.T) {
	t.Parallel()
	if got := FormatBytes(1*KB, false); got != "1 KB" {
		t.Fatalf("expected trimmed decimal: got %q; want %q", got, "1 KB")
	}
	if got := FormatBytes(1*KiB, true); got != "1 KiB" {
		t.Fatalf("expected trimmed decimal: got %q; want %q", got, "1 KiB")
	}
}

func TestFormat_SmallerThanBaseUsesBytes(t *testing.T) {
	t.Parallel()
	if got := FormatBytes(999, false); got != "999 B" {
		t.Fatalf("SI < base should be in B; got %q", got)
	}
	if got := FormatBytes(1023, true); got != "1023 B" {
		t.Fatalf("IEC < base should be in B; got %q", got)
	}
}
