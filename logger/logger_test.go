package logger

import (
	"testing"
	"time"
)

func TestLevel_Constants(t *testing.T) {
	tests := []struct {
		name  string
		level Level
		want  string
	}{
		{"debug", LevelDebug, "debug"},
		{"info", LevelInfo, "info"},
		{"warn", LevelWarn, "warn"},
		{"error", LevelError, "error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.level) != tt.want {
				t.Errorf("Level = %q, want %q", tt.level, tt.want)
			}
		})
	}
}

func TestEntry_Fields(t *testing.T) {
	ts := time.Now()
	entry := Entry{
		Pipeline:  "test-pipeline",
		BuildID:   "build-123",
		Node:      "node-1",
		Step:      "step-1",
		Timestamp: ts,
		Level:     LevelInfo,
		Message:   "test message",
		Output:    "test output",
	}

	if entry.Pipeline != "test-pipeline" {
		t.Errorf("Pipeline = %q, want %q", entry.Pipeline, "test-pipeline")
	}
	if entry.BuildID != "build-123" {
		t.Errorf("BuildID = %q, want %q", entry.BuildID, "build-123")
	}
	if entry.Node != "node-1" {
		t.Errorf("Node = %q, want %q", entry.Node, "node-1")
	}
	if entry.Step != "step-1" {
		t.Errorf("Step = %q, want %q", entry.Step, "step-1")
	}
	if entry.Timestamp != ts {
		t.Errorf("Timestamp = %v, want %v", entry.Timestamp, ts)
	}
	if entry.Level != LevelInfo {
		t.Errorf("Level = %q, want %q", entry.Level, LevelInfo)
	}
	if entry.Message != "test message" {
		t.Errorf("Message = %q, want %q", entry.Message, "test message")
	}
	if entry.Output != "test output" {
		t.Errorf("Output = %q, want %q", entry.Output, "test output")
	}
}
