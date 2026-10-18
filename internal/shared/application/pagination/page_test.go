package pagination

import (
	"errors"
	"strings"
	"testing"
)

func TestCursorContextAndBoundaries(t *testing.T) {
	w, e := Open(Request{Size: 2, Query: "  higiene  "}, "user", "org", "resource")
	if e != nil {
		t.Fatal(e)
	}
	values := []Position{{"Igual", "a"}, {"Igual", "b"}, {"Z", "c"}}
	p := Close(values, w, func(v Position) Position { return v }, "user", "org", "resource")
	if len(p.Items) != 2 || p.NextToken == "" {
		t.Fatal("missing next page")
	}
	next, e := Open(Request{Size: 1, Query: "higiene", Token: p.NextToken}, "user", "org", "resource")
	if e != nil || next.After.ID != "b" {
		t.Fatal("lost tie breaker", e)
	}
	for _, scope := range [][]string{{"other", "org", "resource"}, {"user", "other", "resource"}, {"user", "org", "other"}} {
		if _, e = Open(Request{Token: p.NextToken, Query: "higiene"}, scope...); !errors.Is(e, ErrInvalid) {
			t.Fatal("cursor crossed scope")
		}
	}
	if _, e = Open(Request{Token: p.NextToken, Query: "other"}, "user", "org", "resource"); !errors.Is(e, ErrInvalid) {
		t.Fatal("cursor crossed search")
	}
	if end := Close(p.Items, w, func(v Position) Position { return v }, "user", "org", "resource"); end.NextToken != "" {
		t.Fatal("false next page")
	}
	for _, r := range []Request{{Size: -1}, {Size: 101}, {Token: "bad!"}, {Token: strings.Repeat("a", 2049)}, {Query: strings.Repeat("á", 161)}, {Query: "bad\ntext"}, {Query: "text\n"}, {Token: "a\nb"}} {
		if _, e = Open(r); !errors.Is(e, ErrInvalid) {
			t.Fatal("accepted invalid input", r.Size)
		}
	}
}
