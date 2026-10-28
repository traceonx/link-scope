package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/traceonx/link-scope/internal/config"
	"github.com/traceonx/link-scope/internal/fileops"
	"github.com/traceonx/link-scope/internal/jobs"
	"github.com/traceonx/link-scope/internal/torrents"
	"github.com/traceonx/link-scope/internal/update"
)

func newTestServer(t *testing.T, mutate func(*config.Config)) (*httptest.Server, string) {
	t.Helper()
	root, _ := filepath.EvalSymlinks(t.TempDir())
	src := filepath.Join(root, "Downloads")
	media := filepath.Join(root, "Media")
	_ = os.MkdirAll(src, 0o755)
	_ = os.MkdirAll(media, 0o755)
	_ = os.WriteFile(filepath.Join(src, "a.mkv"), []byte("aaaa"), 0o644)
	_ = os.WriteFile(filepath.Join(src, "b.mkv"), []byte("bbbb"), 0o644)
	_ = os.Link(filepath.Join(src, "a.mkv"), filepath.Join(media, "a.mkv"))

	cfg := &config.Config{Roots: []string{root}, DataDir: filepath.Join(root, "data"), Workers: 4, HashWorkers: 2,
		SkipDirs: map[string]bool{}}
	if mutate != nil {
		mutate(cfg)
	}
	web := fstest.MapFS{"index.html": {Data: []byte("<html>app</html>")}}
	t.Setenv("UPDATE_DOCKER_SOCKET", filepath.Join(root, "none.sock"))
	t.Setenv("UPDATE_HUB_API", "http://127.0.0.1:1") // 测试不访问外网
	dl := torrents.NewStore(cfg.DataDir)
	ops := fileops.New(cfg.Roots, cfg.DataDir, 30, false)
	ts := httptest.NewServer(New(cfg, jobs.New(cfg.DataDir, dl), update.New(cfg.DataDir), ops, dl, web).Handler())
	t.Cleanup(ts.Close)
	return ts, root
}

func getJSON(t *testing.T, url string, v any) int {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	_ = json.NewDecoder(resp.Body).Decode(v)
	return resp.StatusCode
}

func post(t *testing.T, url, body string) int {
	t.Helper()
	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

func TestScanFlow(t *testing.T) {
	ts, root := newTestServer(t, nil)
	body := `{"source":"` + root + `/Downloads","targets":["` + root + `/Media"],"dup_mode":"quick"}`
	if c := post(t, ts.URL+"/api/scan", body); c != 200 {
		t.Fatalf("scan 返回 %d", c)
	}
	var st struct {
		State struct{ Status string } `json:"state"`
	}
	for i := 0; i < 100; i++ {
		getJSON(t, ts.URL+"/api/status", &st)
		if st.State.Status != "running" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if st.State.Status != "done" {
		t.Fatalf("状态 %s", st.State.Status)
	}

	var res struct {
		Count  int            `json:"count"`
		Counts map[string]int `json:"counts"`
		Items  []struct {
			Name  string   `json:"name"`
			Links []string `json:"links"`
		} `json:"items"`
	}
	getJSON(t, ts.URL+"/api/result?filter=linked", &res)
	if res.Count != 1 || res.Items[0].Name != "a.mkv" || len(res.Items[0].Links) != 1 {
		t.Errorf("linked 结果错误: %+v", res)
	}
	if res.Counts["unlinked"] != 1 || res.Counts["all"] != 2 {
		t.Errorf("counts 错误: %+v", res.Counts)
	}
	getJSON(t, ts.URL+"/api/result?q=B.MKV", &res)
	if res.Count != 1 {
		t.Errorf("搜索应不区分大小写: %d", res.Count)
	}

	resp, err := http.Get(ts.URL + "/api/export.csv")
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 4096)
	n, _ := resp.Body.Read(buf)
	resp.Body.Close()
	if !strings.Contains(string(buf[:n]), "已链接") {
		t.Errorf("CSV 内容错误: %s", buf[:n])
	}
}

func TestPathGuard(t *testing.T) {
	ts, _ := newTestServer(t, nil)
	if c := post(t, ts.URL+"/api/scan", `{"source":"/etc"}`); c != 403 {
		t.Errorf("范围外路径应返回 403，实际 %d", c)
	}
	if c := post(t, ts.URL+"/api/scan", `{"source":"relative/path"}`); c != 400 {
		t.Errorf("相对路径应返回 400，实际 %d", c)
	}
	var b map[string]any
	if c := getJSON(t, ts.URL+"/api/browse?path=/etc", &b); c != 403 {
		t.Errorf("browse 范围外应返回 403，实际 %d", c)
	}
}

func TestBrowse(t *testing.T) {
	ts, root := newTestServer(t, nil)
	var b struct {
		Path   string  `json:"path"`
		Parent *string `json:"parent"`
		Dirs   []struct{ Name string }
	}
	getJSON(t, ts.URL+"/api/browse", &b)
	if len(b.Dirs) != 1 || b.Dirs[0].Name != root {
		t.Errorf("根目录列表错误: %+v", b)
	}
	getJSON(t, ts.URL+"/api/browse?path="+root, &b)
	if len(b.Dirs) != 2 || b.Parent == nil || *b.Parent != "" {
		t.Errorf("子目录列表错误: %+v", b)
	}
}

func TestAuthAndSPA(t *testing.T) {
	ts, _ := newTestServer(t, func(c *config.Config) { c.AuthUser, c.AuthPass = "u", "p" })
	resp, _ := http.Get(ts.URL + "/api/config")
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Errorf("未登录应返回 401，实际 %d", resp.StatusCode)
	}
	resp, _ = http.Get(ts.URL + "/api/health")
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("健康检查不应需要登录，实际 %d", resp.StatusCode)
	}
	req, _ := http.NewRequest("GET", ts.URL+"/some/deep/link", nil)
	req.SetBasicAuth("u", "p")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 100)
	n, _ := resp.Body.Read(buf)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(buf[:n]), "app") {
		t.Errorf("前端路由应回落到 index.html: %d %s", resp.StatusCode, buf[:n])
	}
}

func TestUpdateEndpoints(t *testing.T) {
	ts, _ := newTestServer(t, nil)
	var st struct {
		Docker struct {
			Available bool   `json:"available"`
			Reason    string `json:"reason"`
		} `json:"docker"`
	}
	getJSON(t, ts.URL+"/api/update", &st)
	if st.Docker.Available || st.Docker.Reason == "" {
		t.Errorf("没有 socket 时应不可用: %+v", st)
	}
	if c := post(t, ts.URL+"/api/update", ""); c != 400 {
		t.Errorf("不可用时一键更新应返回 400，实际 %d", c)
	}
	req, _ := http.NewRequest("PUT", ts.URL+"/api/update/auto", strings.NewReader(`{"enabled":true,"time":"04:30"}`))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("保存自动更新应返回 200，实际 %d", resp.StatusCode)
	}
}

func postJSON(t *testing.T, url, body string, v any) int {
	t.Helper()
	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if v != nil {
		_ = json.NewDecoder(resp.Body).Decode(v)
	}
	return resp.StatusCode
}

func runScan(t *testing.T, url, root string) {
	t.Helper()
	body := `{"source":"` + root + `/Downloads","targets":["` + root + `/Media"]}`
	if c := post(t, url+"/api/scan", body); c != 200 {
		t.Fatalf("scan 返回 %d", c)
	}
	var st struct {
		State struct{ Status string } `json:"state"`
	}
	for i := 0; i < 200; i++ {
		getJSON(t, url+"/api/status", &st)
		if st.State.Status != "running" {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("扫描未结束")
}

func TestDeleteFlow(t *testing.T) {
	ts, root := newTestServer(t, nil)
	runScan(t, ts.URL, root)
	a := root + "/Downloads/a.mkv"
	b := root + "/Downloads/b.mkv"

	var pv struct {
		Preview struct {
			Freed int64 `json:"freed"`
			Kept  int   `json:"kept"`
		} `json:"preview"`
	}
	postJSON(t, ts.URL+"/api/delete/preview", `{"paths":["`+a+`","`+b+`"]}`, &pv)
	if pv.Preview.Freed != 4 || pv.Preview.Kept != 1 {
		t.Errorf("预览错误: %+v", pv)
	}
	// 按筛选删除：未链接的文件（b.mkv）
	var del struct{ Deleted, Failed int }
	postJSON(t, ts.URL+"/api/delete", `{"filter":"unlinked"}`, &del)
	if del.Deleted != 1 || del.Failed != 0 {
		t.Fatalf("删除结果: %+v", del)
	}
	if _, err := os.Stat(b); !os.IsNotExist(err) {
		t.Error("b.mkv 应已移到回收站")
	}
	var res struct {
		Count   int `json:"count"`
		Summary struct {
			Stale bool `json:"stale"`
		} `json:"summary"`
	}
	getJSON(t, ts.URL+"/api/result", &res)
	if res.Count != 1 || !res.Summary.Stale {
		t.Errorf("删除后结果应同步: %+v", res)
	}
	var tr struct {
		Items []struct{ ID, Path string } `json:"items"`
	}
	getJSON(t, ts.URL+"/api/trash", &tr)
	if len(tr.Items) != 1 || tr.Items[0].Path != b {
		t.Fatalf("回收站: %+v", tr)
	}
	var rr struct{ Restored int }
	postJSON(t, ts.URL+"/api/trash/restore", `{"ids":["`+tr.Items[0].ID+`"]}`, &rr)
	if rr.Restored != 1 {
		t.Error("恢复失败")
	}
	if _, err := os.Stat(b); err != nil {
		t.Error("b.mkv 应已恢复")
	}
	// 范围外与根目录不能删
	var bad struct{ Failed int }
	postJSON(t, ts.URL+"/api/delete", `{"paths":["/etc/hostname","`+root+`"]}`, &bad)
	if bad.Failed != 2 {
		t.Errorf("应拒绝范围外和根目录: %+v", bad)
	}
}

func TestDownloadersAndTorrents(t *testing.T) {
	ts, root := newTestServer(t, nil)
	// 模拟 qBittorrent：b.mkv 在做种
	qb := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/auth/login":
			_, _ = w.Write([]byte("Ok."))
		case "/api/v2/torrents/info":
			_ = json.NewEncoder(w).Encode([]map[string]any{{"hash": "h", "name": "b", "save_path": "/downloads",
				"state": "uploading", "tracker": "https://tracker.hdsky.me/a"}})
		case "/api/v2/torrents/files":
			_ = json.NewEncoder(w).Encode([]map[string]any{{"name": "b.mkv"}})
		default:
			_ = json.NewEncoder(w).Encode([]any{})
		}
	}))
	defer qb.Close()
	body := `{"name":"qb","type":"qbittorrent","url":"` + qb.URL + `","enabled":true,` +
		`"mappings":[{"from":"/downloads","to":"` + root + `/Downloads"}]}`
	var n struct{ Torrents int }
	if c := postJSON(t, ts.URL+"/api/downloaders/test", body, &n); c != 200 || n.Torrents != 1 {
		t.Fatalf("测试连接: %d %+v", c, n)
	}
	if c := postJSON(t, ts.URL+"/api/downloaders", body, nil); c != 200 {
		t.Fatalf("保存下载器: %d", c)
	}
	runScan(t, ts.URL, root) // 扫描结束后自动检测种子
	var res struct {
		Count   int `json:"count"`
		Summary struct {
			Torrents struct {
				Seeding int `json:"seeding"`
				Orphans int `json:"orphans"`
			} `json:"torrents"`
		} `json:"summary"`
	}
	getJSON(t, ts.URL+"/api/result?filter=seeding", &res)
	if res.Count != 1 || res.Summary.Torrents.Seeding != 1 || res.Summary.Torrents.Orphans != 1 {
		t.Errorf("种子检测结果: %+v", res)
	}
	getJSON(t, ts.URL+"/api/result?site="+url.QueryEscape("天空 HDSky"), &res)
	if res.Count != 1 {
		t.Errorf("按站点筛选: %d", res.Count)
	}
	var pv struct {
		Seeding struct {
			Files int      `json:"files"`
			Sites []string `json:"sites"`
		} `json:"seeding"`
	}
	postJSON(t, ts.URL+"/api/delete/preview", `{"paths":["`+root+`/Downloads/b.mkv"]}`, &pv)
	if pv.Seeding.Files != 1 || len(pv.Seeding.Sites) != 1 {
		t.Errorf("删除预览应提示做种: %+v", pv)
	}
	if c := postJSON(t, ts.URL+"/api/torrents/refresh", "", nil); c != 200 {
		t.Errorf("刷新种子: %d", c)
	}
}
