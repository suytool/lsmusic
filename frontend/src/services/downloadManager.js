import { ref, computed } from 'vue'
import { DownloadAPI, SearchAPI } from '../api/client'
import { lxRuntime } from '../engine/lx-runtime'

// 任务数据结构：
// {
//   id: string,
//   songKey: string,
//   song: Object,
//   status: 'pending' | 'resolving' | 'downloading' | 'success' | 'failed',
//   error: string,
//   activeSourceId: string,
//   platform: string,
//   addedAt: number,
//   completedAt: number | null
// }

const tasks = ref([])
const downloadedMap = ref({})
const isOpen = ref(false)
const currentDownloadDir = ref('/vol1/Music')

// ── 下载品质模型（多档，单一来源） ──
// key: lx-runtime/音源脚本可识别的品质标识（含自动降级链）
// kbps: 用于估算文件大小；ext: 后端保存文件的扩展名
const QUALITY_OPTIONS = [
  { key: '128k',   name: '流畅', desc: '128kbps MP3 · 文件最小',   kbps: 128,  ext: 'mp3' },
  { key: '320k',   name: '超高', desc: '320kbps MP3 · 高品质',     kbps: 320,  ext: 'mp3' },
  { key: 'flac',   name: '无损', desc: 'FLAC 无损 · CD级音质',     kbps: 1411, ext: 'flac' },
  { key: 'hires',  name: '至臻', desc: 'Hi-Res 高解析 · 源支持时', kbps: 2304, ext: 'flac' },
  { key: 'master', name: '母带', desc: '母带级音质 · 源支持时',    kbps: 4608, ext: 'flac' },
]

// 规范化品质值：兼容旧配置（'mp3' 视为 '320k'），非法值回退 '320k'
function normalizeQuality(q) {
  if (QUALITY_OPTIONS.some(o => o.key === q)) return q
  if (q === 'mp3' || q === '320k' || q === '128k' || q === 'flac' || q === 'hires' || q === 'master') {
    return q === 'mp3' ? '320k' : q
  }
  return '320k'
}

// 查找品质档位定义
function qualityOption(q) {
  return QUALITY_OPTIONS.find(o => o.key === normalizeQuality(q)) || QUALITY_OPTIONS[1]
}

// ── 下载设置状态 ──
const downloadQuality = ref('320k')       // 全局默认品质（流畅/超高/无损/至臻/母带）
const embedLyrics = ref(true)
const embedCover = ref(true)
const namingFormat = ref('{singer} - {name}')
const appPort = ref(8899)                 // 应用监听端口

// 目录选择状态（简化为纯路径输入）
const showDirModal = ref(false)
const editDownloadDir = ref('/vol1/Music')
const isSavingDir = ref(false)

// 下载设置弹窗
const showSettingsModal = ref(false)
const isSavingSettings = ref(false)

// ── 单次下载选项弹窗状态 ──
const showDownloadOptions = ref(false)
const currentOptionSong = ref(null)
const optionQuality = ref('320k')
const optionEmbedLyrics = ref(true)
const optionEmbedCover = ref(true)
const optionNamingFormat = ref('{singer} - {name}')

// 命名格式预设
const namingPresets = [
  { label: '歌手 - 歌曲', format: '{singer} - {name}' },
  { label: '歌曲 - 歌手', format: '{name} - {singer}' },
  { label: '歌手/专辑/歌曲', format: '{singer}/{album}/{name}' },
  { label: '歌手 - 专辑 - 歌曲', format: '{singer} - {album} - {name}' },
  { label: '仅歌曲名', format: '{name}' },
]

const MAX_CONCURRENCY = 2
let runningCount = 0
const isPaused = ref(false)

// 计算属性
const activeCount = computed(() =>
  tasks.value.filter(t => t.status === 'pending' || t.status === 'resolving' || t.status === 'downloading').length
)

const pausedCount = computed(() =>
  tasks.value.filter(t => t.status === 'paused').length
)

const completedCount = computed(() =>
  tasks.value.filter(t => t.status === 'success').length
)

const failedCount = computed(() =>
  tasks.value.filter(t => t.status === 'failed').length
)

const totalCount = computed(() => tasks.value.length)

// 获取唯一标识
function getSongKey(song) {
  return song.id || `${song.singer || '未知'} - ${song.name || '未知'}`
}

// 检查是否在下载中
function isSongDownloading(song) {
  const key = getSongKey(song)
  return tasks.value.some(t =>
    t.songKey === key && (t.status === 'pending' || t.status === 'resolving' || t.status === 'downloading')
  )
}

// 检查是否已下载：按歌曲 id 精确判断（同歌不同平台/版本互不影响），
// 无 id 时兜底用 歌手-歌名-平台 组合键
function isSongDownloaded(song) {
  if (song.id) return !!downloadedMap.value[song.id]
  return !!downloadedMap.value[`${song.singer} - ${song.name} - ${song.source || song.platform || ''}`]
}

// 将品质设置映射为 lx-runtime 可识别的 quality 参数（多档直通，含自动降级）
function mapQualityForResolver(quality) {
  return qualityOption(quality).key
}

// 初始化加载配置
async function loadConfig() {
  try {
    const res = await DownloadAPI.getConfig()
    if (res.code === 200) {
      currentDownloadDir.value = res.download_dir || res.default_nas_dir || '/vol1/Music'
      editDownloadDir.value = currentDownloadDir.value
      if (res.download_quality) downloadQuality.value = normalizeQuality(res.download_quality)
      if (typeof res.embed_lyrics === 'boolean') embedLyrics.value = res.embed_lyrics
      if (typeof res.embed_cover === 'boolean') embedCover.value = res.embed_cover
      if (res.naming_format) namingFormat.value = res.naming_format
      if (res.port && res.port > 0 && res.port < 65536) appPort.value = res.port
    }
  } catch (e) {
    console.warn('Failed to load download config:', e)
  }
}

// 批量查询已下载状态
async function checkSongsDownloaded(songs) {
  if (!songs || !songs.length) return
  try {
    const payload = songs.map(s => ({
      id: s.id || '',
      name: s.name || '',
      singer: s.singer || '',
      source: s.source || s.platform || '',
    }))
    const res = await DownloadAPI.checkDownloaded(payload)
    if (res.code === 200 && res.exists) {
      downloadedMap.value = { ...downloadedMap.value, ...res.exists }
    }
  } catch (e) {
    console.warn('Failed to check download status:', e)
  }
}

// 预取歌词（用于嵌入到音频文件）
async function fetchLyricsForDownload(song, activeSourceId) {
  if (!embedLyrics.value) return ''
  try {
    const sourceId = activeSourceId || localStorage.getItem('fn_active_source_id') || ''
    const platform = song.source || 'kw'

    // 1. 尝试第三方音源脚本获取歌词
    if (sourceId) {
      try {
        const lrc = await lxRuntime.getLyric(sourceId, platform, song)
        if (lrc?.lyric) return lrc.lyric
      } catch (_) {}
    }

    // 2. 通用歌词引擎兜底
    try {
      const duration = song.interval || song.duration || 0
      const hash = song.hash || ''
      const res = await SearchAPI.lyric(platform, song.songmid || song.id, song.name, song.singer, duration, hash)
      if (res.code === 200 && res.data?.lyric) return res.data.lyric
    } catch (_) {}
  } catch (e) {
    console.warn('Failed to fetch lyrics for download:', e)
  }
  return ''
}

// 打开单次下载选项弹窗
function openDownloadOptions(song) {
  currentOptionSong.value = { ...song }
  optionQuality.value = downloadQuality.value
  optionEmbedLyrics.value = embedLyrics.value
  optionEmbedCover.value = embedCover.value
  optionNamingFormat.value = namingFormat.value
  showDownloadOptions.value = true
}

// 确认下载选项并加入队列
function confirmDownloadOptions(activeSourceId) {
  if (!currentOptionSong.value) return
  const options = {
    quality: optionQuality.value,
    embed_lyrics: optionEmbedLyrics.value,
    embed_cover: optionEmbedCover.value,
    naming_format: optionNamingFormat.value,
  }
  // 记住本次选择，作为下次默认值（命名格式/品质/嵌入选项持久化到配置）
  downloadQuality.value = optionQuality.value
  embedLyrics.value = optionEmbedLyrics.value
  embedCover.value = optionEmbedCover.value
  namingFormat.value = optionNamingFormat.value
  persistDownloadDefaults()
  showDownloadOptions.value = false
  addSong(currentOptionSong.value, activeSourceId, false, options)
  currentOptionSong.value = null
}

// 静默保存本次下载选项为全局默认（不弹窗、不打扰用户）
async function persistDownloadDefaults() {
  try {
    await DownloadAPI.saveConfig({
      download_dir: currentDownloadDir.value,
      download_quality: downloadQuality.value,
      embed_lyrics: embedLyrics.value,
      embed_cover: embedCover.value,
      naming_format: namingFormat.value,
      port: parseInt(appPort.value) || 8899,
    })
  } catch (e) {
    console.warn('Failed to persist download defaults:', e)
  }
}

// 添加单曲下载任务
function addSong(song, activeSourceId, autoOpen = true, options = null) {
  const key = getSongKey(song)
  // 如果当前已在下载队列中且未结束，则直接打开面板
  const existing = tasks.value.find(t =>
    t.songKey === key && (t.status === 'pending' || t.status === 'resolving' || t.status === 'downloading')
  )
  if (existing) {
    if (autoOpen) isOpen.value = true
    return existing
  }

  const task = {
    id: `task_${Date.now()}_${Math.random().toString(36).slice(2, 7)}`,
    songKey: key,
    song: { ...song },
    status: 'pending',
    error: '',
    activeSourceId: activeSourceId || '',
    platform: song.source || 'kw',
    addedAt: Date.now(),
    completedAt: null,
    options: options, // 单次下载选项覆盖
  }

  tasks.value.unshift(task)
  if (autoOpen) {
    isOpen.value = true
  }

  processQueue()
  return task
}

// 批量添加下载任务 (默认 autoOpen 为 false，静默后台加入队列)
function addBatch(songs, activeSourceId, autoOpen = false) {
  if (!songs || !songs.length) return

  for (const song of songs) {
    const key = getSongKey(song)
    // 已经下载过或已经在队列中的跳过
    if (isSongDownloaded(song)) continue
    if (isSongDownloading(song)) continue

    const task = {
      id: `task_${Date.now()}_${Math.random().toString(36).slice(2, 7)}`,
      songKey: key,
      song: { ...song },
      status: isPaused.value ? 'paused' : 'pending',
      error: '',
      activeSourceId: activeSourceId || '',
      platform: song.source || 'kw',
      addedAt: Date.now(),
      completedAt: null,
      _aborted: false
    }
    tasks.value.push(task)
  }

  if (autoOpen) {
    isOpen.value = true
  }

  if (!isPaused.value) {
    processQueue()
  }
}

// 单个任务暂停
function pauseTask(taskId) {
  const task = tasks.value.find(t => t.id === taskId)
  if (task) {
    task.status = 'paused'
    task._aborted = true
  }
}

// 单个任务继续
function resumeTask(taskId, fallbackSourceId) {
  const task = tasks.value.find(t => t.id === taskId)
  if (task) {
    task.status = 'pending'
    task.error = ''
    task._aborted = false
    if (fallbackSourceId) {
      task.activeSourceId = fallbackSourceId
    }
    isPaused.value = false
    processQueue()
  }
}

// 单个任务取消/停止
function cancelTask(taskId) {
  const task = tasks.value.find(t => t.id === taskId)
  if (task) {
    task._aborted = true
  }
  tasks.value = tasks.value.filter(t => t.id !== taskId)
}

// 全局暂停所有下载
function pauseAll() {
  isPaused.value = true
  tasks.value.forEach(t => {
    if (t.status === 'pending' || t.status === 'resolving' || t.status === 'downloading') {
      t.status = 'paused'
      t._aborted = true
    }
  })
}

// 全局继续所有下载
function resumeAll(fallbackSourceId) {
  isPaused.value = false
  tasks.value.forEach(t => {
    if (t.status === 'paused') {
      t.status = 'pending'
      t.error = ''
      t._aborted = false
      if (fallbackSourceId) {
        t.activeSourceId = fallbackSourceId
      }
    }
  })
  processQueue()
}

// 停止并清空所有进行中与等待中任务
function stopAll() {
  tasks.value.forEach(t => {
    if (t.status !== 'success') {
      t._aborted = true
    }
  })
  tasks.value = tasks.value.filter(t => t.status === 'success')
}

// 重试失败任务
function retryTask(taskId, fallbackSourceId) {
  const task = tasks.value.find(t => t.id === taskId)
  if (task) {
    task.status = 'pending'
    task.error = ''
    task._aborted = false
    if (fallbackSourceId) {
      task.activeSourceId = fallbackSourceId
    }
    isPaused.value = false
    processQueue()
  }
}

// 全部重试
function retryAllFailed(fallbackSourceId) {
  isPaused.value = false
  tasks.value.forEach(t => {
    if (t.status === 'failed') {
      t.status = 'pending'
      t.error = ''
      t._aborted = false
      if (fallbackSourceId) {
        t.activeSourceId = fallbackSourceId
      }
    }
  })
  processQueue()
}

// 移除任务
function removeTask(taskId) {
  cancelTask(taskId)
}

// 清空已完成任务
function clearCompleted() {
  tasks.value = tasks.value.filter(t => t.status !== 'success')
}

// 清空全部非进行中任务
function clearAll() {
  tasks.value = tasks.value.filter(t => t.status === 'resolving' || t.status === 'downloading')
}

// 核心队列调度器（页面切换不会打断）
async function processQueue() {
  if (isPaused.value) return
  if (runningCount >= MAX_CONCURRENCY) return

  // 查找下一个等待中的任务
  const nextTask = tasks.value.find(t => t.status === 'pending')
  if (!nextTask) return

  runningCount++
  executeTask(nextTask).finally(() => {
    runningCount--
    // 继续消费后续队列
    if (!isPaused.value) {
      processQueue()
    }
  })

  // 如果并发还有空余，继续拉起
  if (!isPaused.value && runningCount < MAX_CONCURRENCY) {
    processQueue()
  }
}

// 单个任务执行流程
async function executeTask(task) {
  if (task._aborted || task.status === 'paused') return

  const song = task.song
  const payload = { ...song }

  // 单次下载选项优先，否则用全局设置
  const opts = task.options || {}
  const taskQuality = opts.quality || downloadQuality.value
  const taskEmbedLyrics = opts.embed_lyrics !== undefined ? opts.embed_lyrics : embedLyrics.value
  const taskEmbedCover = opts.embed_cover !== undefined ? opts.embed_cover : embedCover.value
  const taskNamingFormat = opts.naming_format || namingFormat.value

  // 1. 直链解析阶段
  task.status = 'resolving'
  task.error = ''

  let audioUrl = payload.url || payload.streamUrl || ''
  const resolverQuality = mapQualityForResolver(taskQuality)

  if (!audioUrl && payload.source !== 'nas') {
    const sourceId = task.activeSourceId || localStorage.getItem('fn_active_source_id') || ''
    if (!sourceId) {
      task.status = 'failed'
      task.error = '未导入第三方音源脚本，无法获取下载直链'
      return
    }

    try {
      const platform = task.platform || song.source || 'kw'
      const urlRes = await lxRuntime.getMusicUrl(sourceId, platform, song, resolverQuality)
      if (task._aborted || task.status === 'paused') return

      if (urlRes?.url) {
        audioUrl = urlRes.url
        payload.url = audioUrl
        if (urlRes.headers?.Referer) {
          payload.referer = urlRes.headers.Referer
        }
      } else {
        throw new Error('音源返回空下载直链')
      }
    } catch (resolveErr) {
      if (task._aborted || task.status === 'paused') return

      // 尝试备选跨平台检索匹配下载
      try {
        const query = `${song.singer} ${song.name}`.trim()
        const searchRes = await SearchAPI.search(query, 'kw', 1)
        const candidates = searchRes?.data?.list || searchRes?.data?.songs || []
        const matched = candidates.find(c =>
          (c.name.includes(song.name) || song.name.includes(c.name)) &&
          (c.singer.includes(song.singer) || song.singer.includes(c.singer))
        )
        if (matched) {
          const fallbackRes = await lxRuntime.getMusicUrl(sourceId, 'kw', matched, resolverQuality)
          if (fallbackRes?.url) {
            audioUrl = fallbackRes.url
            payload.url = audioUrl
          }
        }
      } catch (_) {}

      if (!audioUrl) {
        if (task._aborted || task.status === 'paused') return
        task.status = 'failed'
        task.error = `音源解析直链失败: ${resolveErr.message || '未能获取到有效流'}`
        return
      }
    }
  }

  if (task._aborted || task.status === 'paused') return

  // 2. 预取歌词（用于嵌入）
  let lyricsText = ''
  if (taskEmbedLyrics) {
    lyricsText = await fetchLyricsForDownload(song, task.activeSourceId)
  }

  if (task._aborted || task.status === 'paused') return

  // 3. 组装下载 payload，带上所有设置
  payload.quality = taskQuality
  payload.embed_lyrics = taskEmbedLyrics
  payload.embed_cover = taskEmbedCover
  payload.naming_format = taskNamingFormat
  if (lyricsText) {
    payload.lyrics = lyricsText
  }

  // 4. 发起 NAS 存储下载阶段
  task.status = 'downloading'

  try {
    const res = await DownloadAPI.downloadSong(payload)
    if (task._aborted || task.status === 'paused') return

    if (res.code === 200) {
      task.status = 'success'
      task.completedAt = Date.now()
      // 登记已下载映射：只按歌曲 id（无 id 用 歌手-歌名-平台），避免同歌多版本状态串扰
      if (song.id) {
        downloadedMap.value[song.id] = true
      } else {
        downloadedMap.value[`${song.singer} - ${song.name} - ${song.source || song.platform || ''}`] = true
      }
    } else {
      task.status = 'failed'
      task.error = res.message || '后端下载失败'
    }
  } catch (dlErr) {
    if (task._aborted || task.status === 'paused') return
    task.status = 'failed'
    task.error = dlErr.message || '网络连接异常'
  }
}

// ── 目录管理（简化为纯路径输入） ──
function openDirModal() {
  editDownloadDir.value = currentDownloadDir.value
  showDirModal.value = true
}

async function saveDownloadDir() {
  const dir = editDownloadDir.value.trim()
  if (!dir) return
  isSavingDir.value = true
  try {
    const res = await DownloadAPI.saveConfig({ download_dir: dir })
    if (res.code === 200) {
      currentDownloadDir.value = dir
      showDirModal.value = false
    } else {
      alert('保存目录失败: ' + res.message)
    }
  } catch (e) {
    alert('保存目录失败: ' + e.message)
  } finally {
    isSavingDir.value = false
  }
}

// ── 下载设置管理 ──
function openSettingsModal() {
  showSettingsModal.value = true
}

async function saveDownloadSettings() {
  isSavingSettings.value = true
  try {
    const res = await DownloadAPI.saveConfig({
      download_dir: currentDownloadDir.value,
      download_quality: downloadQuality.value,
      embed_lyrics: embedLyrics.value,
      embed_cover: embedCover.value,
      naming_format: namingFormat.value,
      port: parseInt(appPort.value) || 8899,
    })
    if (res.code === 200) {
      showSettingsModal.value = false
      // 如果端口改变了，提示用户重启
      if (res.port_changed) {
        setTimeout(() => {
          alert('端口已修改，需要重启应用才能生效。请在飞牛NAS应用管理中停止并重新启动LSmusic。')
        }, 300)
      }
    } else {
      alert('保存设置失败: ' + res.message)
    }
  } catch (e) {
    alert('保存设置失败: ' + e.message)
  } finally {
    isSavingSettings.value = false
  }
}

export const downloadManager = {
  tasks,
  downloadedMap,
  isOpen,
  isPaused,
  currentDownloadDir,

  // 下载设置
  downloadQuality,
  embedLyrics,
  embedCover,
  namingFormat,
  namingPresets,
  appPort,

  // 品质模型
  QUALITY_OPTIONS,
  normalizeQuality,
  qualityOption,

  // 目录弹窗
  showDirModal,
  editDownloadDir,
  isSavingDir,

  // 设置弹窗
  showSettingsModal,
  isSavingSettings,

  // 单次下载选项弹窗
  showDownloadOptions,
  currentOptionSong,
  optionQuality,
  optionEmbedLyrics,
  optionEmbedCover,
  optionNamingFormat,
  openDownloadOptions,
  confirmDownloadOptions,

  activeCount,
  pausedCount,
  completedCount,
  failedCount,
  totalCount,

  open: () => { isOpen.value = true },
  close: () => { isOpen.value = false },
  toggle: () => { isOpen.value = !isOpen.value },

  addSong,
  addBatch,
  pauseTask,
  resumeTask,
  cancelTask,
  pauseAll,
  resumeAll,
  stopAll,
  retryTask,
  retryAllFailed,
  removeTask,
  clearCompleted,
  clearAll,

  isSongDownloading,
  isSongDownloaded,
  checkSongsDownloaded,
  loadConfig,

  // 目录
  openDirModal,
  saveDownloadDir,

  // 设置
  openSettingsModal,
  saveDownloadSettings,
}
