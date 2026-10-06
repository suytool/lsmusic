package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type CustomSource struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Author      string `json:"author"`
	Script      string `json:"script"`
	CreatedAt   int64  `json:"created_at"`
}

type AppConfig struct {
	Port           int            `json:"port"`
	DefaultNasDir  string         `json:"default_nas_dir"`
	DownloadDir    string         `json:"download_dir"`
	ActiveSourceID string         `json:"active_source_id"`
	Theme          string         `json:"theme"`
	VisualizerMode string         `json:"visualizer_mode"`
	PreferQuality  string         `json:"prefer_quality"`
	// Download settings
	DownloadQuality string `json:"download_quality"` // 流畅128k / 超高320k / 无损flac / 至臻hires / 母带master
	EmbedLyrics     bool   `json:"embed_lyrics"`
	EmbedCover      bool   `json:"embed_cover"`
	NamingFormat    string `json:"naming_format"` // e.g. "{singer} - {name}", "{name} - {singer}", "{singer}/{album}/{name}"
	AllowedDirs     []string `json:"allowed_dirs"` // 允许访问的目录列表，为空则允许所有
	CustomSources   []CustomSource `json:"custom_sources"`
}

type ConfigManager struct {
	mu       sync.RWMutex
	filePath string
	config   AppConfig
}

// DataDir 返回配置所在数据目录（下载记录等应用数据存放于此）
func (c *ConfigManager) DataDir() string {
	return filepath.Dir(c.filePath)
}

func NewConfigManager(dataDir string, defaultPort int) (*ConfigManager, error) {
	if dataDir == "" {
		dataDir = "./data"
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		dataDir = "/tmp/fn-lx-player-data"
		_ = os.MkdirAll(dataDir, 0755)
	}

	defaultDir := "/vol1/Music"
	if _, err := os.Stat("/vol1"); err != nil {
		defaultDir = filepath.Join(dataDir, "music")
		_ = os.MkdirAll(defaultDir, 0755)
	}

	cm := &ConfigManager{
		filePath: filepath.Join(dataDir, "config.json"),
		config: AppConfig{
			Port:            defaultPort,
			DefaultNasDir:   defaultDir,
			DownloadDir:     defaultDir,
			ActiveSourceID:  "",
			Theme:           "fresh-mint",
			VisualizerMode:  "bars",
			PreferQuality:   "320k",
			DownloadQuality: "320k",
			EmbedLyrics:     true,
			EmbedCover:      true,
			NamingFormat:    "{singer} - {name}",
			AllowedDirs:     []string{}, // 默认为空，表示允许所有目录,
			CustomSources:   make([]CustomSource, 0),
		},
	}

	cm.load()
	return cm, nil
}

func (cm *ConfigManager) load() {
	data, err := os.ReadFile(cm.filePath)
	if err != nil {
		return
	}
	var c AppConfig
	if err := json.Unmarshal(data, &c); err == nil {
		if c.Port > 0 && c.Port < 65536 {
			cm.config.Port = c.Port
		}
		if c.DefaultNasDir != "" {
			cm.config.DefaultNasDir = c.DefaultNasDir
		}
		if c.DownloadDir != "" {
			cm.config.DownloadDir = c.DownloadDir
		} else if cm.config.DefaultNasDir != "" {
			cm.config.DownloadDir = cm.config.DefaultNasDir
		}
		if c.ActiveSourceID != "" {
			cm.config.ActiveSourceID = c.ActiveSourceID
		}
		if c.Theme != "" {
			cm.config.Theme = c.Theme
		}
		if c.VisualizerMode != "" {
			cm.config.VisualizerMode = c.VisualizerMode
		}
		if c.PreferQuality != "" {
			cm.config.PreferQuality = c.PreferQuality
		}
		if c.DownloadQuality != "" {
			cm.config.DownloadQuality = c.DownloadQuality
		}
		cm.config.EmbedLyrics = c.EmbedLyrics
		cm.config.EmbedCover = c.EmbedCover
		if c.NamingFormat != "" {
			cm.config.NamingFormat = c.NamingFormat
		}
		if c.AllowedDirs != nil {
			cm.config.AllowedDirs = c.AllowedDirs
		}
		if c.CustomSources != nil {
			cm.config.CustomSources = c.CustomSources
		}
	}
}

func (cm *ConfigManager) Get() AppConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.config
}

func (cm *ConfigManager) Update(c AppConfig) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if c.Port > 0 && c.Port < 65536 {
		cm.config.Port = c.Port
	}
	if c.DefaultNasDir != "" {
		cm.config.DefaultNasDir = c.DefaultNasDir
	}
	if c.DownloadDir != "" {
		cm.config.DownloadDir = c.DownloadDir
	}
	if c.ActiveSourceID != "" {
		cm.config.ActiveSourceID = c.ActiveSourceID
	}
	if c.Theme != "" {
		cm.config.Theme = c.Theme
	}
	if c.VisualizerMode != "" {
		cm.config.VisualizerMode = c.VisualizerMode
	}
	if c.PreferQuality != "" {
		cm.config.PreferQuality = c.PreferQuality
	}
	if c.DownloadQuality != "" {
		cm.config.DownloadQuality = c.DownloadQuality
	}
	cm.config.EmbedLyrics = c.EmbedLyrics
	cm.config.EmbedCover = c.EmbedCover
	if c.NamingFormat != "" {
		cm.config.NamingFormat = c.NamingFormat
	}
	if c.AllowedDirs != nil {
		cm.config.AllowedDirs = c.AllowedDirs
	}
	if c.CustomSources != nil {
		cm.config.CustomSources = c.CustomSources
	}

	data, err := json.MarshalIndent(cm.config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cm.filePath, data, 0644)
}
