package r2

import "testing"

func TestValidateObjectKey(t *testing.T) {
	valid := "organizations/tenant/locations/location/media/file.jpg"
	if err := validateObjectKey(valid); err != nil {
		t.Fatalf("expected valid object key: %v", err)
	}

	invalid := []string{
		"public/file.jpg",
		"/organizations/tenant/file.jpg",
		"organizations/tenant/../other/file.jpg",
		"organizations\\tenant\\file.jpg",
	}
	for _, key := range invalid {
		if err := validateObjectKey(key); err == nil {
			t.Fatalf("expected invalid object key: %s", key)
		}
	}
}
