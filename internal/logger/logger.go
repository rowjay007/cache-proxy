package logger

import (
	"os"
	"time"

	"github.com/rs/zerolog"
)

type Logger interface {
	Info() *zerolog.Event
	Error() *zerolog.Event
	Debug() *zerolog.Event
	Warn() *zerolog.Event
	With() zerolog.Context
	InfoMsg(msg string)
	ErrorMsg(msg string)
	Errorf(format string, args ...interface{})
}

type ZeroLogger struct {
	logger zerolog.Logger
}

func New() Logger {
	zerolog.TimeFieldFormat = time.RFC3339
	
	logger := zerolog.New(os.Stdout).
		Level(zerolog.InfoLevel).
		With().
		Timestamp().
		Caller().
		Logger()

	return &ZeroLogger{
		logger: logger,
	}
}

func NewWithLevel(level zerolog.Level) Logger {
	zerolog.TimeFieldFormat = time.RFC3339
	
	logger := zerolog.New(os.Stdout).
		Level(level).
		With().
		Timestamp().
		Caller().
		Logger()

	return &ZeroLogger{
		logger: logger,
	}
}

func (l *ZeroLogger) Info() *zerolog.Event {
	return l.logger.Info()
}

func (l *ZeroLogger) Error() *zerolog.Event {
	return l.logger.Error()
}

func (l *ZeroLogger) Debug() *zerolog.Event {
	return l.logger.Debug()
}

func (l *ZeroLogger) Warn() *zerolog.Event {
	return l.logger.Warn()
}

func (l *ZeroLogger) With() zerolog.Context {
	return l.logger.With()
}

func (l *ZeroLogger) InfoMsg(msg string) {
	l.logger.Info().Msg(msg)
}

func (l *ZeroLogger) ErrorMsg(msg string) {
	l.logger.Error().Msg(msg)
}

func (l *ZeroLogger) Errorf(format string, args ...interface{}) {
	l.logger.Error().Msgf(format, args...)
}
