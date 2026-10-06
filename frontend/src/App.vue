<template>
  <div class="flex flex-col h-screen w-screen bg-[#f5f7f9] text-gray-800 overflow-hidden select-none font-sans">

    <!-- ── Immersive Player (shown when playing) ── -->
    <ImmersivePlayer
      v-if="showImmersive && currentSong"
      :current-song="currentSong"
      :is-playing="isPlaying"
      :current-time="currentTime"
      :duration="duration"
      :lyric-data="lyricData"
      :trans-data="transData"
      :playlist-count="playlist.length"
      :play-mode="playMode"
      @update:play-mode="val => playMode = val"
      @collapse="showImmersive = false"
      @toggle-play="togglePlay"
      @prev="playPrev"
      @next="playNext"
      @seek="onSeek"
      @volume-change="onVolumeChange"
    />

    <!-- ── Normal View ── -->
    <!-- Top Navigation Bar -->
    <header class="h-13 bg-white border-b border-gray-100 shadow-sm flex items-center justify-between px-2 sm:px-5 z-20 shrink-0 whitespace-nowrap overflow-hidden">
      <div class="flex items-center min-w-0">
        <nav class="flex items-center gap-1 sm:gap-1.5">
          <button
            v-for="item in navItems"
            :key="item.id"
            @click="currentView = item.id"
            :class="[
              'px-2 sm:px-3 py-1.5 rounded-lg text-xs font-medium transition-all flex items-center gap-1 sm:gap-1.5 whitespace-nowrap shrink-0',
              currentView === item.id
                ? 'bg-emerald-50 text-emerald-600 font-semibold'
                : 'text-gray-500 hover:text-gray-700 hover:bg-gray-50'
            ]"
          >
            <component :is="item.icon" class="w-3.5 h-3.5 shrink-0" />
            <span class="hidden sm:inline">{{ item.name }}</span>
            <span class="sm:hidden">{{ item.shortName }}</span>
          </button>
        </nav>
      </div>

      <div class="flex items-center gap-1.5 sm:gap-2 shrink-0">
        <button
          @click="currentView = 'sources'"
          class="px-2 sm:px-3 py-1.5 rounded-full bg-emerald-50 border border-emerald-200 text-emerald-700 text-xs font-medium flex items-center gap-1 sm:gap-1.5 hover:bg-emerald-100 transition-all max-w-[85px] sm:max-w-none whitespace-nowrap shrink-0"
        >
          <span class="w-1.5 h-1.5 rounded-full shrink-0" :class="activeSourceMeta ? 'bg-emerald-500' : 'bg-amber-400'"></span>
          <span class="truncate text-[11px] sm:text-xs">{{ activeSourceMeta?.name || '音源' }}</span>
        </button>

        <!-- 支持作者按钮 -->
        <button
          @click="showSupportModal = true"
          class="relative p-1.5 sm:px-2.5 sm:py-1.5 rounded-lg border border-gray-200 hover:border-amber-300 text-gray-600 hover:text-amber-600 bg-gray-50 hover:bg-amber-50/60 transition-all flex items-center gap-1.5 shrink-0 cursor-pointer"
          title="支持作者"
        >
          <Heart class="w-4 h-4 text-amber-500" />
          <span class="hidden sm:inline text-xs font-medium">支持作者</span>
        </button>

        <!-- 下载任务独立入口按钮 -->
        <button
          @click="downloadManager.toggle()"
          class="relative p-1.5 sm:px-2.5 sm:py-1.5 rounded-lg border border-gray-200 hover:border-emerald-300 text-gray-600 hover:text-emerald-600 bg-gray-50 hover:bg-emerald-50/60 transition-all flex items-center gap-1.5 shrink-0 cursor-pointer"
          title="下载任务"
        >
          <Download class="w-4 h-4" :class="{ 'text-emerald-600 animate-bounce': downloadManager.activeCount.value > 0 }" />
          <span class="hidden sm:inline text-xs font-medium">下载</span>
          <span
            v-if="downloadManager.activeCount.value > 0"
            class="px-1.5 py-0.2 rounded-full bg-emerald-500 text-white text-[10px] font-bold animate-pulse"
          >
            {{ downloadManager.activeCount.value }}
          </span>
        </button>

        <!-- Open immersive player button -->
        <button
          v-if="currentSong"
          @click="showImmersive = true"
          class="p-1.5 rounded-lg border border-emerald-200 text-emerald-600 bg-emerald-50 hover:bg-emerald-100 transition-all shrink-0"
          title="打开播放器"
        >
          <Disc class="w-4 h-4" />
        </button>
      </div>
    </header>

    <!-- ── 全局下载管理抽屉 ── -->
    <DownloadDrawer :active-source="activeSourceMeta" />

    <!-- ── 全局下载选项弹窗 ── -->
    <DownloadOptionsModal :active-source="activeSourceMeta" />

    <!-- ── 支持作者弹窗 ── -->
    <Teleport to="body">
      <Transition
        enter-active-class="transition-opacity duration-200 ease-out"
        enter-from-class="opacity-0"
        enter-to-class="opacity-100"
        leave-active-class="transition-opacity duration-150 ease-in"
        leave-from-class="opacity-100"
        leave-to-class="opacity-0"
      >
        <div
          v-if="showSupportModal"
          class="fixed inset-0 z-[80] flex items-center justify-center p-4 bg-black/50 backdrop-blur-sm"
          @click.self="showSupportModal = false"
        >
          <Transition
            enter-active-class="transition-all duration-250 ease-out"
            enter-from-class="opacity-0 scale-95 translate-y-4"
            enter-to-class="opacity-100 scale-100 translate-y-0"
            leave-active-class="transition-all duration-150 ease-in"
            leave-from-class="opacity-100 scale-100"
            leave-to-class="opacity-0 scale-95"
          >
            <div class="bg-white rounded-2xl max-w-sm w-full shadow-2xl border border-gray-100 overflow-hidden">
              <div class="px-5 py-4 bg-gradient-to-r from-amber-50 to-orange-50 border-b border-gray-100">
                <div class="flex items-center justify-between">
                  <h3 class="text-sm font-bold text-gray-800 flex items-center gap-2">
                    <Heart class="w-4 h-4 text-amber-500" />
                    支持作者
                  </h3>
                  <button @click="showSupportModal = false" class="text-gray-400 hover:text-gray-600">
                    <X class="w-4 h-4" />
                  </button>
                </div>
                <p class="text-xs text-gray-500 mt-1">如果 LSmusic 对你有帮助，欢迎请作者喝杯咖啡 ☕</p>
              </div>
              <div class="p-5 space-y-4">
                <div class="grid grid-cols-2 gap-4">
                  <div class="text-center">
                    <div class="w-full aspect-square bg-gray-50 rounded-xl border border-gray-200 flex items-center justify-center overflow-hidden">
                      <img src="/wechat-qr.png" alt="微信收款码" class="w-full h-full object-contain" @error="$event.target.style.display='none'; $event.target.parentElement.innerHTML='<div class=&quot;text-center text-gray-400&quot;><div class=&quot;text-2xl mb-1&quot;>💬</div><div class=&quot;text-[10px]&quot;>微信收款码</div></div>'" />
                    </div>
                    <p class="text-xs font-medium text-gray-700 mt-2 flex items-center justify-center gap-1">
                      <span class="w-2 h-2 rounded-full bg-green-500"></span>
                      微信
                    </p>
                  </div>
                  <div class="text-center">
                    <div class="w-full aspect-square bg-gray-50 rounded-xl border border-gray-200 flex items-center justify-center overflow-hidden">
                      <img src="/alipay-qr.png" alt="支付宝收款码" class="w-full h-full object-contain" @error="$event.target.style.display='none'; $event.target.parentElement.innerHTML='<div class=&quot;text-center text-gray-400&quot;><div class=&quot;text-2xl mb-1&quot;>💰</div><div class=&quot;text-[10px]&quot;>支付宝收款码</div></div>'" />
                    </div>
                    <p class="text-xs font-medium text-gray-700 mt-2 flex items-center justify-center gap-1">
                      <span class="w-2 h-2 rounded-full bg-blue-500"></span>
                      支付宝
                    </p>
                  </div>
                </div>
                <div class="bg-amber-50/60 rounded-xl p-3 text-center">
                  <p class="text-[11px] text-gray-500">感谢你的支持！你的鼓励是我持续更新的动力 ❤️</p>
                </div>
              </div>
            </div>
          </Transition>
        </div>
      </Transition>
    </Teleport>

    <!-- ── 全局免责声明与服务条款弹窗 ── -->
    <DisclaimerModal
      v-model="showDisclaimerModal"
      :is-force="isDisclaimerForce"
    />

    <!-- Main Content -->
    <div class="flex flex-1 overflow-hidden">
      <main class="flex-1 overflow-hidden">
        <SearchView
          v-if="currentView === 'search'"
          :active-source="activeSourceMeta"
          :play-mode="playMode"
          @update:play-mode="val => playMode = val"
          @play="playSong"
          @play-all="playAllSongs"
          @navigate="view => currentView = view"
        />
        <SourceManager
          v-else-if="currentView === 'sources'"
          @source-changed="onSourceChanged"
          @open-disclaimer="openDisclaimer(false)"
        />
        <LocalMusic
          v-else-if="currentView === 'local'"
          @play="playSong"
        />
      </main>
    </div>

    <!-- Bottom Player Bar -->
    <PlayerBar
      :current-song="currentSong"
      :is-playing="isPlaying"
      :current-time="currentTime"
      :duration="duration"
      :show-lyrics="false"
      :is-resolving="isResolving"
      :play-mode="playMode"
      @update:play-mode="val => playMode = val"
      @toggle-play="togglePlay"
      @prev="playPrev"
      @next="playNext"
      @seek="onSeek"
      @toggle-lyrics="() => { showImmersive = true }"
      @toggle-fullscreen="() => { showImmersive = true }"
      @toggle-view="v => currentView = v"
      @volume-change="onVolumeChange"
    />

    <audio
      ref="audioRef"
      preload="auto"
      @timeupdate="onTimeUpdate"
      @durationchange="onDurationChange"
      @ended="onEnded"
      @play="isPlaying = true"
      @pause="isPlaying = false"
      @error="onAudioError"
    ></audio>
  </div>
</template>

<script setup>
import { ref, onMounted, watch } from 'vue'
import { lxRuntime } from './engine/lx-runtime'
import { SearchAPI, SourcesAPI } from './api/client'
import SearchView from './components/SearchView.vue'
import SourceManager from './components/SourceManager.vue'
import LocalMusic from './components/LocalMusic.vue'
import AudioVisualizer from './components/AudioVisualizer.vue'
import VinylTurntable from './components/VinylTurntable.vue'
import LyricView from './components/LyricView.vue'
import PlayerBar from './components/PlayerBar.vue'
import ImmersivePlayer from './components/ImmersivePlayer.vue'
import DownloadDrawer from './components/DownloadDrawer.vue'
import DownloadOptionsModal from './components/DownloadOptionsModal.vue'
import DisclaimerModal from './components/DisclaimerModal.vue'
import { downloadManager } from './services/downloadManager'
import { Compass, Cpu, Disc, Download, Heart, Music, X } from 'lucide-vue-next'

const currentView = ref('search')
const showRightPanel = ref(false)
const showImmersive = ref(false)
const stageTab = ref('turntable')

// 免责声明与条款弹窗状态
const showDisclaimerModal = ref(false)
const isDisclaimerForce = ref(false)

// 支持作者弹窗
const showSupportModal = ref(false)

function openDisclaimer(force = false) {
  isDisclaimerForce.value = force
  showDisclaimerModal.value = true
}

const navItems = [
  { id: 'search', name: '发现音乐', shortName: '发现', icon: Compass },
  { id: 'local', name: '本地音乐', shortName: '本地', icon: Music },
  { id: 'sources', name: '音源管理', shortName: '音源', icon: Cpu },
]

const audioRef = ref(null)
const audioEl = ref(null)
const isPlaying = ref(false)
const isResolving = ref(false)
const currentTime = ref(0)
const duration = ref(0)
const currentSong = ref(null)
const playlist = ref([])
const currentIndex = ref(0)
const lyricData = ref('')
const transData = ref('')
const savedSource = localStorage.getItem('fn_active_source_meta')
let initialSource = null
try {
  if (savedSource) initialSource = JSON.parse(savedSource)
} catch (_) {}

const activeSourceMeta = ref(initialSource)

watch(activeSourceMeta, (meta) => {
  if (meta) {
    localStorage.setItem('fn_active_source_meta', JSON.stringify(meta))
  } else {
    localStorage.removeItem('fn_active_source_meta')
  }
}, { deep: true })

const playMode = ref(localStorage.getItem('fn_play_mode') || 'sequence')

watch(playMode, (val) => {
  localStorage.setItem('fn_play_mode', val)
})

// 播放时自动打开沉浸式播放器
watch(isPlaying, (val) => {
  if (val && currentSong.value) {
    showImmersive.value = true
  }
})

onMounted(async () => {
  audioEl.value = audioRef.value
  downloadManager.loadConfig()

  // 0. 首次打开应用强制弹出免责声明与服务条款
  if (!localStorage.getItem('fn_disclaimer_accepted')) {
    setTimeout(() => {
      openDisclaimer(true)
    }, 150)
  }

  // 1. 加载音源
  try {
    const res = await SourcesAPI.list()
    if (res.code === 200) {
      const srcs = res.data.sources || []
      const activeId = res.data.active_id
      const meta = srcs.find(s => s.id === activeId) || srcs[0] || null
      if (meta) {
        activeSourceMeta.value = meta
      } else {
        activeSourceMeta.value = null
      }
      // 预先加载所有已导入的音源脚本，确保主音源失效时能够秒级无缝自动容灾降级！
      for (const s of srcs) {
        try {
          const scriptRes = await SourcesAPI.getScript(s.id)
          if (scriptRes.code === 200) {
            await lxRuntime.loadScript(s, scriptRes.data.script)
          }
        } catch (e) {
          console.warn('Failed to load source:', s.id, e)
        }
      }
    }
  } catch (e) {
    console.warn('Failed to load active source:', e)
  }
})

function onSourceChanged(meta) {
  activeSourceMeta.value = meta
}

function isFakeOrNoticeAudio(url) {
  if (!url || typeof url !== 'string') return true
  const lower = url.toLowerCase()
  return lower.includes('panspace') || lower.includes('notice') || lower.includes('audio_forbidden') || lower.includes('error.mp3')
}

let isSwitchingDirect = false
let currentSessionId = 0
let lastFailedSongId = null

async function playSong(songOrPayload) {
  // 核心体验：点击播放立即自动全屏展示大屏（仿积木音乐）
  showImmersive.value = true

  let song = songOrPayload
  if (songOrPayload && songOrPayload.song) {
    song = songOrPayload.song
    if (songOrPayload.playlist && Array.isArray(songOrPayload.playlist) && songOrPayload.playlist.length > 0) {
      playlist.value = [...songOrPayload.playlist]
      const foundIdx = songOrPayload.index !== undefined ? songOrPayload.index : playlist.value.findIndex(s => s.id === song.id)
      currentIndex.value = foundIdx >= 0 ? foundIdx : 0
    }
  } else if (song) {
    const idx = playlist.value.findIndex(s => s.id === song.id)
    if (idx === -1) {
      playlist.value.push(song)
      currentIndex.value = playlist.value.length - 1
    } else {
      playlist.value[idx] = song
      currentIndex.value = idx
    }
  }

  if (!song) return

  const thisSession = ++currentSessionId
  lastFailedSongId = null

  // 1. 立即停止上一首音频并完全卸载（使用 removeAttribute('src') 避免触发浏览器虚假 error 事件）
  if (audioRef.value) {
    try {
      audioRef.value.pause()
      audioRef.value.removeAttribute('src')
      audioRef.value.load()
    } catch (_) {}
  }
  isPlaying.value = false
  currentTime.value = 0
  duration.value = 0
  lyricData.value = ''
  transData.value = ''
  isResolving.value = true
  currentSong.value = { ...song }

  // 本地音乐：不走在线解析，直接通过本地流接口播放（带 Range 支持）
  if (song.source === 'local' && song.path) {
    isResolving.value = false
    startAudioPlay(`/api/local/stream?path=${encodeURIComponent(song.path)}`, thisSession)
    fetchLocalLyrics(song.path, thisSession)
    return
  }

  const sourceId = activeSourceMeta.value?.id || ''
  if (!sourceId) {
    isResolving.value = false
    const go = confirm('【未导入第三方音源】\n\n本播放器为纯本地播放器容器，不内置任何在线曲库与网络音源。\n\n如需播放网络歌曲，请前往「音源管理」导入第三方音源脚本。\n\n是否立即前往「音源管理」？')
    if (go) {
      currentView.value = 'sources'
    }
    return
  }

  const platform = song.source || 'kw'

  // 3. 立即并行拉取新渠道的对应歌词，无需等待音频直链解析完成
  fetchLyrics(sourceId, platform, song, thisSession)

  let resolvedUrl = ''
  let resolvedHeaders = {}

  try {
    // 主音源解析：瞬时失败自动重试一次（第三方音源上游接口偶发抖动）
    for (let attempt = 0; attempt < 2 && !resolvedUrl; attempt++) {
      if (thisSession !== currentSessionId) break
      try {
        const result = await lxRuntime.getMusicUrl(sourceId, platform, song, song.quality || '320k')
        if (thisSession !== currentSessionId) return
        if (result?.url && /^https?:/.test(result.url) && !isFakeOrNoticeAudio(result.url)) {
          resolvedUrl = result.url
          resolvedHeaders = result.headers || {}
          console.log('[PLAY] Custom source resolved:', resolvedUrl)
        } else if (attempt === 0) {
          console.warn('[PLAY] Custom source returned empty, retrying...')
          await new Promise(r => setTimeout(r, 900))
        }
      } catch (err) {
        console.warn('[PLAY] Custom source resolution failed:', err.message)
        if (attempt === 0) await new Promise(r => setTimeout(r, 900))
      }
    }

    // 智能跨源换源匹配 (Cross-platform song match fallback, 类似洛雪换源播放机制)
    // 当原平台无法解析（例如 WY 接口受限或 VIP 付费独家），自动跨平台匹配 KW、TX 的同名音轨
    if (!resolvedUrl && thisSession === currentSessionId && song?.name) {
      console.log(`[PLAY] 原平台 [${platform}] 解析失败，启动智能跨平台换源匹配: "${song.name}"...`)
      const altPlatforms = ['kw', 'tx', 'kg'].filter(p => p !== platform)
      
      for (const altP of altPlatforms) {
        if (resolvedUrl || thisSession !== currentSessionId) break
        try {
          const cleanName = song.name.replace(/\(.*?\)|（.*?）|\[.*?\]|【.*?】/g, '').trim() || song.name
          const query = `${cleanName} ${song.singer || ''}`.trim()
          const searchRes = await SearchAPI.search(query, altP, 1)
          const candidates = searchRes?.data?.list || searchRes?.list || []
          
          for (let i = 0; i < Math.min(candidates.length, 3); i++) {
            if (resolvedUrl || thisSession !== currentSessionId) break
            const cand = candidates[i]
            try {
              const matchResult = await lxRuntime.getMusicUrl(sourceId, altP, cand, '128k')
              if (matchResult?.url && /^https?:/.test(matchResult.url) && !isFakeOrNoticeAudio(matchResult.url)) {
                resolvedUrl = matchResult.url
                resolvedHeaders = matchResult.headers || {}
                console.log(`[PLAY] 智能跨源匹配成功 [${altP.toUpperCase()}]:`, cand.name, cand.singer, resolvedUrl)
                fetchLyrics(sourceId, altP, cand, thisSession)
                break
              }
            } catch (_) {}
          }
        } catch (e) {
          console.warn(`[PLAY] 跨源检索 [${altP}] 异常:`, e)
        }
      }
    }

    if (thisSession !== currentSessionId) return

    if (resolvedUrl && !isFakeOrNoticeAudio(resolvedUrl)) {
      startAudioPlay(resolvedUrl, thisSession, resolvedHeaders)
    } else {
      const srcName = activeSourceMeta.value?.name || '当前音源'
      alert(`音源【${srcName}】未能解析歌曲《${song.name}》（平台: ${platform.toUpperCase()}）。\n\n可能原因：\n1. 当前生效音源的 ${platform.toUpperCase()} 接口暂时失效或该歌曲受平台版权保护\n2. 建议前往「音源管理」检查生效音源，或切换/导入其他有效音源规则。`)
    }
  } finally {
    if (thisSession === currentSessionId) {
      isResolving.value = false
    }
  }
}

function playAllSongs({ songs, startIndex = 0 }) {
  if (!songs || !songs.length) return
  playlist.value = [...songs]
  if (playMode.value === 'random') {
    currentIndex.value = Math.floor(Math.random() * songs.length)
  } else {
    currentIndex.value = startIndex
  }
  playSong(playlist.value[currentIndex.value])
}

async function fetchLocalLyrics(path, sessionId) {
  try {
    const res = await fetch(`/api/local/lyric?path=${encodeURIComponent(path)}`)
    const data = await res.json()
    if (sessionId !== currentSessionId) return
    if (data.code === 200 && data.data?.lyric) {
      lyricData.value = data.data.lyric
      transData.value = ''
    }
  } catch (e) {
    console.warn('[LYRIC] local lyric failed:', e)
  }
}

async function fetchLyrics(sourceId, platform, song, sessionId) {
  // 1. Try LX custom source lyric if source available
  if (sourceId) {
    try {
      const lrc = await lxRuntime.getLyric(sourceId, platform, song)
      if (sessionId && sessionId !== currentSessionId) return
      if (lrc?.lyric) {
        lyricData.value = lrc.lyric
        transData.value = lrc.tlyric || ''
        return
      }
    } catch (e) {
      console.warn('[LYRIC] Custom source lyric failed:', e)
    }
  }

  // 2. High-precision Universal Lyric Engine (covers 99.9% songs of all platforms)
  try {
    const duration = song.interval || song.duration || 0
    const hash = song.hash || ''
    const res = await SearchAPI.lyric(platform, song.songmid || song.id, song.name, song.singer, duration, hash)
    if (sessionId && sessionId !== currentSessionId) return
    if (res.code === 200 && res.data?.lyric) {
      lyricData.value = res.data.lyric
      transData.value = res.data.tlyric || ''
      console.log('[LYRIC] Universal lyric loaded, length:', res.data.lyric.length)
    }
  } catch (e) {
    console.warn('[LYRIC] Universal lyric failed:', e)
  }
}

let currentRawUrl = ''
let currentPlayingHeaders = {}

function startAudioPlay(url, sessionId, headers = {}) {
  if (sessionId && sessionId !== currentSessionId) return
  if (!audioRef.value) return
  currentRawUrl = url
  currentPlayingHeaders = headers || {}

  let playSrc = url
  // 通过后端流中继代理，彻底消除浏览器跨域CORS、防盗链403以及HTTPS Mixed Content阻断！
  if (playSrc.startsWith('http://') || playSrc.startsWith('https://')) {
    if (!playSrc.includes('/api/player/stream')) {
      let proxyUrl = `/api/player/stream?url=${encodeURIComponent(playSrc)}`
      const ref = headers?.referer || headers?.Referer
      if (ref) {
        proxyUrl += `&referer=${encodeURIComponent(ref)}`
      }
      playSrc = proxyUrl
    }
  }
  audioRef.value.src = playSrc
  audioRef.value.load()

  // 移动端兼容：资源未就绪时立即 play() 会被浏览器拒绝（iOS Safari / 部分安卓 WebView），
  // 失败后自动等待 canplay 或定时重试，最多 3 次；均失败才提示用户
  let playAttempts = 0
  const tryPlay = () => {
    if (!audioRef.value) return
    const p = audioRef.value.play()
    if (p !== undefined) {
      p.then(() => {
        isPlaying.value = true
      }).catch(e => {
        console.warn('Play prevented or error:', e)
        playAttempts++
        if (playAttempts >= 3) {
          isPlaying.value = false
          if (playSrc.includes('/api/local/stream')) {
            alert('本地播放未能启动，请再点一次「播放」')
          }
        } else {
          // 资源可能尚未就绪：稍后重试
          setTimeout(tryPlay, 350 * playAttempts)
        }
      })
    }
  }
  tryPlay()
  audioRef.value.addEventListener('canplay', () => {
    if (audioRef.value && audioRef.value.paused && playAttempts > 0) {
      audioRef.value.play().then(() => { isPlaying.value = true }).catch(() => {})
    }
  }, { once: true })
}

function togglePlay() {
  if (!audioRef.value) return
  if (audioRef.value.paused) {
    audioRef.value.play().then(() => { isPlaying.value = true }).catch(console.warn)
  } else {
    audioRef.value.pause()
    isPlaying.value = false
  }
}

function getRandomIndex() {
  if (playlist.value.length <= 1) return 0
  let nextIdx = currentIndex.value
  while (nextIdx === currentIndex.value) {
    nextIdx = Math.floor(Math.random() * playlist.value.length)
  }
  return nextIdx
}

function playPrev() {
  if (!playlist.value.length) return
  if (playMode.value === 'random') {
    currentIndex.value = getRandomIndex()
  } else {
    currentIndex.value = (currentIndex.value - 1 + playlist.value.length) % playlist.value.length
  }
  playSong(playlist.value[currentIndex.value])
}

function playNext() {
  if (!playlist.value.length) return
  if (playMode.value === 'random') {
    currentIndex.value = getRandomIndex()
  } else {
    currentIndex.value = (currentIndex.value + 1) % playlist.value.length
  }
  playSong(playlist.value[currentIndex.value])
}

function onSeek(time) { if (audioRef.value) audioRef.value.currentTime = time }
function onTimeUpdate() { if (audioRef.value) currentTime.value = audioRef.value.currentTime }

function onDurationChange() {
  if (!audioRef.value) return
  duration.value = audioRef.value.duration || 0
}

async function onAudioError(e) {
  if (isResolving.value) return
  if (!audioRef.value || !audioRef.value.getAttribute('src')) return
  const src = audioRef.value.src || ''
  if (!src || src === window.location.href || src.endsWith('/api/player/stream?url=')) return

  const song = currentSong.value
  if (!song) return

  console.warn('[AUDIO] Audio playback error detected for:', song?.name, 'src:', src, e)

  // 如果后端代理流播放出错，尝试直接降级播放原始直链 URL
  if (src.includes('/api/player/stream') && currentRawUrl && currentRawUrl !== src) {
    console.log('[AUDIO] Proxy stream error, falling back directly to raw URL:', currentRawUrl)
    audioRef.value.src = currentRawUrl
    audioRef.value.load()
    audioRef.value.play().then(() => {
      isPlaying.value = true
    }).catch(err => {
      console.warn('[AUDIO] Direct playback also failed:', err)
      isPlaying.value = false
    })
    return
  }

  isPlaying.value = false

  // 本地音乐流播放失败：给出可见提示，避免“点了没反应”
  if (src.includes('/api/local/stream')) {
    const name = song.name || song.filename || '该歌曲'
    const likely = song.format && !['mp3', 'flac', 'wav', 'm4a', 'aac', 'ogg', 'opus'].includes(String(song.format).toLowerCase())
      ? `（${song.format} 格式浏览器可能不支持播放）` : ''
    alert(`本地播放失败：${name}${likely}\n\n请确认：\n1. 文件未被移动/删除\n2. 应用设置中仍授权了该文件夹\n3. 文件未损坏`)
  }
}
function onEnded() {
  if (playMode.value === 'loop') {
    if (audioRef.value) {
      audioRef.value.currentTime = 0
      audioRef.value.play().catch(console.warn)
    }
    return
  }
  playNext()
}
function onVolumeChange(val) { if (audioRef.value) audioRef.value.volume = val }

function toggleLyrics() {
  if (!showRightPanel.value) { showRightPanel.value = true; stageTab.value = 'lyrics' }
  else stageTab.value = stageTab.value === 'lyrics' ? 'turntable' : 'lyrics'
}

function toggleFullscreen() {
  if (!document.fullscreenElement) document.documentElement.requestFullscreen().catch(console.warn)
  else document.exitFullscreen().catch(console.warn)
}
</script>
