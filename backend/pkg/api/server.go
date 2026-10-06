package api

import (
	"encoding/json"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"fn-lx-player/pkg/charts"
	"fn-lx-player/pkg/config"
	"fn-lx-player/pkg/downloader"
	"fn-lx-player/pkg/fsaccess"
	"fn-lx-player/pkg/nas"
	"fn-lx-player/pkg/proxy"
	"fn-lx-player/pkg/search"
	"fn-lx-player/pkg/sources"
)

func init() {
	_ = mime.AddExtensionType(".js", "text/javascript; charset=utf-8")
	_ = mime.AddExtensionType(".mjs", "text/javascript; charset=utf-8")
	_ = mime.AddExtensionType(".css", "text/css; charset=utf-8")
	_ = mime.AddExtensionType(".html", "text/html; charset=utf-8")
	_ = mime.AddExtensionType(".svg", "image/svg+xml")
	_ = mime.AddExtensionType(".png", "image/png")
	_ = mime.AddExtensionType(".json", "application/json; charset=utf-8")
}

type Server struct {
	cfgMgr     *config.ConfigManager
	sourcesMgr *sources.Manager
	chartMgr   *charts.ChartManager
	downloader *downloader.Downloader
	staticFS   fs.FS
}

func NewServer(cfgMgr *config.ConfigManager, sourcesMgr *sources.Manager, staticFS fs.FS) *Server {
	return &Server{
		cfgMgr:     cfgMgr,
		sourcesMgr: sourcesMgr,
		chartMgr:   charts.NewChartManager(),
		downloader: downloader.NewDownloader(cfgMgr),
		staticFS:   staticFS,
	}
}

func (s *Server) Router() http.Handler {
	mux := http.NewServeMux()

	// 0. App Info
	mux.HandleFunc("/api/app/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code": 200,
			"data": map[string]interface{}{
				"version": "1.1.10",
				"name":    "LSmusic",
			},
		})
	})

	// 1. Proxy
	mux.HandleFunc("/api/proxy/http", proxy.HandleProxyHTTP)

	// 2. Sources
	mux.HandleFunc("/api/sources", s.sourcesMgr.HandleListSources)
	mux.HandleFunc("/api/sources/script", s.sourcesMgr.HandleGetScript)
	mux.HandleFunc("/api/sources/active", s.sourcesMgr.HandleSetActive)
	mux.HandleFunc("/api/sources/custom", s.sourcesMgr.HandleAddCustom)
	mux.HandleFunc("/api/sources/upload", s.sourcesMgr.HandleAddCustom)
	mux.HandleFunc("/api/sources/import_url", s.sourcesMgr.HandleImportURL)
	mux.HandleFunc("/api/sources/custom/delete", s.sourcesMgr.HandleDeleteCustom)
	mux.HandleFunc("/api/sources/", s.sourcesMgr.HandleSourcesRest)

	// 2.5 Charts & Playlists
	mux.HandleFunc("/api/charts/toplists", s.chartMgr.HandleToplists)
	mux.HandleFunc("/api/charts/detail", s.chartMgr.HandleChartDetail)
	mux.HandleFunc("/api/charts/playlists", s.chartMgr.HandlePlaylists)
	mux.HandleFunc("/api/charts/playlist/detail", s.chartMgr.HandlePlaylistDetail)

	// 2.6 Downloader to NAS
	mux.HandleFunc("/api/download/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			s.downloader.HandleSetConfig(w, r)
		} else {
			s.downloader.HandleGetConfig(w, r)
		}
	})
	mux.HandleFunc("/api/download/song", s.downloader.HandleDownloadSong)
	mux.HandleFunc("/api/download/batch", s.downloader.HandleDownloadBatch)
	mux.HandleFunc("/api/download/batch/status", s.downloader.HandleBatchStatus)
	mux.HandleFunc("/api/download/check", s.downloader.HandleCheckDownloaded)

	// 2.8 本地音乐模块（v1.1.2）
	// 选择文件夹 / 扫描本地音乐 / 去重管理 / 批量重命名 / 写入歌词封面（含联网自动补全）/ 本地播放
	// 全部接口严格限定在 fsaccess 授权目录内，越权一律拒绝
	mux.HandleFunc("/api/local/scan", nas.HandleLocalScan)
	mux.HandleFunc("/api/local/delete", nas.HandleLocalDelete)
	mux.HandleFunc("/api/local/rename", nas.HandleLocalRename)
	mux.HandleFunc("/api/local/tags", nas.HandleLocalTags)
	mux.HandleFunc("/api/local/autotag", nas.HandleLocalAutoTag)
	mux.HandleFunc("/api/local/stream", nas.HandleLocalStream)
	mux.HandleFunc("/api/local/cover", nas.HandleLocalCover)
	mux.HandleFunc("/api/local/lyric", nas.HandleLocalLyric)

	// 2.7 Filesystem - 只允许访问「应用设置 → 访问权限」中授权的文件夹
	// （应用以非 root 专用用户运行，未授权目录即使传入也一律拒绝）
	mux.HandleFunc("/api/fs/roots", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		// 前端「重新检测」时强制重新扫描（新授权文件夹立即生效）
		if r.URL.Query().Get("refresh") == "1" {
			fsaccess.Refresh()
		}
		roots := fsaccess.AllowedRoots()
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code": 200,
			"data": map[string]interface{}{
				"roots": roots,
			},
		})
	})

	mux.HandleFunc("/api/fs/list", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		// 顶层视图刷新时同样支持强制重扫
		if r.URL.Query().Get("refresh") == "1" {
			fsaccess.Refresh()
		}
		roots := fsaccess.AllowedRoots()
		path := r.URL.Query().Get("path")

		if path == "" || path == "/" {
			// 顶层视图：只列出授权目录本身
			dirs := make([]map[string]interface{}, 0, len(roots))
			for _, root := range roots {
				dirs = append(dirs, map[string]interface{}{
					"name":     root.Name,
					"path":     root.Path,
					"volume":   root.Volume,
					"modTime":  root.ModTime,
					"writable": root.Writable,
				})
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 200,
				"data": map[string]interface{}{
					"path":       "/",
					"parent":     "",
					"has_parent": false,
					"root_level": true, // 顶层选择界面（列出全部授权目录）
					"dirs":       dirs,
				},
			})
			return
		}

		// 安全检查：防止路径遍历与越权访问
		if !filepath.IsAbs(path) {
			path = "/" + path
		}
		path = filepath.Clean(path)

		// 仅允许访问授权目录及其子目录
		if !fsaccess.IsPathWithinRoots(path, roots) {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code":  403,
				"error": "无权访问该目录（仅可访问应用设置中已授权的文件夹）",
			})
			return
		}

		info, err := os.Stat(path)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code":  404,
				"error": "目录不存在或无权访问",
			})
			return
		}
		if !info.IsDir() {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code":  400,
				"error": "路径不是目录",
			})
			return
		}

		entries, err := os.ReadDir(path)
		if err != nil {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code":  403,
				"error": "无权访问该目录",
			})
			return
		}

		// 父目录：仅在仍处于授权范围内且不是授权目录本身时返回（不可无限向上）
		parent, hasParent := fsaccess.ParentForList(path, roots)

		var dirs []map[string]interface{}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			name := entry.Name()
			// 过滤隐藏目录与系统目录
			if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "@") {
				continue
			}
			fullPath := filepath.Join(path, name)
			if dirInfo, err := os.Stat(fullPath); err == nil {
				dirs = append(dirs, map[string]interface{}{
					"name":     name,
					"path":     fullPath,
					"modTime":  dirInfo.ModTime().Unix(),
					"writable": fsaccess.IsWritableDir(fullPath),
				})
			}
		}
		// 按名称排序
		sort.Slice(dirs, func(i, j int) bool {
			return dirs[i]["name"].(string) < dirs[j]["name"].(string)
		})
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code": 200,
			"data": map[string]interface{}{
				"path":       path,
				"parent":     parent, // "/" = 可返回顶层选择界面；空串 = 不可再向上
				"has_parent": hasParent,
				"root_level": false,
				"writable":   fsaccess.IsWritableDir(path), // 当前目录是否可写（只读目录不可设为下载目录）
				"dirs":       dirs,
			},
		})
	})

	// 3. Search & Direct Audio Resolver
	mux.HandleFunc("/api/search", search.HandleSearch)
	mux.HandleFunc("/api/search/lyric", search.HandleLyric)
	mux.HandleFunc("/api/player/resolve", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code":  403,
			"error": "本播放器为纯本地播放器容器，不内置在线音源直链解析。请在前端「音源管理」导入第三方音源脚本。",
		})
	})

	mux.HandleFunc("/api/player/stream", handleAudioStream)

	// 5. Config
	mux.HandleFunc("/api/config", s.handleConfig)

	// 6. Robust Embedded Frontend Static files handler
	if s.staticFS != nil {
		fileServer := http.FileServer(http.FS(s.staticFS))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			path := strings.TrimPrefix(r.URL.Path, "/")
			if path == "" {
				path = "index.html"
			}
			f, err := s.staticFS.Open(path)
			if err != nil {
				// SPA fallback to index.html
				indexFile, err := s.staticFS.Open("index.html")
				if err != nil {
					http.NotFound(w, r)
					return
				}
				defer indexFile.Close()
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
				w.Header().Set("Pragma", "no-cache")
				w.Header().Set("Expires", "0")
				_, _ = io.Copy(w, indexFile)
				return
			}
			_ = f.Close()

			if path == "index.html" {
				w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
				w.Header().Set("Pragma", "no-cache")
				w.Header().Set("Expires", "0")
			} else if strings.HasPrefix(path, "assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			fileServer.ServeHTTP(w, r)
		})
	}

	return corsMiddleware(mux)
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	switch r.Method {
	case http.MethodGet:
		cfg := s.cfgMgr.Get()
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code":    200,
			"message": "ok",
			"data":    cfg,
			"config":  cfg,
		})
	case http.MethodPost:
		var patch config.AppConfig
		if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		// 下载目录只能设置为应用设置中已授权的文件夹
		if patch.DownloadDir != "" {
			roots := fsaccess.AllowedRoots()
			if !fsaccess.IsPathWithinRoots(patch.DownloadDir, roots) {
				http.Error(w, "下载目录不在应用授权范围内，请在飞牛「应用设置 → 访问权限」中授权该文件夹", http.StatusForbidden)
				return
			}
			if !fsaccess.IsWritableDir(patch.DownloadDir) {
				http.Error(w, "下载目录不可写，请选择应用设置中已授权的文件夹", http.StatusForbidden)
				return
			}
		}
		oldPort := s.cfgMgr.Get().Port
		if err := s.cfgMgr.Update(patch); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		cfg := s.cfgMgr.Get()
		portChanged := patch.Port > 0 && patch.Port < 65536 && oldPort != cfg.Port
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code":         200,
			"message":      "ok",
			"data":         cfg,
			"port_changed": portChanged,
		})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Range, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// 旧的 scanAccessibleRoots 已被 fsaccess 包取代：
// 应用以非 root 专用用户运行后，仅「应用设置 → 访问权限」中授权的目录
// 对应用可读可写，由 fsaccess.DiscoverAllowedRoots 统一探测与约束。

var streamClient = &http.Client{
	Timeout: 30 * time.Second,
}

func handleAudioStream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Range, Content-Type, Accept")
	w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Range, Accept-Ranges")
	w.Header().Set("Accept-Ranges", "bytes")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	targetURL := strings.TrimSpace(r.URL.Query().Get("url"))
	if targetURL == "" {
		http.Error(w, "url required", http.StatusBadRequest)
		return
	}

	outReq, err := http.NewRequest(r.Method, targetURL, nil)
	if err != nil {
		http.Error(w, "invalid url", http.StatusBadRequest)
		return
	}
	outReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36")

	customReferer := strings.TrimSpace(r.URL.Query().Get("referer"))
	if ref := downloader.GetStreamReferer(targetURL, customReferer); ref != "" {
		outReq.Header.Set("Referer", ref)
	}

	if rangeHeader := r.Header.Get("Range"); rangeHeader != "" {
		outReq.Header.Set("Range", rangeHeader)
	}

	resp, err := streamClient.Do(outReq)
	if err != nil {
		http.Error(w, "stream error: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Range, Content-Type, Accept")
	w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Range, Accept-Ranges")
	w.Header().Set("Accept-Ranges", "bytes")

	if ct := resp.Header.Get("Content-Type"); ct != "" {
		w.Header().Set("Content-Type", ct)
	} else {
		w.Header().Set("Content-Type", "audio/mpeg")
	}
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		w.Header().Set("Content-Length", cl)
	}
	if cr := resp.Header.Get("Content-Range"); cr != "" {
		w.Header().Set("Content-Range", cr)
	}

	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

