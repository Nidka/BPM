package input

import "testing"

func TestPersonNamesAndBusinessTextUseDifferentFormats(t *testing.T) {
	for _, value := range []string{"María José", "Muñoz", "O’Connor", "D'Ávila", "Jean-Luc", "Jose\u0301", "李明"} {
		if !PersonName(value, 1, 80) {
			t.Fatalf("rejected personal name %q", value)
		}
	}
	for _, value := range []string{"Ana2", "123", "Ana 🙂", "Ana\nMaría", "-Ana", "Ana_", ""} {
		if PersonName(value, 1, 80) {
			t.Fatalf("accepted invalid personal name %q", value)
		}
	}
	if !Text("Restaurante 24 S.A.C.", 2, 200) || !Text("Sede 2 – Lima", 2, 160) || !PersonName("", 0, 160) {
		t.Fatal("business text and optional names have distinct rules")
	}
}

func TestPhoneAndEmailFormatsRejectNonKeyboardInput(t *testing.T) {
	for _, value := range []string{"+51 987 654 321", "(01) 234-5678", ""} {
		if !Phone(value) {
			t.Fatalf("rejected valid phone %q", value)
		}
	}
	for _, value := range []string{"123", "llámame", "999+999999", "999 999 999\n", "1234567890123456"} {
		if Phone(value) {
			t.Fatalf("accepted invalid phone %q", value)
		}
	}
	for _, value := range []string{"person+chef@example.com", "nombre.apellido@example.com"} {
		if !Email(value) {
			t.Fatalf("rejected valid email %q", value)
		}
	}
	for _, value := range []string{"person", "a@@b.com", "a b@example.com", "<a>@example.com", "a..b@example.com", "a@-example.com", "a@example.com\n"} {
		if Email(value) {
			t.Fatalf("accepted invalid email %q", value)
		}
	}
}
