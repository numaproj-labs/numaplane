package logger

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"testing"
)

type LogJSON struct {
	Level   string `json:"level"`
	Message string `json:"msg"`
	Error   string `json:"error,omitempty"`
	Logger  string `json:"logger,omitempty"`
	FieldA  string `json:"fieldA,omitempty"`
	FieldB  string `json:"fieldB,omitempty"`
}

func mock() (NumaLogger, *bytes.Buffer) {
	var buf bytes.Buffer
	w := io.Writer(&buf)
	lvl := verboseLevel
	return newNumaLogger(&w, &lvl), &buf
}

func TestWrappers(t *testing.T) {
	t.Run("verbose", func(t *testing.T) {
		nl, buf := mock()

		expected := LogJSON{
			"verbose",
			"test verbose message",
			"",
			loggerDefaultName,
			"",
			"",
		}

		nl.Verbose(expected.Message)

		var actual LogJSON
		_ = json.Unmarshal(buf.Bytes(), &actual)

		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("\nActual:\n%+v\nExpected:\n%+v", actual, expected)
		}
	})

	t.Run("debug", func(t *testing.T) {
		nl, buf := mock()

		expected := LogJSON{
			"debug",
			"test debug message",
			"",
			loggerDefaultName,
			"",
			"",
		}

		nl.Debug(expected.Message)

		var actual LogJSON
		_ = json.Unmarshal(buf.Bytes(), &actual)

		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("\nActual:\n%+v\nExpected:\n%+v", actual, expected)
		}
	})

	t.Run("info", func(t *testing.T) {
		nl, buf := mock()

		expected := LogJSON{
			"info",
			"test info message",
			"",
			loggerDefaultName,
			"",
			"",
		}

		nl.Info(expected.Message)

		var actual LogJSON
		_ = json.Unmarshal(buf.Bytes(), &actual)

		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("\nActual:\n%+v\nExpected:\n%+v", actual, expected)
		}
	})

	t.Run("warn", func(t *testing.T) {
		nl, buf := mock()

		expected := LogJSON{
			"warn",
			"test warn message",
			"",
			loggerDefaultName,
			"",
			"",
		}

		nl.Warn(expected.Message)

		var actual LogJSON
		_ = json.Unmarshal(buf.Bytes(), &actual)

		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("\nActual:\n%+v\nExpected:\n%+v", actual, expected)
		}
	})

	t.Run("error", func(t *testing.T) {
		nl, buf := mock()

		err := errors.New("test error")

		expected := LogJSON{
			"error",
			"test error message",
			err.Error(),
			loggerDefaultName,
			"",
			"",
		}

		nl.Error(err, expected.Message)

		var actual LogJSON
		_ = json.Unmarshal(buf.Bytes(), &actual)

		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("\nActual:\n%+v\nExpected:\n%+v", actual, expected)
		}
	})
}

func TestFWrappers(t *testing.T) {
	t.Run("verbosef", func(t *testing.T) {
		nl, buf := mock()

		expected := LogJSON{
			"verbose",
			"test verbosef message 123",
			"",
			loggerDefaultName,
			"",
			"",
		}

		nl.Verbosef("test verbosef message %d", 123)

		var actual LogJSON
		_ = json.Unmarshal(buf.Bytes(), &actual)

		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("\nActual:\n%+v\nExpected:\n%+v", actual, expected)
		}
	})

	t.Run("debugf", func(t *testing.T) {
		nl, buf := mock()

		expected := LogJSON{
			"debug",
			"test debugf message 123 ABC",
			"",
			loggerDefaultName,
			"",
			"",
		}

		nl.Debugf("test debugf message %d %s", 123, "ABC")

		var actual LogJSON
		_ = json.Unmarshal(buf.Bytes(), &actual)

		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("\nActual:\n%+v\nExpected:\n%+v", actual, expected)
		}
	})

	t.Run("infof", func(t *testing.T) {
		nl, buf := mock()

		expected := LogJSON{
			"info",
			"test infof message 456",
			"",
			loggerDefaultName,
			"",
			"",
		}

		nl.Infof("test infof message %d", 456)

		var actual LogJSON
		_ = json.Unmarshal(buf.Bytes(), &actual)

		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("\nActual:\n%+v\nExpected:\n%+v", actual, expected)
		}
	})

	t.Run("warnf", func(t *testing.T) {
		nl, buf := mock()

		expected := LogJSON{
			"warn",
			"test warnf message ABC",
			"",
			loggerDefaultName,
			"",
			"",
		}

		nl.Warnf("test warnf message %s", "ABC")

		var actual LogJSON
		_ = json.Unmarshal(buf.Bytes(), &actual)

		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("\nActual:\n%+v\nExpected:\n%+v", actual, expected)
		}
	})

	t.Run("errorf", func(t *testing.T) {
		nl, buf := mock()

		err := errors.New("test errorf")

		expected := LogJSON{
			"error",
			"test errorf message 123ABC",
			err.Error(),
			loggerDefaultName,
			"",
			"",
		}

		nl.Errorf(err, "test errorf message %d%s", 123, "ABC")

		var actual LogJSON
		_ = json.Unmarshal(buf.Bytes(), &actual)

		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("\nActual:\n%+v\nExpected:\n%+v", actual, expected)
		}
	})
}

func TestWithClauses(t *testing.T) {
	t.Run("withName", func(t *testing.T) {
		nl, buf := mock()

		expected := LogJSON{
			"info",
			"test info message",
			"",
			fmt.Sprintf("%s.logger-name-test", loggerDefaultName),
			"",
			"",
		}

		nl.WithName("logger-name-test").Info(expected.Message)

		var actual LogJSON
		_ = json.Unmarshal(buf.Bytes(), &actual)

		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("\nActual:\n%+v\nExpected:\n%+v", actual, expected)
		}
	})

	t.Run("withFields", func(t *testing.T) {
		nl, buf := mock()

		expected := LogJSON{
			"debug",
			"test debug message",
			"",
			loggerDefaultName,
			"valA",
			"valB",
		}

		nl.Debug(expected.Message, "fieldA", expected.FieldA, "fieldB", expected.FieldB)

		var actual LogJSON
		_ = json.Unmarshal(buf.Bytes(), &actual)

		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("\nActual:\n%+v\nExpected:\n%+v", actual, expected)
		}
	})

	t.Run("withValues", func(t *testing.T) {
		nl, buf := mock()

		expected := LogJSON{
			"debug",
			"test debug message",
			"",
			loggerDefaultName,
			"valA",
			"valB",
		}

		nl.WithValues("fieldA", expected.FieldA, "fieldB", expected.FieldB).Debug(expected.Message)

		var actual LogJSON
		_ = json.Unmarshal(buf.Bytes(), &actual)

		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("\nActual:\n%+v\nExpected:\n%+v", actual, expected)
		}
	})
}
