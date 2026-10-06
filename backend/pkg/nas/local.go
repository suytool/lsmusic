// Package nas 本地音乐模块（v1.1.2）
//
// 在原有 NAS 扫描能力之上，新增「本地音乐」完整功能链路：
//   - 选择文件夹（严格限定在飞牛「应用设置 → 访问权限」授权的目录内）
//   - 扫描本地音乐（递归收集音频文件并探测 时长/码率/品质）
//   - 去重管理（按“歌名 + 歌手”归一化分组，识别同一歌曲的多音质版本，
//     用户可勾选删除非保留版本）
//   - 批量重命名（按命名格式批量改文件名，预览冲突）
//   - 写入歌词/封面等标签（复用 tagwriter，支持 MP3/FLAC），
//     支持联网自动搜索并补全歌词与封面
//   - 本地播放（带 Range 的流式播放，支持拖动进度条）
//
// 所有涉及文件路径的接口一律经过 fsaccess 授权目录校验：
//   未授权路径 → 403；删除/写标签额外要求所在目录可写。
package nas

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"fn-lx-player/pkg/fsaccess"
	"fn-lx-player/pkg/search"
	"fn-lx-player/pkg/tagwriter"
)

// 扫描上限（防止超大目录卡死应用）
const (
	localScanMaxFiles = 5000
	localMaxDepth     = 16
)

// ──────────────────────────────────────────────
// 授权目录校验辅助
// ──────────────────────────────────────────────

// localCheckAccess 校验 path 是否位于授权目录内。
// writable=true 时额外要求所在目录可写（删除、写标签等写操作需要）。
func localCheckAccess(path string, writable bool) (string, error) {
	p := fsaccess.Normalize(path)
	if p == "" || p == "/" {
		return "", fmt.Errorf("无效路径")
	}
	roots := fsaccess.AllowedRoots()
	if !fsaccess.IsPathWithinRoots(p, roots) {
		return "", fmt.Errorf("无权访问该路径（仅可访问应用设置中已授权的文件夹）")
	}
	info, err := os.Stat(p)
	if err != nil {
		return "", fmt.Errorf("路径不存在或不可访问")
	}
	if writable {
		if info.IsDir() {
			if !fsaccess.IsWritableDir(p) {
				return "", fmt.Errorf("目录不可写")
			}
		} else {
			if !fsaccess.IsWritableDir(filepath.Dir(p)) {
				return "", fmt.Errorf("所在目录不可写")
			}
		}
	}
	return p, nil
}

// ──────────────────────────────────────────────
// 音频时长 / 码率探测（零依赖）
// ──────────────────────────────────────────────

// probeDurationBitrate 探测音频时长（秒）与码率（kbps）。
// 支持 MP3（Xing/VBRI/CBR）、FLAC（STREAMINFO）、WAV、MP4/M4A（mvhd）。
// 无法探测时返回 0。
func probeDurationBitrate(path string, size int64) (duration, bitrate int) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer f.Close()

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".mp3":
		return probeMP3(f, size)
	case ".flac":
		return probeFLAC(f, size)
	case ".wav":
		return probeWAV(f)
	case ".m4a", ".mp4", ".aac":
		return probeMP4(f)
	}
	return 0, 0
}

// probeMP3 读取 MP3 时长与码率：优先 Xing/Info 头，其次 CBR 估算。
func probeMP3(f *os.File, size int64) (duration, bitrate int) {
	// 跳过 ID3v2 标签
	offset := int64(0)
	hdr := make([]byte, 10)
	if n, _ := f.ReadAt(hdr, 0); n == 10 && string(hdr[:3]) == "ID3" {
		tagSize := int64(hdr[6])<<21 | int64(hdr[7])<<14 | int64(hdr[8])<<7 | int64(hdr[9])
		offset = 10 + tagSize
	}

	// 寻找第一个 MPEG 帧头（最多前 64KB 内同步）
	buf := make([]byte, 64*1024)
	n, _ := f.ReadAt(buf, offset)
	if n < 4 {
		return 0, 0
	}
	data := buf[:n]
	syncIdx := -1
	for i := 0; i+4 <= len(data); i++ {
		if data[i] == 0xFF && data[i+1]&0xE0 == 0xE0 {
			syncIdx = i
			break
		}
	}
	if syncIdx == -1 {
		return 0, 0
	}
	frameHdr := data[syncIdx : syncIdx+4]

	bitrateIdx := int(frameHdr[2] >> 4)
	sampleIdx := int(frameHdr[2]>>2) & 0x3
	// 码率表（kbps，按 MPEG1 Layer III / V1 / L3 取常用值）
	bitrateTable := []int{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 0}
	sampleTable := []int{44100, 48000, 32000, 0}
	if bitrateIdx >= len(bitrateTable) || sampleIdx >= len(sampleTable) {
		return 0, 0
	}
	kbps := bitrateTable[bitrateIdx]
	sampleRate := sampleTable[sampleIdx]
	if kbps <= 0 || sampleRate <= 0 {
		return 0, 0
	}
	// 帧长（MPEG1 Layer III）
	frameLen := (144 * kbps * 1000 / sampleRate) + 1

	// Xing / Info 头：位于首帧头之后
	xingOffset := syncIdx + 4
	if xingOffset+8 <= len(data) {
		marker := string(data[xingOffset : xingOffset+4])
		if marker == "Xing" || marker == "Info" {
			flags := binary.BigEndian.Uint32(data[xingOffset+4 : xingOffset+8])
			pos := xingOffset + 8
			if flags&0x1 != 0 && pos+4 <= len(data) { // frames 字段
				frames := int(binary.BigEndian.Uint32(data[pos : pos+4]))
				spf := 1152 // MPEG1 Layer III
				if frames > 0 && sampleRate > 0 {
					duration = frames * spf / sampleRate
					// 码率：用文件大小估算更贴近实际（含标签）
					if size > 0 && duration > 0 {
						bitrate = int(size * 8 / int64(duration) / 1000)
					}
					return duration, bitrate
				}
			}
		}
		// VBRI：位于首帧头后 32 字节
		vbriOffset := syncIdx + 4 + 32
		if vbriOffset+26 <= len(data) && string(data[vbriOffset:vbriOffset+4]) == "VBRI" {
			frames := int(binary.BigEndian.Uint32(data[vbriOffset+14 : vbriOffset+18]))
			spf := 1152
			if frames > 0 && sampleRate > 0 {
				duration = frames * spf / sampleRate
				if size > 0 && duration > 0 {
					bitrate = int(size * 8 / int64(duration) / 1000)
				}
				return duration, bitrate
			}
		}
	}

	// CBR 估算
	if kbps > 0 && frameLen > 0 {
		audioBytes := size - offset - 128 // 去掉 ID3v2 与可能的 ID3v1
		if audioBytes <= 0 {
			return 0, 0
		}
		frameCount := int(audioBytes / int64(frameLen))
		spf := 1152
		duration = frameCount * spf / sampleRate
		bitrate = kbps
		return duration, bitrate
	}
	return 0, 0
}

// probeFLAC 读取 STREAMINFO 计算时长与码率。
func probeFLAC(f *os.File, size int64) (duration, bitrate int) {
	hdr := make([]byte, 42)
	n, err := f.ReadAt(hdr, 0)
	if err != nil && n < 42 {
		return 0, 0
	}
	if string(hdr[:4]) != "fLaC" {
		return 0, 0
	}
	// 第一个 metadata block 应为 STREAMINFO（type 0），长度 34
	if hdr[4]&0x7F != 0 {
		return 0, 0
	}
	blockLen := int(hdr[5])<<16 | int(hdr[6])<<8 | int(hdr[7])
	if blockLen != 34 {
		return 0, 0
	}
	si := hdr[8:42]
	// STREAMINFO 位布局（34 字节）：
	//   [10..12] 采样率 20bit；[12] 低4bit=声道-1、bps-1；
	//   [13] 高4bit=bps-1 低4bit、低4bit=总采样数[35:32]；[14..17]=总采样数[31:0]
	sampleRate := int(si[10])<<12 | int(si[11])<<4 | int(si[12])>>4
	totalSamples := int64(si[13]&0x0F)<<32 | int64(si[14])<<24 | int64(si[15])<<16 | int64(si[16])<<8 | int64(si[17])
	if sampleRate <= 0 || totalSamples <= 0 {
		return 0, 0
	}
	duration = int(totalSamples / int64(sampleRate))
	if size > 0 && duration > 0 {
		bitrate = int(size * 8 / int64(duration) / 1000)
	}
	return duration, bitrate
}

// probeWAV 解析 fmt 与 data chunk 计算时长。
func probeWAV(f *os.File) (duration, bitrate int) {
	buf := make([]byte, 12)
	if _, err := f.ReadAt(buf, 0); err != nil || string(buf[:4]) != "RIFF" || string(buf[8:12]) != "WAVE" {
		return 0, 0
	}
	// 扫描 chunk
	offset := int64(12)
	for offset < 4096 {
		hdr := make([]byte, 8)
		if _, err := f.ReadAt(hdr, offset); err != nil {
			break
		}
		chunkID := string(hdr[:4])
		chunkSize := int64(binary.LittleEndian.Uint32(hdr[4:8]))
		if chunkID == "fmt " && chunkSize >= 16 {
			fmtData := make([]byte, 16)
			if _, err := f.ReadAt(fmtData, offset+8); err != nil {
				break
			}
			sampleRate := int(binary.LittleEndian.Uint32(fmtData[4:8]))
			byteRate := int64(binary.LittleEndian.Uint32(fmtData[8:12]))
			if sampleRate <= 0 || byteRate <= 0 {
				break
			}
			if dataSize := wavDataSize(f, offset+8+chunkSize); dataSize > 0 {
				duration = int(dataSize / byteRate)
				bitrate = int(byteRate * 8 / 1000)
			}
			return duration, bitrate
		}
		offset += 8 + chunkSize + (chunkSize % 2)
	}
	return 0, 0
}

func wavDataSize(f *os.File, start int64) int64 {
	offset := start
	for offset < start+65536 {
		hdr := make([]byte, 8)
		if _, err := f.ReadAt(hdr, offset); err != nil {
			return 0
		}
		chunkID := string(hdr[:4])
		chunkSize := int64(binary.LittleEndian.Uint32(hdr[4:8]))
		if chunkID == "data" {
			return chunkSize
		}
		offset += 8 + chunkSize + (chunkSize % 2)
	}
	return 0
}

// probeMP4 解析 mvhd box 计算时长（m4a/mp4/aac）。
func probeMP4(f *os.File) (duration, bitrate int) {
	buf := make([]byte, 1<<20) // 前 1MB 内找 mvhd
	n, err := f.ReadAt(buf, 0)
	if err != nil && n < 8 {
		return 0, 0
	}
	data := buf[:n]
	// 简单递归找 mvhd（只支持在 ftyp/moov 顶层，通常如此）
	idx := 0
	for idx+8 <= len(data) {
		boxSize := int(binary.BigEndian.Uint32(data[idx : idx+4]))
		boxType := string(data[idx+4 : idx+8])
		if boxType == "moov" {
			// 进入 moov 找 mvhd
			inner := idx + 8
			end := idx + boxSize
			if end > len(data) {
				end = len(data)
			}
			for inner+8 <= end {
				s2 := int(binary.BigEndian.Uint32(data[inner : inner+4]))
				t2 := string(data[inner+4 : inner+8])
				if t2 == "mvhd" && inner+32 <= end {
					version := data[inner+8]
					var timescale, dur uint32
					if version == 1 {
						if inner+32 <= end {
							timescale = binary.BigEndian.Uint32(data[inner+20 : inner+24])
							dur = uint32(binary.BigEndian.Uint64(data[inner+24 : inner+32]))
						}
					} else {
						if inner+24 <= end {
							timescale = binary.BigEndian.Uint32(data[inner+12 : inner+16])
							dur = binary.BigEndian.Uint32(data[inner+16 : inner+20])
						}
					}
					if timescale > 0 {
						return int(dur / timescale), 0
					}
					return 0, 0
				}
				if s2 <= 0 {
					break
				}
				inner += s2
			}
			break
		}
		if boxSize <= 0 {
			break
		}
		idx += boxSize
	}
	return 0, 0
}

// qualityLabel 根据格式与码率给出品质标签（用于去重时区分多音质版本）。
func qualityLabel(ext string, bitrate int) string {
	switch {
	case bitrate >= 4000:
		return "母带"
	case bitrate >= 2000:
		return "至臻"
	case ext == ".flac" && bitrate >= 800:
		return "无损"
	case bitrate >= 256:
		return "超高"
	case bitrate >= 128:
		return "流畅"
	case ext == ".flac":
		return "无损"
	case ext == ".wav":
		return "无损"
	case bitrate > 0:
		return fmt.Sprintf("%dk", bitrate)
	default:
		return "未知"
	}
}

// ──────────────────────────────────────────────
// 扫描与去重分组
// ──────────────────────────────────────────────

// LocalSongInfo 本地歌曲条目（含探测到的时长/码率/品质）。
type LocalSongInfo struct {
	ID          string `json:"id"`
	Path        string `json:"path"`
	Filename    string `json:"filename"`
	Title       string `json:"title"`
	Artist      string `json:"artist"`
	Album       string `json:"album"`
	Name        string `json:"name"`   // 播放器兼容字段（= Title）
	Singer      string `json:"singer"` // 播放器兼容字段（= Artist）
	Duration    int    `json:"duration"`
	Interval    int    `json:"interval"`
	Bitrate     int    `json:"bitrate"`
	Quality     string `json:"quality"`
	Size        int64  `json:"size"`
	Format      string `json:"format"`
	Source      string `json:"source"` // 固定 "local"，播放器据此走本地直连播放
	CoverURL    string `json:"cover_url"`
	HasLyric    bool   `json:"has_lyric"`
	UpdatedAt   int64  `json:"updated_at"`
}

// DedupGroup 去重分组：同一歌曲的多音质版本集合。
type DedupGroup struct {
	Key     string          `json:"key"`     // 归一化分组键
	Name    string          `json:"name"`    // 展示歌名
	Artist  string          `json:"artist"`  // 展示歌手
	Songs   []LocalSongInfo `json:"songs"`   // 各版本（按品质从高到低）
	Version int             `json:"version"` // 版本数量
}

// ScanResult 扫描结果。
type ScanResult struct {
	Dir    string        `json:"dir"`
	Songs  []LocalSongInfo `json:"songs"`
	Total  int           `json:"total"`
	Groups []DedupGroup  `json:"groups"`
}

// HandleLocalScan GET /api/local/scan?dir=
// 扫描授权目录内的本地音乐，返回歌曲列表与按歌名归一的去重分组。
func HandleLocalScan(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	dir := strings.TrimSpace(r.URL.Query().Get("dir"))
	if dir == "" {
		roots := fsaccess.AllowedRoots()
		if len(roots) == 0 {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 403, "error": "未检测到授权文件夹，请在飞牛「应用设置 → 访问权限」中授权后再扫描",
			})
			return
		}
		dir = roots[0].Path
	}

	cleanDir, err := localCheckAccess(dir, false)
	if err != nil {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code": 403, "error": err.Error(),
		})
		return
	}

	songs := make([]LocalSongInfo, 0, 256)
	count := 0

	var walk func(p string, depth int)
	walk = func(p string, depth int) {
		if count >= localScanMaxFiles || depth > localMaxDepth {
			return
		}
		entries, err := os.ReadDir(p)
		if err != nil {
			return
		}
		for _, e := range entries {
			if count >= localScanMaxFiles {
				return
			}
			name := e.Name()
			if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "@") {
				continue
			}
			full := filepath.Join(p, name)
			if e.IsDir() {
				walk(full, depth+1)
				continue
			}
			ext := strings.ToLower(filepath.Ext(name))
			if !audioExts[ext] {
				continue
			}
			count++
			info, err := e.Info()
			if err != nil {
				continue
			}
			title, artist, album := parseBasicMeta(full, name)
			duration, bitrate := probeDurationBitrate(full, info.Size())
			song := LocalSongInfo{
				ID:        fmt.Sprintf("local_%d", count),
				Path:      filepath.ToSlash(full),
				Filename:  name,
				Title:     title,
				Artist:    artist,
				Album:     album,
				Name:      title,
				Singer:    artist,
				Duration:  duration,
				Interval:  duration,
				Bitrate:   bitrate,
				Quality:   qualityLabel(ext, bitrate),
				Size:      info.Size(),
				Format:    strings.TrimPrefix(ext, "."),
				Source:    "local",
				CoverURL:  "/api/local/cover?path=" + urlQueryEscape(filepath.ToSlash(full)),
				HasLyric:  checkHasLyric(full),
				UpdatedAt: info.ModTime().Unix(),
			}
			songs = append(songs, song)
		}
	}
	walk(cleanDir, 0)

	groups := buildDedupGroups(songs)

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"code": 200,
		"data": ScanResult{
			Dir:    cleanDir,
			Songs:  songs,
			Total:  len(songs),
			Groups: groups,
		},
	})
}

func urlQueryEscape(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "&", "%26"), "?", "%3F")
}

// buildDedupGroups 按“歌名 + 歌手”归一化分组，识别同一歌曲的多音质版本。
// 归一化：小写、去空白与标点、保留中日韩文字与字母数字。
func buildDedupGroups(songs []LocalSongInfo) []DedupGroup {
	// 归组策略（v1.1.6 增强）：
	//   1. 歌名归一化为主键（无标签/解析缺失时用文件名兜底）
	//   2. 组内歌手匹配放宽：归一化后相等 / 一方未知 / 互相包含 → 视为同一首歌的多个版本
	//      解决「MP3 有标签、FLAC 无标签」或「歌手写法略有差异」导致的漏归组
	groups := make([]DedupGroup, 0)
	for _, s := range songs {
		t := normalizeKey(s.Title)
		if t == "" {
			t = normalizeKey(strings.TrimSuffix(s.Filename, filepath.Ext(s.Filename)))
		}
		if t == "" {
			continue
		}
		ak := normalizeKey(s.Artist)
		vf := versionFeature(s)
		placed := false
		for i := range groups {
			g := &groups[i]
			if normalizeKey(g.Name) != t {
				continue
			}
			// 版本特征必须一致（DJ 版/原版/伴奏/现场等不算重复）
			if versionFeature(g.Songs[0]) != vf {
				continue
			}
			if artistKeysMatch(normalizeKey(g.Artist), ak) {
				g.Songs = append(g.Songs, s)
				placed = true
				break
			}
		}
		if !placed {
			groups = append(groups, DedupGroup{
				Key:    t + "\x00" + ak,
				Name:   s.Title,
				Artist: s.Artist,
				Songs:  []LocalSongInfo{s},
			})
		}
	}

	// 只保留多版本分组（>=2 个文件）用于去重管理
	filtered := make([]DedupGroup, 0)
	for i := range groups {
		g := &groups[i]
		if len(g.Songs) < 2 {
			continue
		}
		// 版本按品质从高到低排序：母带>至臻>无损>超高>流畅>更高码率>未知
		sort.SliceStable(g.Songs, func(a, b int) bool {
			return qualityRank(g.Songs[a]) > qualityRank(g.Songs[b])
		})
		g.Version = len(g.Songs)
		filtered = append(filtered, *g)
	}
	// 按歌名排序
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Name < filtered[j].Name
	})
	return filtered
}

// artistKeysMatch 歌手归一化 key 的匹配：
// 相等或一方未知（标签缺失时为空）视为同一歌手组合；
// 不做包含匹配，避免"周杰伦"与"周杰伦DJ版"误归组。
func artistKeysMatch(a, b string) bool {
	if a == "" || b == "" {
		return true
	}
	return a == b
}

// versionFeature 检测歌曲的版本特征（DJ/混音/伴奏/现场/翻唱/器乐版等）。
// 同一首歌的 DJ 版与原版、伴奏版与原版属于不同演绎，不应归入同一去重组。
func versionFeature(s LocalSongInfo) string {
	text := strings.ToLower(s.Title + "\n" + s.Filename + "\n" + s.Artist)
	features := []struct {
		key  string
		word []string
	}{
		{"dj", []string{"dj", "dj版", "djmix", "dj混音", "慢摇", "电音"}},
		{"remix", []string{"remix", "混音", "重混"}},
		{"伴奏", []string{"伴奏", "inst", "instrumental", "karaoke", "ok版"}},
		{"现场", []string{"live", "现场", "演唱会", "concert"}},
		{"翻唱", []string{"cover", "翻唱"}},
		{"器乐", []string{"钢琴", "吉他", "古筝", "笛子", "纯音乐", "萨克斯", "二胡", "小提琴"}},
	}
	for _, f := range features {
		for _, w := range f.word {
			if strings.Contains(text, w) {
				return f.key
			}
		}
	}
	return ""
}

// qualityRank 品质排序权重（越大越优先保留）。
func qualityRank(s LocalSongInfo) int {
	switch s.Quality {
	case "母带":
		return 100
	case "至臻":
		return 80
	case "无损":
		return 60
	case "超高":
		return 40
	case "流畅":
		return 20
	}
	return 10
}

// normalizeKey 归一化分组键：小写并仅保留中日韩与字母数字。
func normalizeKey(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if unicode.Is(unicode.Han, r) || unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ──────────────────────────────────────────────
// 批量重命名
// ──────────────────────────────────────────────

// HandleLocalRename POST /api/local/rename
// {"items":[{"path":"...","new_name":"歌曲名 - 歌手.mp3"}]}
// 仅允许重命名授权目录内、且所在目录可写的音频文件；
// 新文件名必须包含与源文件一致的扩展名，且不得与同目录现有文件冲突。
func HandleLocalRename(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	var req struct {
		Items []struct {
			Path    string `json:"path"`
			NewName string `json:"new_name"`
		} `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(req.Items) == 0 {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"code": 200, "renamed": []interface{}{}, "failed": []interface{}{}})
		return
	}
	if len(req.Items) > 500 {
		http.Error(w, "一次最多重命名 500 个文件", http.StatusBadRequest)
		return
	}

	// 先统一收集，避免前面成功、后面失败造成部分执行（前端预览已尽量拦截，
	// 这里逐项执行并逐项回报结果，不整体回滚）
	renamed := make([]map[string]string, 0, len(req.Items))
	failed := make([]map[string]interface{}, 0)

	// 预检查：同目录下新名互相冲突（防 A→B 与 B→A 相互覆盖等）
	type planned struct {
		src   string
		target string
	}
	plans := make([]planned, 0, len(req.Items))
	targetSet := make(map[string]bool)

	for _, it := range req.Items {
		clean, err := localCheckAccess(it.Path, true)
		if err != nil {
			failed = append(failed, map[string]interface{}{"path": it.Path, "error": err.Error()})
			continue
		}
		info, err := os.Stat(clean)
		if err != nil || info.IsDir() {
			failed = append(failed, map[string]interface{}{"path": it.Path, "error": "文件不存在或不是普通文件"})
			continue
		}

		newName := strings.TrimSpace(it.NewName)
		// 安全校验：不能含路径分隔符、不能以点开头、不能是空
		if newName == "" || strings.ContainsAny(newName, "/\\") || strings.HasPrefix(newName, ".") {
			failed = append(failed, map[string]interface{}{"path": it.Path, "error": "文件名不合法"})
			continue
		}
		// 扩展名必须与源文件一致（只改名字不改格式）
		srcExt := strings.ToLower(filepath.Ext(clean))
		newExt := strings.ToLower(filepath.Ext(newName))
		if newExt == "" || srcExt != newExt {
			failed = append(failed, map[string]interface{}{"path": it.Path, "error": "扩展名需与原文件一致（如 .mp3）"})
			continue
		}

		target := filepath.ToSlash(filepath.Join(filepath.Dir(clean), newName))
		if target == clean {
			// 新名与旧名相同：跳过，不算失败
			continue
		}
		// 与同目录现有文件冲突（排除源文件本身）
		if _, err := os.Stat(target); err == nil {
			failed = append(failed, map[string]interface{}{"path": it.Path, "error": "目标文件名已存在"})
			continue
		}
		// 与本次计划中的其他目标冲突
		if targetSet[target] {
			failed = append(failed, map[string]interface{}{"path": it.Path, "error": "多个文件将重命名为同名，请调整格式"})
			continue
		}
		targetSet[target] = true
		plans = append(plans, planned{src: clean, target: target})
	}

	for _, p := range plans {
		if err := os.Rename(p.src, p.target); err != nil {
			failed = append(failed, map[string]interface{}{"path": p.src, "error": err.Error()})
			continue
		}
		renamed = append(renamed, map[string]string{
			"old": p.src,
			"new": p.target,
		})
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    200,
		"renamed": renamed,
		"failed":  failed,
	})
}

// ──────────────────────────────────────────────
// 删除（去重管理）
// ──────────────────────────────────────────────

// HandleLocalDelete POST /api/local/delete {"paths":[...]}
// 仅允许删除授权目录内、且所在目录可写的音频文件。
func HandleLocalDelete(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	var req struct {
		Paths []string `json:"paths"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(req.Paths) == 0 {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"code": 200, "deleted": []string{}, "failed": []interface{}{}})
		return
	}
	if len(req.Paths) > 500 {
		http.Error(w, "一次最多删除 500 个文件", http.StatusBadRequest)
		return
	}

	deleted := make([]string, 0, len(req.Paths))
	failed := make([]map[string]interface{}, 0)
	for _, p := range req.Paths {
		clean, err := localCheckAccess(p, true)
		if err != nil {
			failed = append(failed, map[string]interface{}{"path": p, "error": err.Error()})
			continue
		}
		info, err := os.Stat(clean)
		if err != nil || info.IsDir() {
			failed = append(failed, map[string]interface{}{"path": p, "error": "文件不存在或不是普通文件"})
			continue
		}
		if err := os.Remove(clean); err != nil {
			failed = append(failed, map[string]interface{}{"path": p, "error": err.Error()})
			continue
		}
		deleted = append(deleted, clean)
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    200,
		"deleted": deleted,
		"failed":  failed,
	})
}

// ──────────────────────────────────────────────
// 写入歌词 / 封面标签
// ──────────────────────────────────────────────

// HandleLocalTags POST /api/local/tags
// {"path","title","artist","album","lyrics","cover_b64"(可选),"remove_cover"(可选)}
// 支持 MP3（ID3v2.3）与 FLAC（Vorbis Comment + Picture），写入后原子替换文件。
func HandleLocalTags(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	var req struct {
		Path        string `json:"path"`
		Title       string `json:"title"`
		Artist      string `json:"artist"`
		Album       string `json:"album"`
		Lyrics      string `json:"lyrics"`
		CoverB64    string `json:"cover_b64"`
		RemoveCover bool   `json:"remove_cover"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Path == "" {
		http.Error(w, "path required", http.StatusBadRequest)
		return
	}

	clean, err := localCheckAccess(req.Path, true)
	if err != nil {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"code": 403, "error": err.Error()})
		return
	}

	// 读取原文件保留未修改的字段（title/artist/album 为空时保持原值）
	origTitle, origArtist, origAlbum := parseBasicMeta(clean, filepath.Base(clean))

	data := tagwriter.TagData{
		Title:  req.Title,
		Artist: req.Artist,
		Album:  req.Album,
		Lyrics: req.Lyrics,
	}
	if data.Title == "" {
		data.Title = origTitle
	}
	if data.Artist == "" {
		data.Artist = origArtist
	}
	if data.Album == "" {
		data.Album = origAlbum
	}

	if req.RemoveCover {
		// 置空封面：直接写入不含封面的标签
	} else if req.CoverB64 != "" {
		cover, decErr := base64.StdEncoding.DecodeString(req.CoverB64)
		if decErr != nil {
			http.Error(w, "封面图片数据无效", http.StatusBadRequest)
			return
		}
		if len(cover) > 8*1024*1024 {
			http.Error(w, "封面图片过大（限 8MB）", http.StatusBadRequest)
			return
		}
		data.CoverJPEG = cover
	} else {
		// 未提供新封面：保留原内嵌封面
		if orig, _ := extractCoverBytes(clean); len(orig) > 0 {
			data.CoverJPEG = orig
		}
	}

	if err := tagwriter.WriteTags(clean, data); err != nil {
		http.Error(w, "写入标签失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"code": 200,
		"path": clean,
		"data": map[string]interface{}{
			"title":     data.Title,
			"artist":    data.Artist,
			"album":     data.Album,
			"lyrics":    data.Lyrics,
			"has_cover": len(data.CoverJPEG) > 0,
		},
	})
}

// ──────────────────────────────────────────────
// 联网自动补全歌词 / 封面
// ──────────────────────────────────────────────

// HandleLocalAutoTag POST /api/local/autotag
// {"path","title"(可选),"singer"(可选)}
// 按「歌名 + 歌手」联网搜索匹配的在线歌曲，自动获取 LRC 歌词与封面
// 并写入音频文件内嵌标签（MP3 ID3v2.3 / FLAC Vorbis Comment + Picture）。
// 未匹配到在线歌曲或对应资源时对应字段保持原样，不影响其他字段。
func HandleLocalAutoTag(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	var req struct {
		Path   string `json:"path"`
		Title  string `json:"title"`
		Singer string `json:"singer"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Path == "" {
		http.Error(w, "path required", http.StatusBadRequest)
		return
	}

	clean, err := localCheckAccess(req.Path, true)
	if err != nil {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"code": 403, "error": err.Error()})
		return
	}

	// 当前标签（请求里可覆盖）
	title, singer, album := parseBasicMeta(clean, filepath.Base(clean))
	if req.Title != "" {
		title = req.Title
	}
	if req.Singer != "" {
		singer = req.Singer
	}
	if title == "" {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code": 404, "error": "无法确定歌曲名，请先在标签管理中填写标题",
		})
		return
	}

	// 多平台搜索并匹配
	keyword := title
	if singer != "" {
		keyword = title + " " + singer
	}
	candidates := search.SearchSongsForAutoTag(keyword, 1, 10)

	var matched *search.UnifiedSong
	titleLower := strings.ToLower(title)
	for i := range candidates {
		if strings.Contains(strings.ToLower(candidates[i].Name), titleLower) {
			matched = &candidates[i]
			break
		}
	}
	if matched == nil && len(candidates) > 0 {
		matched = &candidates[0]
	}
	if matched == nil {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code": 404, "error": "未找到匹配的在线歌曲，请检查歌名/歌手是否正确",
		})
		return
	}

	// 获取歌词
	lrc := search.FetchLyricForSong(matched.Source, matched.Songmid, matched.Name, matched.Singer, matched.Duration, matched.Hash)

	// 下载封面
	var cover []byte
	if matched.Cover != "" {
		cover = downloadCoverImage(matched.Cover, matched.Source)
	}

	// 写入标签（专辑：本地为空时用在线专辑名补充）
	if album == "" && matched.Album != "" {
		album = matched.Album
	}
	data := tagwriter.TagData{
		Title:     title,
		Artist:    singer,
		Album:     album,
		Lyrics:    lrc,
		CoverJPEG: cover,
	}
	if err := tagwriter.WriteTags(clean, data); err != nil {
		http.Error(w, "写入标签失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"code":        200,
		"path":        clean,
		"source":      matched.Source,
		"online_name": matched.Name,
		"lyric_ok":    strings.TrimSpace(lrc) != "",
		"cover_ok":    len(cover) > 0,
		"data": map[string]interface{}{
			"title":  data.Title,
			"artist": data.Artist,
			"album":  data.Album,
			"lyric_ok": strings.TrimSpace(lrc) != "",
			"cover_ok": len(cover) > 0,
		},
	})
}

// downloadCoverImage 下载封面图片（带来源站 Referer 与 UA），返回 JPEG/PNG 字节。
func downloadCoverImage(url, source string) []byte {
	client := &http.Client{Timeout: 12 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36")
	switch strings.ToLower(source) {
	case "wy":
		req.Header.Set("Referer", "https://music.163.com")
	case "tx":
		req.Header.Set("Referer", "https://y.qq.com")
	case "kg":
		req.Header.Set("Referer", "https://www.kugou.com")
	case "kw":
		req.Header.Set("Referer", "https://www.kuwo.cn")
	case "mg":
		req.Header.Set("Referer", "https://music.migu.cn")
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil || len(data) < 100 {
		return nil
	}
	// 校验图片魔数（JPEG / PNG）
	isJPEG := len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF
	isPNG := len(data) >= 8 && data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47
	if !isJPEG && !isPNG {
		return nil
	}
	return data
}

// ──────────────────────────────────────────────
// 本地播放 / 封面 / 歌词
// ──────────────────────────────────────────────

// HandleLocalStream GET /api/local/stream?path=  （支持 Range 拖动进度）
func HandleLocalStream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	filePath := strings.TrimSpace(r.URL.Query().Get("path"))
	if filePath == "" {
		http.Error(w, "path required", http.StatusBadRequest)
		return
	}
	clean, err := localCheckAccess(filePath, false)
	if err != nil {
		fsaccess.Logf("local stream 拒绝: %s (%v)", filePath, err)
		http.Error(w, "无权访问该文件", http.StatusForbidden)
		return
	}
	fsaccess.Logf("local stream: %s", clean)

	fi, err := os.Stat(clean)
	if err != nil || fi.IsDir() {
		fsaccess.Logf("local stream 文件不存在/是目录: %s (%v)", clean, err)
		http.Error(w, "Audio file not found", http.StatusNotFound)
		return
	}

	f, err := os.Open(clean)
	if err != nil {
		fsaccess.Logf("local stream 打开失败: %s (%v)", clean, err)
		http.Error(w, "Failed to open audio: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer f.Close()

	ext := strings.ToLower(filepath.Ext(clean))
	contentType := "application/octet-stream"
	switch ext {
	case ".mp3":
		contentType = "audio/mpeg"
	case ".flac":
		contentType = "audio/flac"
	case ".wav":
		contentType = "audio/wav"
	case ".m4a", ".mp4", ".aac":
		contentType = "audio/mp4"
	case ".ogg":
		contentType = "audio/ogg"
	case ".ape":
		contentType = "audio/x-ape"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Accept-Ranges", "bytes")
	http.ServeContent(w, r, fi.Name(), fi.ModTime(), f)
}

// HandleLocalCover GET /api/local/cover?path=  返回内嵌封面或同目录封面文件
func HandleLocalCover(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	filePath := strings.TrimSpace(r.URL.Query().Get("path"))
	if filePath == "" {
		http.NotFound(w, r)
		return
	}
	clean, err := localCheckAccess(filePath, false)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	data, mimeType := extractCoverBytes(clean)
	if len(data) > 0 {
		w.Header().Set("Content-Type", mimeType)
		w.Header().Set("Cache-Control", "public, max-age=86400")
		_, _ = w.Write(data)
		return
	}
	http.NotFound(w, r)
}

// HandleLocalLyric GET /api/local/lyric?path=  返回本地歌词（.lrc 文件或内嵌）
func HandleLocalLyric(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	filePath := strings.TrimSpace(r.URL.Query().Get("path"))
	if filePath == "" {
		http.Error(w, "path required", http.StatusBadRequest)
		return
	}
	clean, err := localCheckAccess(filePath, false)
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"code": 403, "error": err.Error()})
		return
	}

	lrcText, source, err := ExtractLyric(clean)
	if err != nil || strings.TrimSpace(lrcText) == "" {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code":    404,
			"message": "Lyric not found",
			"data":    nil,
		})
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"code": 200,
		"data": map[string]interface{}{
			"lyric":  lrcText,
			"source": source,
		},
	})
}

// 确保 io 被引用（wavDataSize 用不到时避免编译告警）
var _ = io.EOF
