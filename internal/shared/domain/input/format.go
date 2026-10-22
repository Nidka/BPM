// Package input reúne formatos puros compartidos por identidad y organizaciones.
// Los límites y la obligatoriedad se eligen dentro de cada caso de uso.
package input

import (
	"net/mail"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

func Text(value string, min, max int) bool {
	return utf8.ValidString(value) && utf8.RuneCountInString(strings.TrimSpace(value)) >= min &&
		utf8.RuneCountInString(value) <= max && strings.IndexFunc(value, unicode.IsControl) < 0
}

var personName = regexp.MustCompile(`^\p{L}([\p{L}\p{M} '\x{2019}-]*[\p{L}\p{M}])?$`)
var phone = regexp.MustCompile(`^\+?[0-9 ()-]+$`)
var emailFormat = regexp.MustCompile("^[\\p{L}\\p{N}!#$%&'*+/=?^_`{|}~.-]+@[\\p{L}\\p{N}](?:[\\p{L}\\p{N}-]*[\\p{L}\\p{N}])?(?:\\.[\\p{L}\\p{N}](?:[\\p{L}\\p{N}-]*[\\p{L}\\p{N}])?)+$")

func PersonName(value string, min, max int) bool {
	return Text(value, min, max) && (strings.TrimSpace(value) == "" && min == 0 || personName.MatchString(strings.TrimSpace(value)))
}

func Email(value string) bool {
	email := strings.TrimSpace(value)
	address, err := mail.ParseAddress(email)
	return err == nil && address.Address == email && len(value) <= 320 && strings.IndexFunc(value, unicode.IsControl) < 0 && emailFormat.MatchString(email)
}

func Phone(value string) bool {
	if len(value) > 30 || strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return false
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return true
	}
	digits := 0
	for _, r := range value {
		if r >= '0' && r <= '9' {
			digits++
		}
	}
	return len(value) <= 30 && phone.MatchString(value) && digits >= 7 && digits <= 15
}
