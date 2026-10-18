package request

import "testing"

func TestPaginationQuery(t *testing.T) {
	for _, raw := range []string{"pageSize=0", "pageSize=101", "pageSize=x", "pageSize=1&pageSize=2", "query=a&query=b", "tenant=other", "pageSize=%zz", "query=%00", "query=text%0A", "query=%FF"} {
		if _, e := Pagination(raw); e == nil {
			t.Fatal("accepted invalid query", raw)
		}
	}
	if _, e := Pagination("query=", "pageSize"); e == nil {
		t.Fatal("workspace accepted query parameter")
	}
	p, e := Pagination("pageSize=10&query=%25_%C3%A1&pageToken=abc")
	if e != nil || p.Size != 10 || p.Query != "%_á" || p.Token != "abc" {
		t.Fatal("lost literal query", p, e)
	}
}
