package downloader

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"fn-lx-player/pkg/config"
	"fn-lx-player/pkg/fsaccess"
	"fn-lx-player/pkg/tagwriter"
)

var illegalChars = regexp.MustCompile(`[\\/:*?"<>|\r\n\t]`)

func sanitizeFilename(s string) string {
	s = illegalChars.ReplaceAllString(s, "_")
	s = strings.TrimSpace(s)
	if s == "" {
		s = "未知"
	}
	return s
}

type SongPayload struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Singer    string `json:"singer"`
	Album     string `json:"album"`
	Cover     string `json:"cover"`
	Source    string `json:"source"`
	Songmid   string `json:"songmid"`
	URL       string `json:"url"`
	StreamURL string `json:"streamUrl"`
	Referer   string `json:"referer"`
	// Download options
	Quality     string `json:"quality"`      // "128k" / "320k" / "flac" / "hires" / "master"
	EmbedLyrics *bool  `json:"embed_lyrics"` // pointer to distinguish unset from false
	EmbedCover  *bool  `json:"embed_cover"`
	NamingFormat string `json:"naming_format"`
	Lyrics      string `json:"lyrics"` // pre-fetched lyrics text
}

// GetStreamReferer determines the appropriate Referer header for music streaming and downloading
func GetStreamReferer(rawURL, customReferer string) string {
	if customReferer != "" {
		return customReferer
	}
	lower := strings.ToLower(rawURL)
	if strings.Contains(lower, "qq.com") {
		return "https://y.qq.com/"
	}
	if strings.Contains(lower, "126.net") || strings.Contains(lower, "163.com") {
		return "https://music.163.com/"
	}
	if strings.Contains(lower, "kuwo.cn") {
		return "http://www.kuwo.cn/"
	}
	if strings.Contains(lower, "kugou.com") {
		return "https://www.kugou.com/"
	}
	if strings.Contains(lower, "migu.cn") {
		return "https://music.migu.cn/"
	}
	return ""
}

type BatchTask struct {
	ID          string        `json:"id"`
	Total       int           `json:"total"`
	Completed   int           `json:"completed"`
	Failed      int           `json:"failed"`
	CurrentSong string        `json:"current_song"`
	Status      string        `json:"status"` // "running", "finished"
	Songs       []SongPayload `json:"-"`
	Results     []SongResult  `json:"results"`
}

type SongResult struct {
	SongName string `json:"song_name"`
	Singer   string `json:"singer"`
	Status   string `json:"status"` // "success", "already_exists", "failed"
	Path     string `json:"path"`
	Error    string `json:"error,omitempty"`
}

type Downloader struct {
	cfgMgr        *config.ConfigManager
	httpClient    *http.Client
	mu            sync.RWMutex
	batchTasks    map[string]*BatchTask
	downloadsPath string               // 下载记录文件（精确记录已下载歌曲 id，避免同歌多版本状态串扰）
	downloadedIDs map[string]bool
}

func NewDownloader(cfgMgr *config.ConfigManager) *Downloader {
	d := &Downloader{
		cfgMgr: cfgMgr,
		httpClient: &http.Client{
			Timeout: 45 * time.Second,
		},
		batchTasks:    make(map[string]*BatchTask),
		downloadsPath: filepath.Join(cfgMgr.DataDir(), "downloads.json"),
		downloadedIDs: make(map[string]bool),
	}
	d.loadDownloads()
	return d
}

// loadDownloads 加载已下载歌曲 id 记录（精确状态判断基础）
func (d *Downloader) loadDownloads() {
	data, err := os.ReadFile(d.downloadsPath)
	if err != nil {
		return
	}
	var ids map[string]bool
	if json.Unmarshal(data, &ids) == nil && ids != nil {
		d.downloadedIDs = ids
	}
}

// markIDDownloaded 持久化记录某歌曲 id 已下载
func (d *Downloader) markIDDownloaded(id string) {
	if id == "" {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.downloadedIDs[id] = true
	data, _ := json.Marshal(d.downloadedIDs)
	_ = os.WriteFile(d.downloadsPath, data, 0644)
}

// isIDDownloaded 精确查询歌曲 id 是否已下载
func (d *Downloader) isIDDownloaded(id string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.downloadedIDs[id]
}

// getDownloadDir 返回当前生效的下载目录。
// 应用以非 root 用户运行，下载目录必须位于授权目录内；
// 若配置中的下载目录不在授权范围内，自动回退到第一个授权目录。
func (d *Downloader) getDownloadDir() string {
	cfg := d.cfgMgr.Get()
	dir := cfg.DownloadDir
	roots := fsaccess.AllowedRoots()
	if dir == "" || !fsaccess.IsPathWithinRoots(dir, roots) {
		if len(roots) > 0 {
			dir = roots[0].Path
		} else {
			dir = cfg.DefaultNasDir
		}
	}
	if dir == "" {
		dir = "/vol1/Music"
	}
	return filepath.Clean(dir)
}

// getEffectiveQuality returns the quality to use: payload override or config default
func (d *Downloader) getEffectiveQuality(song SongPayload) string {
	if song.Quality != "" {
		return strings.ToLower(song.Quality)
	}
	cfg := d.cfgMgr.Get()
	q := strings.ToLower(cfg.DownloadQuality)
	if q == "" {
		q = "mp3"
	}
	return q
}

func (d *Downloader) getEffectiveEmbedLyrics(song SongPayload) bool {
	if song.EmbedLyrics != nil {
		return *song.EmbedLyrics
	}
	return d.cfgMgr.Get().EmbedLyrics
}

func (d *Downloader) getEffectiveEmbedCover(song SongPayload) bool {
	if song.EmbedCover != nil {
		return *song.EmbedCover
	}
	return d.cfgMgr.Get().EmbedCover
}

func (d *Downloader) getEffectiveNamingFormat(song SongPayload) string {
	if song.NamingFormat != "" {
		return song.NamingFormat
	}
	cfg := d.cfgMgr.Get()
	if cfg.NamingFormat != "" {
		return cfg.NamingFormat
	}
	return "{singer} - {name}"
}

// buildFilename generates the filename (without extension) based on naming format.
// Supports subdirectories via "/" in the format.
// Returns the relative path (may contain subdirectories).
func buildFilename(song SongPayload, format string) string {
	cleanSinger := sanitizeFilename(song.Singer)
	cleanName := sanitizeFilename(song.Name)
	cleanAlbum := sanitizeFilename(song.Album)
	cleanSource := sanitizeFilename(song.Source)

	result := format
	result = strings.ReplaceAll(result, "{singer}", cleanSinger)
	result = strings.ReplaceAll(result, "{artist}", cleanSinger)
	result = strings.ReplaceAll(result, "{name}", cleanName)
	result = strings.ReplaceAll(result, "{title}", cleanName)
	result = strings.ReplaceAll(result, "{album}", cleanAlbum)
	result = strings.ReplaceAll(result, "{source}", cleanSource)
	result = strings.ReplaceAll(result, "{platform}", cleanSource)

	// Clean each path segment of illegal chars (already done via replacements, but ensure no trailing issues)
	return strings.TrimSpace(result)
}

func (d *Downloader) HandleGetConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	cfg := d.cfgMgr.Get()
	dir := d.getDownloadDir()
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"code":             200,
		"download_dir":     dir,
		"default_nas_dir":  cfg.DefaultNasDir,
		"download_quality": cfg.DownloadQuality,
		"embed_lyrics":     cfg.EmbedLyrics,
		"embed_cover":      cfg.EmbedCover,
		"naming_format":    cfg.NamingFormat,
		"allowed_roots":    fsaccess.RootPaths(),
	})
}

func (d *Downloader) HandleSetConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		DownloadDir    string `json:"download_dir"`
		DownloadQuality string `json:"download_quality"`
		EmbedLyrics    *bool  `json:"embed_lyrics"`
		EmbedCover     *bool  `json:"embed_cover"`
		NamingFormat   string `json:"naming_format"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	cfg := d.cfgMgr.Get()

	// Update download dir if provided（只能设置为授权目录内的路径）
	if req.DownloadDir != "" {
		targetDir := strings.TrimSpace(req.DownloadDir)
		roots := fsaccess.AllowedRoots()
		if !fsaccess.IsPathWithinRoots(targetDir, roots) {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code":    403,
				"message": "下载目录不在应用授权范围内，请在飞牛「应用设置 → 访问权限」中授权该文件夹",
			})
			return
		}
		if !fsaccess.IsWritableDir(targetDir) {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code":    403,
				"message": "下载目录不可写，请选择应用设置中已授权的文件夹",
			})
			return
		}
		_ = os.MkdirAll(targetDir, 0755)
		cfg.DownloadDir = targetDir
	}

	// Update quality（多档：流畅128k / 超高320k / 无损flac / 至臻hires / 母带master）
	if req.DownloadQuality != "" {
		q := strings.ToLower(req.DownloadQuality)
		switch q {
		case "128k", "320k", "flac", "hires", "master":
			cfg.DownloadQuality = q
		case "mp3": // 兼容旧配置
			cfg.DownloadQuality = "320k"
		}
	}

	// Update embed options
	if req.EmbedLyrics != nil {
		cfg.EmbedLyrics = *req.EmbedLyrics
	}
	if req.EmbedCover != nil {
		cfg.EmbedCover = *req.EmbedCover
	}

	// Update naming format
	if req.NamingFormat != "" {
		cfg.NamingFormat = req.NamingFormat
	}

	if err := d.cfgMgr.Update(cfg); err != nil {
		http.Error(w, "failed to update config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"code":             200,
		"message":          "ok",
		"download_dir":     cfg.DownloadDir,
		"download_quality": cfg.DownloadQuality,
		"embed_lyrics":     cfg.EmbedLyrics,
		"embed_cover":      cfg.EmbedCover,
		"naming_format":    cfg.NamingFormat,
	})
}

func (d *Downloader) HandleDownloadSong(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var song SongPayload
	if err := json.NewDecoder(r.Body).Decode(&song); err != nil {
		http.Error(w, "invalid song payload", http.StatusBadRequest)
		return
	}

	res, err := d.downloadSingleSong(song)
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code":    500,
			"message": err.Error(),
			"result":  res,
		})
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    200,
		"message": "success",
		"data":    res,
		"result":  res,
	})
}

func (d *Downloader) downloadSingleSong(song SongPayload) (*SongResult, error) {
	destDir := d.getDownloadDir()
	quality := d.getEffectiveQuality(song)
	namingFormat := d.getEffectiveNamingFormat(song)
	embedLyrics := d.getEffectiveEmbedLyrics(song)
	embedCover := d.getEffectiveEmbedCover(song)

	// 防御性校验：下载目录必须可写且在授权范围内
	if !fsaccess.IsWritableDir(destDir) {
		errMsg := "下载目录不可写（" + destDir + "）。请先在飞牛「应用设置 → 访问权限」中授权下载文件夹，再在应用「下载设置」中重新选择下载目录。"
		return &SongResult{
			SongName: song.Name,
			Singer:   song.Singer,
			Status:   "failed",
			Error:    errMsg,
		}, fmt.Errorf("%s", errMsg)
	}

	// Determine file extension（无损档统一 .flac：flac / hires / master）
	ext := ".mp3"
	if quality == "flac" || quality == "hires" || quality == "master" {
		ext = ".flac"
	}

	// Build filename from naming format (may include subdirectories)
	relPath := buildFilename(song, namingFormat)
	fullPath := filepath.Join(destDir, relPath+ext)

	// Ensure subdirectories exist
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return &SongResult{
			SongName: song.Name,
			Singer:   song.Singer,
			Status:   "failed",
			Error:    "创建下载目录失败: " + err.Error(),
		}, err
	}

	// Check if already exists and size > 500KB
	if fi, err := os.Stat(fullPath); err == nil && fi.Size() > 500*1024 {
		return &SongResult{
			SongName: song.Name,
			Singer:   song.Singer,
			Status:   "already_exists",
			Path:     filepath.ToSlash(fullPath),
		}, nil
	}

	// Resolve download audio URL
	audioURL := song.URL
	if audioURL == "" || strings.Contains(audioURL, "notice") || strings.Contains(audioURL, "panspace") {
		if song.StreamURL != "" && strings.HasPrefix(song.StreamURL, "http") {
			audioURL = song.StreamURL
		}
	}

	if audioURL == "" || strings.Contains(audioURL, "notice") || strings.Contains(audioURL, "panspace") {
		return &SongResult{
			SongName: song.Name,
			Singer:   song.Singer,
			Status:   "failed",
			Error:    "未提供有效音频流地址，请先通过第三方音源解析后再发起下载",
		}, fmt.Errorf("未提供有效音频流地址")
	}

	// Download to temp file
	tmpPath := fullPath + ".downloading"
	req, err := http.NewRequest("GET", audioURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	if ref := GetStreamReferer(audioURL, song.Referer); ref != "" {
		req.Header.Set("Referer", ref)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("下载请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP 错误: %d", resp.StatusCode)
	}

	out, err := os.Create(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("创建临时文件失败: %w", err)
	}

	written, err := io.Copy(out, resp.Body)
	_ = out.Close()
	if err != nil {
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("写入文件失败: %w", err)
	}

	if written < 300*1024 {
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("下载文件过小 (可能为提示音频或无效流)")
	}

	// Rename temp to final
	if err := os.Rename(tmpPath, fullPath); err != nil {
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("保存最终文件失败: %w", err)
	}

	// 精确登记已下载（供搜索结果按 id 判断，避免同歌多版本全部显示已下载）
	d.markIDDownloaded(song.ID)

	// Embed metadata (lyrics + cover) into the audio file
	if embedLyrics || embedCover {
		tagData := tagwriter.TagData{
			Title:  song.Name,
			Artist: song.Singer,
			Album:  song.Album,
		}
		if embedLyrics && song.Lyrics != "" {
			tagData.Lyrics = song.Lyrics
		}
		if embedCover && song.Cover != "" && strings.HasPrefix(song.Cover, "http") {
			if coverBytes, err := d.downloadCoverBytes(song.Cover, song.Source); err == nil && len(coverBytes) > 0 {
				tagData.CoverJPEG = coverBytes
			} else {
				fmt.Printf("[TAGGER] Cover download failed for %s: %v\n", song.Name, err)
			}
		}
		// Always write tags when embed is enabled (at least title/artist/album)
		if err := tagwriter.WriteTags(fullPath, tagData); err != nil {
			fmt.Printf("[TAGGER] Failed to write tags for %s: %v\n", fullPath, err)
		} else {
			fmt.Printf("[TAGGER] Tags written for %s (lyrics=%v, cover=%v)\n", song.Name, tagData.Lyrics != "", len(tagData.CoverJPEG) > 0)
		}
	}

	return &SongResult{
		SongName: song.Name,
		Singer:   song.Singer,
		Status:   "success",
		Path:     filepath.ToSlash(fullPath),
	}, nil
}

// downloadCoverBytes downloads cover image and returns raw bytes + mime type
func (d *Downloader) downloadCoverBytes(coverURL, platform string) ([]byte, error) {
	req, err := http.NewRequest("GET", coverURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	// Use platform-appropriate referer
	if ref := GetStreamReferer(coverURL, ""); ref != "" {
		req.Header.Set("Referer", ref)
	} else if platform == "wy" {
		req.Header.Set("Referer", "https://music.163.com/")
	} else if platform == "tx" {
		req.Header.Set("Referer", "https://y.qq.com/")
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cover HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (d *Downloader) HandleCheckDownloaded(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Songs []SongPayload `json:"songs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	destDir := d.getDownloadDir()
	cfg := d.cfgMgr.Get()
	namingFormat := cfg.NamingFormat
	if namingFormat == "" {
		namingFormat = "{singer} - {name}"
	}
	quality := strings.ToLower(cfg.DownloadQuality)
	if quality == "" {
		quality = "mp3"
	}
	ext := ".mp3"
	if quality == "flac" || quality == "hires" || quality == "master" {
		ext = ".flac"
	}

	existsMap := make(map[string]bool)

	for _, s := range req.Songs {
		if s.ID != "" {
			// 精确判断：只认下载记录中的歌曲 id（同歌不同平台/版本互不影响）
			existsMap[s.ID] = d.isIDDownloaded(s.ID)
			continue
		}
		// 无 id 兜底：按文件名存在性判断（键含 source 以区分平台）
		relPath := buildFilename(s, namingFormat)
		fullPath := filepath.Join(destDir, relPath+ext)
		key := fmt.Sprintf("%s - %s - %s", s.Singer, s.Name, s.Source)
		if fi, err := os.Stat(fullPath); err == nil && fi.Size() > 300*1024 {
			existsMap[key] = true
		} else {
			existsMap[key] = false
		}
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"code":   200,
		"exists": existsMap,
	})
}

func (d *Downloader) HandleDownloadBatch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Songs []SongPayload `json:"songs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Songs) == 0 {
		http.Error(w, "empty songs list", http.StatusBadRequest)
		return
	}

	taskID := fmt.Sprintf("task_%d", time.Now().UnixNano())
	task := &BatchTask{
		ID:        taskID,
		Total:     len(req.Songs),
		Completed: 0,
		Failed:    0,
		Status:    "running",
		Songs:     req.Songs,
		Results:   make([]SongResult, 0, len(req.Songs)),
	}

	d.mu.Lock()
	d.batchTasks[taskID] = task
	d.mu.Unlock()

	go d.processBatchTask(task)

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    200,
		"task_id": taskID,
		"message": "Batch download started",
		"total":   len(req.Songs),
	})
}

func (d *Downloader) processBatchTask(task *BatchTask) {
	sem := make(chan struct{}, 3) // Concurrency 3
	var wg sync.WaitGroup

	for _, song := range task.Songs {
		sem <- struct{}{}
		wg.Add(1)

		d.mu.Lock()
		task.CurrentSong = fmt.Sprintf("%s - %s", song.Singer, song.Name)
		d.mu.Unlock()

		go func(s SongPayload) {
			defer func() {
				<-sem
				wg.Done()
			}()

			res, err := d.downloadSingleSong(s)
			d.mu.Lock()
			defer d.mu.Unlock()
			if err != nil {
				task.Failed++
				if res != nil {
					task.Results = append(task.Results, *res)
				}
			} else {
				task.Completed++
				if res != nil {
					task.Results = append(task.Results, *res)
				}
			}
		}(song)
	}

	wg.Wait()

	d.mu.Lock()
	task.Status = "finished"
	task.CurrentSong = "全部下载完成"
	d.mu.Unlock()
}

func (d *Downloader) HandleBatchStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	taskID := r.URL.Query().Get("task_id")
	if taskID == "" {
		http.Error(w, "task_id required", http.StatusBadRequest)
		return
	}

	d.mu.RLock()
	task, exists := d.batchTasks[taskID]
	d.mu.RUnlock()

	if !exists {
		http.Error(w, "task not found", http.StatusNotFound)
		return
	}

	d.mu.RLock()
	defer d.mu.RUnlock()
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"code": 200,
		"data": task,
	})
}
