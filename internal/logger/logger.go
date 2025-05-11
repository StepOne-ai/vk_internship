package logger

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
)

type Level int

const (
	DebugLevel Level = iota
	InfoLevel
	WarnLevel
	ErrorLevel
)

var (
	globalLogger Logger
	once         sync.Once
)

type Logger interface {
	Debug(ctx context.Context, msg string, fields ...Field)
	Info(ctx context.Context, msg string, fields ...Field)
	Warn(ctx context.Context, msg string, fields ...Field)
	Error(ctx context.Context, msg string, err error, fields ...Field)

	With(fields ...Field) Logger
	Level() Level
}

type Field struct {
	Key   string
	Value any
}

type logger struct {
	level  Level
	fields []Field
	mutex  sync.Mutex
}

func init() {
	SetupLogger(DebugLevel)
}

func SetupLogger(level Level) {
	once.Do(func() {
		globalLogger = &logger{
			level: level,
		}
	})
}

func With(fields ...Field) Logger {
	return globalLogger.With(fields...)
}

func Debug(ctx context.Context, msg string, fields ...Field) {
	globalLogger.Debug(ctx, msg, fields...)
}

func Info(ctx context.Context, msg string, fields ...Field) {
	globalLogger.Info(ctx, msg, fields...)
}

func Warn(ctx context.Context, msg string, fields ...Field) {
	globalLogger.Warn(ctx, msg, fields...)
}

func Error(ctx context.Context, msg string, err error, fields ...Field) {
	globalLogger.Error(ctx, msg, err, fields...)
}

func (l *logger) With(fields ...Field) Logger {
	return &logger{
		level:  l.level,
		fields: append(l.fields, fields...),
	}
}

func (l *logger) Level() Level {
	return l.level
}

func (l *logger) log(_ context.Context, level Level, msg string, err error, fields ...Field) {
	if level < l.level {
		return
	}

	l.mutex.Lock()
	defer l.mutex.Unlock()

	var output string
	switch level {
	case DebugLevel:
		output = fmt.Sprintf("[DEBUG] %s", msg)
	case InfoLevel:
		output = fmt.Sprintf("[INFO] %s", msg)
	case WarnLevel:
		output = fmt.Sprintf("[WARN] %s", msg)
	case ErrorLevel:
		output = fmt.Sprintf("[ERROR] %s", msg)
	}

	for _, field := range append(l.fields, fields...) {
		output += fmt.Sprintf(" %s=%v", field.Key, field.Value)
	}

	if err != nil {
		output += fmt.Sprintf(" error=%v", err)
	}

	log.Println(output)
}

func (l *logger) Debug(ctx context.Context, msg string, fields ...Field) {
	l.log(ctx, DebugLevel, msg, nil, fields...)
}

func (l *logger) Info(ctx context.Context, msg string, fields ...Field) {
	l.log(ctx, InfoLevel, msg, nil, fields...)
}

func (l *logger) Warn(ctx context.Context, msg string, fields ...Field) {
	l.log(ctx, WarnLevel, msg, nil, fields...)
}

func (l *logger) Error(ctx context.Context, msg string, err error, fields ...Field) {
	l.log(ctx, ErrorLevel, msg, err, fields...)
}

func SetOutput(file *os.File) {
	log.SetOutput(file)
}

func SetFlags(flag int) {
	log.SetFlags(flag)
}
