package logging

import "log/slog"

func ErrorAttr(value error) slog.Attr {
	return slog.Any(ErrorKey, value)
}
