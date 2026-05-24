package or

import (
	"testing"
	"time"
)

func TestOrReturnsOnEarliest(t *testing.T) {
	start := time.Now()
	ch1 := sig(200 * time.Millisecond)
	ch2 := sig(100 * time.Millisecond)
	ch3 := sig(300 * time.Millisecond)

	<-Or(ch1, ch2, ch3)
	elapsed := time.Since(start)
	if elapsed > 150*time.Millisecond {
		t.Fatalf("Or did not return early enough, elapsed=%v", elapsed)
	}
}

func TestOrSingleChannel(t *testing.T) {
	start := time.Now()
	ch := sig(50 * time.Millisecond)
	<-Or(ch)
	if time.Since(start) < 50*time.Millisecond {
		t.Fatalf("Or returned before channel closed")
	}
}

func TestOrNoChannels(t *testing.T) {
	if Or() != nil {
		t.Fatalf("Or with no channels should return nil")
	}
}
