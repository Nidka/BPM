// Package pagination define páginas por clave y cursores de continuación sin estado.
package pagination

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

const DefaultSize = 25
const MaxSize = 100

var ErrInvalid = errors.New("invalid pagination")

type Request struct {
	Size         int
	Token, Query string
}
type Position struct{ Key, ID string }
type Window struct {
	Size  int
	Query string
	After Position
}
type Page[T any] struct {
	Items     []T
	NextToken string
}

// Normalize valida límites de entrada sin interpretar el cursor ni su contexto.
func Normalize(r Request) (Request, error) {
	if r.Size == 0 {
		r.Size = DefaultSize
	}
	if r.Size < 1 || r.Size > MaxSize || len(r.Token) > 2048 || !utf8.ValidString(r.Query) || utf8.RuneCountInString(r.Query) > 160 || strings.IndexFunc(r.Query, unicode.IsControl) >= 0 {
		return Request{}, ErrInvalid
	}
	for _, c := range r.Token {
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
			return Request{}, ErrInvalid
		}
	}
	r.Query = strings.TrimSpace(r.Query)
	return r, nil
}

// Open valida el contexto del cursor. No autentica: cada página requiere autorización nueva.
func Open(r Request, scope ...string) (Window, error) {
	r, e := Normalize(r)
	if e != nil {
		return Window{}, e
	}
	w := Window{Size: r.Size, Query: r.Query}
	if r.Token == "" {
		return w, nil
	}
	raw, e := base64.RawURLEncoding.DecodeString(r.Token)
	if e != nil {
		return Window{}, ErrInvalid
	}
	// Formato binario privado y versionado; nunca es un contrato que deba interpretar el cliente.
	// La huella vincula el contexto, pero no firma ni autoriza: SQL vuelve a comprobar permisos.
	if len(raw) < 37 || raw[0] != 1 {
		return Window{}, ErrInvalid
	}
	scopeHash := binding(r.Query, scope)
	if !bytes.Equal(raw[1:33], scopeHash[:]) {
		return Window{}, ErrInvalid
	}
	keySize := int(binary.BigEndian.Uint16(raw[33:35]))
	idSize := int(binary.BigEndian.Uint16(raw[35:37]))
	if keySize > 1000 || idSize < 1 || idSize > 128 || len(raw) != 37+keySize+idSize {
		return Window{}, ErrInvalid
	}
	w.After = Position{Key: string(raw[37 : 37+keySize]), ID: string(raw[37+keySize:])}
	if !utf8.ValidString(w.After.Key+w.After.ID) || strings.IndexFunc(w.After.Key+w.After.ID, unicode.IsControl) >= 0 {
		return Window{}, ErrInvalid
	}
	return w, nil
}
func binding(query string, scope []string) [32]byte {
	b, _ := json.Marshal(append(append([]string{}, scope...), query))
	return sha256.Sum256(b)
}

// Close consume el registro adicional sin exponerlo; el cursor conserva la última clave emitida.
func Close[T any](items []T, w Window, position func(T) Position, scope ...string) Page[T] {
	p := Page[T]{Items: items}
	if p.Items == nil {
		p.Items = []T{}
	}
	if len(items) > w.Size {
		p.Items = items[:w.Size]
		anchor := position(p.Items[len(p.Items)-1])
		scopeHash := binding(w.Query, scope)
		raw := make([]byte, 37, 37+len(anchor.Key)+len(anchor.ID))
		raw[0] = 1
		copy(raw[1:33], scopeHash[:])
		binary.BigEndian.PutUint16(raw[33:35], uint16(len(anchor.Key)))
		binary.BigEndian.PutUint16(raw[35:37], uint16(len(anchor.ID)))
		raw = append(raw, anchor.Key...)
		raw = append(raw, anchor.ID...)
		p.NextToken = base64.RawURLEncoding.EncodeToString(raw)
	}
	return p
}
