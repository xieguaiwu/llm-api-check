package models

import "testing"

// baiRecordsFixture --stats 聚合输入（形状对照 2026-09-06 usage.records 实测）
func baiRec(model, at string, in, out, total, cost int64) BaiRecord {
	return BaiRecord{Model: model, CreatedAt: at, InputTokens: in, OutputTokens: out, TotalTokens: total, CostPoints: cost}
}

func TestAggregateBaiUsage(t *testing.T) {
	recs := []BaiRecord{
		baiRec("glm-5.3-flash", "2026-09-06T05:28:30.000Z", 100, 10, 110, 0),
		baiRec("qwen3.8-flash", "2026-09-06T05:09:11.000Z", 200, 20, 220, 5),
		baiRec("glm-5.3-flash", "2026-09-06T05:15:00.000Z", 300, 30, 330, 0),
	}
	got := AggregateBaiUsage(recs)
	if got.RecordsFetched != 3 || got.TotalRequests != 3 {
		t.Fatalf("记录数不符: %+v", got)
	}
	if got.TotalTokens != 660 || got.TotalCostPoints != 5 {
		t.Errorf("总量不符: tokens=%d cost=%d", got.TotalTokens, got.TotalCostPoints)
	}
	if got.WindowStart != "2026-09-06T05:09:11.000Z" || got.WindowEnd != "2026-09-06T05:28:30.000Z" {
		t.Errorf("窗口不符: %s ~ %s", got.WindowStart, got.WindowEnd)
	}
	if len(got.PerModel) != 2 {
		t.Fatalf("模型数不符: %+v", got.PerModel)
	}
	// glm 2 次在前，qwen 1 次在后
	if got.PerModel[0].Model != "glm-5.3-flash" || got.PerModel[0].Requests != 2 {
		t.Errorf("降序不符: %+v", got.PerModel)
	}
	if got.PerModel[0].InputTokens != 400 || got.PerModel[0].OutputTokens != 40 || got.PerModel[0].TotalTokens != 440 {
		t.Errorf("glm 聚合不符: %+v", got.PerModel[0])
	}
	if got.PerModel[1].Model != "qwen3.8-flash" || got.PerModel[1].CostPoints != 5 {
		t.Errorf("qwen 聚合不符: %+v", got.PerModel[1])
	}
	if got.Complete {
		t.Errorf("纯函数不设 Complete（由 repo 翻页层根据 has_more 决定），此处应保持零值")
	}
}

func TestAggregateBaiUsageTiesAndEmpty(t *testing.T) {
	// 同请求数按字典序
	got := AggregateBaiUsage([]BaiRecord{
		baiRec("b-model", "2026-09-06T05:00:00.000Z", 1, 1, 2, 0),
		baiRec("a-model", "2026-09-06T05:01:00.000Z", 1, 1, 2, 0),
	})
	if got.PerModel[0].Model != "a-model" || got.PerModel[1].Model != "b-model" {
		t.Errorf("同数应按字典序: %+v", got.PerModel)
	}
	if got.WindowStart != "2026-09-06T05:00:00.000Z" || got.WindowEnd != "2026-09-06T05:01:00.000Z" {
		t.Errorf("窗口不符: %s ~ %s", got.WindowStart, got.WindowEnd)
	}
	// 空输入零值
	empty := AggregateBaiUsage(nil)
	if empty.RecordsFetched != 0 || len(empty.PerModel) != 0 || empty.WindowStart != "" {
		t.Errorf("空输入应为零值: %+v", empty)
	}
	// CreatedAt 缺席不参与窗口
	noTime := AggregateBaiUsage([]BaiRecord{{Model: "m", TotalTokens: 9}})
	if noTime.WindowStart != "" || noTime.WindowEnd != "" {
		t.Errorf("无时间戳不应产生窗口: %+v", noTime)
	}
}
