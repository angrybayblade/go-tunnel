package headers

import (
	"errors"
)

var ErrIncompleteHeaderLine = errors.New("Could not read the header line")
var ErrInvalidHeaderStart = errors.New("Invalid header start")
