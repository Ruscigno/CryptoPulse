package timeframe

import (
	"testing"
	"time"
)

func TestNativeLookup(t *testing.T) {
	tf, ok := Get("1h")
	if !ok {
		t.Fatal("1h not found")
	}
	if !tf.Native {
		t.Error("1h should be native")
	}
	if tf.YahooInterval != "60m" {
		t.Errorf("1h YahooInterval = %q, want 60m", tf.YahooInterval)
	}
}

func TestDerivedLookup(t *testing.T) {
	tf, ok := Get("4h")
	if !ok {
		t.Fatal("4h not found")
	}
	if tf.Native {
		t.Error("4h should be derived")
	}
	if tf.Parent != "1h" || tf.GroupSize != 4 {
		t.Errorf("4h parent=%q group=%d, want 1h/4", tf.Parent, tf.GroupSize)
	}
}

func TestUnknown(t *testing.T) {
	if _, ok := Get("7m"); ok {
		t.Error("7m should not exist")
	}
}

func TestBucketStart4h(t *testing.T) {
	tf, _ := Get("4h")
	in := time.Date(2026, 6, 16, 14, 30, 0, 0, time.UTC) // 14:30 -> 12:00 bucket
	got := tf.BucketStart(in)
	want := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("BucketStart = %v, want %v", got, want)
	}
}

func TestBucketStart3d(t *testing.T) {
	tf, _ := Get("3d")
	in := time.Date(2026, 6, 16, 9, 0, 0, 0, time.UTC)
	got := tf.BucketStart(in)
	// 3-day buckets anchored to the Unix epoch; verified literal.
	// 2026-06-16 is Unix day 20620; 20620 % 3 == 1, so bucket starts at day 20619 = 2026-06-15.
	want := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("BucketStart = %v, want %v", got, want)
	}
}
