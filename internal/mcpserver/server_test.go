package mcpserver

import (
	"testing"
	"time"
)

func TestDurationMillisecondsRejectsOverflow(t *testing.T) {
	if _, err := durationMilliseconds(-1); err == nil {
		t.Fatal("expected negative timeout rejection")
	}
	maximum := int(^uint64(0)>>1) / int(time.Millisecond)
	if _, err := durationMilliseconds(maximum); err != nil {
		t.Fatal(err)
	}
	if maximum < int(^uint(0)>>1) {
		if _, err := durationMilliseconds(maximum + 1); err == nil {
			t.Fatal("expected overflowing timeout rejection")
		}
	}
}

func TestValidateBatchCountRejectsTransportAmplification(t *testing.T) {
	for _, count := range []int{0, 101} {
		if err := validateBatchCount(count); err == nil {
			t.Fatalf("expected batch count %d rejection", count)
		}
	}
	if err := validateBatchCount(100); err != nil {
		t.Fatal(err)
	}
}
