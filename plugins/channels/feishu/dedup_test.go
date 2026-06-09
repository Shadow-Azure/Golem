package feishu

import (
	"testing"
	"time"
)

func TestDeduplicator_IsDuplicate(t *testing.T) {
	d := NewDeduplicator(time.Minute)

	if d.IsDuplicate("msg1") {
		t.Error("first message should not be duplicate")
	}

	if !d.IsDuplicate("msg1") {
		t.Error("second message should be duplicate")
	}

	if d.IsDuplicate("msg2") {
		t.Error("different message should not be duplicate")
	}
}

func TestDeduplicator_Expiration(t *testing.T) {
	d := NewDeduplicator(50 * time.Millisecond)

	d.IsDuplicate("msg1")
	time.Sleep(100 * time.Millisecond)

	if d.IsDuplicate("msg1") {
		t.Error("message should not be duplicate after expiration")
	}
}
