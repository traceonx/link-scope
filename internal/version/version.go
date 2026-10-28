// Package version 保存构建信息，构建时通过 -ldflags 注入：
//
//	-X github.com/traceonx/link-scope/internal/version.Version=0.2.0
//	-X github.com/traceonx/link-scope/internal/version.Commit=<git sha>
//	-X 'github.com/traceonx/link-scope/internal/version.BuildTime=2026-10-28 12:00:00'
package version

var (
	Version   = "0.0.0-dev"
	Commit    = "dev"
	BuildTime = ""
)

// Info 是对外展示的版本信息。
type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"build_time"`
}

func Get() Info { return Info{Version: Version, Commit: Short(), BuildTime: BuildTime} }

// Short 返回 7 位提交号。
func Short() string {
	if len(Commit) > 7 {
		return Commit[:7]
	}
	return Commit
}

// Banner 是启动日志第一行。
func Banner() string {
	s := "LinkScope 链接透视 v" + Version + "（提交 " + Short()
	if BuildTime != "" {
		s += "，构建于 " + BuildTime
	}
	return s + "）"
}
