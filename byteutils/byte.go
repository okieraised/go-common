package byteutils

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Decimal (SI) units (powers of 1000)
const (
	KB int64 = 1000
	MB       = KB * 1000
	GB       = MB * 1000
	TB       = GB * 1000
	PB       = TB * 1000
	EB       = PB * 1000
)

// Binary (IEC) units (powers of 1024)
const (
	KiB int64 = 1024
	MiB       = KiB * 1024
	GiB       = MiB * 1024
	TiB       = GiB * 1024
	PiB       = TiB * 1024
	EiB       = PiB * 1024
)

// FormatBytes returns a human-readable string for a byte size.
// If useBinary is true, uses IEC units (KiB, MiB, ...); otherwise SI (KB, MB, ...).
// Rounds to one decimal place for values >= the base unit.
func FormatBytes(b int64, useBinary bool) string {
	val, unit := formatCore(b, useBinary)
	return fmt.Sprintf("%s %s", val, unit)
}

// HumanizeBytes auto-picks the most suitable unit (SI by default).
// Set useBinary=true if you prefer IEC units by default.
func HumanizeBytes(b int64) string {
	val, unit := formatCore(b, false)
	return fmt.Sprintf("%s %s", val, unit)
}

func formatCore(b int64, useBinary bool) (string, string) {
	if b == 0 {
		return "0", "B"
	}
	base := float64(1000)
	suffix := []string{"B", "KB", "MB", "GB", "TB", "PB", "EB"}
	if useBinary {
		base = 1024
		suffix = []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}
	}

	abs := math.Abs(float64(b))
	if abs < base {
		return strconv.Itoa(int(b)), "B"
	}

	exp := int(math.Floor(math.Log(abs) / math.Log(base)))
	if exp >= len(suffix) {
		exp = len(suffix) - 1
	}
	val := abs / math.Pow(base, float64(exp))

	// 1 decimal, strip trailing .0
	s := fmt.Sprintf("%.1f", val)
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")

	if b < 0 {
		s = "-" + s
	}
	return s, suffix[exp]
}

func ToKB(b int64) float64  { return float64(b) / float64(KB) }
func ToMB(b int64) float64  { return float64(b) / float64(MB) }
func ToGB(b int64) float64  { return float64(b) / float64(GB) }
func ToTB(b int64) float64  { return float64(b) / float64(TB) }
func ToKiB(b int64) float64 { return float64(b) / float64(KiB) }
func ToMiB(b int64) float64 { return float64(b) / float64(MiB) }
func ToGiB(b int64) float64 { return float64(b) / float64(GiB) }
func ToTiB(b int64) float64 { return float64(b) / float64(TiB) }

func FromKB(kb float64) int64   { return int64(kb * float64(KB)) }
func FromMB(mb float64) int64   { return int64(mb * float64(MB)) }
func FromGB(gb float64) int64   { return int64(gb * float64(GB)) }
func FromTB(tb float64) int64   { return int64(tb * float64(TB)) }
func FromKiB(kib float64) int64 { return int64(kib * float64(KiB)) }
func FromMiB(mib float64) int64 { return int64(mib * float64(MiB)) }
func FromGiB(gib float64) int64 { return int64(gib * float64(GiB)) }
func FromTiB(tib float64) int64 { return int64(tib * float64(TiB)) }

// Accepts: "1.5GiB", "200MB", "42 kb", "3g", "7miB", "1024", "512B"
// Case-insensitive; spaces allowed.
// Short SI without 'B' also accepted: "1k", "2m", "3g" (→ KB, MB, GB).
var (
	reSize = regexp.MustCompile(`^\s*([+-]?\d+(?:\.\d+)?)\s*([a-zA-Z]+)?\s*$`)

	unitMap = map[string]int64{
		"b":  1,
		"kb": KB, "mb": MB, "gb": GB, "tb": TB, "pb": PB, "eb": EB,
		"k": KB, "m": MB, "g": GB, "t": TB, "p": PB, "e": EB,
		"kib": KiB, "mib": MiB, "gib": GiB, "tib": TiB, "pib": PiB, "eib": EiB,
		"ki": KiB, "mi": MiB, "gi": GiB, "ti": TiB, "pi": PiB, "ei": EiB,
	}
)

// ParseBytes parses strings like "1.5GiB" or "200MB" into bytes.
func ParseBytes(s string) (int64, error) {
	m := reSize.FindStringSubmatch(strings.TrimSpace(s))
	if len(m) != 3 {
		return 0, fmt.Errorf("bytesize: invalid size %q", s)
	}
	numStr := m[1]
	unitStr := strings.ToLower(strings.TrimSpace(m[2]))

	v, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, fmt.Errorf("bytesize: invalid number %q", numStr)
	}

	// default to bytes if no unit given
	if unitStr == "" {
		return int64(v), nil
	}

	mult, ok := unitMap[unitStr]
	if !ok {
		return 0, fmt.Errorf("bytesize: unknown unit %q", unitStr)
	}

	// protect against overflows by capping at MaxInt64
	res := v * float64(mult)
	if res > float64(math.MaxInt64) {
		return 0, fmt.Errorf("bytesize: value overflows int64")
	}
	if res < float64(math.MinInt64) {
		return 0, fmt.Errorf("bytesize: value underflows int64")
	}
	return int64(res), nil
}

// MustParseBytes panics if parsing fails.
func MustParseBytes(s string) int64 {
	n, err := ParseBytes(s)
	if err != nil {
		panic(err)
	}
	return n
}
