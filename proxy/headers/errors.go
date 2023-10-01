package headers

import (
	"errors"
)

var IncompleteHeaderLine = errors.New("Could not read the header line")
var InvalidHeaderStart = errors.New("Invalid header start")
