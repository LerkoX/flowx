package logger

import (
	"context"
	"testing"
	"time"
)

func TestNewConsolePusher(t *testing.T) {
	p := NewConsolePusher()
	if p == nil {
		t.Fatal("NewConsolePusher() returned nil")
	}
	if !p.showTime {
		t.Error("Default showTime should be true")
	}
	if !p.showNode {
		t.Error("Default showNode should be true")
	}
}

func TestConsolePusher_Push_WithMessage(t *testing.T) {
	p := NewConsolePusher()
	err := p.Push(context.Background(), Entry{
		Workflow:  "test",
		Timestamp: time.Now(),
		Level:     LevelInfo,
		Message:   "hello world",
	})
	if err != nil {
		t.Errorf("Push() error = %v", err)
	}
}

func TestConsolePusher_Push_WithOutput(t *testing.T) {
	p := NewConsolePusher()
	err := p.Push(context.Background(), Entry{
		Workflow:  "test",
		Node:      "node1",
		Timestamp: time.Now(),
		Level:     LevelDebug,
		Output:    "some output content",
	})
	if err != nil {
		t.Errorf("Push() error = %v", err)
	}
}

func TestConsolePusher_Push_ZeroTimestamp(t *testing.T) {
	p := NewConsolePusher()
	err := p.Push(context.Background(), Entry{
		Level:   LevelInfo,
		Message: "auto timestamp",
	})
	if err != nil {
		t.Errorf("Push() with zero timestamp error = %v", err)
	}
}

func TestConsolePusher_Push_UnknownLevel(t *testing.T) {
	p := NewConsolePusher()
	err := p.Push(context.Background(), Entry{
		Level:   Level("custom"),
		Message: "unknown level",
	})
	if err != nil {
		t.Errorf("Push() with unknown level error = %v", err)
	}
}

func TestConsolePusher_Push_EmptyEntry(t *testing.T) {
	p := NewConsolePusher()
	err := p.Push(context.Background(), Entry{})
	if err != nil {
		t.Errorf("Push() with empty entry error = %v", err)
	}
}

func TestConsolePusher_PushBatch(t *testing.T) {
	p := NewConsolePusher()
	entries := []Entry{
		{Level: LevelInfo, Message: "msg1"},
		{Level: LevelWarn, Message: "msg2"},
		{Level: LevelError, Message: "msg3"},
	}
	err := p.PushBatch(context.Background(), entries)
	if err != nil {
		t.Errorf("PushBatch() error = %v", err)
	}
}

func TestConsolePusher_PushBatch_Empty(t *testing.T) {
	p := NewConsolePusher()
	err := p.PushBatch(context.Background(), []Entry{})
	if err != nil {
		t.Errorf("PushBatch() with empty slice error = %v", err)
	}
}

func TestConsolePusher_Close(t *testing.T) {
	p := NewConsolePusher()
	err := p.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestConsolePusher_SetShowTime(t *testing.T) {
	p := NewConsolePusher()
	p.SetShowTime(false)
	if p.showTime {
		t.Error("showTime should be false after SetShowTime(false)")
	}
	p.SetShowTime(true)
	if !p.showTime {
		t.Error("showTime should be true after SetShowTime(true)")
	}
}

func TestConsolePusher_SetShowNode(t *testing.T) {
	p := NewConsolePusher()
	p.SetShowNode(false)
	if p.showNode {
		t.Error("showNode should be false after SetShowNode(false)")
	}
	p.SetShowNode(true)
	if !p.showNode {
		t.Error("showNode should be true after SetShowNode(true)")
	}
}

func TestConsolePusher_Push_AllLevels(t *testing.T) {
	p := NewConsolePusher()
	levels := []Level{LevelDebug, LevelInfo, LevelWarn, LevelError}
	for _, level := range levels {
		err := p.Push(context.Background(), Entry{
			Level:   level,
			Message: "test",
		})
		if err != nil {
			t.Errorf("Push() with level %q error = %v", level, err)
		}
	}
}

func TestConsolePusher_Push_NoMessageNoOutput(t *testing.T) {
	p := NewConsolePusher()
	err := p.Push(context.Background(), Entry{
		Level: LevelInfo,
	})
	if err != nil {
		t.Errorf("Push() with no message/output error = %v", err)
	}
}
