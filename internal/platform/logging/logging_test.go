package logging

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestNew_JSONCarriesServiceAndEnv(t *testing.T) {
	var buf bytes.Buffer
	New(&buf, slog.LevelInfo, "json", "platform-backend", "stage").Info("hello", "k", "v")

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("not JSON: %q", buf.String())
	}
	for key, want := range map[string]string{"msg": "hello", "service": "platform-backend", "env": "stage", "k": "v"} {
		if record[key] != want {
			t.Errorf("%s = %v, want %q", key, record[key], want)
		}
	}
}

func TestNew_RespectsLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, slog.LevelWarn, "text", "svc", "local")
	logger.Info("dropped")
	logger.Warn("kept")

	if strings.Contains(buf.String(), "dropped") || !strings.Contains(buf.String(), "kept") {
		t.Fatalf("unexpected output: %q", buf.String())
	}
}

func TestNew_TimestampsAreUTC(t *testing.T) {
	var buf bytes.Buffer
	New(&buf, slog.LevelInfo, "json", "svc", "local").Info("x")

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	if ts, _ := record["time"].(string); !strings.HasSuffix(ts, "Z") {
		t.Fatalf("time %q is not UTC", ts)
	}
}
