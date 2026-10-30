package logs

import (
	"fmt"
	"testing"
	"time"
)

func TestBuffer(t *testing.T) {
	b := New()
	b.now = func() time.Time { return time.UnixMilli(1000) }
	_, _ = b.Write([]byte("2026/10/30 12:00:00 [扫描] 开始\n"))
	_, _ = b.Write([]byte("2026/10/30 12:00:01 [WARN] [下载器] qb 读取失败\n2026/10/30 12:00:01 [ERROR] 出错了\n"))
	es, last := b.Since(0, 0)
	if len(es) != 3 || last != 3 {
		t.Fatalf("条数 %d last %d", len(es), last)
	}
	if es[0].Level != "INFO" || es[0].Msg != "[扫描] 开始" || es[0].Time != 1000 {
		t.Errorf("第一条解析错误: %+v", es[0])
	}
	if es[1].Level != "WARN" || es[1].Msg != "[下载器] qb 读取失败" || es[2].Level != "ERROR" {
		t.Errorf("级别解析错误: %+v %+v", es[1], es[2])
	}
	if es, _ := b.Since(2, 0); len(es) != 1 || es[0].ID != 3 {
		t.Errorf("增量读取错误: %+v", es)
	}

	for i := 0; i < Capacity+10; i++ {
		_, _ = fmt.Fprintf(b, "line %d\n", i)
	}
	es, last = b.Since(0, 0)
	if len(es) != Capacity || es[len(es)-1].ID != last {
		t.Errorf("超出容量后应只保留最近 %d 条: %d", Capacity, len(es))
	}
	if es, _ := b.Since(0, 50); len(es) != 50 || es[49].ID != last {
		t.Errorf("limit 应返回最新的 50 条")
	}
}
