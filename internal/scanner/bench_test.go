package scanner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// 生成 n 个文件（每目录 100 个），一半在 Media 中有硬链接。
func genTree(tb testing.TB, root string, n int) (string, string) {
	src := filepath.Join(root, "Downloads")
	media := filepath.Join(root, "Media")
	for i := 0; i < n; i++ {
		d := fmt.Sprintf("d%04d", i/100)
		p := filepath.Join(src, d, fmt.Sprintf("f%06d.mkv", i))
		if i%100 == 0 {
			_ = os.MkdirAll(filepath.Dir(p), 0o755)
			_ = os.MkdirAll(filepath.Join(media, d), 0o755)
		}
		if err := os.WriteFile(p, []byte(strconv.Itoa(i)), 0o644); err != nil {
			tb.Fatal(err)
		}
		if i%2 == 0 {
			_ = os.Link(p, filepath.Join(media, d, filepath.Base(p)))
		}
	}
	return src, media
}

func BenchmarkScan20k(b *testing.B) {
	src, media := genTree(b, b.TempDir(), 20000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Scan(context.Background(), Options{Source: src, Targets: []string{media}, Recursive: true, DupMode: DupQuick}); err != nil {
			b.Fatal(err)
		}
	}
}

// LS_BENCH_N=200000 go test -run TestLargeScan -v ./internal/scanner/
func TestLargeScan(t *testing.T) {
	n, _ := strconv.Atoi(os.Getenv("LS_BENCH_N"))
	if n == 0 {
		t.Skip("设置 LS_BENCH_N 才运行")
	}
	root := t.TempDir()
	t0 := time.Now()
	src, media := genTree(t, root, n)
	t.Logf("生成 %d 个文件用时 %v", n, time.Since(t0))
	t1 := time.Now()
	mode := DupMode(os.Getenv("LS_BENCH_DUP"))
	if mode == "" {
		mode = DupQuick
	}
	r, err := Scan(context.Background(), Options{Source: src, Targets: []string{media}, Recursive: true, DupMode: mode})
	if err != nil {
		t.Fatal(err)
	}
	el := time.Since(t1)
	t.Logf("扫描 %d 个源文件 + %d 个目标文件用时 %v（%.0f 文件/秒），已链接 %d", r.Summary.Total, n/2, el,
		float64(n+n/2)/el.Seconds(), r.Summary.Linked)
}
