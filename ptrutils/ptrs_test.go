package ptrutils

import (
	"reflect"
	"testing"
)

func mustPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic, but did not panic")
		}
	}()
	fn()
}

type cmpStruct struct {
	A int
	B string
}

func TestPtrAndDeref(t *testing.T) {
	p := Ptr(42)
	if p == nil || *p != 42 {
		t.Fatalf("Ptr failed: got %v, *p=%v", p, func() any {
			if p == nil {
				return nil
			}
			return *p
		}())
	}

	if got := Deref(p); got != 42 {
		t.Fatalf("Deref failed: got %v, want 42", got)
	}
	mustPanic(t, func() { _ = Deref[int](nil) })
}

//func TestPtrOrNil(t *testing.T) {
//	if got := PtrOrNil; got != nil {
//		t.Fatalf("PtrOrNil zero int: got non-nil")
//	}
//	if got := PtrOrNil[string](""); got != nil {
//		t.Fatalf("PtrOrNil zero string: got non-nil")
//	}
//	s := cmpStruct{} // zero
//	if got := PtrOrNil(s); got != nil {
//		t.Fatalf("PtrOrNil zero struct: got non-nil")
//	}
//
//	if got := PtrOrNil; got == nil || *got != 5 {
//		t.Fatalf("PtrOrNil non-zero int: got %v,*=%v", got, func() any {
//			if got == nil {
//				return nil
//			}
//			return *got
//		}())
//	}
//	if got := PtrOrNil[string]("x"); got == nil || *got != "x" {
//		t.Fatalf("PtrOrNil non-zero string: got %v", got)
//	}
//}

func TestDerefZero(t *testing.T) {
	if got := DerefZero[int](nil); got != 0 {
		t.Fatalf("DerefZero(nil int) = %v, want 0", got)
	}
	if got := DerefZero[string](nil); got != "" {
		t.Fatalf("DerefZero(nil string) = %q, want empty", got)
	}
	v := 7
	if got := DerefZero(&v); got != 7 {
		t.Fatalf("DerefZero(&7) = %v, want 7", got)
	}
}

func TestIsNil(t *testing.T) {
	if !IsNil[int](nil) {
		t.Fatalf("IsNil(nil) = false, want true")
	}
	x := 3
	if IsNil(&x) {
		t.Fatalf("IsNil(non-nil) = true, want false")
	}
}

func TestIsZero(t *testing.T) {
	var pInt *int
	if IsZero(pInt) {
		t.Fatalf("IsZero(nil) = true, want false")
	}
	z := 0
	nz := 1
	if !IsZero(&z) {
		t.Fatalf("IsZero(&0) = false, want true")
	}
	if IsZero(&nz) {
		t.Fatalf("IsZero(&1) = true, want false")
	}

	zs := cmpStruct{}
	ns := cmpStruct{A: 1}
	if !IsZero(&zs) || IsZero(&ns) == true {
	}
	if !IsZero(&zs) {
		t.Fatalf("IsZero(&zero struct) = false, want true")
	}
	if IsZero(&ns) {
		t.Fatalf("IsZero(&non-zero struct) = true, want false")
	}
}

func TestIsZeroOrNil(t *testing.T) {
	if !IsZeroOrNil[int](nil) {
		t.Fatalf("IsZeroOrNil(nil) = false, want true")
	}
	z := 0
	nz := 2
	if !IsZeroOrNil(&z) {
		t.Fatalf("IsZeroOrNil(&0) = false, want true")
	}
	if IsZeroOrNil(&nz) {
		t.Fatalf("IsZeroOrNil(&2) = true, want false")
	}
}

func TestEqual(t *testing.T) {
	var a, b *int
	if !Equal(a, b) {
		t.Fatalf("Equal(nil,nil) = false, want true")
	}
	x := 1
	if Equal(&x, nil) || Equal(nil, &x) {
		t.Fatalf("Equal(one nil) = true, want false")
	}
	y := 1
	if !Equal(&x, &y) {
		t.Fatalf("Equal(&1,&1) = false, want true")
	}
	z := 2
	if Equal(&x, &z) {
		t.Fatalf("Equal(&1,&2) = true, want false")
	}
}

func TestSame(t *testing.T) {
	x := 5
	y := 5
	if !Same(&x, &x) {
		t.Fatalf("Same(&x,&x) = false, want true")
	}
	if Same(&x, &y) {
		t.Fatalf("Same(&x,&y same value different addr) = true, want false")
	}
	if Same[int](nil, nil) != true {
		t.Fatalf("Same(nil,nil) = false, want true (both nil addresses are equal)")
	}
}

func TestClone(t *testing.T) {
	if Clone[int](nil) != nil {
		t.Fatalf("Clone(nil) != nil")
	}
	v := 9
	c := Clone(&v)
	if c == nil || *c != 9 {
		t.Fatalf("Clone(&9) failed: %v,*=%v", c, func() any {
			if c == nil {
				return nil
			}
			return *c
		}())
	}
	if c == &v {
		t.Fatalf("Clone returned the same pointer; want a copy")
	}
}

func TestCopyToPtr(t *testing.T) {
	def := 77

	// nil -> copy of def (distinct pointer)
	c1 := CopyToPtr[int](nil, def)
	if c1 == nil || *c1 != def {
		t.Fatalf("CopyToPtr(nil,def) failed: %v,*=%v", c1, func() any {
			if c1 == nil {
				return nil
			}
			return *c1
		}())
	}

	src := 12
	c2 := CopyToPtr(&src, def)
	if c2 == nil || *c2 != 12 {
		t.Fatalf("CopyToPtr(&12,def) -> %v,*=%v", c2, func() any {
			if c2 == nil {
				return nil
			}
			return *c2
		}())
	}
	if c2 == &src {
		t.Fatalf("CopyToPtr returned alias; want distinct pointer")
	}
}

func TestSetIfNil(t *testing.T) {
	var p *int
	SetIfNil(&p, 3)
	if p == nil || *p != 3 {
		t.Fatalf("SetIfNil should set when nil; got %v,*=%v", p, func() any {
			if p == nil {
				return nil
			}
			return *p
		}())
	}

	SetIfNil(&p, 5)
	if *p != 3 {
		t.Fatalf("SetIfNil overwrote existing value; got %v, want 3", *p)
	}
}

func TestZeroIfNil(t *testing.T) {
	if got := ZeroIfNil[int](nil); got == nil || *got != 0 {
		t.Fatalf("ZeroIfNil(nil) -> %v,*=%v; want non-nil &0", got, func() any {
			if got == nil {
				return nil
			}
			return *got
		}())
	}
	x := 10
	if got := ZeroIfNil(&x); got != &x {
		t.Fatalf("ZeroIfNil(non-nil) should return same pointer")
	}
}

func TestSliceOfPtrsAndPtrsToValues(t *testing.T) {
	vals := []int{1, 2, 3, 0}
	ptrs := SliceOfPtrs(vals)

	if len(ptrs) != len(vals) {
		t.Fatalf("SliceOfPtrs length mismatch: got %d, want %d", len(ptrs), len(vals))
	}

	for i := range vals {
		if ptrs[i] == nil || *ptrs[i] != vals[i] {
			t.Fatalf("ptrs[%d] -> %v,*=%v; want &vals[%d]=%d", i, ptrs[i], func() any {
				if ptrs[i] == nil {
					return nil
				}
				return *ptrs[i]
			}(), i, vals[i])
		}
	}

	vals[1] = 20
	if *ptrs[1] != 20 {
		t.Fatalf("pointer did not reflect underlying mutation: got %d, want 20", *ptrs[1])
	}

	ptrs2 := []*int{ptrs[0], nil, ptrs[2]}
	back := PtrsToValues(ptrs2)
	want := []int{1, 0, 3}
	if !reflect.DeepEqual(back, want) {
		t.Fatalf("PtrsToValues mismatch: got %v, want %v", back, want)
	}
}

func TestIsNilInterface(t *testing.T) {
	var i1 interface{} = nil
	if !IsNilInterface(i1) {
		t.Fatalf("IsNilInterface(nil) = false, want true")
	}

	var p *int = nil
	var i2 interface{} = p
	if !IsNilInterface(i2) {
		t.Fatalf("IsNilInterface((*int)(nil)) = false, want true")
	}

	x := 5
	var i3 interface{} = &x
	if IsNilInterface(i3) {
		t.Fatalf("IsNilInterface(&x) = true, want false")
	}

	var s []int
	var m map[string]int
	var ch chan int
	var fn func()
	if !IsNilInterface(s) || !IsNilInterface(m) || !IsNilInterface(ch) || !IsNilInterface(fn) {
		t.Fatalf("IsNilInterface on nil slice/map/chan/func should be true")
	}

	s = []int{}
	m = map[string]int{}
	if IsNilInterface(s) || IsNilInterface(m) {
		t.Fatalf("IsNilInterface on non-nil empty slice/map should be false")
	}

	if IsNilInterface(0) || IsNilInterface(struct{}{}) {
		t.Fatalf("IsNilInterface on zero concrete types should be false")
	}
}
