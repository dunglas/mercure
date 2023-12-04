package mercure

import "go.uber.org/zap/zapcore"

// stringArray has been copied from https://github.com/uber-go/zap/blob/master/array.go#L250-L257
// Copyright (c) 2016 Uber Technologies, Inc.
type stringArray []string

func (ss stringArray) MarshalLogArray(arr zapcore.ArrayEncoder) error {
	for i := range ss {
		arr.AppendString(ss[i])
	}

	return nil
}

// LogField is an alias of zapcore.Field, it could be replaced by a custom contract when Go will support generics.
type LogField = zapcore.Field

// Level is an alias of zapcore.Level, it could be replaced by a custom contract when Go will support generics.
type Level = zapcore.Level

// CheckedEntry is an alias of zapcore.CheckedEntry, it could be replaced by a custom contract when Go will support generics.
type CheckedEntry = zapcore.CheckedEntry

// Logger defines the Mercure logger.
type Logger interface {
	Info(msg string, fields ...LogField)
	Error(msg string, fields ...LogField)
	Check(level Level, msg string) *CheckedEntry
	Level() Level
}
