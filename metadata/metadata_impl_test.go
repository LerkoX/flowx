package metadata

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LerkoX/flowx/core"
)

// ========== HTTPMetadataStore 测试 ==========

func TestNewHTTPMetadataStore_MissingURL(t *testing.T) {
	config := core.MetadataConfig{Type: "http", Data: map[string]interface{}{}}
	_, err := NewHTTPMetadataStore(config, "pipe-123")
	if err == nil {
		t.Error("Expected error when URL is missing")
	}
}

func TestNewHTTPMetadataStore_WithWorkflowId(t *testing.T) {
	config := core.MetadataConfig{
		Type: "http",
		Data: map[string]interface{}{
			"url":    "http://example.com",
			"method": "POST",
			"headers": map[string]interface{}{
				"Authorization": "Bearer token123",
			},
		},
	}
	store, err := NewHTTPMetadataStore(config, "pipe-123")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if store.workflowId != "pipe-123" {
		t.Errorf("Expected workflowId='pipe-123', got '%s'", store.workflowId)
	}
	if store.method != "POST" {
		t.Errorf("Expected method='POST', got '%s'", store.method)
	}
	if store.headers["Authorization"] != "Bearer token123" {
		t.Errorf("Expected Authorization header, got %v", store.headers)
	}
}

func TestNewHTTPMetadataStore_DefaultMethod(t *testing.T) {
	config := core.MetadataConfig{
		Type: "http",
		Data: map[string]interface{}{
			"url": "http://example.com",
		},
	}
	store, err := NewHTTPMetadataStore(config, "")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if store.method != "GET" {
		t.Errorf("Expected default method='GET', got '%s'", store.method)
	}
}

func TestHTTPMetadataStore_Get_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证 X-Workflow-ID Header
		if r.Header.Get("X-Workflow-ID") != "pipe-123" {
			t.Errorf("Expected X-Workflow-ID='pipe-123', got '%s'", r.Header.Get("X-Workflow-ID"))
		}
		key := r.URL.Query().Get("key")
		if key != "testKey" {
			t.Errorf("Expected key='testKey', got '%s'", key)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("testValue"))
	}))
	defer server.Close()

	store, _ := NewHTTPMetadataStore(core.MetadataConfig{
		Type: "http",
		Data: map[string]interface{}{"url": server.URL},
	}, "pipe-123")

	val, err := store.Get(context.Background(), "testKey")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if val != "testValue" {
		t.Errorf("Expected 'testValue', got '%s'", val)
	}
}

func TestHTTPMetadataStore_Get_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	store, _ := NewHTTPMetadataStore(core.MetadataConfig{
		Type: "http",
		Data: map[string]interface{}{"url": server.URL},
	}, "")

	_, err := store.Get(context.Background(), "missing")
	if err == nil {
		t.Error("Expected error for 404 response")
	}
}

func TestHTTPMetadataStore_Set_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.Header.Get("X-Workflow-ID") != "pipe-456" {
			t.Errorf("Expected X-Workflow-ID='pipe-456', got '%s'", r.Header.Get("X-Workflow-ID"))
		}

		var payload map[string]string
		json.NewDecoder(r.Body).Decode(&payload)
		if payload["key"] != "myKey" || payload["value"] != "myValue" {
			t.Errorf("Unexpected payload: %v", payload)
		}

		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	store, _ := NewHTTPMetadataStore(core.MetadataConfig{
		Type: "http",
		Data: map[string]interface{}{"url": server.URL},
	}, "pipe-456")

	err := store.Set(context.Background(), "myKey", "myValue")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestHTTPMetadataStore_Set_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	store, _ := NewHTTPMetadataStore(core.MetadataConfig{
		Type: "http",
		Data: map[string]interface{}{"url": server.URL},
	}, "")

	err := store.Set(context.Background(), "key", "value")
	if err == nil {
		t.Error("Expected error for 500 response")
	}
}

func TestHTTPMetadataStore_Delete_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("Expected DELETE, got %s", r.Method)
		}
		if r.Header.Get("X-Workflow-ID") != "pipe-789" {
			t.Errorf("Expected X-Workflow-ID='pipe-789', got '%s'", r.Header.Get("X-Workflow-ID"))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	store, _ := NewHTTPMetadataStore(core.MetadataConfig{
		Type: "http",
		Data: map[string]interface{}{"url": server.URL},
	}, "pipe-789")

	err := store.Delete(context.Background(), "delKey")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestHTTPMetadataStore_Close(t *testing.T) {
	store, _ := NewHTTPMetadataStore(core.MetadataConfig{
		Type: "http",
		Data: map[string]interface{}{"url": "http://example.com"},
	}, "")
	if err := store.Close(); err != nil {
		t.Errorf("Close should not return error: %v", err)
	}
}

// ========== RedisMetadataStore buildRedisKey 测试 ==========

func TestBuildRedisKey_WithWorkflowId(t *testing.T) {
	store := &RedisMetadataStore{workflowId: "pipe-abc"}
	result := store.buildRedisKey("mykey")
	expected := "flowx/pipe-abc/mykey"
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

func TestBuildRedisKey_WithoutWorkflowId(t *testing.T) {
	store := &RedisMetadataStore{workflowId: ""}
	result := store.buildRedisKey("mykey")
	if result != "mykey" {
		t.Errorf("Expected 'mykey', got '%s'", result)
	}
}

// ========== DefaultMetadataStoreFactory 测试 ==========

func TestDefaultMetadataStoreFactory_Create_InConfig(t *testing.T) {
	factory := NewMetadataStoreFactory()
	config := core.MetadataConfig{
		Type: "in-config",
		Data: map[string]interface{}{"key1": "val1"},
	}
	store, err := factory.Create(config, "pipe-1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if store == nil {
		t.Fatal("Expected non-nil store")
	}
	// InConfig 不依赖 workflowId
	store.Close()
}

func TestDefaultMetadataStoreFactory_Create_HTTP(t *testing.T) {
	factory := NewMetadataStoreFactory()
	config := core.MetadataConfig{
		Type: "http",
		Data: map[string]interface{}{"url": "http://example.com"},
	}
	store, err := factory.Create(config, "pipe-2")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if store == nil {
		t.Fatal("Expected non-nil store")
	}
	store.Close()
}

func TestDefaultMetadataStoreFactory_Create_Unsupported(t *testing.T) {
	factory := NewMetadataStoreFactory()
	config := core.MetadataConfig{Type: "unknown"}
	_, err := factory.Create(config, "pipe-3")
	if err == nil {
		t.Error("Expected error for unsupported type")
	}
}

// ========== NewRedisMetadataStore 配置解析测试 ==========

func TestNewRedisMetadataStore_Defaults(t *testing.T) {
	// 使用一个不可达的地址，只测试配置解析
	config := core.MetadataConfig{
		Type: "redis",
		Data: map[string]interface{}{},
	}
	store, err := NewRedisMetadataStore(config, "pipe-1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if store.workflowId != "pipe-1" {
		t.Errorf("Expected workflowId='pipe-1', got '%s'", store.workflowId)
	}
	store.Close()
}

func TestNewRedisMetadataStore_CustomPort(t *testing.T) {
	config := core.MetadataConfig{
		Type: "redis",
		Data: map[string]interface{}{
			"host": "127.0.0.1",
			"port": 6380,
		},
	}
	store, err := NewRedisMetadataStore(config, "")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	store.Close()
}

func TestNewRedisMetadataStore_PortAsString(t *testing.T) {
	config := core.MetadataConfig{
		Type: "redis",
		Data: map[string]interface{}{
			"port": "6381",
		},
	}
	store, err := NewRedisMetadataStore(config, "")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	store.Close()
}

func TestNewRedisMetadataStore_PortAsFloat64(t *testing.T) {
	config := core.MetadataConfig{
		Type: "redis",
		Data: map[string]interface{}{
			"port": float64(6382),
		},
	}
	store, err := NewRedisMetadataStore(config, "")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	store.Close()
}

func TestNewRedisMetadataStore_DBAsString(t *testing.T) {
	config := core.MetadataConfig{
		Type: "redis",
		Data: map[string]interface{}{
			"db": "2",
		},
	}
	store, err := NewRedisMetadataStore(config, "")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	store.Close()
}

func TestNewRedisMetadataStore_WithAuth(t *testing.T) {
	config := core.MetadataConfig{
		Type: "redis",
		Data: map[string]interface{}{
			"username": "user1",
			"password": "pass1",
		},
	}
	store, err := NewRedisMetadataStore(config, "pipe-auth")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if store.workflowId != "pipe-auth" {
		t.Errorf("Expected workflowId='pipe-auth', got '%s'", store.workflowId)
	}
	store.Close()
}

// ========== InConfigMetadataStore 测试（已有但补充边界） ==========

func TestNewInConfigMetadataStore_NonStringValue(t *testing.T) {
	config := core.MetadataConfig{
		Type: "in-config",
		Data: map[string]interface{}{
			"count": 42,
			"flag":  true,
		},
	}
	store, err := NewInConfigMetadataStore(config)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	count, _ := store.Get(context.Background(), "count")
	if count != "42" {
		t.Errorf("Expected '42', got '%s'", count)
	}

	flag, _ := store.Get(context.Background(), "flag")
	if flag != "true" {
		t.Errorf("Expected 'true', got '%s'", flag)
	}
}

func TestInConfigMetadataStore_GetAll(t *testing.T) {
	config := core.MetadataConfig{
		Type: "in-config",
		Data: map[string]interface{}{
			"a": "1",
			"b": "2",
		},
	}
	store, _ := NewInConfigMetadataStore(config)
	all := store.GetAll()
	if len(all) != 2 {
		t.Errorf("Expected 2 items, got %d", len(all))
	}
	// 验证返回的是副本
	all["c"] = "3"
	_, err := store.Get(context.Background(), "c")
	if err == nil {
		t.Error("GetAll should return a copy, modifications should not affect store")
	}
}

func TestInConfigMetadataStore_Keys(t *testing.T) {
	config := core.MetadataConfig{
		Type: "in-config",
		Data: map[string]interface{}{
			"x": "1",
			"y": "2",
		},
	}
	store, _ := NewInConfigMetadataStore(config)
	keys := store.Keys()
	if len(keys) != 2 {
		t.Errorf("Expected 2 keys, got %d", len(keys))
	}
}

func TestInConfigMetadataStore_Get_NotFound(t *testing.T) {
	store, _ := NewInConfigMetadataStore(core.MetadataConfig{Type: "in-config"})
	_, err := store.Get(context.Background(), "missing")
	if err == nil {
		t.Error("Expected error for missing key")
	}
}

func TestInConfigMetadataStore_SetAndGet(t *testing.T) {
	store, _ := NewInConfigMetadataStore(core.MetadataConfig{Type: "in-config"})
	ctx := context.Background()

	store.Set(ctx, "newKey", "newValue")
	val, err := store.Get(ctx, "newKey")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if val != "newValue" {
		t.Errorf("Expected 'newValue', got '%s'", val)
	}
}

func TestInConfigMetadataStore_Delete(t *testing.T) {
	store, _ := NewInConfigMetadataStore(core.MetadataConfig{
		Type: "in-config",
		Data: map[string]interface{}{"toDelete": "val"},
	})
	ctx := context.Background()

	store.Delete(ctx, "toDelete")
	_, err := store.Get(ctx, "toDelete")
	if err == nil {
		t.Error("Expected error after delete")
	}
}
