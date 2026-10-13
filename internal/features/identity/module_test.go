package identity

import "testing"

func TestNewRequiresDatabase(t *testing.T) {
	module, err := New(Dependencies{})
	if err == nil || module != nil {
		t.Fatalf("expected missing database error, got module=%v error=%v", module, err)
	}
}
