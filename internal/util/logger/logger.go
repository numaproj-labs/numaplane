package logger

import (
	"context"
	"io"
	"os"

	"github.com/go-logr/logr"
	"github.com/rs/zerolog"
)

var (
	// LoggerFieldName is the field key for logr.WithName.
	LoggerFieldName = "logger"
	// LoggerFieldSeparator separates names for logr.WithName.
	LoggerFieldSeparator = "."
)

/*
Log Level Mapping:
| logr verbosity | NumaLogger semantic | zerolog 			 |
| -------------- | ------------------- | ------------- |
| 	error				 | 	 error						 |	 3		  		 |
| 	0				 		 | 	 warn							 |	 2		 			 |
| 	1				 		 | 	 info						 	 |	 1		 			 |
| 	2				 		 | 	 debug						 |	 0		 			 |
| 	4				 		 | 	 verbose					 |	 -2 (custom) |
*/
const (
	verboseLevel = 4
	debugLevel   = 2
	infoLevel    = 1
	warnLevel    = 0
)

const defaultLevel = infoLevel

type loggerKey struct{}

// NumaLogger is the struct containing a pointer to a logr.Logger instance.
type NumaLogger struct {
	LogrLogger *logr.Logger
}

// LogSink implements logr.LogSink using zerolog as base logger.
type LogSink struct {
	l     *zerolog.Logger
	name  string
	depth int
}

// New returns a new NumaLogger with a logr.Logger instance with a default setup for zerolog.
// The writer argument sets the output the logs will be written to. If it is nil, os.Stdout will be used.
// The level argument sets the log level value for this logger instance.
func New(writer *io.Writer, level *int) NumaLogger {
	// Adds a way to convert a custom zerolog.Level values to strings.
	zerolog.LevelFieldMarshalFunc = func(lvl zerolog.Level) string {
		switch lvl {
		case zerolog.WarnLevel - verboseLevel:
			return "verbose"
		}
		return lvl.String()
	}

	// Set zerolog global level to the most verbose level
	zerolog.SetGlobalLevel(zerolog.WarnLevel - verboseLevel)

	w := io.Writer(os.Stdout)
	if writer != nil {
		w = *writer
	}

	lvl := int(defaultLevel)
	if level != nil {
		lvl = *level
	}

	zl := zerolog.New(w)
	zl = setLoggerLevel(zl, lvl).
		With().
		Caller().
		Timestamp().
		Logger()

	sink := &LogSink{l: &zl}
	ll := logr.New(sink)

	return NumaLogger{&ll}
}

// Init receives optional information about the logr library
// and sets the call depth accordingly.
func (ls *LogSink) Init(ri logr.RuntimeInfo) {
	ls.depth = ri.CallDepth + 3
}

// Enabled tests whether this LogSink is enabled at the specified V-level and per-package.
func (ls *LogSink) Enabled(level int) bool {
	// TODO: this should return true based on level settings (global, log level, etc.) and also based on caller package (per-module logging feature)
	zlLevel := getZerologLevel(level)
	return zlLevel >= ls.l.GetLevel() && zlLevel >= zerolog.GlobalLevel()
}

// Info logs a non-error message (msg) with the given key/value pairs as context and the specified level.
func (ls *LogSink) Info(level int, msg string, keysAndValues ...any) {
	zlEvent := ls.l.WithLevel(getZerologLevel(level))
	ls.log(zlEvent, msg, keysAndValues)
}

// Error logs an error, with the given message, and key/value pairs as context.
func (ls *LogSink) Error(err error, msg string, keysAndValues ...any) {
	zlEvent := ls.l.Err(err)
	ls.log(zlEvent, msg, keysAndValues)
}

// Reusable function to be used in LogSink Info and Error functions.
func (ls *LogSink) log(zlEvent *zerolog.Event, msg string, keysAndValues []any) {
	if zlEvent == nil {
		return
	}

	if ls.name != "" {
		zlEvent = zlEvent.Str(LoggerFieldName, ls.name)
	}

	zlEvent.Fields(keysAndValues).
		CallerSkipFrame(ls.depth).
		Msg(msg)
}

// WithValues returns a new LogSink with additional key/value pairs.
func (ls LogSink) WithValues(keysAndValues ...any) logr.LogSink {
	l := ls.l.With().Fields(keysAndValues).Logger()
	ls.l = &l
	return &ls
}

// WithName returns a new LogSink with the specified name appended.
func (ls LogSink) WithName(name string) logr.LogSink {
	if ls.name != "" {
		ls.name += LoggerFieldSeparator + name
	} else {
		ls.name = name
	}

	return &ls
}

// WithCallDepth returns a LogSink that will offset the call stack
// by the specified number of frames when logging call site information.
func (ls LogSink) WithCallDepth(depth int) logr.LogSink {
	ls.depth += depth
	return &ls
}

// WithLogger returns a copy of parent context in which the
// value associated with logger key is the supplied logger.
func WithLogger(ctx context.Context, logger NumaLogger) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}

// FromContext returns the logger in the context.
// If there is no logger in context, a new one is created.
func FromContext(ctx context.Context) NumaLogger {
	if logger, ok := ctx.Value(loggerKey{}).(NumaLogger); ok {
		return logger
	}

	return New(nil, nil)
}

// WithName appends a given name to the logger.
func (nl NumaLogger) WithName(name string) NumaLogger {
	// TODO: it should create a new logger instance
	ll := nl.LogrLogger.WithName(name)
	nl.LogrLogger = &ll
	return nl
}

// WithValues appends additional key/value pairs to the logger.
func (nl NumaLogger) WithValues(keysAndValues ...any) NumaLogger {
	// TODO: it should create a new logger instance
	ll := nl.LogrLogger.WithValues(keysAndValues)
	nl.LogrLogger = &ll
	return nl
}

// Error logs an error with a message and optional key/value pairs.
func (nl NumaLogger) Error(err error, msg string, keysAndValues ...any) {
	nl.LogrLogger.Error(err, msg, keysAndValues...)
}

// Fatal logs an error with a message and optional key/value pairs. Then, exits with code 1.
func (nl NumaLogger) Fatal(err error, msg string, keysAndValues ...any) {
	// TODO: implement a way to display the level as "fatal" instead of "error" in the logs
	nl.LogrLogger.Error(err, msg, keysAndValues...)
	os.Exit(1)
}

// Warn logs a warning-level message with optional key/value pairs.
func (nl NumaLogger) Warn(msg string, keysAndValues ...any) {
	nl.LogrLogger.V(warnLevel).Info(msg, keysAndValues...)
}

// Info logs an info-level message with optional key/value pairs.
func (nl NumaLogger) Info(msg string, keysAndValues ...any) {
	nl.LogrLogger.V(infoLevel).Info(msg, keysAndValues...)
}

// Debug logs a debug-level message with optional key/value pairs.
func (nl NumaLogger) Debug(msg string, keysAndValues ...any) {
	nl.LogrLogger.V(debugLevel).Info(msg, keysAndValues...)
}

// Verbose logs a verbose-level message with optional key/value pairs.
func (nl NumaLogger) Verbose(msg string, keysAndValues ...any) {
	nl.LogrLogger.V(verboseLevel).Info(msg, keysAndValues...)
}

func setLoggerLevel(logger zerolog.Logger, level int) zerolog.Logger {
	return logger.Level(getZerologLevel(level))
}

func getZerologLevel(verbosity int) zerolog.Level {
	return zerolog.WarnLevel - zerolog.Level(verbosity)
}
