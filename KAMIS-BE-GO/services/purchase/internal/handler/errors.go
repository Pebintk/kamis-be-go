package handler

import "github.com/pebintk/kamis-be-go/pkg/apierr"

// badQuery reports an unparseable query parameter as a caller mistake.
func badQuery(name, value, want string) error {
	return apierr.Invalidf("Parameter %s tidak valid (%q): harus berupa %s", name, value, want)
}
