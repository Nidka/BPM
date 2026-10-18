package request

import (
	"net/url"
	"strconv"

	"github.com/Nidka/BPM-service/internal/shared/application/pagination"
)

// Pagination rechaza parámetros repetidos o desconocidos antes de consultar datos.
func Pagination(raw string, allowed ...string) (pagination.Request, error) {
	q, e := url.ParseQuery(raw)
	if e != nil {
		return pagination.Request{}, pagination.ErrInvalid
	}
	r := pagination.Request{}
	permitted := map[string]bool{"pageSize": true, "pageToken": true, "query": true}
	if len(allowed) > 0 {
		permitted = map[string]bool{}
		for _, key := range allowed {
			permitted[key] = true
		}
	}
	for k, v := range q {
		if len(v) != 1 || !permitted[k] {
			return r, pagination.ErrInvalid
		}
		switch k {
		case "pageSize":
			r.Size, e = strconv.Atoi(v[0])
			if e != nil || r.Size < 1 || strconv.Itoa(r.Size) != v[0] {
				return r, pagination.ErrInvalid
			}
		case "pageToken":
			r.Token = v[0]
		case "query":
			r.Query = v[0]
		default:
			return r, pagination.ErrInvalid
		}
	}
	return pagination.Normalize(r)
}
