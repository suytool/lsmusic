import axios from 'axios'

const api = axios.create({
  baseURL: '/api',
  timeout: 30000,
})

export const ConfigAPI = {
  get: () => api.get('/config').then(res => res.data),
  save: (cfg) => api.post('/config', cfg).then(res => res.data),
}

export const SourcesAPI = {
  list: () => api.get('/sources').then(res => res.data),
  getScript: (id) => api.get(`/sources/${encodeURIComponent(id)}/script`).then(res => res.data),
  setActive: (id) => api.post('/sources/active', { id }).then(res => res.data),
  importUrl: (url, name) => api.post('/sources/import_url', { url, name }).then(res => res.data),
  upload: (data) => api.post('/sources/upload', data).then(res => res.data),
  delete: (id) => api.delete(`/sources/${encodeURIComponent(id)}`).then(res => res.data),
}

export const ChartsAPI = {
  toplists: (source = 'kw') => api.get('/charts/toplists', { params: { source } }).then(res => res.data),
  chartDetail: (id, source = 'kw', name = '') => api.get('/charts/detail', { params: { id, source, name } }).then(res => res.data),
  playlists: (category = '全部', page = 1, limit = 24, source = 'kw') => 
    api.get('/charts/playlists', { params: { category, page, limit, source } }).then(res => res.data),
  playlistDetail: (id, source = 'kw', name = '') => api.get('/charts/playlist/detail', { params: { id, source, name } }).then(res => res.data),
}

export const DownloadAPI = {
  getConfig: () => api.get('/download/config').then(res => res.data),
  saveConfig: (settings) => api.post('/download/config', settings).then(res => res.data),
  downloadSong: (song) => api.post('/download/song', song).then(res => res.data),
  downloadBatch: (songs) => api.post('/download/batch', { songs }).then(res => res.data),
  getBatchStatus: (taskId) => api.get('/download/batch/status', { params: { task_id: taskId } }).then(res => res.data),
  checkDownloaded: (songs) => api.post('/download/check', { songs }).then(res => res.data),
}

export const SearchAPI = {
  search: (keyword, source = 'all', page = 1) => 
    api.get('/search', { params: { q: keyword, source, page } }).then(res => res.data),
  lyric: (source, songmid, title = '', singer = '', duration = 0, hash = '') => 
    api.get('/search/lyric', { params: { source, songmid, title, singer, duration, hash } }).then(res => res.data),
}

export const NasAPI = {
  directories: () => api.get('/nas/directories').then(res => res.data),
  browse: (path = '') => api.get('/nas/browse', { params: { path } }).then(res => res.data),
  scan: (dir, refresh = false) => 
    api.get('/nas/scan', { params: { dir, refresh } }).then(res => res.data),
  streamUrl: (path) => `/api/nas/stream?path=${encodeURIComponent(path)}`,
  coverUrl: (path) => `/api/nas/cover?path=${encodeURIComponent(path)}`,
  lyric: (path) => api.get('/nas/lyric', { params: { path } }).then(res => res.data),
}

// 本地音乐模块（v1.1.2）：扫描 / 去重删除 / 批量重命名 / 写标签（含联网自动补全）/ 本地播放
export const LocalAPI = {
  scan: (dir) => api.get('/local/scan', { params: { dir } }).then(res => res.data),
  delete: (paths) => api.post('/local/delete', { paths }).then(res => res.data),
  rename: (items) => api.post('/local/rename', { items }).then(res => res.data),
  tags: (payload) => api.post('/local/tags', payload).then(res => res.data),
  autotag: (payload) => api.post('/local/autotag', payload).then(res => res.data),
  streamUrl: (path) => `/api/local/stream?path=${encodeURIComponent(path)}`,
  coverUrl: (path) => `/api/local/cover?path=${encodeURIComponent(path)}`,
  lyric: (path) => api.get('/local/lyric', { params: { path } }).then(res => res.data),
}

export const ProxyAPI = {
  request: (options) => api.post('/proxy/http', options).then(res => res.data),
}

export const AppAPI = {
  checkUpdate: (force = false) => api.get('/app/check_update', { params: { force: force ? 1 : 0 } }).then(res => res.data),
  getVersion: () => api.get('/app/version').then(res => res.data),
}

export default api
