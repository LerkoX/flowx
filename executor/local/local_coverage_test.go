package local

import (
	"testing"

	"github.com/LerkoX/flowx/executor"
)

func TestParseInputRequest_YAML(t *testing.T) {
	content := `type: text
prompt: "Enter your name:"`
	req := parseInputRequest(content)
	if req == nil {
		t.Fatal("Expected non-nil request")
	}
	if req.Type != "text" {
		t.Errorf("Type = %q, want %q", req.Type, "text")
	}
	if req.Prompt != "Enter your name:" {
		t.Errorf("Prompt = %q, want %q", req.Prompt, "Enter your name:")
	}
}

func TestParseInputRequest_JSON(t *testing.T) {
	content := `{"type":"password","prompt":"Enter password:"}`
	req := parseInputRequest(content)
	if req == nil {
		t.Fatal("Expected non-nil request")
	}
	if req.Type != "password" {
		t.Errorf("Type = %q, want %q", req.Type, "password")
	}
}

func TestParseInputRequest_Empty(t *testing.T) {
	req := parseInputRequest("")
	if req != nil {
		t.Error("Expected nil for empty content")
	}
}

func TestParseInputRequest_InvalidFormat(t *testing.T) {
	req := parseInputRequest("not yaml or json at all {{{")
	if req != nil {
		t.Error("Expected nil for unparseable content")
	}
}

func TestParseInputRequest_MissingType(t *testing.T) {
	content := `prompt: "hello"`
	req := parseInputRequest(content)
	if req != nil {
		t.Error("Expected nil when type field is missing")
	}
}

func TestLocalExecutor_GetInstanceId(t *testing.T) {
	exec := NewLocalExecutor()
	if id := exec.GetInstanceId(); id != "" {
		t.Errorf("GetInstanceId() = %q, want empty string", id)
	}
}

// 接口合规检查
func TestLocalExecutor_InterfaceCompliance(t *testing.T) {
	var _ executor.Executor = (*LocalExecutor)(nil)
	var _ executor.ExecutorInfoProvider = (*LocalExecutor)(nil)
}
