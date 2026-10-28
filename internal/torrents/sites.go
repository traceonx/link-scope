package torrents

import (
	"net/url"
	"strings"
)

// 常见站点的 Tracker 域名 → 显示名称；未收录的站点显示主域名。
var knownSites = map[string]string{
	"m-team.cc":           "馒头 M-Team",
	"m-team.io":           "馒头 M-Team",
	"hdsky.me":            "天空 HDSky",
	"chdbits.co":          "彩虹岛 CHDBits",
	"ptchdbits.co":        "彩虹岛 CHDBits",
	"ourbits.club":        "我堡 OurBits",
	"pthome.net":          "铂金家 PTHome",
	"hdhome.org":          "家园 HDHome",
	"audiences.me":        "观众 Audiences",
	"pterclub.com":        "猫站 PTerClub",
	"hdtime.org":          "高清时间 HDTime",
	"keepfrds.com":        "朋友 FRDS",
	"springsunday.net":    "春天 SSD",
	"hddolby.com":         "杜比 HDDolby",
	"btschool.club":       "学校 BTSchool",
	"hdfans.org":          "红豆饭 HDFans",
	"hdarea.club":         "好大 HDArea",
	"pttime.org":          "PTTime",
	"1ptba.com":           "1PTBar",
	"leaves.red":          "红叶 Leaves",
	"ubits.club":          "UBits",
	"qingwapt.com":        "青蛙 QingWa",
	"carpt.net":           "车站 CarPT",
	"piggo.me":            "猪猪 PigGo",
	"zmpt.cc":             "织梦 ZMPT",
	"ptsbao.club":         "烧包 PTSBao",
	"agsvpt.com":          "末日 AGSV",
	"hdatmos.club":        "阿童木 HDAtmos",
	"open.cd":             "皇后 OpenCD",
	"totheglory.im":       "听听歌 TTG",
	"hdchina.org":         "瓷器 HDChina",
	"dicmusic.com":        "海豚 DICMusic",
	"greatposterwall.com": "GPW",
	"nicept.net":          "老师 NicePT",
	"tjupt.org":           "北洋园 TJUPT",
	"byr.pt":              "北邮人 BYR",
	"nwsuaf6.edu.cn":      "麦田 NWSUAF6",
	"hdupt.com":           "好多油 HDUPT",
	"hhanclub.top":        "憨憨 HHan",
	"wintersakura.net":    "冬樱 WinterSakura",
	"discfan.net":         "碟粉 DiscFan",
	"haidan.video":        "海胆 Haidan",
	"pt.btschool.club":    "学校 BTSchool",
	"kamept.com":          "KamePT",
	"hdmayi.com":          "蚂蚁 HDMaYi",
	"soulvoice.club":      "铃音 SoulVoice",
	"pt.0ff.cc":           "自由农场 0ff",
	"icc2022.com":         "冰淇淋 ICC",
	"cyanbug.net":         "大青虫 CyanBug",
	"ptvicomo.net":        "象站 VicomoVS",
	"pt.keepfrds.com":     "朋友 FRDS",
	"filelist.io":         "FileList",
	"torrentleech.org":    "TorrentLeech",
	"iptorrents.com":      "IPTorrents",
	"passthepopcorn.me":   "PTP",
	"broadcasthe.net":     "BTN",
	"blutopia.cc":         "Blutopia",
	"beyond-hd.me":        "BeyondHD",
	"aither.cc":           "Aither",
	"hdbits.org":          "HDBits",
}

// 公共 Tracker（非 PT 站）
var publicHints = []string{"opentrackr", "openbittorrent", "publicbt", "tracker.leechers-paradise", "explodie", "torrent.eu.org", "stealth.si"}

// siteOf 根据 Tracker 地址识别站点名称。
func siteOf(trackers []string) string {
	for _, t := range trackers {
		u, err := url.Parse(strings.TrimSpace(t))
		if err != nil || u.Hostname() == "" {
			continue
		}
		host := strings.ToLower(u.Hostname())
		for _, h := range publicHints {
			if strings.Contains(host, h) {
				return "公共 Tracker"
			}
		}
		// 从完整域名逐级向上匹配（tracker.hdsky.me → hdsky.me）
		parts := strings.Split(host, ".")
		for i := 0; i < len(parts)-1; i++ {
			if name, ok := knownSites[strings.Join(parts[i:], ".")]; ok {
				return name
			}
		}
		return mainDomain(host)
	}
	return "未知站点"
}

// mainDomain 返回主域名：去掉 tracker. / t. / pt. 等前缀，保留最后两段（.edu.cn 等保留三段）。
func mainDomain(host string) string {
	parts := strings.Split(host, ".")
	if len(parts) <= 2 {
		return host
	}
	n := 2
	if l := parts[len(parts)-2]; len(l) <= 3 && (l == "com" || l == "edu" || l == "net" || l == "org" || l == "co") {
		n = 3
	}
	if n > len(parts) {
		n = len(parts)
	}
	return strings.Join(parts[len(parts)-n:], ".")
}
