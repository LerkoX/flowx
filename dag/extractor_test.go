package dag

import (
	"testing"

	"github.com/LerkoX/flowx/core"
)

func TestCodecBlockExtractor_ExtractYAML(t *testing.T) {
	extractor := NewCodecBlockExtractor(1024 * 1024)

	output := `
Build process started...

` + "```flowx-yaml\n" + `version: 2.0.0
buildTime: 2024-01-15T11:30:00Z
artifacts:
  - name: app
    path: /app/binary
  - name: config
    path: /app/config.yaml` + "\n```\n"

	result, err := extractor.Extract(output)
	if err != nil {
		t.Fatalf("Failed to extract: %v", err)
	}

	if len(result) != 3 {
		t.Errorf("Expected 3 extracted values, got %d", len(result))
	}

	if core.GetValue(result["version"].Value) != "2.0.0" {
		t.Errorf("Expected version=2.0.0, got %v", result["version"])
	}

	artifacts, ok := core.GetValue(result["artifacts"].Value).([]interface{})
	if !ok {
		t.Fatalf("Expected artifacts to be a list, got %T", result["artifacts"])
	}

	if len(artifacts) != 2 {
		t.Errorf("Expected 2 artifacts, got %d", len(artifacts))
	}
}

func TestCodecBlockExtractor_ExtractYAMLWithComments(t *testing.T) {
	extractor := NewCodecBlockExtractor(1024 * 1024)

	output := `
Build process started...

` + "```flowx-yaml\n" + `version: 1.0.0  # 版本号
buildStatus: success  # 构建状态
imageTag: "myapp:v1.0.0"  # 镜像标签
` + "\n```\n"

	result, err := extractor.Extract(output)
	if err != nil {
		t.Fatalf("Failed to extract: %v", err)
	}

	if len(result) != 3 {
		t.Errorf("Expected 3 extracted values, got %d", len(result))
	}

	// 验证值
	if core.GetValue(result["version"].Value) != "1.0.0" {
		t.Errorf("Expected version=1.0.0, got %v", result["version"].Value)
	}
	if core.GetValue(result["buildStatus"].Value) != "success" {
		t.Errorf("Expected buildStatus=success, got %v", result["buildStatus"].Value)
	}

	// 验证描述
	if result["version"].Description != "版本号" {
		t.Errorf("Expected version description='版本号', got '%s'", result["version"].Description)
	}
	if result["buildStatus"].Description != "构建状态" {
		t.Errorf("Expected buildStatus description='构建状态', got '%s'", result["buildStatus"].Description)
	}
	if result["imageTag"].Description != "镜像标签" {
		t.Errorf("Expected imageTag description='镜像标签', got '%s'", result["imageTag"].Description)
	}

	// SrcNode 应该在调用 extractOutput 时由调用方设置
}

func TestRegexExtractor(t *testing.T) {
	patterns := map[string]string{
		"coverage":    "coverage: (\\d+\\.\\d+)%",
		"testsPassed": "(\\d+) tests passed",
		"buildStatus": "Build (\\w+)",
	}

	extractor, err := NewRegexExtractor(patterns, 1024*1024)
	if err != nil {
		t.Fatalf("Failed to create extractor: %v", err)
	}

	output := `
Running tests...
coverage: 85.5%
42 tests passed
3 tests failed
Build SUCCESS
`

	result, err := extractor.Extract(output)
	if err != nil {
		t.Fatalf("Failed to extract: %v", err)
	}

	if core.GetValue(result["coverage"].Value) != "85.5" {
		t.Errorf("Expected coverage=85.5, got %v", result["coverage"].Value)
	}

	if core.GetValue(result["testsPassed"].Value) != "42" {
		t.Errorf("Expected testsPassed=42, got %v", result["testsPassed"].Value)
	}

	if core.GetValue(result["buildStatus"].Value) != "SUCCESS" {
		t.Errorf("Expected buildStatus=SUCCESS, got %v", result["buildStatus"].Value)
	}
}

func TestRegexExtractor_NoMatches(t *testing.T) {
	patterns := map[string]string{
		"version": "v(\\d+\\.\\d+\\.\\d+)",
	}

	extractor, err := NewRegexExtractor(patterns, 1024*1024)
	if err != nil {
		t.Fatalf("Failed to create extractor: %v", err)
	}

	output := `
No version information here
Just plain text without any pattern matches
`

	result, err := extractor.Extract(output)
	if err != nil {
		t.Fatalf("Failed to extract: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("Expected no matches, got %d matches", len(result))
	}
}

func TestRegexExtractor_InvalidPattern(t *testing.T) {
	patterns := map[string]string{
		"invalid": "[", // Invalid regex
	}

	_, err := NewRegexExtractor(patterns, 1024*1024)
	if err == nil {
		t.Fatal("Expected error for invalid regex pattern")
	}
}

func TestRegexExtractor_WithoutCaptureGroup(t *testing.T) {
	patterns := map[string]string{
		"filename": "file: \\w+\\.txt", // No capture group
	}

	extractor, err := NewRegexExtractor(patterns, 1024*1024)
	if err != nil {
		t.Fatalf("Failed to create extractor: %v", err)
	}

	output := `
Processing file: report.txt
`

	result, err := extractor.Extract(output)
	if err != nil {
		t.Fatalf("Failed to extract: %v", err)
	}

	// Without capture group, should use the whole match
	if core.GetValue(result["filename"].Value) != "file: report.txt" {
		t.Errorf("Expected filename=\"file: report.txt\", got %v", result["filename"].Value)
	}
}

func TestRegexExtractor_MultipleCapturingGroups(t *testing.T) {
	patterns := map[string]string{
		"complex": "(\\w+): (\\d+)-(\\w+)", // Multiple groups
	}

	extractor, err := NewRegexExtractor(patterns, 1024*1024)
	if err != nil {
		t.Fatalf("Failed to create extractor: %v", err)
	}

	output := `
Result: 123-ABC
`

	result, err := extractor.Extract(output)
	if err != nil {
		t.Fatalf("Failed to extract: %v", err)
	}

	// Should use the first group
	if core.GetValue(result["complex"].Value) != "Result" {
		t.Errorf("Expected first group 'Result', got %v", result["complex"].Value)
	}
}
