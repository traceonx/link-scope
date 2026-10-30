package jobs

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/traceonx/link-scope/internal/scanner"
	"github.com/traceonx/link-scope/internal/torrents"
)

var torrentTags = map[string]bool{"torrent": true, "seeding": true, "unregistered": true, "orphan": true}

func stripTags(tags []string, drop map[string]bool) []string {
	out := tags[:0]
	for _, t := range tags {
		if !drop[t] {
			out = append(out, t)
		}
	}
	return out
}

func under(p string, dirs []string) bool {
	for _, d := range dirs {
		if p == d || strings.HasPrefix(p, strings.TrimRight(d, "/")+"/") {
			return true
		}
	}
	return false
}

// annotate 把下载器中的种子信息写入扫描结果（会先清除旧的种子标注）。
func annotate(r *scanner.Result, snap *torrents.Snapshot) {
	byPath := map[string][]scanner.TorrentRef{}
	dirSet := map[string]bool{}
	ts := &scanner.TorrentSummary{FetchedAt: snap.FetchedAt, Clients: []scanner.ClientStat{}, Sites: []scanner.SiteStat{}}
	clientIdx := map[string]int{}
	for _, c := range snap.Clients {
		cs := scanner.ClientStat{Name: c.Name, OK: c.Error == "", Error: c.Error, Torrents: len(c.Torrents)}
		for _, t := range c.Torrents {
			if len(t.Files) > 0 {
				cs.Sample = t.Files[0]
				break
			}
		}
		clientIdx[c.Name] = len(ts.Clients)
		ts.Clients = append(ts.Clients, cs)
		for i, t := range c.Torrents {
			ref := scanner.TorrentRef{Client: c.Name, Hash: t.Hash, Name: t.Name, Site: c.Sites[i], State: t.State,
				Unregistered: t.Unregistered, Message: t.Message}
			for _, f := range t.Files {
				byPath[filepath.Clean(f)] = append(byPath[filepath.Clean(f)], ref)
			}
			if t.SaveDir != "" {
				dirSet[filepath.Clean(t.SaveDir)] = true
			}
		}
	}
	saveDirs := make([]string, 0, len(dirSet))
	for d := range dirSet {
		saveDirs = append(saveDirs, d)
	}

	sites := map[string]*scanner.SiteStat{}
	for i := range r.Items {
		it := &r.Items[i]
		it.Tags = stripTags(it.Tags, torrentTags)
		it.Torrents = nil
		if it.Type != "file" {
			continue
		}
		// 自身路径或任一硬链接路径属于种子都算（例如扫描媒体库、下载目录作为查找目录时）
		seen := map[string]bool{}
		for _, p := range append([]string{it.Path}, it.Links...) {
			for _, ref := range byPath[p] {
				key := ref.Client + "/" + ref.Hash
				if !seen[key] {
					seen[key] = true
					it.Torrents = append(it.Torrents, ref)
				}
			}
		}
		if len(it.Torrents) == 0 {
			if under(it.Path, saveDirs) {
				it.Tags = append(it.Tags, "orphan")
				ts.Orphans++
				ts.OrphanSize += it.Size
			}
			continue
		}
		ts.Matched++
		counted := map[string]bool{}
		for _, ref := range it.Torrents {
			if j, ok := clientIdx[ref.Client]; ok && !counted[ref.Client] {
				counted[ref.Client] = true
				ts.Clients[j].Matched++
			}
		}
		it.Tags = append(it.Tags, "torrent")
		seeding, unreg := false, false
		itemSites := map[string]bool{}
		for _, ref := range it.Torrents {
			seeding = seeding || ref.State == "seeding"
			unreg = unreg || ref.Unregistered
			itemSites[ref.Site] = true
		}
		if seeding {
			it.Tags = append(it.Tags, "seeding")
			ts.Seeding++
		}
		if unreg {
			it.Tags = append(it.Tags, "unregistered")
			ts.Unregistered++
		}
		for site := range itemSites {
			st := sites[site]
			if st == nil {
				st = &scanner.SiteStat{Site: site}
				sites[site] = st
			}
			st.Files++
			st.Size += it.Size
			if seeding {
				st.Seeding++
			}
			if unreg {
				st.Unregistered++
			}
		}
	}
	for _, st := range sites {
		ts.Sites = append(ts.Sites, *st)
	}
	sort.Slice(ts.Sites, func(i, j int) bool { return ts.Sites[i].Size > ts.Sites[j].Size })
	r.Summary.Torrents = ts
}

// recount 删除 / 恢复后重新计算可直接得出的统计，并标记为估算。
func recount(r *scanner.Result) {
	s := &r.Summary
	s.Total, s.TotalSize, s.Linked, s.Saved, s.DupFiles, s.Symlinks, s.BrokenLinks, s.Strm, s.BrokenStrm = 0, 0, 0, 0, 0, 0, 0, 0, 0
	s.LinkedSize, s.UnlinkedSize = 0, 0
	var wasted float64
	var groups float64
	for i := range r.Items {
		it := &r.Items[i]
		switch it.Type {
		case "file":
			s.Total++
			s.TotalSize += it.Size
			if it.Linked {
				s.Linked++
				s.LinkedSize += it.Size
				s.Saved += it.Size * int64(it.Nlink-1)
			} else {
				s.UnlinkedSize += it.Size
			}
			if len(it.Dups) > 0 {
				s.DupFiles++
				k := float64(len(it.Dups) + 1)
				wasted += float64(it.Size) * (k - 1) / k
				groups += 1 / k
			}
		case "symlink":
			s.Symlinks++
			if it.Broken {
				s.BrokenLinks++
			}
		case "strm":
			s.Strm++
			if it.Broken {
				s.BrokenStrm++
			}
		}
	}
	s.Unlinked = s.Total - s.Linked
	s.DupWasted = int64(wasted)
	s.DupGroups = int(groups + 0.5)
	s.Stale = true
}

func inSet(p string, set map[string]bool, dirs []string) bool {
	return set[p] || under(p, dirs)
}

// removePaths 从结果中去掉已删除的路径，并修正其他条目的硬链接数与重复副本。
func removePaths(r *scanner.Result, deleted []string) {
	set := map[string]bool{}
	for _, d := range deleted {
		set[filepath.Clean(d)] = true
	}
	dirs := deleted
	keep := r.Items[:0]
	for _, it := range r.Items {
		if inSet(it.Path, set, dirs) {
			continue
		}
		if len(it.Links) > 0 {
			links := it.Links[:0]
			for _, l := range it.Links {
				if inSet(l, set, dirs) {
					if it.Nlink > 1 {
						it.Nlink--
					}
				} else {
					links = append(links, l)
				}
			}
			it.Links = links
		}
		if len(it.Dups) > 0 {
			dups := it.Dups[:0]
			for _, d := range it.Dups {
				if !inSet(d, set, dirs) {
					dups = append(dups, d)
				}
			}
			it.Dups = dups
		}
		if it.Type == "file" {
			it.Linked = it.Nlink > 1
			tags := stripTags(it.Tags, map[string]bool{"linked": true, "unlinked": true, "dup": true})
			if it.Linked {
				tags = append([]string{"linked"}, tags...)
			} else {
				tags = append([]string{"unlinked"}, tags...)
			}
			if len(it.Dups) > 0 {
				tags = append(tags, "dup")
			}
			it.Tags = tags
		}
		keep = append(keep, it)
	}
	r.Items = keep
	recount(r)
}
