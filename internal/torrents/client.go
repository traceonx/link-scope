// Package torrents 连接下载器（qBittorrent / Transmission，可多个实例），读取种子、文件与 Tracker 状态。
package torrents

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"
	"time"
)

// Downloader 是一个下载器的配置。
type Downloader struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Type     string    `json:"type"` // qbittorrent | transmission
	URL      string    `json:"url"`
	Username string    `json:"username"`
	Password string    `json:"password,omitempty"`
	Mappings []Mapping `json:"mappings"`
	Enabled  bool      `json:"enabled"`
}

// Mapping 把下载器里的路径映射为本工具看到的路径。
type Mapping struct {
	From string `json:"from"` // 下载器中的路径，如 /downloads
	To   string `json:"to"`   // 本工具中的路径，如 /vol1/1000/Downloads
}

// Torrent 是从下载器读到的一个种子。
type Torrent struct {
	Hash         string
	Name         string
	SaveDir      string   // 已做路径映射
	Files        []string // 完整路径（已做路径映射）
	Size         int64
	State        string // seeding | downloading | paused | checking | queued | error
	Trackers     []string
	Message      string // Tracker 返回的错误信息
	Unregistered bool
}

type client interface {
	torrents(ctx context.Context) ([]Torrent, error)
	// remove 从下载器中移除种子，不删除文件（文件已由本工具删除或移入回收站）。须在 torrents 之后调用。
	remove(ctx context.Context, hashes []string) error
}

var httpClient = &http.Client{Timeout: 60 * time.Second}

func (d *Downloader) mapPath(p string) string {
	p = path.Clean(strings.ReplaceAll(p, "\\", "/"))
	best := -1
	out := p
	for _, m := range d.Mappings {
		from := strings.TrimRight(path.Clean(m.From), "/")
		if from == "" || m.To == "" {
			continue
		}
		if (p == from || strings.HasPrefix(p, from+"/")) && len(from) > best {
			best = len(from)
			out = path.Join(m.To, strings.TrimPrefix(p, from))
		}
	}
	return out
}

// pathExists 判断本工具（容器内）能否看到该路径；测试中可替换。
var pathExists = func(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// fixMappings 纠正填反的映射：若「下载器路径」在本工具中存在、而「本工具路径」不存在，
// 说明两边填反了（常见于 /vol2/1000/xxx → /xxx），自动交换。返回新切片，不修改原配置。
func fixMappings(ms []Mapping) []Mapping {
	out := make([]Mapping, len(ms))
	for i, m := range ms {
		if m.From != "" && m.To != "" && !pathExists(m.To) && pathExists(m.From) {
			m.From, m.To = m.To, m.From
		}
		out[i] = m
	}
	return out
}

func newClient(d *Downloader) (client, error) {
	d.Mappings = fixMappings(d.Mappings)
	switch d.Type {
	case "qbittorrent":
		jar, _ := cookiejar.New(nil)
		return &qbit{d: d, http: &http.Client{Timeout: 60 * time.Second, Jar: jar}}, nil
	case "transmission":
		return &transmission{d: d}, nil
	}
	return nil, fmt.Errorf("不支持的下载器类型: %s", d.Type)
}

// ---- qBittorrent（WebUI API v2） ----

type qbit struct {
	d    *Downloader
	http *http.Client
}

func (q *qbit) base() string { return strings.TrimRight(q.d.URL, "/") }

func (q *qbit) login(ctx context.Context) error {
	form := url.Values{"username": {q.d.Username}, "password": {q.d.Password}}
	req, _ := http.NewRequestWithContext(ctx, "POST", q.base()+"/api/v2/auth/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", q.base())
	resp, err := q.http.Do(req)
	if err != nil {
		return fmt.Errorf("无法连接：%v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if resp.StatusCode == 403 {
		return errors.New("登录失败次数过多，IP 已被 qBittorrent 暂时封禁")
	}
	if resp.StatusCode != 200 || !strings.HasPrefix(strings.TrimSpace(string(b)), "Ok") {
		return errors.New("用户名或密码错误")
	}
	return nil
}

func (q *qbit) get(ctx context.Context, p string, v any) error {
	req, _ := http.NewRequestWithContext(ctx, "GET", q.base()+p, nil)
	req.Header.Set("Referer", q.base())
	resp, err := q.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("%s 返回 HTTP %d", p, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

func qbState(s string) string {
	switch s {
	case "uploading", "stalledUP", "forcedUP", "queuedUP":
		if s == "queuedUP" {
			return "queued"
		}
		return "seeding"
	case "pausedUP", "stoppedUP", "pausedDL", "stoppedDL":
		return "paused"
	case "checkingUP", "checkingDL", "checkingResumeData", "moving", "allocating":
		return "checking"
	case "queuedDL":
		return "queued"
	case "error", "missingFiles", "unknown":
		return "error"
	default: // downloading / stalledDL / metaDL / forcedDL / forcedMetaDL
		return "downloading"
	}
}

func (q *qbit) torrents(ctx context.Context) ([]Torrent, error) {
	if err := q.login(ctx); err != nil {
		return nil, err
	}
	var list []struct {
		Hash        string `json:"hash"`
		Name        string `json:"name"`
		SavePath    string `json:"save_path"`
		ContentPath string `json:"content_path"`
		State       string `json:"state"`
		Size        int64  `json:"size"`
		Tracker     string `json:"tracker"`
	}
	if err := q.get(ctx, "/api/v2/torrents/info", &list); err != nil {
		return nil, err
	}
	out := make([]Torrent, len(list))
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	var errMu sync.Mutex
	var firstErr error
	for i, t := range list {
		out[i] = Torrent{Hash: t.Hash, Name: t.Name, SaveDir: q.d.mapPath(t.SavePath), Size: t.Size, State: qbState(t.State)}
		if t.Tracker != "" {
			out[i].Trackers = []string{t.Tracker}
		}
		wg.Add(1)
		go func(i int, hash, save, tracker string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			var files []struct {
				Name string `json:"name"`
			}
			if err := q.get(ctx, "/api/v2/torrents/files?hash="+hash, &files); err != nil {
				errMu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				errMu.Unlock()
				return
			}
			for _, f := range files {
				out[i].Files = append(out[i].Files, q.d.mapPath(path.Join(save, f.Name)))
			}
			// tracker 为空表示没有正常工作的 Tracker，读取详情判断是否被站点删种
			var trs []struct {
				URL    string `json:"url"`
				Status int    `json:"status"`
				Msg    string `json:"msg"`
			}
			if err := q.get(ctx, "/api/v2/torrents/trackers?hash="+hash, &trs); err == nil {
				working := tracker != ""
				for _, tr := range trs {
					if strings.HasPrefix(tr.URL, "** [") { // DHT / PeX / LSD
						continue
					}
					if tracker == "" {
						out[i].Trackers = append(out[i].Trackers, tr.URL)
					}
					if tr.Msg != "" && (tr.Status == 4 || !working) {
						out[i].Message = tr.Msg
						if isUnregistered(tr.Msg) {
							out[i].Unregistered = true
						}
					}
				}
			}
		}(i, t.Hash, t.SavePath, t.Tracker)
	}
	wg.Wait()
	if firstErr != nil {
		return nil, fmt.Errorf("读取种子文件列表失败：%v", firstErr)
	}
	return out, nil
}

func (q *qbit) remove(ctx context.Context, hashes []string) error {
	form := url.Values{"hashes": {strings.Join(hashes, "|")}, "deleteFiles": {"false"}}
	req, _ := http.NewRequestWithContext(ctx, "POST", q.base()+"/api/v2/torrents/delete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", q.base())
	resp, err := q.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("删除种子返回 HTTP %d", resp.StatusCode)
	}
	return nil
}

// ---- Transmission（RPC） ----

type transmission struct {
	d       *Downloader
	session string
}

func (t *transmission) rpcURL() string {
	u := strings.TrimRight(t.d.URL, "/")
	if !strings.HasSuffix(u, "/rpc") {
		u += "/transmission/rpc"
	}
	return u
}

func (t *transmission) call(ctx context.Context, body any, out any) error {
	b, _ := json.Marshal(body)
	for attempt := 0; attempt < 2; attempt++ {
		req, _ := http.NewRequestWithContext(ctx, "POST", t.rpcURL(), bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		if t.session != "" {
			req.Header.Set("X-Transmission-Session-Id", t.session)
		}
		if t.d.Username != "" || t.d.Password != "" {
			req.SetBasicAuth(t.d.Username, t.d.Password)
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("无法连接：%v", err)
		}
		if resp.StatusCode == 409 {
			t.session = resp.Header.Get("X-Transmission-Session-Id")
			resp.Body.Close()
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode == 401 {
			return errors.New("用户名或密码错误")
		}
		if resp.StatusCode != 200 {
			return fmt.Errorf("RPC 返回 HTTP %d", resp.StatusCode)
		}
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return errors.New("无法获取 Transmission 会话")
}

func trState(status, errCode int) string {
	if errCode == 3 { // 本地错误（如文件丢失）
		return "error"
	}
	switch status {
	case 0:
		return "paused"
	case 1, 2:
		return "checking"
	case 3, 5:
		return "queued"
	case 4:
		return "downloading"
	default:
		return "seeding"
	}
}

func (t *transmission) torrents(ctx context.Context) ([]Torrent, error) {
	var resp struct {
		Result    string `json:"result"`
		Arguments struct {
			Torrents []struct {
				HashString   string                  `json:"hashString"`
				Name         string                  `json:"name"`
				DownloadDir  string                  `json:"downloadDir"`
				Status       int                     `json:"status"`
				TotalSize    int64                   `json:"totalSize"`
				Error        int                     `json:"error"`
				ErrorString  string                  `json:"errorString"`
				Files        []struct{ Name string } `json:"files"`
				TrackerStats []struct {
					Announce              string `json:"announce"`
					LastAnnounceResult    string `json:"lastAnnounceResult"`
					LastAnnounceSucceeded bool   `json:"lastAnnounceSucceeded"`
					HasAnnounced          bool   `json:"hasAnnounced"`
				} `json:"trackerStats"`
			} `json:"torrents"`
		} `json:"arguments"`
	}
	err := t.call(ctx, map[string]any{
		"method": "torrent-get",
		"arguments": map[string]any{"fields": []string{
			"hashString", "name", "downloadDir", "status", "totalSize", "error", "errorString", "files", "trackerStats",
		}},
	}, &resp)
	if err != nil {
		return nil, err
	}
	if resp.Result != "success" {
		return nil, fmt.Errorf("RPC 错误：%s", resp.Result)
	}
	out := make([]Torrent, 0, len(resp.Arguments.Torrents))
	for _, x := range resp.Arguments.Torrents {
		tr := Torrent{Hash: x.HashString, Name: x.Name, SaveDir: t.d.mapPath(x.DownloadDir), Size: x.TotalSize,
			State: trState(x.Status, x.Error)}
		for _, f := range x.Files {
			tr.Files = append(tr.Files, t.d.mapPath(path.Join(x.DownloadDir, f.Name)))
		}
		for _, ts := range x.TrackerStats {
			tr.Trackers = append(tr.Trackers, ts.Announce)
			if ts.HasAnnounced && !ts.LastAnnounceSucceeded && ts.LastAnnounceResult != "" {
				tr.Message = ts.LastAnnounceResult
			}
		}
		if x.Error == 2 && x.ErrorString != "" { // Tracker 错误
			tr.Message = x.ErrorString
		}
		tr.Unregistered = isUnregistered(tr.Message)
		out = append(out, tr)
	}
	return out, nil
}

func (t *transmission) remove(ctx context.Context, hashes []string) error {
	var resp struct {
		Result string `json:"result"`
	}
	err := t.call(ctx, map[string]any{
		"method":    "torrent-remove",
		"arguments": map[string]any{"ids": hashes, "delete-local-data": false},
	}, &resp)
	if err != nil {
		return err
	}
	if resp.Result != "success" {
		return fmt.Errorf("RPC 错误：%s", resp.Result)
	}
	return nil
}

var unregisteredWords = []string{
	"unregistered", "not registered", "torrent not found", "torrent does not exist", "not exist", "infohash not found",
	"unknown torrent", "torrent has been deleted", "deleted", "trumped",
	"未注册", "不存在", "已删除", "被删除", "已被移除", "种子已失效",
}

func isUnregistered(msg string) bool {
	m := strings.ToLower(msg)
	if m == "" {
		return false
	}
	for _, w := range unregisteredWords {
		if strings.Contains(m, w) {
			return true
		}
	}
	return false
}
