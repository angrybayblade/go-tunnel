package headers

import (
	"errors"
)

var IncompleteHeaderLine = errors.New("Could not read the header line")
