package reactor

import "errors"

var (
	ErrProtocolUnsupport = errors.New("protocol unsupport")
	ErrPlatformUnsupport = errors.New("platform unsupport")
)
