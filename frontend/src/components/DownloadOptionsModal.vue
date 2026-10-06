<template>
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
        v-if="downloadManager.showDownloadOptions.value"
        class="fixed inset-0 z-[70] flex items-center justify-center p-4 bg-black/50 backdrop-blur-sm"
        @click.self="closeModal"
      >
        <Transition
          enter-active-class="transition-all duration-250 ease-out"
          enter-from-class="opacity-0 scale-95 translate-y-4"
          enter-to-class="opacity-100 scale-100 translate-y-0"
          leave-active-class="transition-all duration-150 ease-in"
          leave-from-class="opacity-100 scale-100"
          leave-to-class="opacity-0 scale-95"
        >
          <div
            v-if="downloadManager.showDownloadOptions.value"
            class="bg-white rounded-2xl max-w-md w-full shadow-2xl border border-gray-100 overflow-hidden max-h-[calc(100vh-120px)] flex flex-col"
          >
            <!-- 头部 -->
            <div class="px-5 py-4 bg-gradient-to-r from-emerald-50 to-teal-50 border-b border-gray-100">
              <div class="flex items-center gap-3">
                <div class="w-12 h-12 rounded-xl bg-white shadow-sm overflow-hidden flex items-center justify-center shrink-0">
                  <img
                    v-if="song?.cover"
                    :src="song.cover"
                    referrerpolicy="no-referrer"
                    class="w-full h-full object-cover"
                  />
                  <span v-else class="text-xl">🎵</span>
                </div>
                <div class="flex-1 min-w-0">
                  <h3 class="text-sm font-bold text-gray-800 truncate">{{ song?.name || '下载歌曲' }}</h3>
                  <p class="text-xs text-gray-500 truncate">{{ song?.singer }}{{ song?.album ? ' · ' + song.album : '' }}</p>
                </div>
                <button
                  @click="closeModal"
                  class="p-1.5 rounded-lg text-gray-600 hover:text-gray-600 hover:bg-white/60 transition-colors shrink-0"
                >
                  <X class="w-4 h-4" />
                </button>
              </div>
            </div>

            <!-- 内容区 -->
            <div class="flex-1 overflow-y-auto p-5 space-y-5">
              <!-- 下载品质 -->
              <div>
                <label class="block text-xs font-bold text-gray-700 mb-2 flex items-center gap-1.5">
                  <AudioLines class="w-3.5 h-3.5 text-emerald-500" />
                  下载品质
                </label>
                <div class="grid grid-cols-2 gap-2">
                  <button
                    v-for="opt in downloadManager.QUALITY_OPTIONS"
                    :key="opt.key"
                    @click="downloadManager.optionQuality.value = opt.key"
                    :class="[
                      'p-3 rounded-xl border-2 text-left transition-all relative',
                      downloadManager.optionQuality.value === opt.key
                        ? 'border-emerald-400 bg-emerald-50'
                        : 'border-gray-100 bg-gray-50 hover:border-gray-200'
                    ]"
                  >
                    <div class="flex items-center justify-between mb-0.5">
                      <span class="text-xs font-bold text-gray-800">
                        {{ opt.name }}<span class="text-[9px] text-gray-600 font-normal ml-1">{{ opt.key.toUpperCase() }}</span>
                      </span>
                      <Check v-if="downloadManager.optionQuality.value === opt.key" class="w-3.5 h-3.5 text-emerald-500" />
                    </div>
                    <p class="text-[10px] text-gray-600">{{ opt.desc }}</p>
                    <span class="text-[10px] text-emerald-600 font-medium mt-0.5 block">约 {{ sizeFor(opt) }} MB</span>
                  </button>
                </div>
                <p class="text-[10px] text-gray-600 mt-1.5">文件大小为按码率估算，实际以下载结果为准</p>
              </div>

              <!-- 元数据嵌入 -->
              <div>
                <label class="block text-xs font-bold text-gray-700 mb-2 flex items-center gap-1.5">
                  <Tag class="w-3.5 h-3.5 text-emerald-500" />
                  写入歌曲文件内部
                </label>
                <div class="space-y-2">
                  <div class="flex items-center justify-between p-2.5 rounded-xl bg-gray-50 border border-gray-100">
                    <div class="flex items-center gap-2">
                      <FileText class="w-4 h-4 text-blue-500" />
                      <div>
                        <p class="text-xs font-medium text-gray-700">嵌入歌词</p>
                        <p class="text-[10px] text-gray-600">将歌词写入音频文件标签</p>
                      </div>
                    </div>
                    <div
                      @click="downloadManager.optionEmbedLyrics.value = !downloadManager.optionEmbedLyrics.value"
                      :class="[
                        'w-9 h-5 rounded-full transition-colors relative cursor-pointer shrink-0',
                        downloadManager.optionEmbedLyrics.value ? 'bg-emerald-500' : 'bg-gray-300'
                      ]"
                    >
                      <div
                        :class="[
                          'absolute top-0.5 w-4 h-4 rounded-full bg-white shadow transition-transform',
                          downloadManager.optionEmbedLyrics.value ? 'translate-x-4' : 'translate-x-0.5'
                        ]"
                      ></div>
                    </div>
                  </div>
                  <div class="flex items-center justify-between p-2.5 rounded-xl bg-gray-50 border border-gray-100">
                    <div class="flex items-center gap-2">
                      <Image class="w-4 h-4 text-pink-500" />
                      <div>
                        <p class="text-xs font-medium text-gray-700">嵌入封面</p>
                        <p class="text-[10px] text-gray-600">将封面图片写入音频文件标签</p>
                      </div>
                    </div>
                    <div
                      @click="downloadManager.optionEmbedCover.value = !downloadManager.optionEmbedCover.value"
                      :class="[
                        'w-9 h-5 rounded-full transition-colors relative cursor-pointer shrink-0',
                        downloadManager.optionEmbedCover.value ? 'bg-emerald-500' : 'bg-gray-300'
                      ]"
                    >
                      <div
                        :class="[
                          'absolute top-0.5 w-4 h-4 rounded-full bg-white shadow transition-transform',
                          downloadManager.optionEmbedCover.value ? 'translate-x-4' : 'translate-x-0.5'
                        ]"
                      ></div>
                    </div>
                  </div>
                </div>
              </div>

              <!-- 下载位置 -->
              <div>
                <label class="block text-xs font-bold text-gray-700 mb-2 flex items-center gap-1.5">
                  <Download class="w-3.5 h-3.5 text-emerald-500" />
                  下载位置
                </label>
                <div class="grid grid-cols-2 gap-2">
                  <button
                    @click="downloadTarget = 'nas'"
                    :class="[
                      'p-2.5 rounded-xl border-2 transition-all text-center',
                      downloadTarget === 'nas'
                        ? 'border-emerald-500 bg-emerald-50 text-emerald-700'
                        : 'border-gray-200 bg-gray-50 text-gray-500 hover:border-gray-300'
                    ]"
                  >
                    <HardDrive class="w-4 h-4 mx-auto mb-1" />
                    <p class="text-[11px] font-medium">下载到 NAS</p>
                  </button>
                  <button
                    @click="downloadTarget = 'local'"
                    :class="[
                      'p-2.5 rounded-xl border-2 transition-all text-center',
                      downloadTarget === 'local'
                        ? 'border-emerald-500 bg-emerald-50 text-emerald-700'
                        : 'border-gray-200 bg-gray-50 text-gray-500 hover:border-gray-300'
                    ]"
                  >
                    <Laptop class="w-4 h-4 mx-auto mb-1" />
                    <p class="text-[11px] font-medium">浏览器下载到本地</p>
                  </button>
                </div>
                <p v-if="downloadTarget === 'local'" class="text-[10px] text-amber-600 mt-1.5">本地下载不支持嵌入歌词和封面，仅下载原始音频文件</p>
              </div>

              <!-- 命名格式 -->
              <div>
                <label class="block text-xs font-bold text-gray-700 mb-2 flex items-center gap-1.5">
                  <FileSignature class="w-3.5 h-3.5 text-emerald-500" />
                  文件命名格式
                </label>
                <div class="flex flex-wrap gap-1.5 mb-2">
                  <button
                    v-for="preset in downloadManager.namingPresets" :key="preset.format"
                    @click="downloadManager.optionNamingFormat.value = preset.format"
                    :class="[
                      'px-2 py-1 rounded-lg text-[11px] transition-colors',
                      downloadManager.optionNamingFormat.value === preset.format
                        ? 'bg-emerald-100 text-emerald-700 font-medium'
                        : 'bg-gray-100 text-gray-500 hover:bg-gray-200'
                    ]"
                  >
                    {{ preset.label }}
                  </button>
                </div>
                <input
                  v-model="downloadManager.optionNamingFormat.value"
                  type="text"
                  class="w-full px-3 py-2 rounded-xl bg-gray-50 border border-gray-200 text-xs font-mono text-gray-700 focus:outline-none focus:border-emerald-400 focus:ring-1 focus:ring-emerald-400"
                  placeholder="{singer} - {name}"
                />
                <div class="bg-emerald-50/60 rounded-lg px-2.5 py-1.5">
                  <p class="text-[10px] text-gray-500">预览：<span class="text-emerald-600 font-medium">{{ previewFilename }}.{{ fileExt }}</span></p>
                </div>
              </div>
            </div>

            <!-- 底部按钮 -->
            <div class="px-5 py-3.5 bg-gray-50 border-t border-gray-100 flex gap-2 shrink-0">
              <button
                @click="closeModal"
                class="flex-1 px-4 py-2.5 rounded-xl text-xs text-gray-500 hover:bg-gray-200 font-medium transition-colors"
              >
                取消
              </button>
              <button
                @click="confirmDownload"
                :disabled="localDownloading"
                class="flex-1 px-4 py-2.5 rounded-xl bg-emerald-500 hover:bg-emerald-600 text-white font-bold text-xs shadow-sm transition-colors flex items-center justify-center gap-1.5 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                <Loader2 v-if="localDownloading" class="w-3.5 h-3.5 animate-spin" />
                <Download v-else class="w-3.5 h-3.5" />
                {{ localDownloading ? '下载中...' : '确认下载' }}
              </button>
            </div>
          </div>
        </Transition>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup>
import { ref, computed } from 'vue'
import { downloadManager } from '../services/downloadManager'
import { lxRuntime } from '../engine/lx-runtime'
import {
  X, Download, Check, AudioLines, Tag, FileText, Image,
  FileSignature, Loader2, HardDrive, Laptop
} from 'lucide-vue-next'

const props = defineProps({
  activeSource: Object
})

const song = computed(() => downloadManager.currentOptionSong.value)
const downloadTarget = ref('nas')

// 估算文件大小（按码率：kbps / 8 = KB/s，MB = KB/1024）
const sizeFor = (opt) => {
  const dur = song.value?.interval || song.value?.duration || 0
  if (!dur || dur <= 0) return '?'
  return (dur * (opt.kbps / 8) / 1024).toFixed(1)
}

const fileExt = computed(() => {
  return downloadManager.qualityOption(downloadManager.optionQuality.value).ext
})

const previewFilename = computed(() => {
  const format = downloadManager.optionNamingFormat.value
  const s = song.value || {}
  return format
    .replaceAll('{singer}', s.singer || '未知歌手')
    .replaceAll('{artist}', s.singer || '未知歌手')
    .replaceAll('{name}', s.name || '未知歌曲')
    .replaceAll('{title}', s.name || '未知歌曲')
    .replaceAll('{album}', s.album || '未知专辑')
    .replaceAll('{source}', s.source || 'unknown')
    .replaceAll('{platform}', s.source || 'unknown')
})

function closeModal() {
  downloadManager.showDownloadOptions.value = false
  downloadManager.currentOptionSong.value = null
}

function confirmDownload() {
  if (downloadTarget.value === 'local') {
    downloadToLocal()
  } else {
    downloadManager.confirmDownloadOptions(props.activeSource?.id)
  }
}

// 浏览器下载到本地
const localDownloading = ref(false)
async function downloadToLocal() {
  const s = song.value
  if (!s) return

  localDownloading.value = true
  try {
    const sourceId = props.activeSource?.id || localStorage.getItem('fn_active_source_id') || ''
    if (!sourceId) {
      alert('未导入第三方音源脚本，无法获取下载直链')
      return
    }

    const platform = s.source || s.platform || 'kw'
    const quality = downloadManager.qualityOption(downloadManager.optionQuality.value).key

    const urlInfo = await lxRuntime.getMusicUrl(sourceId, platform, s, quality)
    if (!urlInfo || !urlInfo.url) {
      alert('获取歌曲下载链接失败，请尝试切换音源')
      return
    }

    const ext = downloadManager.qualityOption(downloadManager.optionQuality.value).ext
    let filename = downloadManager.optionNamingFormat.value
      .replaceAll('{singer}', s.singer || '未知歌手')
      .replaceAll('{artist}', s.singer || '未知歌手')
      .replaceAll('{name}', s.name || '未知歌曲')
      .replaceAll('{title}', s.name || '未知歌曲')
      .replaceAll('{album}', s.album || '未知专辑')
      .replaceAll('{source}', s.source || 'unknown')
      .replaceAll('{platform}', s.source || 'unknown')
    filename = `${filename}.${ext}`

    try {
      const proxyRes = await fetch('/api/proxy/http', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          url: urlInfo.url,
          is_binary: true,
          headers: urlInfo.headers || {}
        })
      })
      if (proxyRes.ok) {
        const proxyData = await proxyRes.json()
        if (proxyData.statusCode === 200 && proxyData.body) {
          const binaryString = atob(proxyData.body)
          const bytes = new Uint8Array(binaryString.length)
          for (let i = 0; i < binaryString.length; i++) {
            bytes[i] = binaryString.charCodeAt(i)
          }
          const blob = new Blob([bytes], { type: ext === 'flac' ? 'audio/flac' : 'audio/mpeg' })
          downloadBlob(blob, filename)
          closeModal()
          return
        }
      }
    } catch (_) {}

    try {
      const directRes = await fetch(urlInfo.url)
      if (directRes.ok) {
        const blob = await directRes.blob()
        downloadBlob(blob, filename)
        closeModal()
        return
      }
    } catch (_) {}

    window.open(urlInfo.url, '_blank')
    closeModal()
  } catch (e) {
    console.error('本地下载失败', e)
    alert('本地下载失败：' + e.message)
  } finally {
    localDownloading.value = false
  }
}

function downloadBlob(blob, filename) {
  const downloadUrl = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = downloadUrl
  a.download = filename
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(downloadUrl)
}
</script>
