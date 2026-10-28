package scanner

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

// entry 是遍历时得到的一条普通文件或软链接记录。
type entry struct {
	path  string
	size  int64
	mtime int64
	dev   uint64
	ino   uint64
	nlink uint64
	link  bool // 是否为软链接
}

type inodeKey struct{ dev, ino uint64 }

func (e *entry) key() inodeKey { return inodeKey{e.dev, e.ino} }

// walk 并发遍历 root，对每个普通文件 / 软链接调用 emit（emit 需自行保证并发安全）。
// workers 限制同时进行目录读取的数量，避免机械硬盘被过多随机读拖慢。
// 软链接不会被跟随进入目录。
func walk(ctx context.Context, root string, recursive bool, workers int, skip map[string]bool,
	emit func(entry), errf func(string)) {
	if workers < 1 {
		workers = 1
	}
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup

	var visit func(dir string)
	visit = func(dir string) {
		defer wg.Done()
		if ctx.Err() != nil {
			return
		}
		sem <- struct{}{}
		des, err := os.ReadDir(dir)
		if err != nil {
			<-sem
			errf(dir + ": " + err.Error())
			return
		}
		var subdirs []string
		for _, de := range des {
			name := de.Name()
			p := filepath.Join(dir, name)
			t := de.Type()
			if t.IsDir() {
				if recursive && !skip[name] {
					subdirs = append(subdirs, p)
				}
				continue
			}
			isLink := t&fs.ModeSymlink != 0
			if !t.IsRegular() && !isLink {
				continue
			}
			var st syscall.Stat_t
			if err := syscall.Lstat(p, &st); err != nil {
				errf(p + ": " + err.Error())
				continue
			}
			emit(entry{
				path:  p,
				size:  st.Size,
				mtime: st.Mtim.Sec,
				dev:   uint64(st.Dev), //nolint:unconvert // 不同平台类型不同
				ino:   st.Ino,
				nlink: uint64(st.Nlink), //nolint:unconvert
				link:  isLink,
			})
		}
		<-sem
		for _, s := range subdirs {
			wg.Add(1)
			go visit(s)
		}
	}

	wg.Add(1)
	visit(root)
	wg.Wait()
}

func devOf(path string) (uint64, error) {
	var st syscall.Stat_t
	if err := syscall.Stat(path, &st); err != nil {
		return 0, err
	}
	return uint64(st.Dev), nil //nolint:unconvert
}
