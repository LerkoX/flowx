package dag

import (
	"context"
	"strings"
	"testing"

	"github.com/LerkoX/flowx/core"
	"github.com/LerkoX/flowx/executor"
)

// 零提取护栏：节点声明了 extract 但输出里没有 codec 块（如 docker exec 流被截断，
// exec 364），必须留下显式标记，而不是静默什么都不写。
func TestExtractOutput_MissingBlockMarksMetadata(t *testing.T) {
	wf := &WorkflowImpl{metadata: make(Metadata)}
	node := NewDGANodeWithConfig("KSampler", "", "", "", nil, map[string]any{
		"extract": map[string]interface{}{"type": "codec-block"},
	})

	// 输出被截断：只有进度行，没有 ```flowx-yaml 块
	fullOutput := "[job] running 19/30\n"
	if err := wf.extractOutput(context.Background(), node, nil, fullOutput); err != nil {
		t.Fatalf("extractOutput returned error: %v", err)
	}

	marker, ok := wf.metadata["KSampler.__extract_missing"]
	if !ok {
		t.Fatalf("expected metadata marker KSampler.__extract_missing, got keys %v", wf.Metadata())
	}
	if core.GetValue(marker.Value) != "true" {
		t.Fatalf("marker value = %v, want true", marker.Value)
	}
	if marker.SrcNode != "KSampler" {
		t.Fatalf("marker SrcNode = %q, want KSampler", marker.SrcNode)
	}
}

// 有输出块时不打标记
func TestExtractOutput_PresentBlockNoMarker(t *testing.T) {
	wf := &WorkflowImpl{metadata: make(Metadata)}
	node := NewDGANodeWithConfig("KSampler", "", "", "", nil, map[string]any{
		"extract": map[string]interface{}{"type": "codec-block"},
	})

	fullOutput := "[job] running 30/30\n```flowx-yaml\nlatent: \"abc\"\nseed: \"42\"\n```\n"
	if err := wf.extractOutput(context.Background(), node, nil, fullOutput); err != nil {
		t.Fatalf("extractOutput returned error: %v", err)
	}
	if _, ok := wf.metadata["KSampler.__extract_missing"]; ok {
		t.Fatal("marker should not be set when extraction succeeded")
	}
	if core.GetValue(wf.metadata["KSampler.latent"].Value) != "abc" {
		t.Fatalf("latent not extracted: %v", wf.Metadata())
	}
}

// 只有声明 extract 的节点才应启用容器内 tee 兜底（CaptureOutput）：
// 包壳会给每个这类步骤多一次清理 exec，不能给无输出块的节点白花。
func TestSendCommands_CaptureOutputOnlyForExtractNodes(t *testing.T) {
	steps := []core.Step{{Name: "run", Run: "echo hi"}}
	cases := []struct {
		name string
		cfg  map[string]any
		want bool
	}{
		{"有 extract", map[string]any{"extract": map[string]interface{}{"type": "codec-block"}}, true},
		{"无 extract", map[string]any{"command": "echo hi"}, false},
		{"extract 为 null", map[string]any{"extract": nil}, false},
		{"无配置", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wf := &WorkflowImpl{}
			node := NewDGANodeWithConfig("KSampler", "", "", "", nil, tc.cfg)
			ch := make(chan any, 4)
			wf.sendCommands(context.Background(), node, ch, steps)

			select {
			case got := <-ch:
				wrapper, ok := got.(executor.CommandWrapper)
				if !ok {
					t.Fatalf("unexpected payload type %T", got)
				}
				if wrapper.CaptureOutput != tc.want {
					t.Fatalf("CaptureOutput = %v, want %v", wrapper.CaptureOutput, tc.want)
				}
				if !strings.Contains(wrapper.Command, "echo hi") {
					t.Fatalf("命令不应被改写: %q", wrapper.Command)
				}
			default:
				t.Fatal("no command sent")
			}
		})
	}
}

// 执行器报告流被截断：必须留标记（Studio 据此提示"输出可能不完整"），且不改变节点状态
func TestHandleResult_StreamTruncatedMarksMetadata(t *testing.T) {
	wf := &WorkflowImpl{metadata: make(Metadata)}
	node := NewDGANodeWithConfig("KSampler", "", "", "", nil, map[string]any{
		"extract": map[string]interface{}{"type": "codec-block"},
	})

	wf.handleResult(context.Background(), node, nil, &executor.StepResult{
		StepName:        "run",
		StreamTruncated: true,
	}, 0, nil)

	marker, ok := wf.metadata["KSampler.__stream_truncated"]
	if !ok {
		t.Fatalf("expected marker KSampler.__stream_truncated, got keys %v", wf.Metadata())
	}
	if core.GetValue(marker.Value) != "true" || marker.SrcNode != "KSampler" {
		t.Fatalf("marker = %+v, want true/@KSampler", marker)
	}
}

// 未截断不标记（正常路径不能出现假阳性）
func TestHandleResult_NoTruncationNoMarker(t *testing.T) {
	wf := &WorkflowImpl{metadata: make(Metadata)}
	node := NewDGANodeWithConfig("KSampler", "", "", "", nil, nil)

	wf.handleResult(context.Background(), node, nil, &executor.StepResult{StepName: "run"}, 0, nil)

	if _, ok := wf.metadata["KSampler.__stream_truncated"]; ok {
		t.Fatal("marker should not be set when stream was intact")
	}
}
