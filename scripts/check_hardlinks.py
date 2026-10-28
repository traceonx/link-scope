#!/usr/bin/env python3
"""
检测源目录中哪些文件存在硬链接，并可选地找出硬链接所在位置。

用法:
  python check_hardlinks.py <源目录> [-t 目标目录 ...] [--unlinked] [--csv 输出.csv]

示例:
  python check_hardlinks.py /data/downloads
  python check_hardlinks.py /data/downloads -t /data/media
  python check_hardlinks.py D:\\Downloads -t D:\\Media --unlinked --csv report.csv

说明:
  - 链接数 > 1 即表示该文件存在硬链接（含自身在内的总数）。
  - 指定 -t 后，会扫描目标目录，按 (设备, inode) 匹配出每个硬链接的具体路径。
  - --unlinked 额外列出没有硬链接的文件（即还没整理过的文件）。
  - 支持 Linux / macOS / NAS / Windows(NTFS)。
"""
import argparse
import csv
import os
import sys


def walk_files(root):
    for dirpath, _dirs, files in os.walk(root):
        for name in files:
            path = os.path.join(dirpath, name)
            try:
                st = os.stat(path, follow_symlinks=False)
            except OSError:
                continue
            if os.path.islink(path):
                continue
            yield path, st


def human(n):
    for unit in ("B", "KB", "MB", "GB", "TB"):
        if n < 1024:
            return f"{n:.1f}{unit}"
        n /= 1024
    return f"{n:.1f}PB"


def main():
    ap = argparse.ArgumentParser(description="检测源文件的硬链接")
    ap.add_argument("source", help="源目录")
    ap.add_argument("-t", "--targets", nargs="*", default=[], help="硬链接所在的目标目录（可多个）")
    ap.add_argument("--unlinked", action="store_true", help="同时列出没有硬链接的文件")
    ap.add_argument("--csv", help="导出 CSV 报告")
    args = ap.parse_args()

    src = os.path.abspath(args.source)
    if not os.path.isdir(src):
        sys.exit(f"源目录不存在: {src}")

    # 建立目标目录索引: (dev, ino) -> [路径]
    index = {}
    for t in args.targets:
        t = os.path.abspath(t)
        for path, st in walk_files(t):
            if st.st_nlink > 1:
                index.setdefault((st.st_dev, st.st_ino), []).append(path)

    linked, unlinked = [], []
    for path, st in walk_files(src):
        if st.st_nlink > 1:
            others = [p for p in index.get((st.st_dev, st.st_ino), []) if p != path]
            linked.append((path, st.st_nlink, st.st_size, others))
        else:
            unlinked.append((path, st.st_size))

    print(f"\n=== 有硬链接的文件: {len(linked)} 个 ===")
    for path, n, size, others in sorted(linked):
        print(f"[{n}] {path}  ({human(size)})")
        for o in others:
            print(f"     └─ {o}")
        if args.targets and n - 1 > len(others):
            print(f"     └─ (另有 {n - 1 - len(others)} 个链接不在目标目录中)")

    if args.unlinked:
        print(f"\n=== 没有硬链接的文件: {len(unlinked)} 个 ===")
        for path, size in sorted(unlinked):
            print(f"    {path}  ({human(size)})")

    total = len(linked) + len(unlinked)
    print(f"\n合计 {total} 个文件，已硬链接 {len(linked)}，未链接 {len(unlinked)}")

    if args.csv:
        with open(args.csv, "w", newline="", encoding="utf-8-sig") as f:
            w = csv.writer(f)
            w.writerow(["源文件", "链接数", "大小(字节)", "硬链接位置"])
            for path, n, size, others in sorted(linked):
                w.writerow([path, n, size, " | ".join(others)])
            for path, size in sorted(unlinked):
                w.writerow([path, 1, size, ""])
        print(f"CSV 已导出: {args.csv}")


if __name__ == "__main__":
    main()
