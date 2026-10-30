// Package server 提供 HTTP 接口和前端页面。
package server

import (
	"crypto/subtle"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/traceonx/link-scope/internal/config"
	"github.com/traceonx/link-scope/internal/fileops"
	"github.com/traceonx/link-scope/internal/jobs"
	"github.com/traceonx/link-scope/internal/logs"
	"github.com/traceonx/link-scope/internal/scanner"
	"github.com/traceonx/link-scope/internal/torrents"
	"github.com/traceonx/link-scope/internal/update"
	"github.com/traceonx/link-scope/internal/version"
)

type Server struct {
	cfg     *config.Config
	jobs    *jobs.Manager
	updater *update.Manager
	ops     *fileops.Manager
	dl      *torrents.Store
	web     fs.FS
}

func New(cfg *config.Config, jm *jobs.Manager, um *update.Manager, ops *fileops.Manager, dl *torrents.Store, web fs.FS) *Server {
	return &Server{cfg: cfg, jobs: jm, updater: um, ops: ops, dl: dl, web: web}
}

// Handler 返回完整的 HTTP 处理器。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("GET /api/config", s.getConfig)
	mux.HandleFunc("GET /api/update", s.updateStatus)
	mux.HandleFunc("POST /api/update", s.startUpdate)
	mux.HandleFunc("PUT /api/update/auto", s.setAutoUpdate)
	mux.HandleFunc("GET /api/browse", s.browse)
	mux.HandleFunc("POST /api/scan", s.startScan)
	mux.HandleFunc("POST /api/scan/cancel", s.cancelScan)
	mux.HandleFunc("GET /api/status", s.status)
	mux.HandleFunc("GET /api/result", s.result)
	mux.HandleFunc("GET /api/export.csv", s.exportCSV)
	mux.HandleFunc("GET /api/folders", s.folders)
	mux.HandleFunc("GET /api/folders.csv", s.foldersCSV)
	mux.HandleFunc("POST /api/delete/preview", s.deletePreview)
	mux.HandleFunc("POST /api/delete", s.deletePaths)
	mux.HandleFunc("GET /api/trash", s.trashList)
	mux.HandleFunc("POST /api/trash/restore", s.trashRestore)
	mux.HandleFunc("POST /api/trash/purge", s.trashPurge)
	mux.HandleFunc("GET /api/downloaders", s.downloaderList)
	mux.HandleFunc("POST /api/downloaders", s.downloaderSave)
	mux.HandleFunc("POST /api/downloaders/test", s.downloaderTest)
	mux.HandleFunc("DELETE /api/downloaders/{id}", s.downloaderDelete)
	mux.HandleFunc("POST /api/torrents/refresh", s.torrentsRefresh)
	mux.HandleFunc("GET /api/logs", s.logList)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) { fail(w, 404, "接口不存在") })
	mux.Handle("/", s.spa())
	return s.auth(mux)
}

// ---- 工具函数 ----

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"detail": msg})
}

type httpError struct {
	code int
	msg  string
}

func (e *httpError) Error() string { return e.msg }

// checkPath 规范化路径，并确保它位于允许的根目录之内（解析软链接后判断）。
func (s *Server) checkPath(p string) (string, error) {
	if !strings.HasPrefix(p, "/") {
		return "", &httpError{400, "请填写以 / 开头的完整路径"}
	}
	clean := filepath.Clean(p)
	real, err := filepath.EvalSymlinks(clean)
	if err != nil {
		if os.IsNotExist(err) {
			return "", &httpError{404, "文件夹不存在: " + p}
		}
		return "", &httpError{400, err.Error()}
	}
	if len(s.cfg.Roots) > 0 {
		ok := false
		for _, r := range s.cfg.Roots {
			if real == r || strings.HasPrefix(real, strings.TrimRight(r, "/")+"/") {
				ok = true
				break
			}
		}
		if !ok {
			return "", &httpError{403, "路径不在允许范围内（LS_ROOTS）: " + p}
		}
	}
	return real, nil
}

func sendErr(w http.ResponseWriter, err error) {
	var he *httpError
	if errors.As(err, &he) {
		fail(w, he.code, he.msg)
		return
	}
	fail(w, 500, err.Error())
}

// ---- 接口 ----

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	v := version.Get()
	writeJSON(w, 200, map[string]any{"status": "ok", "version": v.Version, "commit": v.Commit, "build_time": v.BuildTime})
}

func (s *Server) updateStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.updater.Status(r.Context()))
}

func (s *Server) startUpdate(w http.ResponseWriter, r *http.Request) {
	st, err := s.updater.Start(r.Context())
	if errors.Is(err, update.ErrBusy) {
		fail(w, 409, err.Error())
		return
	}
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, st)
}

func (s *Server) setAutoUpdate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled bool   `json:"enabled"`
		Time    string `json:"time"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		fail(w, 400, "请求格式错误")
		return
	}
	a, err := s.updater.SetAuto(req.Enabled, req.Time)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, a)
}

func (s *Server) getConfig(w http.ResponseWriter, _ *http.Request) {
	canDel, delReason := s.ops.Capability()
	writeJSON(w, 200, map[string]any{
		"version":         version.Get(),
		"can_delete":      canDel,
		"delete_reason":   delReason,
		"trash_days":      s.ops.TrashDays(),
		"has_downloaders": s.dl.HasAny(),
		"roots":           nonNil(s.cfg.Roots),
		"default_source":  s.cfg.DefaultSource,
		"default_targets": nonNil(s.cfg.DefaultTargets),
	})
}

func nonNil(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

type dirEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func (s *Server) browse(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Query().Get("path")
	if p == "" {
		roots := s.cfg.Roots
		if len(roots) == 0 {
			roots = []string{"/"}
		}
		dirs := []dirEntry{}
		for _, root := range roots {
			if fi, err := os.Stat(root); err == nil && fi.IsDir() {
				dirs = append(dirs, dirEntry{root, root})
			}
		}
		writeJSON(w, 200, map[string]any{"path": "", "parent": nil, "dirs": dirs})
		return
	}
	real, err := s.checkPath(p)
	if err != nil {
		sendErr(w, err)
		return
	}
	des, err := os.ReadDir(real)
	if err != nil {
		if os.IsPermission(err) {
			fail(w, 403, "没有权限读取该文件夹")
			return
		}
		fail(w, 404, "文件夹不存在")
		return
	}
	dirs := []dirEntry{}
	for _, de := range des {
		if de.IsDir() && !strings.HasPrefix(de.Name(), ".") && !s.cfg.SkipDirs[de.Name()] {
			dirs = append(dirs, dirEntry{de.Name(), filepath.Join(real, de.Name())})
		}
	}
	sort.Slice(dirs, func(i, j int) bool { return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name) })
	if len(dirs) > 2000 {
		dirs = dirs[:2000]
	}
	var parent any = filepath.Dir(real)
	isRoot := real == "/"
	for _, root := range s.cfg.Roots {
		isRoot = isRoot || real == root
	}
	if isRoot {
		parent = nil
		if len(s.cfg.Roots) > 0 {
			parent = ""
		}
	}
	writeJSON(w, 200, map[string]any{"path": real, "parent": parent, "dirs": dirs})
}

type scanRequest struct {
	Source    string   `json:"source"`
	Targets   []string `json:"targets"`
	Recursive *bool    `json:"recursive"`
	MinSizeMB float64  `json:"min_size_mb"`
	DupMode   string   `json:"dup_mode"`
}

func (s *Server) startScan(w http.ResponseWriter, r *http.Request) {
	var req scanRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		fail(w, 400, "请求格式错误")
		return
	}
	src, err := s.checkPath(strings.TrimSpace(req.Source))
	if err != nil {
		sendErr(w, err)
		return
	}
	if fi, err := os.Stat(src); err != nil || !fi.IsDir() {
		fail(w, 404, "文件夹不存在: "+req.Source)
		return
	}
	var targets []string
	for _, t := range req.Targets {
		if t = strings.TrimSpace(t); t == "" {
			continue
		}
		real, err := s.checkPath(t)
		if err != nil {
			sendErr(w, err)
			return
		}
		targets = append(targets, real)
	}
	mode := scanner.DupMode(req.DupMode)
	switch mode {
	case "":
		mode = scanner.DupQuick
	case scanner.DupOff, scanner.DupQuick, scanner.DupFull:
	default:
		fail(w, 400, "dup_mode 只能是 off / quick / full")
		return
	}
	recursive := req.Recursive == nil || *req.Recursive
	if req.MinSizeMB < 0 {
		req.MinSizeMB = 0
	}
	err = s.jobs.Start(scanner.Options{
		Source: src, Targets: targets, Recursive: recursive, MinSize: int64(req.MinSizeMB * 1024 * 1024),
		DupMode: mode, Workers: s.cfg.Workers, HashWorkers: s.cfg.HashWorkers, SkipDirs: s.cfg.SkipDirs,
	})
	if errors.Is(err, jobs.ErrBusy) {
		fail(w, 409, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (s *Server) cancelScan(w http.ResponseWriter, _ *http.Request) {
	s.jobs.Cancel()
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (s *Server) status(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{"state": s.jobs.State(), "summary": s.jobs.Summary()})
}

func validFilter(f string) bool {
	for _, x := range jobs.Filters {
		if f == x {
			return true
		}
	}
	return false
}

func (s *Server) result(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := q.Get("filter")
	if filter == "" {
		filter = "all"
	}
	if !validFilter(filter) {
		fail(w, 400, "未知的筛选项")
		return
	}
	offset, _ := strconv.Atoi(q.Get("offset"))
	limit, err := strconv.Atoi(q.Get("limit"))
	if err != nil || limit <= 0 || limit > 2000 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	count, items := s.jobs.Query(jobs.Q{Filter: filter, Site: q.Get("site"), Text: q.Get("q"), Dir: q.Get("dir"),
		Sort: q.Get("sort"), Order: q.Get("order"), Offset: offset, Limit: limit})
	writeJSON(w, 200, map[string]any{
		"summary": s.jobs.Summary(), "counts": s.jobs.Counts(), "count": count, "items": items,
	})
}

var typeLabel = map[string]string{"file": "文件", "symlink": "软链接", "strm": "STRM"}

func statusLabel(it *scanner.Item) string {
	switch {
	case it.Type == "file":
		s := "未链接"
		if it.Linked {
			s = "已链接"
		}
		if len(it.Dups) > 0 {
			s += "·有重复副本"
		}
		return s
	case it.Broken:
		return "失效"
	case it.Remote:
		return "远程地址"
	default:
		return "有效"
	}
}

func (s *Server) exportCSV(w http.ResponseWriter, r *http.Request) {
	if s.jobs.Summary() == nil {
		fail(w, 404, "还没有扫描结果")
		return
	}
	filter := r.URL.Query().Get("filter")
	if filter == "" {
		filter = "all"
	}
	if !validFilter(filter) {
		fail(w, 400, "未知的筛选项")
		return
	}
	_, items := s.jobs.Query(jobs.Q{Filter: filter, Site: r.URL.Query().Get("site"), Text: r.URL.Query().Get("q"),
		Dir: r.URL.Query().Get("dir")})
	name := time.Now().Format("links_20060102_150405.csv")
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	_, _ = w.Write([]byte("\xEF\xBB\xBF")) // 让 Excel 正确识别 UTF-8
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"类型", "状态", "路径", "链接数", "大小(字节)", "修改时间", "硬链接位置", "重复副本", "指向", "站点", "种子状态"})
	for i := range items {
		it := &items[i]
		nlink, mtime := "", ""
		if it.Nlink > 0 {
			nlink = strconv.FormatUint(it.Nlink, 10)
		}
		if it.Mtime > 0 {
			mtime = time.Unix(it.Mtime, 0).Format("2006-01-02 15:04:05")
		}
		_ = cw.Write([]string{
			typeLabel[it.Type], statusLabel(it), it.Path, nlink, strconv.FormatInt(it.Size, 10), mtime,
			strings.Join(it.Links, " | "), strings.Join(it.Dups, " | "), it.Target, sitesOf(it), torrentStates(it),
		})
	}
	cw.Flush()
}

// ---- 前端与登录 ----

func (s *Server) spa() http.Handler {
	files := http.FileServer(http.FS(s.web))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if p == "" {
			p = "index.html"
		}
		if _, err := fs.Stat(s.web, p); err != nil {
			r = r.Clone(r.Context())
			r.URL.Path = "/"
		}
		if strings.HasPrefix(p, "assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		files.ServeHTTP(w, r)
	})
}

func (s *Server) auth(next http.Handler) http.Handler {
	if !s.cfg.AuthEnabled() {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/health" { // 供 Docker 健康检查使用
			next.ServeHTTP(w, r)
			return
		}
		u, p, ok := r.BasicAuth()
		if !ok || subtle.ConstantTimeCompare([]byte(u), []byte(s.cfg.AuthUser)) != 1 ||
			subtle.ConstantTimeCompare([]byte(p), []byte(s.cfg.AuthPass)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="LinkScope"`)
			http.Error(w, "需要登录", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

var stateLabel = map[string]string{"seeding": "做种中", "downloading": "下载中", "paused": "已暂停", "checking": "校验中",
	"queued": "排队中", "error": "出错"}

func sitesOf(it *scanner.Item) string {
	var out []string
	seen := map[string]bool{}
	for _, r := range it.Torrents {
		if !seen[r.Site] {
			seen[r.Site] = true
			out = append(out, r.Site)
		}
	}
	if len(out) == 0 && hasTagStr(it.Tags, "orphan") {
		return "孤儿文件"
	}
	return strings.Join(out, " | ")
}

func torrentStates(it *scanner.Item) string {
	var out []string
	for _, r := range it.Torrents {
		l := stateLabel[r.State]
		if r.Unregistered {
			l = "站点已删种"
		}
		out = append(out, l)
	}
	return strings.Join(out, " | ")
}

func hasTagStr(tags []string, t string) bool {
	for _, x := range tags {
		if x == t {
			return true
		}
	}
	return false
}

// ---- 删除与回收站 ----

type deleteRequest struct {
	Paths     []string `json:"paths"`
	Filter    string   `json:"filter"` // 与 paths 二选一：删除当前筛选的全部结果
	Site      string   `json:"site"`
	Q         string   `json:"q"`
	Dir       string   `json:"dir"`
	View      string   `json:"view"` // folders：按文件夹视图的筛选结果删除整个文件夹
	Permanent bool     `json:"permanent"`
	// RemoveTorrents 删除成功后，同时从下载器移除文件已全部删除的种子
	RemoveTorrents bool `json:"remove_torrents"`
}

func (s *Server) deleteTargets(w http.ResponseWriter, r *http.Request) ([]string, *deleteRequest, bool) {
	var req deleteRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<20)).Decode(&req); err != nil {
		fail(w, 400, "请求格式错误")
		return nil, nil, false
	}
	paths := req.Paths
	if len(paths) == 0 && req.Filter != "" {
		if req.View != "folders" && !validFilter(req.Filter) {
			fail(w, 400, "未知的筛选项")
			return nil, nil, false
		}
		if req.View == "folders" {
			if !validFolderFilter(req.Filter) {
				fail(w, 400, "未知的筛选项")
				return nil, nil, false
			}
			_, folders := s.jobs.Folders(jobs.Q{Filter: req.Filter, Site: req.Site, Text: req.Q})
			for _, f := range folders {
				paths = append(paths, f.Path)
			}
		} else {
			_, items := s.jobs.Query(jobs.Q{Filter: req.Filter, Site: req.Site, Text: req.Q, Dir: req.Dir})
			for _, it := range items {
				paths = append(paths, it.Path)
			}
		}
	}
	if len(paths) == 0 {
		fail(w, 400, "没有要删除的内容")
		return nil, nil, false
	}
	return paths, &req, true
}

type seedingInfo struct {
	Files int      `json:"files"`
	Sites []string `json:"sites"`
}

func (s *Server) seeding(paths []string) seedingInfo {
	var info seedingInfo
	seen := map[string]bool{}
	for _, it := range s.jobs.ItemsByPath(paths) {
		active := false
		for _, t := range it.Torrents {
			if t.State == "seeding" || t.State == "downloading" || t.State == "queued" || t.State == "checking" {
				active = true
				if !seen[t.Site] {
					seen[t.Site] = true
					info.Sites = append(info.Sites, t.Site)
				}
			}
		}
		if active {
			info.Files++
		}
	}
	if info.Sites == nil {
		info.Sites = []string{}
	}
	return info
}

// torrentCount 返回待删除内容涉及的种子数（按上次扫描的标注，仅用于预览提示）。
func (s *Server) torrentCount(paths []string) int {
	seen := map[string]bool{}
	for _, it := range s.jobs.ItemsByPath(paths) {
		for _, t := range it.Torrents {
			seen[t.Client+"/"+t.Hash] = true
		}
	}
	return len(seen)
}

func (s *Server) deletePreview(w http.ResponseWriter, r *http.Request) {
	paths, _, ok := s.deleteTargets(w, r)
	if !ok {
		return
	}
	pv, err := s.ops.Preview(paths)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"preview": pv, "seeding": s.seeding(paths), "torrents": s.torrentCount(paths),
		"trash_days": s.ops.TrashDays()})
}

func (s *Server) deletePaths(w http.ResponseWriter, r *http.Request) {
	if s.jobs.Running() {
		fail(w, 409, "扫描进行中，请等扫描结束后再删除")
		return
	}
	paths, req, ok := s.deleteTargets(w, r)
	if !ok {
		return
	}
	results, done, err := s.ops.Delete(paths, req.Permanent)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	s.jobs.RemovePaths(done)
	var removed *torrents.RemoveResult
	if req.RemoveTorrents && len(done) > 0 && s.dl.HasAny() {
		res := s.dl.RemoveByPaths(r.Context(), done)
		removed = &res
	}
	failed := 0
	var errs []fileops.Result
	for _, x := range results {
		if !x.OK {
			failed++
			if len(errs) < 50 {
				errs = append(errs, x)
			}
		}
	}
	if errs == nil {
		errs = []fileops.Result{}
	}
	writeJSON(w, 200, map[string]any{"deleted": len(done), "failed": failed, "errors": errs, "torrents": removed})
}

func (s *Server) trashList(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{"items": s.ops.Trash(), "days": s.ops.TrashDays()})
}

type idsRequest struct {
	IDs []string `json:"ids"`
	All bool     `json:"all"`
}

func (s *Server) trashRestore(w http.ResponseWriter, r *http.Request) {
	var req idsRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil || len(req.IDs) == 0 {
		fail(w, 400, "请求格式错误")
		return
	}
	results, restored := s.ops.Restore(req.IDs)
	if len(restored) > 0 {
		s.jobs.MarkStale()
	}
	writeJSON(w, 200, map[string]any{"results": results, "restored": len(restored)})
}

func (s *Server) trashPurge(w http.ResponseWriter, r *http.Request) {
	var req idsRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil || (!req.All && len(req.IDs) == 0) {
		fail(w, 400, "请求格式错误")
		return
	}
	writeJSON(w, 200, map[string]any{"results": s.ops.Purge(req.IDs, req.All)})
}

// ---- 下载器与种子 ----

func (s *Server) downloaderList(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, s.dl.List())
}

func (s *Server) downloaderSave(w http.ResponseWriter, r *http.Request) {
	var d torrents.Downloader
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&d); err != nil {
		fail(w, 400, "请求格式错误")
		return
	}
	saved, err := s.dl.Save(d)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	log.Printf("[下载器] 已保存「%s」（%s，%s）", saved.Name, saved.Type, map[bool]string{true: "启用", false: "停用"}[saved.Enabled])
	writeJSON(w, 200, saved)
}

func (s *Server) downloaderTest(w http.ResponseWriter, r *http.Request) {
	var d torrents.Downloader
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&d); err != nil {
		fail(w, 400, "请求格式错误")
		return
	}
	n, err := s.dl.Test(r.Context(), d)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, map[string]int{"torrents": n})
}

func (s *Server) downloaderDelete(w http.ResponseWriter, r *http.Request) {
	if err := s.dl.Delete(r.PathValue("id")); err != nil {
		fail(w, 404, err.Error())
		return
	}
	log.Printf("[下载器] 已删除一个下载器配置")
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (s *Server) torrentsRefresh(w http.ResponseWriter, r *http.Request) {
	ts, err := s.jobs.RefreshTorrents(r.Context())
	if errors.Is(err, jobs.ErrBusy) {
		fail(w, 409, "扫描进行中，结束后会自动检测种子")
		return
	}
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, ts)
}

// ---- 文件夹视图 ----

func validFolderFilter(f string) bool {
	for _, x := range jobs.FolderFilters {
		if f == x {
			return true
		}
	}
	return false
}

func (s *Server) folders(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := q.Get("filter")
	if filter == "" {
		filter = "all"
	}
	if !validFolderFilter(filter) {
		fail(w, 400, "未知的筛选项")
		return
	}
	offset, _ := strconv.Atoi(q.Get("offset"))
	limit, err := strconv.Atoi(q.Get("limit"))
	if err != nil || limit <= 0 || limit > 2000 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	counts, list := s.jobs.Folders(jobs.Q{Filter: filter, Site: q.Get("site"), Text: q.Get("q"),
		Sort: q.Get("sort"), Order: q.Get("order")})
	total := len(list)
	if offset > total {
		offset = total
	}
	end := min(total, offset+limit)
	writeJSON(w, 200, map[string]any{
		"summary": s.jobs.Summary(), "counts": counts, "totals": s.jobs.FolderTotals(), "count": total,
		"items": list[offset:end],
	})
}

var folderStatusLabel = map[string]string{"linked": "全部链接", "partial": "部分链接", "unlinked": "未链接"}

func (s *Server) foldersCSV(w http.ResponseWriter, r *http.Request) {
	if s.jobs.Summary() == nil {
		fail(w, 404, "还没有扫描结果")
		return
	}
	q := r.URL.Query()
	filter := q.Get("filter")
	if filter == "" {
		filter = "all"
	}
	if !validFolderFilter(filter) {
		fail(w, 400, "未知的筛选项")
		return
	}
	_, list := s.jobs.Folders(jobs.Q{Filter: filter, Site: q.Get("site"), Text: q.Get("q")})
	name := time.Now().Format("folders_20060102_150405.csv")
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	_, _ = w.Write([]byte("\xEF\xBB\xBF"))
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"状态", "文件夹", "文件数", "已链接", "未链接", "大小(字节)", "重复副本", "站点", "做种中", "已删种", "孤儿文件"})
	for _, f := range list {
		_ = cw.Write([]string{
			folderStatusLabel[f.Status], f.Path, strconv.Itoa(f.Files), strconv.Itoa(f.Linked), strconv.Itoa(f.Unlinked),
			strconv.FormatInt(f.Size, 10), strconv.Itoa(f.Dups), strings.Join(f.Sites, " | "),
			strconv.Itoa(f.Seeding), strconv.Itoa(f.Unregistered), strconv.Itoa(f.Orphans),
		})
	}
	cw.Flush()
}

// logList 返回内存中的最近日志；after 为上次拿到的最大 ID，用于增量轮询。
func (s *Server) logList(w http.ResponseWriter, r *http.Request) {
	after, _ := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
	items, last := logs.Default.Since(after, logs.Capacity)
	writeJSON(w, 200, map[string]any{"items": items, "last": last})
}
