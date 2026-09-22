package dag

import (
	"strings"
	"testing"

	"github.com/LerkoX/flowx/core"
)

// 上游绑定引用校验（dag/binding_check.go）：
// exec 364 事故——docker exec 流被截断导致 KSampler 绿灯但无输出，pongo2 把
// {{ KSampler.latent }} 渲染成空串，错误最终落到下游 VAEDecode 的"缺参"上。
// 这里验证：缺失即失败（并指出上游），允许缺省的两种写法不受影响。
func TestValidateNodeBindings(t *testing.T) {
	newWF := func(metadata Metadata) *WorkflowImpl {
		wf := &WorkflowImpl{metadata: metadata}
		g := NewDGAGraph()
		g.AddVertex(NewDGANode("KSampler", "SUCCESS"))
		g.AddVertex(NewDGANode("VAEDecode", "UNKNOWN"))
		wf.SetGraph(g)
		return wf
	}

	cases := []struct {
		name    string
		meta    Metadata
		params  map[string]string
		wantErr string // 空串表示期望通过；非空表示期望错误信息包含该子串
	}{
		{
			name:   "字段存在则通过",
			meta:   Metadata{"KSampler.latent": core.FieldItem{Value: "abc"}, "KSampler.seed": core.FieldItem{Value: "42"}},
			params: map[string]string{"latent": "{{ KSampler.latent }}", "seed": "{{ KSampler.seed }}"},
		},
		{
			name:    "上游无输出（流被截断）应失败",
			meta:    Metadata{},
			params:  map[string]string{"latent": "{{ KSampler.latent }}"},
			wantErr: "上游节点 KSampler 没有任何输出",
		},
		{
			name:    "上游有输出但缺该字段应失败并列出已输出字段",
			meta:    Metadata{"KSampler.seed": core.FieldItem{Value: "42"}},
			params:  map[string]string{"latent": "{{ KSampler.latent }}"},
			wantErr: "未输出字段 latent",
		},
		{
			name:   "Param 引用允许缺省",
			meta:   Metadata{},
			params: map[string]string{"service_url": "{{ Param.service_url }}", "other": "{{ Param.not_exist }}"},
		},
		{
			name:   "带 default 过滤器允许缺省",
			meta:   Metadata{},
			params: map[string]string{"latent": `{{ KSampler.latent|default:"" }}`},
		},
		{
			name:   "字符串内嵌引用同样校验（存在即通过）",
			meta:   Metadata{"KSampler.seed": core.FieldItem{Value: "42"}},
			params: map[string]string{"prompt": "seed={{ KSampler.seed }} end"},
		},
		{
			name:   "嵌套字段：只校验第一层",
			meta:   Metadata{"EmbedNeg.outputs_json": core.FieldItem{Value: map[string]interface{}{"clip": "x"}}},
			params: map[string]string{"clip": "{{ EmbedNeg.outputs_json.clip }}"},
		},
		{
			name:   "非节点根（扁平化 Param）不校验",
			meta:   Metadata{},
			params: map[string]string{"x": "{{ some_param }}"},
		},
		{
			name:   "无模板的纯字面量不校验",
			meta:   Metadata{},
			params: map[string]string{"x": "plain-value"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wf := newWF(tc.meta)
			node := NewDGANodeWithConfig("VAEDecode", "", "", "", nil, map[string]any{
				"params": tc.params,
			})
			err := wf.validateNodeBindings(node)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected pass, got error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
			if !strings.Contains(err.Error(), "VAEDecode") || !strings.Contains(err.Error(), "latent") {
				t.Fatalf("error should locate the node/param, got: %v", err)
			}
		})
	}
}

// env 模板同样在守卫范围内（节点包参数经 env 注入）
func TestValidateNodeBindings_EnvTemplate(t *testing.T) {
	wf := &WorkflowImpl{metadata: Metadata{}}
	g := NewDGAGraph()
	g.AddVertex(NewDGANode("KSampler", "SUCCESS"))
	wf.SetGraph(g)

	node := NewDGANodeWithConfig("VAEDecode", "", "", "", nil, map[string]any{
		"env": map[string]string{"LATENT": "{{ KSampler.latent }}"},
	})
	err := wf.validateNodeBindings(node)
	if err == nil {
		t.Fatal("expected error for missing upstream field in env template")
	}
	if !strings.Contains(err.Error(), "env LATENT") {
		t.Fatalf("error should name the env key, got: %v", err)
	}
}

// 空图（无节点定义）时不校验，避免误伤非图执行场景
func TestValidateNodeBindings_NoGraph(t *testing.T) {
	wf := &WorkflowImpl{metadata: Metadata{}}
	node := NewDGANodeWithConfig("VAEDecode", "", "", "", nil, map[string]any{
		"params": map[string]string{"latent": "{{ KSampler.latent }}"},
	})
	if err := wf.validateNodeBindings(node); err != nil {
		t.Fatalf("expected no error without graph, got %v", err)
	}
}
