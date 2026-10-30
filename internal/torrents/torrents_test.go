package torrents

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// qbDeleted 记录模拟 qBittorrent 收到的删除请求
var qbDeleted []string

func fakeQbit(t *testing.T) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v2/auth/login", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Form.Get("username") != "admin" || r.Form.Get("password") != "pw" {
			_, _ = w.Write([]byte("Fails."))
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "SID", Value: "s1", Path: "/"})
		_, _ = w.Write([]byte("Ok."))
	})
	auth := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if c, err := r.Cookie("SID"); err != nil || c.Value != "s1" {
				w.WriteHeader(403)
				return
			}
			h(w, r)
		}
	}
	mux.HandleFunc("/api/v2/torrents/info", auth(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"hash": "h1", "name": "Movie.2024", "save_path": "/downloads/movies", "state": "stalledUP", "size": 100,
				"tracker": "https://tracker.hdsky.me/announce.php?passkey=x"},
			{"hash": "h2", "name": "Show.S01", "save_path": "/downloads/tv", "state": "pausedUP", "size": 50, "tracker": ""},
		})
	}))
	mux.HandleFunc("/api/v2/torrents/files", auth(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("hash") {
		case "h1":
			_ = json.NewEncoder(w).Encode([]map[string]any{{"name": "Movie.2024/movie.mkv"}})
		default:
			_ = json.NewEncoder(w).Encode([]map[string]any{{"name": "Show.S01/e01.mkv"}, {"name": "Show.S01/e02.mkv"}})
		}
	}))
	mux.HandleFunc("POST /api/v2/torrents/delete", auth(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Form.Get("deleteFiles") != "false" {
			w.WriteHeader(400)
			return
		}
		qbDeleted = append(qbDeleted, strings.Split(r.Form.Get("hashes"), "|")...)
	}))
	mux.HandleFunc("/api/v2/torrents/trackers", auth(func(w http.ResponseWriter, r *http.Request) {
		trs := []map[string]any{{"url": "** [DHT] **", "status": 2, "msg": ""}}
		if r.URL.Query().Get("hash") == "h2" {
			trs = append(trs, map[string]any{"url": "https://kp.m-team.cc/announce?x", "status": 4, "msg": "Torrent not registered with this tracker"})
		}
		_ = json.NewEncoder(w).Encode(trs)
	}))
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

// trRemoved 记录模拟 Transmission 收到的移除请求（仅不删数据的）
var trRemoved []string

func fakeTransmission(t *testing.T) *httptest.Server {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Transmission-Session-Id") != "sess" {
			w.Header().Set("X-Transmission-Session-Id", "sess")
			w.WriteHeader(409)
			return
		}
		if u, p, _ := r.BasicAuth(); u != "tr" || p != "pw" {
			w.WriteHeader(401)
			return
		}
		var body struct {
			Method    string `json:"method"`
			Arguments struct {
				IDs        []string `json:"ids"`
				DeleteData bool     `json:"delete-local-data"`
			} `json:"arguments"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Method == "torrent-remove" {
			if !body.Arguments.DeleteData {
				trRemoved = append(trRemoved, body.Arguments.IDs...)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"result": "success"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": "success",
			"arguments": map[string]any{"torrents": []map[string]any{{
				"hashString": "t1", "name": "Album", "downloadDir": "/data/music", "status": 6, "totalSize": 10, "error": 0,
				"files":        []map[string]any{{"name": "Album/01.flac"}},
				"trackerStats": []map[string]any{{"announce": "https://tracker.example-pt.org:443/announce", "hasAnnounced": true, "lastAnnounceSucceeded": true}},
			}}},
		})
	}))
	t.Cleanup(ts.Close)
	return ts
}

func TestQbittorrent(t *testing.T) {
	ts := fakeQbit(t)
	d := &Downloader{Name: "qb", Type: "qbittorrent", URL: ts.URL, Username: "admin", Password: "pw",
		Mappings: []Mapping{{From: "/downloads", To: "/vol1/1000/Downloads"}}}
	c, _ := newClient(d)
	list, err := c.torrents(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("种子数 %d", len(list))
	}
	m := list[0]
	if m.State != "seeding" || m.Files[0] != "/vol1/1000/Downloads/movies/Movie.2024/movie.mkv" || siteOf(m.Trackers) != "天空 HDSky" {
		t.Errorf("种子 1 解析错误: %+v site=%s", m, siteOf(m.Trackers))
	}
	s := list[1]
	if s.State != "paused" || !s.Unregistered || len(s.Files) != 2 || siteOf(s.Trackers) != "馒头 M-Team" {
		t.Errorf("种子 2 解析错误: %+v site=%s", s, siteOf(s.Trackers))
	}

	d.Password = "wrong"
	c, _ = newClient(d)
	if _, err := c.torrents(context.Background()); err == nil || !strings.Contains(err.Error(), "密码") {
		t.Errorf("密码错误应报错: %v", err)
	}
}

func TestTransmission(t *testing.T) {
	ts := fakeTransmission(t)
	d := &Downloader{Name: "tr", Type: "transmission", URL: ts.URL, Username: "tr", Password: "pw",
		Mappings: []Mapping{{From: "/data", To: "/vol2/media"}}}
	c, _ := newClient(d)
	list, err := c.torrents(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].State != "seeding" || list[0].Files[0] != "/vol2/media/music/Album/01.flac" {
		t.Errorf("解析错误: %+v", list)
	}
	if site := siteOf(list[0].Trackers); site != "example-pt.org" {
		t.Errorf("未收录站点应显示主域名: %s", site)
	}
}

func TestStoreFetch(t *testing.T) {
	qb, tr := fakeQbit(t), fakeTransmission(t)
	s := NewStore(t.TempDir())
	if _, err := s.Save(Downloader{Name: "坏地址", Type: "qbittorrent", URL: "ftp://x"}); err == nil {
		t.Error("非法地址应报错")
	}
	d1, err := s.Save(Downloader{Name: "qb", Type: "qbittorrent", URL: qb.URL, Username: "admin", Password: "pw", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Save(Downloader{Name: "tr", Type: "transmission", URL: tr.URL, Username: "tr", Password: "pw", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if d1.Password != "" || s.List()[0].Password != "" {
		t.Error("列表不应返回密码")
	}
	// 修改时密码留空表示不变
	d1.Name = "qb-主"
	if _, err := s.Save(d1); err != nil {
		t.Fatal(err)
	}
	if n, err := s.Test(context.Background(), d1); err != nil || n != 2 {
		t.Errorf("测试连接（沿用已保存密码）失败: %d %v", n, err)
	}
	snap := s.Fetch(context.Background())
	if len(snap.Clients) != 2 || snap.Clients[0].Error != "" || snap.Clients[1].Error != "" {
		t.Fatalf("读取失败: %+v", snap)
	}
	if NewStore(s.file[:len(s.file)-len("/downloaders.json")]).List()[0].Name != "qb-主" {
		t.Error("配置未持久化")
	}
}

func TestHelpers(t *testing.T) {
	d := &Downloader{Mappings: []Mapping{{From: "/downloads", To: "/vol1/dl"}, {From: "/downloads/tv", To: "/vol2/tv"}}}
	if p := d.mapPath("/downloads/tv/a.mkv"); p != "/vol2/tv/a.mkv" {
		t.Errorf("应使用最长匹配: %s", p)
	}
	if p := d.mapPath("/downloadsX/a"); p != "/downloadsX/a" {
		t.Errorf("前缀不应误匹配: %s", p)
	}
	for msg, want := range map[string]bool{"Unregistered torrent": true, "种子不存在": true, "": false, "Timed out": false} {
		if isUnregistered(msg) != want {
			t.Errorf("isUnregistered(%q)", msg)
		}
	}
	if s := siteOf([]string{"udp://tracker.opentrackr.org:1337/announce"}); s != "公共 Tracker" {
		t.Errorf("公共 tracker: %s", s)
	}
	if s := mainDomain("pt.nwsuaf6.edu.cn"); s != "nwsuaf6.edu.cn" {
		t.Errorf("mainDomain: %s", s)
	}
}

func TestFixReversedMapping(t *testing.T) {
	orig := pathExists
	defer func() { pathExists = orig }()
	pathExists = func(p string) bool { return strings.HasPrefix(p, "/vol2") }
	d := Downloader{Mappings: []Mapping{{From: "/vol2/1000/video", To: "/video"}, {From: "/downloads", To: "/vol2/1000/dl"}}}
	ms := fixMappings(d.Mappings)
	d2 := Downloader{Mappings: ms}
	if got := d2.mapPath("/video/download/a.mkv"); got != "/vol2/1000/video/download/a.mkv" {
		t.Errorf("填反的映射未纠正：%s", got)
	}
	if got := d2.mapPath("/downloads/b.mkv"); got != "/vol2/1000/dl/b.mkv" {
		t.Errorf("正确的映射被改动：%s", got)
	}
	if d.Mappings[0].From != "/vol2/1000/video" {
		t.Error("不应修改原配置")
	}
}

func TestRemoveByPaths(t *testing.T) {
	qb, tr := fakeQbit(t), fakeTransmission(t)
	qbDeleted, trRemoved = nil, nil
	// 未被删除的文件都视为仍存在
	old := pathExists
	pathExists = func(string) bool { return true }
	t.Cleanup(func() { pathExists = old })

	s := &Store{list: []Downloader{
		{Name: "qb", Type: "qbittorrent", URL: qb.URL, Username: "admin", Password: "pw", Enabled: true,
			Mappings: []Mapping{{From: "/downloads", To: "/vol1/dl"}}},
		{Name: "tr", Type: "transmission", URL: tr.URL, Username: "tr", Password: "pw", Enabled: true,
			Mappings: []Mapping{{From: "/data", To: "/vol2/media"}}},
	}}
	// 删除电影整个文件夹（h1 全部文件）+ 剧集的一集（h2 只删一部分），专辑未动
	res := s.RemoveByPaths(context.Background(), []string{"/vol1/dl/movies/Movie.2024", "/vol1/dl/tv/Show.S01/e01.mkv"})
	if len(res.Errors) != 0 {
		t.Fatalf("不应出错: %v", res.Errors)
	}
	if len(res.Removed) != 1 || res.Removed[0].Name != "Movie.2024" || res.Partial != 1 {
		t.Errorf("结果错误: %+v", res)
	}
	if len(qbDeleted) != 1 || qbDeleted[0] != "h1" || len(trRemoved) != 0 {
		t.Errorf("下载器收到的删除请求错误: qb=%v tr=%v", qbDeleted, trRemoved)
	}

	// 剩下的一集也删除后（e01 已不存在），h2 被移除；Transmission 的专辑整体删除
	pathExists = func(p string) bool { return p != "/vol1/dl/tv/Show.S01/e01.mkv" }
	qbDeleted = nil
	res = s.RemoveByPaths(context.Background(), []string{"/vol1/dl/tv/Show.S01/e02.mkv", "/vol2/media/music/Album"})
	if len(res.Removed) != 2 || res.Partial != 0 || len(qbDeleted) != 1 || qbDeleted[0] != "h2" || len(trRemoved) != 1 || trRemoved[0] != "t1" {
		t.Errorf("第二次结果错误: %+v qb=%v tr=%v", res, qbDeleted, trRemoved)
	}
}
