<template>
  <div class="flex flex-col h-full overflow-hidden">
    <!-- ── 顶部工具栏 ── -->
    <div class="px-4 sm:px-6 py-3 bg-white border-b border-gray-100 flex items-center gap-2 flex-wrap shrink-0">
      <div class="flex items-center gap-2 min-w-0 flex-1">
        <span class="w-1.5 h-1.5 rounded-full bg-emerald-500 shrink-0"></span>
        <div class="min-w-0">
          <p class="text-sm font-semibold text-gray-800 truncate">{{ selectedDir || '本地音乐' }}</p>
          <p class="text-[11px] text-gray-600 truncate">
            {{ selectedDir ? `${songs.length} 首歌曲 · ${dupGroupCount} 组多音质重复` : '选择文件夹后可扫描本地音乐' }}
          </p>
        </div>
      </div>

      <button
        @click="showFolderPicker = true"
        class="px-3 py-1.5 rounded-lg bg-emerald-50 hover:bg-emerald-100 text-emerald-600 text-xs font-medium flex items-center gap-1.5 transition-colors shrink-0"
      >
        <Folder class="w-3.5 h-3.5" />
        <span class="hidden sm:inline">{{ selectedDir ? '更换文件夹' : '选择文件夹' }}</span>
        <span class="sm:hidden">选文件夹</span>
      </button>
      <button
        @click="scan"
        :disabled="!selectedDir || scanning"
        class="px-3 py-1.5 rounded-lg bg-emerald-500 hover:bg-emerald-600 disabled:opacity-40 text-white text-xs font-medium flex items-center gap-1.5 transition-colors shrink-0"
      >
        <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': scanning }" />
        {{ scanning ? '扫描中…' : '扫描' }}
      </button>
      <button
        @click="openRenameModal"
        :disabled="!scanned || songs.length === 0 || scanning"
        class="px-3 py-1.5 rounded-lg bg-white border border-gray-200 hover:border-emerald-300 hover:text-emerald-600 text-gray-600 text-xs font-medium flex items-center gap-1.5 transition-colors shrink-0"
        title="批量重命名（如：歌曲名 - 歌手）"
      >
        <PenLine class="w-3.5 h-3.5" />
        <span class="hidden sm:inline">批量重命名</span>
        <span class="sm:hidden">重命名</span>
      </button>
    </div>

    <!-- ── 未选择文件夹 ── -->
    <div v-if="!selectedDir" class="flex-1 flex flex-col items-center justify-center gap-3 text-gray-500">
      <div class="w-16 h-16 rounded-2xl bg-emerald-50 flex items-center justify-center">
        <Music class="w-8 h-8 text-emerald-400" />
      </div>
      <div class="text-center">
        <p class="text-sm font-medium text-gray-600">扫描你的本地音乐</p>
        <p class="text-[11px] mt-1 max-w-[280px]">
          仅能选择飞牛「应用设置 → 访问权限」中已授权的文件夹，不可浏览全盘
        </p>
      </div>
      <button
        @click="showFolderPicker = true"
        class="px-4 py-2 rounded-xl bg-emerald-500 hover:bg-emerald-600 text-white text-xs font-medium transition-colors"
      >
        选择文件夹
      </button>
    </div>

    <!-- ── 已选择文件夹 ── -->
    <template v-else>
      <!-- 状态提示 -->
      <div v-if="scanning" class="flex-1 flex flex-col items-center justify-center gap-2 text-gray-500">
        <RefreshCw class="w-6 h-6 text-emerald-500 animate-spin" />
        <p class="text-xs">正在扫描 {{ selectedDir }} …</p>
      </div>
      <div v-else-if="scanError" class="flex-1 flex flex-col items-center justify-center gap-2 text-gray-500">
        <AlertTriangle class="w-8 h-8 text-amber-400" />
        <p class="text-xs max-w-[300px] text-center">{{ scanError }}</p>
        <button @click="scan" class="px-3 py-1.5 rounded-lg bg-emerald-50 text-emerald-600 text-[11px] font-medium">重试</button>
      </div>
      <div v-else-if="!scanned" class="flex-1 flex flex-col items-center justify-center gap-3 text-gray-500">
        <Music class="w-10 h-10 opacity-40" />
        <p class="text-xs">点击「扫描」开始收集本地音乐</p>
      </div>

      <!-- ── 扫描结果 ── -->
      <template v-else>
        <!-- Tabs -->
        <div class="px-4 sm:px-6 pt-3 flex items-center gap-1 shrink-0">
          <button
            @click="tab = 'all'"
            :class="tab === 'all' ? 'bg-emerald-50 text-emerald-600 font-semibold' : 'text-gray-500 hover:bg-gray-50'"
            class="px-3 py-1.5 rounded-lg text-xs transition-colors"
          >
            全部歌曲 <span class="opacity-60">{{ songs.length }}</span>
          </button>
          <button
            @click="tab = 'dedup'"
            :class="tab === 'dedup' ? 'bg-emerald-50 text-emerald-600 font-semibold' : 'text-gray-500 hover:bg-gray-50'"
            class="px-3 py-1.5 rounded-lg text-xs transition-colors"
          >
            去重管理 <span v-if="dupGroups.length" class="px-1.5 rounded-full bg-amber-100 text-amber-600 text-[9px] font-bold">{{ dupGroups.length }}</span>
          </button>
          <span v-if="tab === 'dedup' && markCount > 0" class="ml-auto text-[11px] text-amber-600 font-medium">
            已标记 {{ markCount }} 个文件待删除
          </span>
        </div>

        <!-- 本地音乐搜索框：按歌名 / 歌手 / 专辑 / 文件名实时过滤 -->
        <div v-if="tab === 'all'" class="max-w-3xl mx-auto w-full px-4 sm:px-6 pt-3 shrink-0">
          <div class="relative">
            <Search class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400 pointer-events-none" />
            <input
              v-model="searchQuery"
              type="text"
              placeholder="搜索本地歌曲（歌名 / 歌手 / 专辑 / 文件名）"
              class="w-full pl-9 pr-9 py-2 rounded-xl bg-white border border-gray-200 text-sm text-gray-700 placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-emerald-400 focus:border-emerald-400"
            />
            <button
              v-if="searchQuery"
              @click="searchQuery = ''"
              class="absolute right-2 top-1/2 -translate-y-1/2 p-1 text-gray-400 hover:text-gray-600 transition-colors"
              title="清空搜索"
            >
              <X class="w-4 h-4" />
            </button>
          </div>
        </div>

        <div class="flex-1 overflow-y-auto px-4 sm:px-6 py-3">
          <!-- 全部歌曲列表 -->
          <div v-if="tab === 'all'" class="max-w-3xl mx-auto space-y-1 pb-10">
            <div v-if="filteredSongs.length === 0" class="text-center text-sm text-gray-600 py-12">
              {{ songs.length === 0 ? '该文件夹下没有找到音频文件' : '未找到与「' + searchQuery + '」匹配的歌曲' }}
            </div>
            <div
              v-for="(song, idx) in visibleFilteredSongs"
              :key="song.path"
              class="flex items-center gap-3 px-3 py-2 rounded-xl hover:bg-white cursor-pointer transition-colors group"
              @click="playSong(song, idx)"
            >
              <button @click.stop="playSong(song, idx)" class="shrink-0 w-10 h-10 rounded-lg overflow-hidden bg-gray-100 flex items-center justify-center relative">
                <img v-if="song.cover_url" :src="song.cover_url" :alt="song.name" loading="lazy" class="w-full h-full object-cover" @error="$event.target.style.display='none'" referrerpolicy="no-referrer" />
                <Music v-else class="w-4 h-4 text-gray-400" />
                <span class="absolute inset-0 flex items-center justify-center bg-black/40 opacity-0 group-hover:opacity-100 transition-opacity">
                  <Play class="w-4 h-4 text-white" />
                </span>
              </button>
              <div class="flex-1 min-w-0">
                <p class="text-sm text-gray-700 truncate">{{ song.name }}</p>
                <p class="text-[11px] text-gray-600 truncate">{{ song.singer }} · {{ song.album }}
                  <span v-if="!playableExts.includes(String(song.format).toLowerCase())" class="text-amber-500 font-medium">{{ String(song.format).toUpperCase() }} 浏览器可能无法播放</span>
                </p>
              </div>
              <span class="shrink-0 px-1.5 py-0.5 rounded text-[9px] font-medium"
                :class="qualityBadgeClass(song.quality)">{{ song.quality }}</span>
              <span class="shrink-0 text-[11px] text-gray-600 w-10 text-right hidden sm:block">{{ fmtDuration(song.duration) }}</span>
              <button
                @click.stop="openTagModal(song)"
                class="shrink-0 p-1.5 rounded-lg border border-gray-200 text-gray-500 hover:text-emerald-600 hover:border-emerald-300 transition-colors"
                title="写入歌词/封面标签"
              >
                <Tag class="w-3.5 h-3.5" />
              </button>
              <button
                @click.stop="playSong(song, idx)"
                class="shrink-0 px-2.5 py-1 rounded-lg bg-emerald-50 hover:bg-emerald-500 hover:text-white text-emerald-600 text-[11px] font-medium transition-all"
              >
                播放
              </button>
            </div>
            <!-- 懒加载哨兵：滚动到底自动加载更多，避免千首歌曲一次性渲染卡死 -->
            <div v-if="visibleCount < filteredSongs.length" data-list-sentinel class="flex items-center justify-center gap-2 py-3 text-gray-500 text-[11px]">
              <RefreshCw class="w-3.5 h-3.5 animate-spin" />
              已显示 {{ visibleCount }} / {{ filteredSongs.length }} 首，继续滚动加载…
            </div>
            <p v-else-if="filteredSongs.length > 0" class="text-center text-[11px] text-gray-600 py-2">已全部加载（{{ filteredSongs.length }} 首）</p>
          </div>

          <!-- 去重管理 -->
          <div v-else class="max-w-3xl mx-auto space-y-3 pb-24">
            <div v-if="dupGroups.length === 0" class="text-center text-sm text-gray-600 py-12">
              未发现多音质重复文件 🎉
            </div>
            <div
              v-for="g in dupGroups"
              :key="g.key"
              class="rounded-xl bg-white border border-gray-100 overflow-hidden"
            >
              <!-- 组头 -->
              <div class="px-3 py-2.5 flex items-center gap-2 border-b border-gray-50 bg-gradient-to-r from-emerald-50/50 to-transparent">
                <Copy class="w-3.5 h-3.5 text-amber-500 shrink-0" />
                <div class="flex-1 min-w-0">
                  <p class="text-xs font-semibold text-gray-700 truncate">{{ g.name }}</p>
                  <p class="text-[11px] text-gray-600 truncate">{{ g.artist ? g.artist + ' · ' : '' }}{{ g.version }} 个音质版本</p>
                </div>
                <button
                  @click="keepHighest(g)"
                  class="px-2 py-1 rounded-lg bg-amber-50 hover:bg-amber-100 text-amber-600 text-[10px] font-medium transition-colors shrink-0"
                  title="勾选除最高音质外的所有版本"
                >
                  只留最高音质
                </button>
              </div>
              <!-- 版本列表：checkbox / 文件名+建议保留 / 品质 / 试听，各列对齐 -->
              <div class="divide-y divide-gray-50">
                <div
                  v-for="(s, si) in g.songs"
                  :key="s.path"
                  class="flex items-center gap-2.5 px-3 py-2 cursor-pointer hover:bg-emerald-50/40 transition-colors"
                  @click="playSong(s, 0)"
                >
                  <input
                    type="checkbox"
                    :checked="marked.has(s.path)"
                    @click.stop
                    @change="toggleMark(s.path)"
                    class="w-4 h-4 shrink-0 accent-red-500"
                    title="勾选后可从底部删除"
                  />
                  <div class="flex-1 min-w-0">
                    <div class="flex items-center gap-1.5 min-w-0">
                      <p class="text-xs text-gray-700 truncate">{{ s.filename }}</p>
                      <span v-if="si === 0" class="shrink-0 text-[9px] px-1.5 py-0.5 rounded bg-emerald-50 text-emerald-600 font-medium whitespace-nowrap">建议保留</span>
                    </div>
                    <p class="text-[11px] text-gray-600 truncate">
                      {{ s.format.toUpperCase() }} · {{ fmtSize(s.size) }} · {{ fmtDuration(s.duration) }}
                      <span v-if="s.bitrate > 0"> · {{ s.bitrate }}kbps</span>
                      <span v-if="!playableExts.includes(String(s.format).toLowerCase())" class="text-amber-500 font-medium">浏览器可能无法播放</span>
                    </p>
                  </div>
                  <span class="shrink-0 w-12 text-center px-1.5 py-0.5 rounded text-[9px] font-medium"
                    :class="qualityBadgeClass(s.quality)">{{ s.quality }}</span>
                  <button
                    @click.stop="playSong(s, 0)"
                    class="shrink-0 w-7 h-7 rounded-lg text-gray-500 hover:text-emerald-600 hover:bg-emerald-50 flex items-center justify-center transition-colors"
                    title="试听"
                  >
                    <Play class="w-3.5 h-3.5" />
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- 去重底部删除栏：fixed 全宽横条，抬升到播放器条之上，手机端不遮挡播放器 -->
        <div
          v-if="tab === 'dedup' && markCount > 0"
          class="fixed left-3 right-3 z-[45] flex items-center justify-between gap-2 px-4 py-3 rounded-2xl bg-gray-900/95 backdrop-blur text-white shadow-2xl"
          style="bottom: calc(76px + env(safe-area-inset-bottom, 0px))"
        >
          <span class="text-[13px] font-medium min-w-0 truncate">已选 <b class="text-red-400">{{ markCount }}</b> 个低音质版本</span>
          <div class="flex items-center gap-1.5 shrink-0">
            <button @click="marked.clear()" class="px-3 py-2 rounded-xl text-gray-300 hover:text-white text-xs transition-colors">取消</button>
            <button @click="deleteMarked" :disabled="deleting" class="px-4 py-2 rounded-xl bg-red-500 hover:bg-red-600 disabled:opacity-50 text-xs font-semibold transition-colors">
              {{ deleting ? '删除中…' : '删除选中' }}
            </button>
          </div>
        </div>
      </template>
    </template>

    <!-- ── 文件夹选择弹窗（复用授权目录选择器） ── -->
    <FolderPickerModal v-model="showFolderPicker" @select="onSelectDir" />

    <!-- ── 批量重命名弹窗 ── -->
    <Teleport to="body">
      <Transition
        enter-active-class="transition-opacity duration-200 ease-out"
        enter-from-class="opacity-0"
        enter-to-class="opacity-100"
        leave-active-class="transition-opacity duration-150 ease-in"
        leave-from-class="opacity-100"
        leave-to-class="opacity-0"
      >
        <div v-if="showRenameModal" class="fixed inset-0 z-[90] flex items-center justify-center p-4 bg-black/50 backdrop-blur-sm" @click.self="closeRenameModal">
          <Transition
            enter-active-class="transition-all duration-250 ease-out"
            enter-from-class="opacity-0 scale-95 translate-y-4"
            enter-to-class="opacity-100 scale-100 translate-y-0"
            leave-active-class="transition-all duration-150 ease-in"
            leave-from-class="opacity-100 scale-100"
            leave-to-class="opacity-0 scale-95"
          >
            <div class="bg-white rounded-2xl w-full max-w-lg shadow-2xl border border-gray-100 overflow-hidden max-h-[85vh] flex flex-col">
              <div class="px-5 py-4 bg-gradient-to-r from-emerald-50 to-teal-50 border-b border-gray-100 shrink-0">
                <div class="flex items-center justify-between">
                  <h3 class="text-sm font-bold text-gray-800 flex items-center gap-2">
                    <PenLine class="w-4 h-4 text-emerald-600" />
                    批量重命名
                  </h3>
                  <button @click="closeRenameModal" class="text-gray-500 hover:text-gray-700">
                    <X class="w-4 h-4" />
                  </button>
                </div>
                <p class="text-[10px] text-gray-500 mt-1">仅重命名文件名，不改动音频标签内容</p>
              </div>

              <!-- 命名格式 -->
              <div class="px-5 pt-4 shrink-0">
                <label class="text-xs text-gray-600 font-semibold mb-1.5 block">命名格式（默认：歌曲名 - 歌手）</label>
                <div class="flex items-center gap-2">
                  <input
                    ref="renameInputRef"
                    v-model="renameFormatInput"
                    class="flex-1 px-3 py-2.5 rounded-lg border-2 border-gray-300 focus:border-emerald-400 focus:outline-none text-base font-mono text-gray-900 bg-white"
                    placeholder="{name} - {singer}"
                  />
                  <button @click="renameFormatInput = '{name} - {singer}'" class="px-2.5 py-2 rounded-lg bg-gray-50 hover:bg-gray-100 text-gray-600 text-xs font-medium transition-colors shrink-0">重置</button>
                </div>
                <div class="flex items-center gap-1.5 mt-2 flex-wrap">
                  <span class="text-[11px] text-gray-500">变量：</span>
                  <button @click="insertVar('{name}')" class="px-2 py-1 rounded bg-emerald-50 text-emerald-700 text-[11px] font-mono font-semibold hover:bg-emerald-100 transition-colors">{name} 歌名</button>
                  <button @click="insertVar('{singer}')" class="px-2 py-1 rounded bg-emerald-50 text-emerald-700 text-[11px] font-mono font-semibold hover:bg-emerald-100 transition-colors">{singer} 歌手</button>
                  <button @click="insertVar('{album}')" class="px-2 py-1 rounded bg-emerald-50 text-emerald-700 text-[11px] font-mono font-semibold hover:bg-emerald-100 transition-colors">{album} 专辑</button>
                </div>
              </div>

              <!-- 预览列表 -->
              <div class="flex-1 overflow-y-auto px-5 py-3">
                <div class="flex items-center justify-between mb-1.5">
                  <span class="text-xs text-gray-600">预览（{{ renameCount }} 个将重命名 / {{ renameConflictCount }} 个冲突）</span>
                  <button @click="toggleRenameAll" class="text-xs text-emerald-600 font-semibold">{{ renameAllChecked ? '取消全选' : '全选' }}</button>
                </div>
                <div v-if="renamePreviews.length === 0" class="text-center text-sm text-gray-600 py-8">
                  没有可重命名的文件
                </div>
                <div
                  v-for="pv in renamePreviewShown"
                  :key="pv.song.path"
                  class="flex items-start gap-2.5 py-2 border-b border-gray-50 last:border-0"
                >
                  <input
                    type="checkbox"
                    :checked="renameSel.has(pv.song.path)"
                    :disabled="pv.skip || pv.conflict"
                    @change="toggleRenameSel(pv.song.path)"
                    class="mt-0.5 w-4 h-4 accent-emerald-500 shrink-0"
                  />
                  <div class="flex-1 min-w-0">
                    <p class="text-xs text-gray-500 truncate line-through decoration-gray-300">{{ pv.song.filename }}</p>
                    <p class="text-sm text-gray-900 truncate">→ <span :class="pv.conflict ? 'text-red-500 font-semibold' : pv.skip ? 'text-gray-500' : 'text-emerald-600 font-semibold'">{{ pv.newName }}</span></p>
                    <p v-if="pv.skip" class="text-[11px] text-gray-600 mt-0.5">{{ pv.reason }}</p>
                    <p v-else-if="pv.conflict" class="text-[11px] text-red-500 mt-0.5">重名冲突：同目录已存在同名文件</p>
                  </div>
                  <span v-if="!pv.skip && !pv.conflict" class="shrink-0 text-[11px] px-1.5 py-0.5 rounded bg-emerald-50 text-emerald-600 font-medium">将重命名</span>
                </div>
                <p v-if="renameHiddenCount > 0" class="text-center text-[11px] text-gray-600 py-2">
                  仅展示前 {{ renamePreviewLimit }} 条，其余 {{ renameHiddenCount }} 条同样参与重命名（可全选/取消全选）
                </p>
              </div>

              <div class="px-5 py-3 bg-gray-50 border-t border-gray-100 flex items-center justify-between gap-3 shrink-0">
                <span class="text-[11px] text-gray-600">仅重命名授权目录内文件</span>
                <div class="flex items-center gap-2">
                  <button @click="closeRenameModal" class="px-4 py-2 text-xs font-medium text-gray-600 bg-white border border-gray-200 rounded-lg hover:bg-gray-50 transition-colors">取消</button>
                  <button
                    @click="doRename"
                    :disabled="renaming || renameCount === 0"
                    class="px-4 py-2 text-xs font-medium text-white bg-emerald-500 rounded-lg hover:bg-emerald-600 transition-colors disabled:opacity-40"
                  >
                    {{ renaming ? '重命名中…' : `重命名 ${renameCount} 个文件` }}
                  </button>
                </div>
              </div>
            </div>
          </Transition>
        </div>
      </Transition>
    </Teleport>

    <!-- ── 标签管理弹窗 ── -->
    <Teleport to="body">
      <Transition
        enter-active-class="transition-opacity duration-200 ease-out"
        enter-from-class="opacity-0"
        enter-to-class="opacity-100"
        leave-active-class="transition-opacity duration-150 ease-in"
        leave-from-class="opacity-100"
        leave-to-class="opacity-0"
      >
        <div v-if="tagModal.show" class="fixed inset-0 z-[90] flex items-center justify-center p-4 bg-black/50 backdrop-blur-sm" @click.self="closeTagModal">
          <Transition
            enter-active-class="transition-all duration-250 ease-out"
            enter-from-class="opacity-0 scale-95 translate-y-4"
            enter-to-class="opacity-100 scale-100 translate-y-0"
            leave-active-class="transition-all duration-150 ease-in"
            leave-from-class="opacity-100 scale-100"
            leave-to-class="opacity-0 scale-95"
          >
            <div class="bg-white rounded-2xl w-full max-w-md shadow-2xl border border-gray-100 overflow-hidden max-h-[85vh] flex flex-col">
              <div class="px-5 py-4 bg-gradient-to-r from-emerald-50 to-teal-50 border-b border-gray-100 shrink-0">
                <div class="flex items-center justify-between">
                  <h3 class="text-sm font-bold text-gray-800 flex items-center gap-2">
                    <Tag class="w-4 h-4 text-emerald-600" />
                    歌词 / 封面标签
                  </h3>
                  <button @click="closeTagModal" class="text-gray-500 hover:text-gray-700">
                    <X class="w-4 h-4" />
                  </button>
                </div>
                <p class="text-xs text-gray-700 mt-0.5 truncate">{{ tagModal.song?.filename }}</p>
              </div>

              <div class="flex-1 overflow-y-auto p-5 space-y-4">
                <!-- 封面 -->
                <div class="flex items-center gap-4">
                  <div class="w-20 h-20 rounded-xl bg-gray-50 border border-gray-200 flex items-center justify-center overflow-hidden shrink-0">
                    <img v-if="tagModal.coverPreview" :src="tagModal.coverPreview" class="w-full h-full object-cover" alt="封面" @error="tagModal.coverPreview = ''" />
                    <Music v-else class="w-6 h-6 text-gray-400" />
                  </div>
                  <div class="flex flex-col gap-1.5">
                    <label class="px-3 py-1.5 rounded-lg bg-emerald-50 hover:bg-emerald-100 text-emerald-600 text-[11px] font-medium cursor-pointer text-center transition-colors">
                      上传封面
                      <input type="file" accept="image/*" class="hidden" @change="onCoverFile" />
                    </label>
                    <button v-if="tagModal.coverPreview" @click="removeCover" class="px-3 py-1.5 rounded-lg bg-gray-50 hover:bg-gray-100 text-gray-500 text-[11px] font-medium transition-colors">
                      移除封面
                    </button>
                    <p class="text-[10px] text-gray-600">写入音频文件内嵌标签</p>
                  </div>
                </div>

                <!-- 自动联网补全 -->
                <div class="flex items-center gap-2.5 px-3 py-2.5 rounded-xl bg-emerald-50/60 border border-emerald-100">
                  <Sparkles class="w-4 h-4 text-emerald-500 shrink-0" />
                  <div class="flex-1 min-w-0">
                    <p class="text-xs font-medium text-gray-700">自动联网补全</p>
                    <p class="text-[11px] text-gray-600">按「歌名 + 歌手」在线搜索，自动写入歌词与封面</p>
                  </div>
                  <button
                    @click="autoTag"
                    :disabled="autoTagging"
                    class="shrink-0 px-3 py-1.5 rounded-lg bg-emerald-500 hover:bg-emerald-600 disabled:opacity-40 text-white text-[11px] font-medium transition-colors"
                  >
                    {{ autoTagging ? '获取中…' : '获取并写入' }}
                  </button>
                </div>

                <!-- 标题 / 歌手 / 专辑 -->
                <div class="grid grid-cols-2 gap-3">
                  <div>
                    <label class="text-xs text-gray-700 font-medium mb-1 block">标题</label>
                    <input v-model="tagModal.title" class="w-full px-3 py-2 rounded-lg border border-gray-300 focus:border-emerald-400 focus:outline-none text-sm text-gray-900" placeholder="歌曲标题" />
                  </div>
                  <div>
                    <label class="text-xs text-gray-700 font-medium mb-1 block">歌手</label>
                    <input v-model="tagModal.artist" class="w-full px-3 py-2 rounded-lg border border-gray-300 focus:border-emerald-400 focus:outline-none text-sm text-gray-900" placeholder="歌手" />
                  </div>
                  <div class="col-span-2">
                    <label class="text-xs text-gray-700 font-medium mb-1 block">专辑</label>
                    <input v-model="tagModal.album" class="w-full px-3 py-2 rounded-lg border border-gray-300 focus:border-emerald-400 focus:outline-none text-sm text-gray-900" placeholder="专辑" />
                  </div>
                </div>

                <!-- 歌词 -->
                <div>
                  <div class="flex items-center justify-between mb-1">
                    <label class="text-xs text-gray-700 font-medium">歌词（LRC 格式）</label>
                    <span class="text-[10px] text-gray-500">留空则不修改</span>
                  </div>
                  <textarea
                    v-model="tagModal.lyrics"
                    rows="6"
                    placeholder="[00:01.00]歌词内容…"
                    class="w-full px-3 py-2 rounded-lg border border-gray-300 focus:border-emerald-400 focus:outline-none text-sm text-gray-900 font-mono leading-relaxed resize-none"
                  ></textarea>
                </div>
              </div>

              <div class="px-5 py-3 bg-gray-50 border-t border-gray-100 flex items-center justify-end gap-2 shrink-0">
                <button @click="closeTagModal" class="px-4 py-2 text-xs font-medium text-gray-600 bg-white border border-gray-200 rounded-lg hover:bg-gray-50 transition-colors">取消</button>
                <button
                  @click="saveTags"
                  :disabled="savingTags"
                  class="px-4 py-2 text-xs font-medium text-white bg-emerald-500 rounded-lg hover:bg-emerald-600 transition-colors disabled:opacity-40"
                >
                  {{ savingTags ? '保存中…' : '保存标签' }}
                </button>
              </div>
            </div>
          </Transition>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { Folder, RefreshCw, Music, Play, Tag, X, Copy, AlertTriangle, PenLine, Sparkles, Search } from 'lucide-vue-next'
import FolderPickerModal from './FolderPickerModal.vue'
import { LocalAPI } from '../api/client'

const emit = defineEmits(['play'])

// 长期记住上次选择的文件夹（下次进入自动恢复并扫描）
const selectedDir = ref(localStorage.getItem('fn_local_dir') || '')
const showFolderPicker = ref(false)
const scanning = ref(false)
const scanned = ref(false)
const scanError = ref('')
const songs = ref([])
const groups = ref([])
const tab = ref('all')
const marked = ref(new Set())
const deleting = ref(false)

const tagModal = ref({ show: false, song: null, title: '', artist: '', album: '', lyrics: '', coverPreview: '', coverB64: '' })
const savingTags = ref(false)
const autoTagging = ref(false)

// 浏览器可直接播放的音频格式（APE/DSF/DFF 等浏览器无解码器，行内给出提示）
const playableExts = ['mp3', 'flac', 'wav', 'm4a', 'aac', 'ogg', 'opus']

// 进入本地音乐时自动恢复上次选择的文件夹并扫描
onMounted(() => {
  if (selectedDir.value) scan()
})

// ── 本地音乐搜索：按 歌名/歌手/专辑/文件名 实时过滤 ──
const searchQuery = ref('')
const filteredSongs = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return songs.value
  return songs.value.filter(s => [s.name, s.singer, s.album, s.filename, s.format]
    .join(' ').toLowerCase().includes(q))
})

// ── 歌曲列表懒加载（千首歌曲不再一次性渲染卡死） ──
const visibleCount = ref(100)
const visibleSongs = computed(() => songs.value.slice(0, visibleCount.value))
const visibleFilteredSongs = computed(() => filteredSongs.value.slice(0, visibleCount.value))
let listIO = null

function resetVisible() {
  visibleCount.value = 100
}

watch(searchQuery, () => {
  resetVisible()
  setupListObserver()
})

function setupListObserver() {
  listIO?.disconnect()
  listIO = null
  nextTick(() => {
    const el = document.querySelector('[data-list-sentinel]')
    if (!el) return
    listIO = new IntersectionObserver((entries) => {
      if (entries.some(e => e.isIntersecting) && visibleCount.value < filteredSongs.value.length) {
        visibleCount.value = Math.min(filteredSongs.value.length, visibleCount.value + 300)
      }
    }, { rootMargin: '600px' })
    listIO.observe(el)
  })
}

onBeforeUnmount(() => {
  listIO?.disconnect()
})

// ── 批量重命名状态 ──
const showRenameModal = ref(false)
// 输入框即时绑定（防抖后进入 renameFormat，避免每次按键重算上千条预览）
const renameFormatInput = ref(localStorage.getItem('fn_local_rename_format') || '{name} - {singer}')
const renameFormat = ref(renameFormatInput.value)
const renameInputRef = ref(null)
const renameSel = ref(new Set())
const renaming = ref(false)

let fmtTimer = null
watch(renameFormatInput, (v) => {
  localStorage.setItem('fn_local_rename_format', v)
  clearTimeout(fmtTimer)
  fmtTimer = setTimeout(() => {
    renameFormat.value = v
  }, 200)
})

onBeforeUnmount(() => clearTimeout(fmtTimer))

// 根据格式计算新文件名（替换变量 + 清理非法字符 + 保留原扩展名）
function buildNewName(song) {
  const fmt = renameFormat.value || ''
  let base = fmt
    .replace(/\{name\}|\{title\}/g, (song.name || song.title || '').trim())
    .replace(/\{singer\}|\{artist\}/g, (song.singer || song.artist || '').trim())
    .replace(/\{album\}/g, (song.album || '').trim())
  // 清理非法字符：\/:*?"<>| → 下划线；连续空白合并
  base = base.replace(/[\\/:*?"<>|]/g, '_').replace(/\s+/g, ' ').trim()
  if (!base) return ''
  const ext = (song.format || 'mp3').startsWith('.') ? song.format : '.' + (song.format || 'mp3')
  return base + ext
}

// 预览列表（O(n) 冲突检测，千首歌曲不再卡顿）
const renamePreviewLimit = 300
const renamePreviews = computed(() => {
  const songsList = songs.value
  const n = songsList.length
  const items = new Array(n)
  // 同目录：不移动文件（skip）的当前名集合 + 重命名文件的新名计数
  const skipNamesByDir = new Map() // dir -> Set<filenameLower>
  const newNameCount = new Map()   // dir\x00newNameLower -> count

  for (let i = 0; i < n; i++) {
    const song = songsList[i]
    let newName = ''
    let skip = false
    let reason = ''
    const fmt = renameFormat.value
    if (!fmt || !fmt.trim()) {
      skip = true
      reason = '命名格式为空'
    } else {
      newName = buildNewName(song)
      if (!newName) {
        skip = true
        reason = '缺少歌名，无法生成新名'
      } else if (newName.toLowerCase() === (song.filename || '').toLowerCase()) {
        skip = true
        reason = '已符合命名格式'
      }
    }
    items[i] = { song, newName, skip, conflict: false, reason }
    const dir = song.path.slice(0, song.path.lastIndexOf('/'))
    if (skip) {
      let set = skipNamesByDir.get(dir)
      if (!set) {
        set = new Set()
        skipNamesByDir.set(dir, set)
      }
      set.add((song.filename || '').toLowerCase())
    } else if (newName) {
      const key = dir + '\u0000' + newName.toLowerCase()
      newNameCount.set(key, (newNameCount.get(key) || 0) + 1)
    }
  }

  // 冲突判定：同目录下新名与不移动文件同名，或两个新名互相冲突
  for (let i = 0; i < n; i++) {
    const it = items[i]
    if (it.skip || !it.newName) continue
    const dir = it.song.path.slice(0, it.song.path.lastIndexOf('/'))
    const newLower = it.newName.toLowerCase()
    if (skipNamesByDir.get(dir)?.has(newLower)) {
      it.conflict = true
      continue
    }
    if ((newNameCount.get(dir + '\u0000' + newLower) || 0) > 1) {
      it.conflict = true
    }
  }
  return items
})

// 渲染截断：仅渲染前 300 条预览（全部仍参与重命名）
const renamePreviewShown = computed(() => renamePreviews.value.slice(0, renamePreviewLimit))
const renameHiddenCount = computed(() => renamePreviews.value.length - renamePreviewShown.value.length)

const renameCount = computed(() => renamePreviews.value.filter(p => !p.skip && !p.conflict && renameSel.value.has(p.song.path)).length)
const renameConflictCount = computed(() => renamePreviews.value.filter(p => !p.skip && p.conflict).length)
const renameAllChecked = computed(() => {
  const selectable = renamePreviews.value.filter(p => !p.skip && !p.conflict)
  return selectable.length > 0 && selectable.every(p => renameSel.value.has(p.song.path))
})

function openRenameModal() {
  renameSel.value = new Set(songs.value.map(s => s.path)) // 默认全选
  renameFormatInput.value = renameFormat.value // 打开时同步显示
  showRenameModal.value = true
}

function closeRenameModal() {
  showRenameModal.value = false
}

// 变量按钮：插入到输入框光标位置，而不是简单追加（避免误重复）
function insertVar(v) {
  const input = renameInputRef.value
  if (input) {
    const start = input.selectionStart ?? renameFormatInput.value.length
    const end = input.selectionEnd ?? start
    const cur = renameFormatInput.value
    renameFormatInput.value = cur.slice(0, start) + v + cur.slice(end)
    nextTick(() => {
      const pos = start + v.length
      input.focus()
      input.setSelectionRange(pos, pos)
    })
  } else {
    renameFormatInput.value = (renameFormatInput.value || '') + v
  }
}

function toggleRenameSel(path) {
  const m = new Set(renameSel.value)
  if (m.has(path)) m.delete(path)
  else m.add(path)
  renameSel.value = m
}

function toggleRenameAll() {
  const selectable = renamePreviews.value.filter(p => !p.skip && !p.conflict)
  if (selectable.every(p => renameSel.value.has(p.song.path))) {
    const m = new Set(renameSel.value)
    selectable.forEach(p => m.delete(p.song.path))
    renameSel.value = m
  } else {
    const m = new Set(renameSel.value)
    selectable.forEach(p => m.add(p.song.path))
    renameSel.value = m
  }
}

async function doRename() {
  // 执行前强制同步最新格式（不等 200ms 防抖）
  renameFormat.value = renameFormatInput.value
  const items = []
  for (const pv of renamePreviews.value) {
    if (pv.skip || pv.conflict || !renameSel.value.has(pv.song.path)) continue
    items.push({ path: pv.song.path, new_name: pv.newName })
  }
  if (!items.length) {
    alert('没有可重命名的文件（已排除冲突与重复项）')
    return
  }
  renaming.value = true
  try {
    const res = await LocalAPI.rename(items)
    const n = (res.renamed || []).length
    const fails = res.failed || []
    showRenameModal.value = false
    await scan()
    if (fails.length) {
      alert(`成功重命名 ${n} 个，失败 ${fails.length} 个：\n` + fails.map(f => `  ${String(f.path).split('/').pop()}: ${f.error}`).join('\n'))
    } else {
      alert(`已重命名 ${n} 个文件 ✓`)
    }
  } catch (e) {
    alert('重命名失败：' + (e.message || e))
  } finally {
    renaming.value = false
  }
}

const dupGroups = computed(() => groups.value)
const dupGroupCount = computed(() => groups.value.length)
const markCount = computed(() => marked.value.size)

function onSelectDir(path) {
  selectedDir.value = path
  localStorage.setItem('fn_local_dir', path)
  tab.value = 'all'
  marked.value.clear()
  scan()
}

// 切换 Tab 时重置列表懒加载
watch(tab, () => {
  resetVisible()
  nextTick(setupListObserver)
})

async function scan() {
  if (!selectedDir.value || scanning.value) return
  scanning.value = true
  scanError.value = ''
  try {
    const res = await LocalAPI.scan(selectedDir.value)
    if (res.code === 200) {
      const data = res.data || {}
      songs.value = (data.songs || []).map(s => ({
        ...s,
        name: s.name || s.title || s.filename,
        singer: s.singer || s.artist || '',
        cover: s.cover_url || '',
      }))
      groups.value = data.groups || []
      scanned.value = true
      marked.value.clear()
      resetVisible()
      setupListObserver()
    } else {
      scanError.value = res.error || '扫描失败'
    }
  } catch (e) {
    scanError.value = '扫描失败：' + (e.message || e)
  } finally {
    scanning.value = false
  }
}

function playSong(song, index) {
  // 本地歌曲走 App 的本地直连播放分支（source === 'local'）
  emit('play', { song, playlist: songs.value, index })
}

function toggleMark(path) {
  const m = new Set(marked.value)
  if (m.has(path)) m.delete(path)
  else m.add(path)
  marked.value = m
}

// 只保留最高音质：勾选除第一项（最高品质）外的所有版本
function keepHighest(g) {
  const m = new Set(marked.value)
  g.songs.forEach((s, i) => {
    if (i > 0) m.add(s.path)
    else m.delete(s.path)
  })
  marked.value = m
}

async function deleteMarked() {
  const paths = [...marked.value]
  if (!paths.length) return
  const names = paths.slice(0, 3).map(p => p.split('/').pop())
  const tip = names.length < paths.length ? `等 ${paths.length} 个文件` : ''
  if (!confirm(`确定删除以下 ${paths.length} 个文件吗？\n${names.join('\n')}${tip}\n\n删除后不可恢复！`)) return
  deleting.value = true
  try {
    const res = await LocalAPI.delete(paths)
    const failed = (res.failed || []).length
    const deleted = (res.deleted || []).length
    marked.value.clear()
    if (deleted > 0) {
      await scan()
    }
    if (failed > 0) {
      alert(`成功删除 ${deleted} 个，${failed} 个失败：\n` + (res.failed || []).map(f => `  ${f.path.split('/').pop()}: ${f.error}`).join('\n'))
    }
  } catch (e) {
    alert('删除失败：' + (e.message || e))
  } finally {
    deleting.value = false
  }
}

// ── 标签管理 ──
async function openTagModal(song) {
  tagModal.value = {
    show: true,
    song,
    title: song.title || '',
    artist: song.artist || '',
    album: song.album || '',
    lyrics: '',
    coverPreview: '',
    coverB64: '',
  }
  // 加载已有歌词（.lrc 文件或内嵌）
  try {
    const res = await LocalAPI.lyric(song.path)
    if (res.code === 200 && res.data?.lyric) {
      tagModal.value.lyrics = res.data.lyric
    }
  } catch (_) {}
  // 封面预览
  tagModal.value.coverPreview = LocalAPI.coverUrl(song.path)
}

function onCoverFile(e) {
  const file = e.target.files && e.target.files[0]
  if (!file) return
  if (file.size > 8 * 1024 * 1024) {
    alert('封面图片不能超过 8MB')
    return
  }
  const reader = new FileReader()
  reader.onload = () => {
    const dataUrl = reader.result
    tagModal.value.coverPreview = dataUrl
    tagModal.value.coverB64 = dataUrl.split(',')[1] || ''
  }
  reader.readAsDataURL(file)
}

function removeCover() {
  tagModal.value.coverPreview = ''
  tagModal.value.coverB64 = ''
  tagModal.value.removeCover = true
}

async function saveTags() {
  const song = tagModal.value.song
  if (!song) return
  savingTags.value = true
  try {
    const payload = {
      path: song.path,
      title: tagModal.value.title,
      artist: tagModal.value.artist,
      album: tagModal.value.album,
      lyrics: tagModal.value.lyrics,
    }
    if (tagModal.value.removeCover) payload.remove_cover = true
    else if (tagModal.value.coverB64) payload.cover_b64 = tagModal.value.coverB64

    const res = await LocalAPI.tags(payload)
    if (res.code === 200) {
      closeTagModal()
      // 刷新扫描结果以显示最新标签
      await scan()
      alert('标签已写入音频文件 ✓')
    } else {
      alert('写入失败：' + (res.error || '未知错误'))
    }
  } catch (e) {
    alert('写入失败：' + (e.message || e))
  } finally {
    savingTags.value = false
  }
}

// 联网自动获取并写入歌词 / 封面（按歌名+歌手在线搜索）
async function autoTag() {
  const song = tagModal.value.song
  if (!song) return
  autoTagging.value = true
  try {
    const res = await LocalAPI.autotag({
      path: song.path,
      title: tagModal.value.title || song.title || '',
      singer: tagModal.value.artist || song.artist || '',
    })
    if (res.code === 200) {
      // 重新读取写入后的歌词与封面预览
      try {
        const lres = await LocalAPI.lyric(song.path)
        if (lres.code === 200 && lres.data?.lyric) {
          tagModal.value.lyrics = lres.data.lyric
        }
      } catch (_) {}
      tagModal.value.coverB64 = ''
      tagModal.value.coverPreview = LocalAPI.coverUrl(song.path) + '&t=' + Date.now()
      const msg = []
      if (res.lyric_ok) msg.push('歌词 ✓')
      if (res.cover_ok) msg.push('封面 ✓')
      const from = res.source ? `（来源：${res.source}）` : ''
      if (msg.length) {
        alert(`已联网写入：${msg.join(' ')} ${from}`)
      } else {
        alert('已联网获取，但未找到可用的歌词/封面，可手动填写')
      }
    } else {
      alert('获取失败：' + (res.error || '未知错误'))
    }
  } catch (e) {
    alert('获取失败：' + (e.message || e))
  } finally {
    autoTagging.value = false
  }
}

function closeTagModal() {
  tagModal.value.show = false
}

// ── 展示辅助 ──
function fmtDuration(sec) {
  if (!sec || sec <= 0) return '--:--'
  const m = Math.floor(sec / 60)
  const s = Math.floor(sec % 60)
  return `${m}:${String(s).padStart(2, '0')}`
}

function fmtSize(bytes) {
  if (!bytes) return '0B'
  if (bytes > 1024 * 1024 * 1024) return (bytes / 1024 / 1024 / 1024).toFixed(1) + 'GB'
  if (bytes > 1024 * 1024) return (bytes / 1024 / 1024).toFixed(1) + 'MB'
  return (bytes / 1024).toFixed(0) + 'KB'
}

function qualityBadgeClass(q) {
  switch (q) {
    case '母带': return 'bg-purple-100 text-purple-600'
    case '至臻': return 'bg-indigo-100 text-indigo-600'
    case '无损': return 'bg-emerald-100 text-emerald-600'
    case '超高': return 'bg-sky-100 text-sky-600'
    case '流畅': return 'bg-gray-100 text-gray-500'
    default: return 'bg-gray-100 text-gray-500'
  }
}
</script>
