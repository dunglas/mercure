package mercure

import "go.uber.org/zap/zapcore"

type stringArray []string

func (ss stringArray) MarshalLogArray(arr zapcore.ArrayEncoder) error {
	for i := range ss {
		arr.AppendString(ss[i])
	}

	return nil
}

// LogField is an alias of zapcore.Field, it could be replaced by a custom contract when Go will support generics.
type LogField = zapcore.Field

// Logger defines the Mercure logger.
type Logger interface {
	Debug(msg string, fields ...LogField)
	Info(msg string, fields ...LogField)
	Warn(msg string, fields ...LogField)
	Error(msg string, fields ...LogField)
}
