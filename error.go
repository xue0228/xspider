package xspider

import (
	"errors"
	"fmt"
)

var ErrDropItem = errors.New("drop_item")
var ErrDropRequest = errors.New("drop_request")
var ErrDropSignal = errors.New("drop_signal")

var ErrHttpCode = fmt.Errorf("http_code: %w", ErrDropRequest)

//var ErrUnhandledError = errors.New("unhandled_error")
//var ErrNotImplemented = errors.New("not_implemented")
