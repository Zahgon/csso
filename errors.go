package csso

import (
	"errors"

	"github.com/css/csso-go/internal/csso/jscompat"
	"github.com/css/csso-go/internal/csstree"
)

// Error is an error csso reports by throwing: invalid usage data
// (Name "Error"), syntax errors the parser can't recover from
// ("SyntaxError") and the TypeErrors csso runs into on trees it does not
// expect, such as a selector `a:not` without arguments ("TypeError").
// Name and Message are those of the JavaScript exception.
type Error struct {
	Name    string
	Message string
}

// Error returns the error as `Name: Message`, like JavaScript's
// String(error).
func (e *Error) Error() string {
	return e.Name + ": " + e.Message
}

// convertError turns the errors of the internal packages into *Error.
func convertError(err error) error {
	var jsErr *jscompat.Error
	if errors.As(err, &jsErr) {
		return &Error{Name: jsErr.Name, Message: jsErr.Message}
	}
	var syntaxErr *csstree.SyntaxError
	if errors.As(err, &syntaxErr) {
		return &Error{Name: "SyntaxError", Message: syntaxErr.Message}
	}
	return err
}
