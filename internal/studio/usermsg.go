package studio

import "errors"

// UserError is a failure the visitor can act on. The text stays in Spanish as
// the source and the fallback: browsers translate Code, while logs, older
// clients and Go tests keep reading the same text as before.
type UserError struct {
	Code string
	Text string
}

// Error keeps *UserError usable anywhere an error was expected before.
func (e *UserError) Error() string { return e.Text }

// Userf builds a coded user error.
func Userf(code, text string) error { return &UserError{Code: code, Text: text} }

// CodeOf returns the stable code of a user error, or "" when the failure is
// internal and there is nothing to translate.
func CodeOf(err error) string {
	var u *UserError
	if errors.As(err, &u) {
		return u.Code
	}
	return ""
}

// ErrorMsg is the signalling reply for a failed action: the text keeps working
// for clients that ignore Code.
func ErrorMsg(err error, reqID, action string) Msg {
	return Msg{Type: "error", Error: err.Error(), Code: CodeOf(err), ReqID: reqID, Action: action}
}
