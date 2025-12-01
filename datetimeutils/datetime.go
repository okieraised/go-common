package datetimeutils

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

// CommonLayouts is a curated list of practical formats tried by ParseAny.
// Add or override at call sites if you like.
var CommonLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	time.RFC1123Z, time.RFC1123,
	time.RFC850,
	time.ANSIC, time.UnixDate, time.RubyDate,
	"2006-01-02 15:04:05.999999999Z07:00",
	"2006-01-02 15:04:05Z07:00",
	"2006-01-02 15:04:05",
	"2006/01/02 15:04:05",
	"2006-01-02",
	"02/01/2006 15:04:05",
	"02/01/2006",
	"01/02/2006",
	"20060102T150405Z07:00",
	"20060102",
}

// MustParseLayout parses s with layout in loc (or time.Local if nil) and panics on error.
func MustParseLayout(layout, s string, loc *time.Location) time.Time {
	t, err := ParseLayout(layout, s, loc)
	if err != nil {
		panic(err)
	}
	return t
}

// ParseLayout parses s with layout in loc (or time.Local if nil).
func ParseLayout(layout, s string, loc *time.Location) (time.Time, error) {
	if loc == nil {
		loc = time.Local
	}
	return time.ParseInLocation(layout, s, loc)
}

// ParseAny tries many layouts (CommonLayouts by default). If a layout has no zone,
// it parses in loc (or time.Local).
func ParseAny(s string, loc *time.Location, layouts ...string) (time.Time, error) {
	if loc == nil {
		loc = time.Local
	}
	ls := layouts
	if len(ls) == 0 {
		ls = CommonLayouts
	}
	// try time.Parse first (it respects zones when present)
	for _, layout := range ls {
		// If the layout contains 'Z07:00' or MST, use time.Parse; else ParseInLocation
		if strings.Contains(layout, "Z07") || strings.Contains(layout, "MST") {
			if t, err := time.Parse(layout, s); err == nil {
				return t, nil
			}
		}
		if t, err := time.ParseInLocation(layout, s, loc); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("no format matched: %q", s)
}

// StartOfDay returns 00:00:00.000 in loc.
func StartOfDay(t time.Time, loc *time.Location) time.Time {
	if loc == nil {
		loc = t.Location()
	}
	y, m, d := t.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, loc)
}

// EndOfDay returns the last representable instant within the day in loc.
func EndOfDay(t time.Time, loc *time.Location) time.Time {
	sod := StartOfDay(t, loc)
	return sod.Add(24*time.Hour - time.Nanosecond)
}

// StartOfWeek returns the start of the week at 00:00:00 in loc,
// where weekStart is the first day (e.g., time.Monday or time.Sunday).
func StartOfWeek(t time.Time, weekStart time.Weekday, loc *time.Location) time.Time {
	if loc == nil {
		loc = t.Location()
	}
	ti := t.In(loc)
	offset := (7 + int(ti.Weekday()) - int(weekStart)) % 7
	return StartOfDay(ti.AddDate(0, 0, -offset), loc)
}

func EndOfWeek(t time.Time, weekStart time.Weekday, loc *time.Location) time.Time {
	return StartOfWeek(t, weekStart, loc).AddDate(0, 0, 7).Add(-time.Nanosecond)
}

func StartOfMonth(t time.Time, loc *time.Location) time.Time {
	if loc == nil {
		loc = t.Location()
	}
	y, m, _ := t.In(loc).Date()
	return time.Date(y, m, 1, 0, 0, 0, 0, loc)
}

func EndOfMonth(t time.Time, loc *time.Location) time.Time {
	som := StartOfMonth(t, loc)
	return som.AddDate(0, 1, 0).Add(-time.Nanosecond)
}

// Truncate to arbitrary duration.
func Truncate(t time.Time, d time.Duration) time.Time {
	return t.Truncate(d)
}

func Floor(t time.Time, d time.Duration) time.Time {
	// equal to Truncate for positive durations
	return t.Truncate(d)
}

func Ceil(t time.Time, d time.Duration) time.Time {
	if d <= 0 {
		return t
	}
	tr := t.Truncate(d)
	if tr.Equal(t) {
		return t
	}
	return tr.Add(d)
}

// AddMonthsSafe adds months while clamping day to the end of target month.
// Example: Jan 31 + 1 month => Feb 28 (or 29), not Mar 02 via overflow.
func AddMonthsSafe(t time.Time, months int) time.Time {
	y, m, d := t.Date()
	h, mm, s := t.Clock()
	ns := t.Nanosecond()

	// Compute target year/month without overflow
	totalMonths := int(m) + months
	targetY := y + (totalMonths-1)/12
	targetM := time.Month((totalMonths-1)%12 + 1)

	// Clamp day into target month
	lastDay := time.Date(targetY, targetM+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if d > lastDay {
		d = lastDay
	}
	return time.Date(targetY, targetM, d, h, mm, s, ns, t.Location())
}

func lastDayOfMonth(y int, m time.Month) int {
	// day 0 of next month is last day of current
	return time.Date(y, m+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// AddYearsSafe like AddMonthsSafe but for years.
func AddYearsSafe(t time.Time, years int) time.Time {
	return AddMonthsSafe(t, years*12)
}

// HolidaySet stores YYYY-MM-DD keys in a specific timezone (loc).
type HolidaySet struct {
	loc   *time.Location
	dates map[string]struct{}
}

func NewHolidaySet(loc *time.Location, dates ...time.Time) *HolidaySet {
	if loc == nil {
		loc = time.Local
	}
	m := make(map[string]struct{}, len(dates))
	hs := &HolidaySet{loc: loc, dates: m}
	for _, d := range dates {
		hs.Add(d)
	}
	return hs
}

func (h *HolidaySet) Add(d time.Time) {
	key := ymdKey(d.In(h.loc))
	h.dates[key] = struct{}{}
}

func (h *HolidaySet) Has(d time.Time) bool {
	_, ok := h.dates[ymdKey(d.In(h.loc))]
	return ok
}

func ymdKey(t time.Time) string {
	y, m, d := t.Date()
	return fmt.Sprintf("%04d-%02d-%02d", y, m, d)
}

func IsBusinessDay(t time.Time, holidays *HolidaySet) bool {
	loc := holoc(holidays, t.Location())
	ti := t.In(loc)
	w := ti.Weekday()
	if w == time.Saturday || w == time.Sunday {
		return false
	}
	return holidays == nil || !holidays.Has(ti)
}

func NextBusinessDay(t time.Time, holidays *HolidaySet) time.Time {
	ti := StartOfDay(t, holidays.loc)
	for {
		ti = ti.AddDate(0, 0, 1)
		if IsBusinessDay(ti, holidays) {
			return ti
		}
	}
}

// AddBusinessDays moves forward (k>0) or backward (k<0) by business days.
func AddBusinessDays(t time.Time, k int, holidays *HolidaySet) time.Time {
	step := 1
	if k < 0 {
		step = -1
		k = -k
	}
	ti := StartOfDay(t, holidays.loc)
	for k > 0 {
		ti = ti.AddDate(0, 0, step)
		if IsBusinessDay(ti, holidays) {
			k--
		}
	}
	// keep the original wall time-of-day
	h, m, s := t.Clock()
	return time.Date(ti.Year(), ti.Month(), ti.Day(), h, m, s, t.Nanosecond(), holidays.loc)
}

// BusinessDaysBetween counts business days in [start, end) (start inclusive, end exclusive).
func BusinessDaysBetween(start, end time.Time, holidays *HolidaySet) int {
	if end.Before(start) {
		start, end = end, start
	}
	n := 0
	for d := StartOfDay(start, holidays.loc); d.Before(end); d = d.AddDate(0, 0, 1) {
		if IsBusinessDay(d, holidays) {
			n++
		}
	}
	return n
}

// HumanizeDuration produces friendly text, e.g., "2d 3h", "1h 2m", "5s".
// Use maxParts to limit components (e.g., 2 -> "2h 3m" instead of "2h 3m 4s").
func HumanizeDuration(d time.Duration, maxParts int) string {
	if d == 0 {
		return "0s"
	}
	if d < 0 {
		return "-" + HumanizeDuration(-d, maxParts)
	}
	type part struct {
		name string
		secs int64
	}
	parts := []part{
		{"w", 7 * 24 * 3600},
		{"d", 24 * 3600},
		{"h", 3600},
		{"m", 60},
		{"s", 1},
		{"ms", 0}, // handled specially
	}
	remain := d
	out := []string{}
	add := func(v int64, suffix string) {
		out = append(out, fmt.Sprintf("%d%s", v, suffix))
	}
	seconds := int64(remain / time.Second)
	nanos := remain - time.Duration(seconds)*time.Second
	for _, p := range parts {
		if p.name == "ms" {
			ms := nanos / time.Millisecond
			if ms > 0 && (maxParts <= 0 || len(out) < maxParts) {
				add(int64(ms), "ms")
			}
			break
		}
		if seconds >= p.secs {
			v := seconds / p.secs
			add(v, p.name)
			seconds -= v * p.secs
			if maxParts > 0 && len(out) >= maxParts {
				break
			}
		}
	}
	return strings.Join(out, " ")
}

// SinceApprox returns "x ago" / "in x" with coarse units.
func SinceApprox(t time.Time, ref time.Time) string {
	d := ref.Sub(t)
	f := "ago"
	if d < 0 {
		d = -d
		f = "from now"
	}
	sec := d.Seconds()
	switch {
	case sec < 60:
		return fmt.Sprintf("%ds %s", int(sec), f)
	case sec < 3600:
		return fmt.Sprintf("%dm %s", int(sec/60), f)
	case sec < 86400:
		return fmt.Sprintf("%dh %s", int(sec/3600), f)
	case sec < 86400*7:
		return fmt.Sprintf("%dd %s", int(sec/86400), f)
	case sec < 86400*30:
		return fmt.Sprintf("%dw %s", int(sec/(86400*7)), f)
	case sec < 86400*365:
		return fmt.Sprintf("%dmo %s", int(sec/(86400*30)), f)
	default:
		return fmt.Sprintf("%dy %s", int(sec/(86400*365)), f)
	}
}

// ParseDurationFlexible extends time.ParseDuration with units like "day(s)" and "week(s)".
// Supports examples: "90m", "1.5h", "2h30m", "2 days", "1w2d3h".
func ParseDurationFlexible(s string) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0, errors.New("empty duration")
	}
	// Fast path: stdlib understands it
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}
	type unit struct {
		keys []string
		dur  time.Duration
	}
	units := []unit{
		{[]string{"w", "week", "weeks"}, 7 * 24 * time.Hour},
		{[]string{"d", "day", "days"}, 24 * time.Hour},
		{[]string{"h", "hour", "hours"}, time.Hour},
		{[]string{"m", "min", "mins", "minute", "minutes"}, time.Minute},
		{[]string{"s", "sec", "secs", "second", "seconds"}, time.Second},
		{[]string{"ms", "msec", "millisecond", "milliseconds"}, time.Millisecond},
		{[]string{"us", "usec", "microsecond", "microseconds"}, time.Microsecond},
	}
	// tokenize like "1w2d3h", "2 days 4 hours", "1.5h"
	var total time.Duration
	num := ""
	for i := 0; i < len(s); {
		ch := s[i]
		if (ch >= '0' && ch <= '9') || ch == '.' {
			num += string(ch)
			i++
			continue
		}
		// read unit word
		j := i
		for j < len(s) && s[j] != ' ' && ((s[j] < '0' || s[j] > '9') && s[j] != '.') {
			j++
		}
		word := strings.TrimSpace(s[i:j])
		// skip spaces
		for j < len(s) && s[j] == ' ' {
			j++
		}
		i = j
		if num == "" || word == "" {
			return 0, fmt.Errorf("invalid duration near %q", s[i:])
		}
		val, err := strconv.ParseFloat(num, 64)
		if err != nil {
			return 0, err
		}
		num = ""
		found := false
		for _, u := range units {
			for _, k := range u.keys {
				if word == k {
					total += time.Duration(val * float64(u.dur))
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			// allow concatenated like "1w2d" where word could be "w" and next token starts with digit
			// if not a known unit, error
			return 0, fmt.Errorf("unknown unit %q", word)
		}
	}
	// trailing number without a unit? reject, to avoid silent mistakes
	if num != "" {
		return 0, fmt.Errorf("dangling number %q without unit", num)
	}
	return total, nil
}

func UnixMillis(t time.Time) int64 {
	return t.UnixNano() / int64(time.Millisecond)
}
func UnixMicros(t time.Time) int64 {
	return t.UnixNano() / int64(time.Microsecond)
}
func FromUnixMillis(ms int64, loc *time.Location) time.Time {
	t := time.Unix(0, ms*int64(time.Millisecond))
	if loc == nil {
		return t
	}
	return t.In(loc)
}
func FromUnixMicros(us int64, loc *time.Location) time.Time {
	t := time.Unix(0, us*int64(time.Microsecond))
	if loc == nil {
		return t
	}
	return t.In(loc)
}

// ExpoBackoff returns a function that yields an exponential backoff duration
// with an optional full jitter. Example: base=100ms, factor=2, max=10s.
// If jitter=true, each call returns rand in [0, backoff] (AWS "Full Jitter").
// If jitter=false, returns the deterministic backoff.
func ExpoBackoff(base time.Duration, factor float64, max time.Duration, jitter bool, r *rand.Rand) func(int) time.Duration {
	if factor <= 1 {
		factor = 2
	}
	if base <= 0 {
		base = time.Millisecond
	}
	return func(retry int) time.Duration {
		if retry < 0 {
			retry = 0
		}
		// calculate backoff = base * factor^retry, cap at max
		pow := math.Pow(factor, float64(retry))
		backoff := time.Duration(float64(base) * pow)
		if max > 0 && backoff > max {
			backoff = max
		}
		if !jitter {
			return backoff
		}
		if r == nil {
			r = rand.New(rand.NewSource(time.Now().UnixNano()))
		}
		if backoff <= 0 {
			return 0
		}
		return time.Duration(r.Int63n(int64(backoff) + 1))
	}
}

func holoc(holidays *HolidaySet, fallback *time.Location) *time.Location {
	if holidays != nil && holidays.loc != nil {
		return holidays.loc
	}
	if fallback != nil {
		return fallback
	}
	return time.Local
}

// StartOfISOWeek returns Monday 00:00:00 of t's ISO week in loc.
func StartOfISOWeek(t time.Time, loc *time.Location) time.Time {
	if loc == nil {
		loc = t.Location()
	}
	ti := t.In(loc)
	// Shift to Thursday to get the correct ISO week-year, then go back to Monday
	weekday := int(ti.Weekday())
	if weekday == 0 {
		weekday = 7
	} // Sunday=>7
	// Move to Monday
	return time.Date(ti.Year(), ti.Month(), ti.Day()-weekday+1, 0, 0, 0, 0, loc)
}

func EndOfISOWeek(t time.Time, loc *time.Location) time.Time {
	return StartOfISOWeek(t, loc).AddDate(0, 0, 7).Add(-time.Nanosecond)
}

// ISOWeekYear matches the semantics of ISO-8601: returns (year, week).
func ISOWeekYear(t time.Time, loc *time.Location) (int, int) {
	if loc == nil {
		loc = t.Location()
	}
	y, w := t.In(loc).ISOWeek()
	return y, w
}

// StartOfISOYear returns the Monday of the week containing Jan 4 (ISO rule).
func StartOfISOYear(year int, loc *time.Location) time.Time {
	if loc == nil {
		loc = time.Local
	}
	jan4 := time.Date(year, 1, 4, 0, 0, 0, 0, loc)
	return StartOfISOWeek(jan4, loc)
}

// EndOfISOYear returns the last nanosecond of the last ISO week in year.
func EndOfISOYear(year int, loc *time.Location) time.Time {
	return StartOfISOYear(year+1, loc).Add(-time.Nanosecond)
}

// BucketIterator iterates [start,end) in fixed-size buckets aligned by alignFn.
type BucketIterator struct {
	Start time.Time
	End   time.Time
	Step  time.Duration
	// AlignFn takes a time and returns the aligned floor (e.g., StartOfDay).
	AlignFn func(time.Time) time.Time
}

// NewBucketIterator creates an iterator over [start,end) in Step-sized buckets.
// It aligns the first bucket start using AlignFn.
func NewBucketIterator(start, end time.Time, step time.Duration, alignFn func(time.Time) time.Time) BucketIterator {
	if alignFn == nil {
		alignFn = func(t time.Time) time.Time { return t.Truncate(step) }
	}
	s0 := alignFn(start)
	if s0.Before(start) {
		// move to the first bucket that overlaps start ceil to the next boundary: aligned + k*step >= start
		delta := start.Sub(s0)
		k := int64((delta + step - 1) / step)
		s0 = s0.Add(time.Duration(k) * step)
	}
	return BucketIterator{Start: s0, End: end, Step: step, AlignFn: alignFn}
}

// Each calls fn(bucketStart, bucketEnd) for each bucket in [Start,End).
func (b BucketIterator) Each(fn func(time.Time, time.Time)) {
	for s := b.Start; s.Before(b.End); s = s.Add(b.Step) {
		e := s.Add(b.Step)
		if e.After(b.End) {
			e = b.End
		}
		fn(s, e)
	}
}

// NewDailyBuckets builds a day-aligned iterator in loc.
func NewDailyBuckets(start, end time.Time, loc *time.Location) BucketIterator {
	align := func(t time.Time) time.Time { return StartOfDay(t, loc) }
	return NewBucketIterator(start, end, 24*time.Hour, align)
}

// Clock abstracts time functions used in code to aid testing.
type Clock interface {
	Now() time.Time
	Sleep(d time.Duration)
	After(d time.Duration) <-chan time.Time
	Since(t time.Time) time.Duration
}

// RealClock uses the actual time package.
type RealClock struct{}

func (RealClock) Now() time.Time                         { return time.Now() }
func (RealClock) Sleep(d time.Duration)                  { time.Sleep(d) }
func (RealClock) After(d time.Duration) <-chan time.Time { return time.After(d) }
func (RealClock) Since(t time.Time) time.Duration        { return time.Since(t) }

// FrozenClock is a controllable clock for tests.
type FrozenClock struct {
	now time.Time
	ch  chan time.Time
}

// NewFrozenClock initializes a frozen clock at the given time.
func NewFrozenClock(t time.Time) *FrozenClock {
	return &FrozenClock{now: t, ch: make(chan time.Time, 1)}
}

func (f *FrozenClock) Now() time.Time        { return f.now }
func (f *FrozenClock) Sleep(d time.Duration) { f.Advance(d) }
func (f *FrozenClock) After(d time.Duration) <-chan time.Time {
	// A simple implementation: advance externally and send when appropriate.
	// For many tests, you can manually Advance(d) and read from the channel.
	return f.ch
}
func (f *FrozenClock) Since(t time.Time) time.Duration { return f.now.Sub(t) }

// Advance moves the clock forward and triggers After channel once per call.
func (f *FrozenClock) Advance(d time.Duration) {
	f.now = f.now.Add(d)
	select {
	case f.ch <- f.now:
	default:
	}
}

// NewISOWeeklyBuckets iterates ISO weeks [start,end) using Monday 00:00 boundaries.
func NewISOWeeklyBuckets(start, end time.Time, loc *time.Location) BucketIterator {
	align := func(t time.Time) time.Time { return StartOfISOWeek(t, loc) }
	// step is 7 days, but we use alignment to handle DST nicely
	return NewBucketIterator(start, end, 7*24*time.Hour, align)
}
