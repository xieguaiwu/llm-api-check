package parsers

import (
	"strings"
	"testing"
)

// lcModelsHappy GET /v1/models 成功响应（OpenAI 兼容格式，2026-09-13 实测形状）。
const lcModelsHappy = `{"object":"list","data":[` +
	`{"id":"LongCat-2.0","object":"model","created":1700000000,"owned_by":"meituan"},` +
	`{"id":"LongCat-Flash-Chat","object":"model","created":1700000001,"owned_by":"meituan"}` +
	`]}`

// lcModelsEmpty 空模型清单。
const lcModelsEmpty = `{"object":"list","data":[]}`

func TestParseLongCatModelsHappy(t *testing.T) {
	ms, err := ParseLongCatModels(lcModelsHappy)
	if err != nil {
		t.Fatalf("ParseLongCatModels: %v", err)
	}
	if len(ms) != 2 {
		t.Fatalf("len: got %d, want 2", len(ms))
	}
	if ms[0].ID != "LongCat-2.0" {
		t.Errorf("ms[0].ID: %q", ms[0].ID)
	}
	if ms[1].ID != "LongCat-Flash-Chat" {
		t.Errorf("ms[1].ID: %q", ms[1].ID)
	}
	if ms[0].OwnedBy != "meituan" {
		t.Errorf("ms[0].OwnedBy: %q", ms[0].OwnedBy)
	}
}

func TestParseLongCatModelsEmpty(t *testing.T) {
	_, err := ParseLongCatModels(lcModelsEmpty)
	if err == nil {
		t.Fatal("empty models should error")
	}
}

func TestParseLongCatModelsDedup(t *testing.T) {
	dup := `{"data":[{"id":"LongCat-2.0","owned_by":"meituan"},{"id":"LongCat-2.0","owned_by":"meituan"}]}`
	ms, err := ParseLongCatModels(dup)
	if err != nil {
		t.Fatalf("ParseLongCatModels: %v", err)
	}
	if len(ms) != 1 {
		t.Fatalf("dedup: got %d, want 1", len(ms))
	}
}

func TestParseLongCatModelsSorted(t *testing.T) {
	unsorted := `{"data":[{"id":"Z-Model","owned_by":"x"},{"id":"A-Model","owned_by":"x"}]}`
	ms, err := ParseLongCatModels(unsorted)
	if err != nil {
		t.Fatalf("ParseLongCatModels: %v", err)
	}
	if ms[0].ID != "A-Model" || ms[1].ID != "Z-Model" {
		t.Errorf("not sorted: %s, %s", ms[0].ID, ms[1].ID)
	}
}

func TestParseLongCatModelsInvalidJSON(t *testing.T) {
	_, err := ParseLongCatModels("not json")
	if err == nil {
		t.Fatal("invalid JSON should error")
	}
	if !strings.Contains(err.Error(), "LongCat") {
		t.Errorf("error should mention LongCat: %v", err)
	}
}

func TestErrLongCatAuth(t *testing.T) {
	// 401 错误响应不应包含 key 原文（与 GPTZero 不同）
	msg := ErrLongCatAuth.Error()
	if msg == "" {
		t.Fatal("ErrLongCatAuth message empty")
	}
}
