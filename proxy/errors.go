package proxy

import "errors"

var ErrForwardFailedNoFreeConnection = errors.New("Forwarding connection failed, no free connection available")
var ErrProxyAuth = errors.New("Authentication error while connecting to the proxy")
var ErrProxyInvalidSessionKey = errors.New("Invalid session key")
