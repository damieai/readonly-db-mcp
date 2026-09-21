package elasticsearch

import "fmt"

type Error struct{ Code, Message string }

func (e *Error) Error() string           { return fmt.Sprintf("%s: %s", e.Code, e.Message) }
func failure(code, message string) error { return &Error{code, message} }
