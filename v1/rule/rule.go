// Package rule is the stable error returned when a Montage document is rejected.
package rule

import (
	"errors"
	"fmt"
)

// Error names the slot and the rule that rejected a document.
// Slot is 1-based. Zero means the document as a whole.
type Error struct {
	Slot int
	Code string
	Msg  string
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.Slot > 0 {
		return fmt.Sprintf("slot %d: %s: %s", e.Slot, e.Code, e.Msg)
	}
	return e.Code + ": " + e.Msg
}

// E builds a rule error. format is the message, without the slot or code.
func E(slot int, code, format string, args ...any) *Error {
	return &Error{Slot: slot, Code: code, Msg: fmt.Sprintf(format, args...)}
}

// As reports the rule error in err, if any.
func As(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}
