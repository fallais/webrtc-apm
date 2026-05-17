package framer

import "testing"

func TestFramer_Empty(t *testing.T) {
	f := New(480)
	dst := make([]int16, 480)
	if f.Pop(dst) {
		t.Fatal("Pop on empty framer should return false")
	}
	if f.Buffered() != 0 {
		t.Fatalf("Buffered=%d, want 0", f.Buffered())
	}
}

func TestFramer_PartialFill(t *testing.T) {
	f := New(480)
	f.Push(make([]int16, 100))
	dst := make([]int16, 480)
	if f.Pop(dst) {
		t.Fatal("Pop with insufficient data should return false")
	}
	if f.Buffered() != 100 {
		t.Fatalf("Buffered=%d, want 100", f.Buffered())
	}
}

func TestFramer_MultipleFrames(t *testing.T) {
	f := New(480)
	f.Push(make([]int16, 1000))
	dst := make([]int16, 480)

	if !f.Pop(dst) {
		t.Fatal("first Pop should succeed")
	}
	if !f.Pop(dst) {
		t.Fatal("second Pop should succeed")
	}
	if f.Pop(dst) {
		t.Fatal("third Pop should fail with only 40 samples buffered")
	}
	if got, want := f.Buffered(), 40; got != want {
		t.Fatalf("Buffered=%d, want %d", got, want)
	}
}

func TestFramer_PreservesContent(t *testing.T) {
	f := New(480)

	sig := make([]int16, 480)
	for i := range sig {
		sig[i] = int16(i)
	}
	f.Push(sig)

	out := make([]int16, 480)
	if !f.Pop(out) {
		t.Fatal("Pop should succeed")
	}
	for i, v := range out {
		if v != int16(i) {
			t.Fatalf("out[%d]=%d, want %d", i, v, int16(i))
		}
	}
}

func TestFramer_BadDstSize(t *testing.T) {
	f := New(480)
	f.Push(make([]int16, 480))
	short := make([]int16, 100)
	if f.Pop(short) {
		t.Fatal("Pop with mismatched dst size should return false")
	}
	if f.Buffered() != 480 {
		t.Fatalf("Buffered=%d, want 480 (Pop should not consume on bad dst)", f.Buffered())
	}
}

func TestFramer_Reset(t *testing.T) {
	f := New(480)
	f.Push(make([]int16, 300))
	f.Reset()
	if f.Buffered() != 0 {
		t.Fatalf("Buffered=%d, want 0 after Reset", f.Buffered())
	}
}

func TestFramer_CrossBoundary(t *testing.T) {
	f := New(480)
	f.Push(make([]int16, 500))
	dst := make([]int16, 480)
	if !f.Pop(dst) {
		t.Fatal("Pop with 500 buffered should succeed")
	}
	if f.Buffered() != 20 {
		t.Fatalf("Buffered=%d, want 20", f.Buffered())
	}
	f.Push(make([]int16, 460)) // total 480 again
	if !f.Pop(dst) {
		t.Fatal("Pop after additional push should succeed")
	}
	if f.Buffered() != 0 {
		t.Fatalf("Buffered=%d, want 0", f.Buffered())
	}
}
