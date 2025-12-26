package config

import (
	"errors"
	"fmt"
)

// s2error formats according to a format specifier and returns the resulting string.
func s2error(s string, a ...any) error {
	//s2 := Sprintf("%s", s)
	return errors.New(fmt.Sprintf(s, a...))
}
