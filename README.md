# hardlink-checker

飞牛 NAS（fnOS）上的硬链接检测工具：扫描指定文件夹，标出哪些文件存在硬链接、链接到了哪里，哪些还没整理。

计划形态：独立网页 + Docker，部署在 NAS 上。

## 现状

- `scripts/check_hardlinks.py`：命令行版检测脚本（可直接在 NAS 上运行）
- `docs/design.md`：界面设计稿说明

## 命令行用法

```bash
python3 scripts/check_hardlinks.py /vol1/1000/Downloads -t /vol1/1000/Media --unlinked --csv report.csv
```

## 注意

- 硬链接不能跨存储空间（vol），下载目录和媒体库需在同一个 vol。
- Docker 部署时，需把源目录和目标目录放在同一个挂载点下映射进容器，并以与主机一致的路径扫描。
