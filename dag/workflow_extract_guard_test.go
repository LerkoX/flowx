package dag

import (
	"context"
	"testing"

	"github.com/LerkoX/flowx/core"
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
