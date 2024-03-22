package logger

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"testing"
)

type LogJSON struct {
	Level   string `json:"level"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
	Logger  string `json:"logger,omitempty"`
	FieldA  string `json:"fieldA,omitempty"`
	FieldB  string `json:"fieldB,omitempty"`
}

func mock() (NumaLogger, *bytes.Buffer) {
	var buf bytes.Buffer
	w := io.Writer(&buf)
	lvl := verboseLevel
	return New(&w, &lvl), &buf
}

func TestLogger(t *testing.T) {
	t.Run("verbose", func(t *testing.T) {
		nl, buf := mock()

		expected := LogJSON{
			"verbose",
			"test verbose message",
			"",
			"",
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
			"",
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
			"",
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
			"",
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
			"",
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

	t.Run("withName", func(t *testing.T) {
		nl, buf := mock()

		expected := LogJSON{
			"info",
			"test info message",
			"",
			"logger-name-test",
			"",
			"",
		}

		nl.WithName(expected.Logger).Info(expected.Message)

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
			"",
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
}
