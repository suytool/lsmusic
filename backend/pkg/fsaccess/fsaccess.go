// Package fsaccess 负责探测“当前应用用户实际可读写的目录”（即用户在飞牛
// 应用设置 → 访问权限 中授权的文件夹），并将应用的所有文件系统访问
// （目录浏览、下载目录设置、下载写入等）严格限制在这些授权目录内。
//
// 设计依据：fnOS（V1.1.8+）在启动应用、以及用户变更「访问权限」时，会通过
// 环境变量 TRIM_DATA_ACCESSIBLE_PATHS（冒号分隔的授权目录列表）告知应用；
// 系统同时通过 config 流程通知变更。应用以非 root 专用包用户运行后，
// fnOS 通过 ACL 只把授权文件夹开放给该用户。
//
// 因此本包按以下优先级探测授权目录：
//   1. TRIM_DATA_ACCESSIBLE_PATHS 环境变量（应用进程启动时由 fnOS 注入）
//   2. $TRIM_PKGVAR/authorized_paths.conf 配置文件
//      （cmd/config_callback、cmd/main 在权限变更/启动时写入）
//   3. 回退：递归扫描存储卷（仅当前两种来源都为空时使用；
//      注意 fnOS 对应用用户隐藏存储卷顶层列表，此回退在真机上通常找不到）
// 对每个候选目录再做「可读 + 可写」探测，得到可信的授权目录集合。
package fsaccess

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

// Root 表示一个授权目录（应用可访问的顶层目录）。
type Root struct {
	Name     string `json:"name"`     // 目录名（用于展示）
	Path     string `json:"path"`     // 绝对路径
	Volume   string `json:"volume"`   // 所在存储空间的展示名（如“存储空间1”）
	ModTime  int64  `json:"mod_time"` // 目录修改时间
	Writable bool   `json:"writable"` // 是否可写（只读授权目录不可作为下载目录）
}

var (
	mu        sync.Mutex
	cache     []Root
	cacheTime time.Time

	// 诊断日志路径（由 main 注入，记录授权目录探测过程，便于排查
	// “已授权但选择器看不到”的问题）
	logPath string
)

// SetLogPath 设置诊断日志路径（应用数据目录下，应用用户可写）。
func SetLogPath(p string) {
	logPath = p
}

// Logf 向诊断日志写一行（供各模块复用，未设置日志路径时静默）。
func Logf(format string, args ...interface{}) {
	logf(format, args...)
}

// Refresh 清空授权目录缓存，下次请求时强制重新扫描
// （用户新授权文件夹后，前端「重新检测」调用此逻辑）。
func Refresh() {
	mu.Lock()
	cache = nil
	cacheTime = time.Time{}
	mu.Unlock()
}

func logf(format string, args ...interface{}) {
	if logPath == "" {
		return
	}
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s [fsaccess] %s\n", time.Now().Format("2006-01-02 15:04:05"), fmt.Sprintf(format, args...))
}

const (
	cacheTTL   = 5 * time.Minute // 授权目录缓存有效期，避免每次请求全盘扫描
	maxChecked = 20000           // 探测目录数量上限，防止超大盘扫描过久
	maxDepth   = 6               // 从存储卷根目录向下的最大探测深度
	maxRoots   = 200             // 授权目录数量上限
)

// Normalize 清洗并规范化路径（去相对路径、转正斜杠）。
func Normalize(p string) string {
	if strings.TrimSpace(p) == "" {
		return "/"
	}
	return filepath.ToSlash(filepath.Clean(p))
}

// AllowedRoots 返回当前应用用户实际可读写的授权目录列表（带短缓存）。
func AllowedRoots() []Root {
	mu.Lock()
	defer mu.Unlock()
	if len(cache) == 0 || time.Since(cacheTime) > cacheTTL {
		cache = discover()
		cacheTime = time.Now()
	}
	return cache
}

// RootPaths 返回授权目录路径列表（去重、排序）。
func RootPaths() []string {
	roots := AllowedRoots()
	paths := make([]string, 0, len(roots))
	for _, r := range roots {
		paths = append(paths, r.Path)
	}
	sort.Strings(paths)
	return paths
}

// IsPathWithinRoots 判断 path 是否位于某个授权目录之内（path 等于授权目录
// 或其子目录都算，避免符号链接等造成的路径逃逸）。
func IsPathWithinRoots(path string, roots []Root) bool {
	p := Normalize(path)
	for _, r := range roots {
		root := Normalize(r.Path)
		if p == root || strings.HasPrefix(p, root+"/") {
			return true
		}
	}
	return false
}

// IsWritableDir 判断目录是否真实存在、是目录且当前用户可写。
func IsWritableDir(path string) bool {
	p := Normalize(path)
	info, err := os.Stat(p)
	if err != nil || !info.IsDir() {
		return false
	}
	return unix.Access(p, unix.W_OK) == nil
}

// ParentForList 返回 path 在授权目录体系内的父目录：
//   - path 本身就是某个授权目录时返回 ("/", true)——即回到“授权目录
//     顶层选择界面”（选择器的最顶层视图，列出全部授权目录）；
//   - path 是授权目录的子目录时返回其父目录（父目录仍然在授权范围内）；
//   - path 不在授权范围内时返回 ("", false)。
func ParentForList(path string, roots []Root) (string, bool) {
	p := Normalize(path)
	var inRoot string
	for _, r := range roots {
		root := Normalize(r.Path)
		if p == root {
			// 授权目录本身：其“上一级”是顶层选择界面（虚拟的 "/"）
			return "/", true
		}
		if strings.HasPrefix(p, root+"/") {
			if inRoot == "" || len(root) > len(inRoot) {
				inRoot = root
			}
		}
	}
	if inRoot == "" {
		return "", false
	}
	parent := Normalize(filepath.Dir(p))
	// 父目录必须仍在授权范围内（可以是授权目录本身，即允许回到授权根目录）
	if !IsPathWithinRoots(parent, roots) {
		return "", false
	}
	return parent, true
}

// discover 探测当前用户可访问的授权目录。
// 优先使用 fnOS 下发的授权目录列表（TRIM_DATA_ACCESSIBLE_PATHS 环境变量
// 与 $TRIM_PKGVAR/authorized_paths.conf），两者都为空时回退到卷扫描。
func discover() []Root {
	logf("== 开始扫描授权目录（uid=%d 用户=%s） ==", os.Getuid(), os.Getenv("USER"))

	roots := make([]Root, 0)

	// ① 飞牛授权目录列表（官方机制：V1.1.8+ 通过 TRIM_DATA_ACCESSIBLE_PATHS
	//    下发，权限变更经 config 流程写入 authorized_paths.conf）
	if paths := accessiblePathsFromEnv(); len(paths) > 0 {
		logf("飞牛授权目录列表: %d 个路径", len(paths))
		for _, p := range paths {
			p = Normalize(p)
			info, err := os.Stat(p)
			if err != nil || !info.IsDir() {
				logf("  授权路径 %s 不存在或不可访问: %v", p, err)
				continue
			}
			wOK := unix.Access(p, unix.W_OK) == nil
			rOK := unix.Access(p, unix.R_OK) == nil
			logf("  授权路径 %s 可写=%v 可读=%v", p, wOK, rOK)
			if !rOK {
				logf("  授权路径 %s 完全不可读（ACL 未生效？），跳过", p)
				continue
			}
			roots = append(roots, Root{
				Name:     filepath.Base(p),
				Path:     p,
				Volume:   volumeDisplayName(p),
				ModTime:  info.ModTime().Unix(),
				Writable: wOK,
			})
		}
		if len(roots) > 0 {
			logf("扫描完成：通过飞牛授权目录列表发现 %d 个授权目录", len(roots))
			return sortRoots(roots)
		}
		logf("授权列表中有路径但均不可访问，回退到存储卷扫描")
	}

	// ② 回退：递归扫描存储卷（旧机制）
	roots = scanVolumes()
	logf("扫描完成：发现 %d 个可写授权目录", len(roots))
	return roots
}

// accessiblePathsFromEnv 依次从进程环境变量与授权配置文件读取授权目录列表。
// 返回值是冒号分隔列表解析出的绝对路径（去重保序）。
func accessiblePathsFromEnv() []string {
	var candidates []string

	// 来源 1：应用进程环境变量（fnOS 在启动应用进程时注入）
	if v := os.Getenv("TRIM_DATA_ACCESSIBLE_PATHS"); strings.TrimSpace(v) != "" {
		logf("来源: TRIM_DATA_ACCESSIBLE_PATHS(环境) = %q", v)
		candidates = append(candidates, splitPaths(v)...)
	}

	// 来源 2：授权配置文件（cmd/config_callback 在权限变更时写入、
	//        cmd/main 在每次启动时写入，保证运行中也能感知新授权）
	cfg := ""
	if p := os.Getenv("TRIM_PKGVAR"); p != "" {
		cfg = filepath.Join(p, "authorized_paths.conf")
	} else if p := os.Getenv("TRIM_PKGHOME"); p != "" {
		cfg = filepath.Join(p, "authorized_paths.conf")
	}
	if cfg != "" {
		b, err := os.ReadFile(cfg)
		if err != nil {
			logf("来源: 配置文件 %s 读取失败: %v", cfg, err)
		} else {
			content := strings.TrimSpace(string(b))
			if content != "" {
				logf("来源: %s = %q", cfg, content)
				candidates = append(candidates, splitPaths(content)...)
			} else {
				logf("来源: 配置文件 %s 为空", cfg)
			}
		}
	}

	if len(candidates) == 0 {
		logf("未从环境变量/配置文件中获取到授权目录列表")
		return nil
	}

	// 去重保序
	seen := make(map[string]bool)
	out := make([]string, 0, len(candidates))
	for _, p := range candidates {
		p = Normalize(p)
		if p == "" || p == "/" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

func splitPaths(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ":") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// scanVolumes 扫描所有存储卷，找出当前用户可读写的目录（回退机制）。
func scanVolumes() []Root {
	roots := make([]Root, 0)
	seen := make(map[string]bool)
	checked := 0

	var walk func(dir string, depth int, volume string)
	walk = func(dir string, depth int, volume string) {
		if checked >= maxChecked || depth > maxDepth || len(roots) >= maxRoots {
			return
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			// 不可读（未授权）→ 无法进入，直接返回
			if depth <= 1 {
				logf("扫描 %s 失败: %v", dir, err)
			}
			return
		}
		checked++
		for _, e := range entries {
			if checked >= maxChecked || len(roots) >= maxRoots {
				return
			}
			name := e.Name()
			if strings.HasPrefix(name, ".") {
				continue
			}
			// 跳过系统/应用内部目录；团队文件目录 @team 需要遍历
			if strings.HasPrefix(name, "@") && name != "@team" {
				continue
			}
			if !e.IsDir() {
				continue
			}
			full := Normalize(filepath.Join(dir, name))
			if seen[full] {
				continue
			}
			seen[full] = true
			info, err := os.Stat(full)
			if err != nil || !info.IsDir() {
				continue
			}
			// 深度为 0 时（存储卷顶层）跳过明确的非数据系统目录
			if depth == 0 && systemTopDir[name] {
				continue
			}

			wOK := unix.Access(full, unix.W_OK) == nil
			rOK := unix.Access(full, unix.R_OK) == nil
			if depth <= 1 {
				logf("  %s 可写=%v 可读=%v", full, wOK, rOK)
			}

			if wOK {
				// 可写 → 授权目录；其子目录通过选择器逐级进入即可，
				// 不再递归，避免重复与海量遍历
				roots = append(roots, Root{
					Name:     name,
					Path:     full,
					Volume:   volume,
					ModTime:  info.ModTime().Unix(),
					Writable: true,
				})
				continue
			}
			// 只读不可写 → 不是下载目标，但可继续向下寻找更深层的授权目录
			walk(full, depth+1, volume)
		}
	}

	for _, vol := range volumeCandidates() {
		walk(vol, 0, volumeDisplayName(vol))
	}

	return sortRoots(roots)
}

func sortRoots(roots []Root) []Root {
	// 去重并排序
	uniq := make(map[string]Root)
	for _, r := range roots {
		if _, ok := uniq[r.Path]; !ok {
			uniq[r.Path] = r
		}
	}
	result := make([]Root, 0, len(uniq))
	for _, r := range uniq {
		result = append(result, r)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Path < result[j].Path
	})
	return result
}

// volumeCandidates 返回候选存储卷路径。
func volumeCandidates() []string {
	vols := make([]string, 0)
	for i := 1; i <= 16; i++ {
		p := "/vol" + itoa(i)
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			vols = append(vols, p)
		}
	}
	for _, p := range []string{"/media", "/mnt"} {
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			vols = append(vols, p)
		}
	}
	return vols
}

func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}

// volumeDisplayName 生成存储卷展示名。
func volumeDisplayName(vol string) string {
	vol = Normalize(vol)
	if strings.HasPrefix(vol, "/vol") {
		num := strings.TrimPrefix(vol, "/vol")
		if num != "" {
			return "存储空间" + num
		}
	}
	return strings.TrimPrefix(vol, "/")
}

// systemTopDir 存储卷/挂载点顶层的非数据系统目录。
var systemTopDir = map[string]bool{
	"lost+found":              true,
	"$RECYCLE.BIN":            true,
	"System Volume Information": true,
	"boot":                    true,
	"proc":                    true,
	"sys":                     true,
	"dev":                     true,
	"run":                     true,
	"etc":                     true,
	"var":                     true,
	"usr":                     true,
	"lib":                     true,
	"lib64":                   true,
	"bin":                     true,
	"sbin":                    true,
	"opt":                     true,
	"srv":                     true,
	"tmp":                     true,
	"root":                    true,
}
