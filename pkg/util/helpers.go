package util

import "log/slog"

func ErrorAttr(value error) slog.Attr {
	return slog.Any(ErrorKey, value)
}

func SubsystemAttr(value string) slog.Attr {
	return slog.Any(SubsystemKey, value)
}
