package cmd

import "errors"

var ErrSigterm = errors.New("Termination signal received.")
