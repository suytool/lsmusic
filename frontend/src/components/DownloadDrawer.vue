<template>
  <div>
    <!-- ── 右侧滑出式下载任务抽屉 ── -->
    <Teleport to="body">
      <!-- 遮罩背景 -->
      <Transition
        enter-active-class="transition-opacity duration-300 ease-out"
        enter-from-class="opacity-0"
        enter-to-class="opacity-100"
        leave-active-class="transition-opacity duration-200 ease-in"
        leave-from-class="opacity-100"
        leave-to-class="opacity-0"
      >
        <div
          v-if="downloadManager.isOpen.value"
          @click="downloadManager.close()"
          class="fixed inset-0 z-50 bg-black/40 backdrop-blur-xs"
        ></div>
      </Transition>

      <!-- 抽屉主体 -->
      <Transition
        enter-active-class="transition-transform duration-300 ease-out"
        enter-from-class="translate-x-full"
        enter-to-class="translate-x-0"
        leave-active-class="transition-transform duration-250 ease-in"
        leave-from-class="translate-x-0"
        leave-to-class="translate-x-full"
      >
        <div
          v-if="downloadManager.isOpen.value"
          class="fixed inset-y-0 right-0 z-50 w-full sm:w-[480px] bg-white shadow-2xl flex flex-col overflow-hidden border-l border-gray-100"
        >
          <!-- 1. 顶部 Header -->
          <div class="px-5 py-4 border-b border-gray-100 flex items-center justify-between bg-white shrink-0">
            <div class="flex items-center gap-2.5">
              <div class="w-8 h-8 rounded-xl bg-emerald-50 text-emerald-600 flex items-center justify-center shadow-2xs">
                <Download class="w-4 h-4" :class="{ 'animate-bounce': downloadManager.activeCount.value > 0 }" />
              </div>
              <div>
                <h3 class="text-sm font-bold text-gray-800 flex items-center gap-2">
                  <span>下载任务</span>
                  <span
                    v-if="downloadManager.activeCount.value > 0"
                    class="px-2 py-0.5 rounded-full bg-emerald-500 text-white text-[10px] font-semibold animate-pulse"
                  >
                    {{ downloadManager.activeCount.value }} 首下载中
                  </span>
                </h3>
                <p class="text-[11px] text-gray-600">
                  后台持久下载，切换页面不中断
                </p>
              </div>
            </div>

            <div class="flex items-center gap-1">
              <!-- 下载设置按钮 -->
              <button
                @click="downloadManager.openSettingsModal()"
                class="p-1.5 rounded-xl text-gray-600 hover:text-emerald-600 hover:bg-emerald-50 transition-colors"
                title="下载设置（品质 / 歌词封面 / 命名格式）"
              >
                <Settings class="w-4 h-4" />
              </button>
              <button
                @click="downloadManager.close()"
                class="p-1.5 rounded-xl text-gray-600 hover:text-gray-600 hover:bg-gray-100 transition-colors"
                title="关闭下载面板"
              >
                <X class="w-4 h-4" />
              </button>
            </div>
          </div>

          <!-- 2. 当前存储目录 & 品质提示条 -->
          <div class="px-5 py-2.5 bg-gray-50/80 border-b border-gray-100 flex items-center justify-between text-xs text-gray-600 shrink-0 gap-2">
            <div class="flex items-center gap-1.5 min-w-0 flex-1">
              <Folder class="w-3.5 h-3.5 text-amber-500 shrink-0" />
              <span class="text-gray-600 shrink-0">存储:</span>
              <span class="font-mono text-gray-700 truncate text-[11px]" :title="downloadManager.currentDownloadDir.value">
                {{ downloadManager.currentDownloadDir.value }}
              </span>
            </div>
            <div class="flex items-center gap-2 shrink-0">
              <span class="px-1.5 py-0.5 rounded bg-emerald-100 text-emerald-700 text-[10px] font-semibold">
                {{ qualityLabel }}
              </span>
              <button
                @click="downloadManager.showSettingsModal.value = true"
                class="text-emerald-600 hover:text-emerald-700 hover:underline shrink-0 text-[11px] font-medium"
              >
                更改目录
              </button>
            </div>
          </div>

          <!-- 3. 筛选标签 & 操作栏 -->
          <div class="px-5 py-2 border-b border-gray-100 flex flex-wrap items-center justify-between gap-2 bg-white shrink-0">
            <!-- 标签切换 -->
            <div class="flex items-center gap-1 bg-gray-100 p-0.5 rounded-lg text-xs overflow-x-auto">
              <button
                v-for="tab in filterTabs" :key="tab.id"
                @click="currentFilter = tab.id"
                :class="[
                  'px-2.5 py-1 rounded-md text-[11px] font-medium transition-all whitespace-nowrap',
                  currentFilter === tab.id
                    ? 'bg-white text-emerald-600 shadow-xs font-semibold'
                    : 'text-gray-500 hover:text-gray-700'
                ]"
              >
                {{ tab.name }}
                <span class="opacity-70 ml-0.5 text-[10px]">({{ tab.count }})</span>
              </button>
            </div>

            <!-- 批量控制操作 -->
            <div class="flex items-center gap-1.5">
              <!-- 全部暂停 -->
              <button
                v-if="downloadManager.activeCount.value > 0"
                @click="downloadManager.pauseAll()"
                class="px-2 py-1 rounded-md bg-amber-50 text-amber-600 hover:bg-amber-100 text-[11px] font-medium transition-colors flex items-center gap-1 cursor-pointer"
                title="暂停所有下载中的任务"
              >
                <Pause class="w-3 h-3" />
                <span>全部暂停</span>
              </button>

              <!-- 全部继续 -->
              <button
                v-if="downloadManager.pausedCount.value > 0"
                @click="downloadManager.resumeAll(activeSource?.id)"
                class="px-2 py-1 rounded-md bg-emerald-50 text-emerald-600 hover:bg-emerald-100 text-[11px] font-medium transition-colors flex items-center gap-1 cursor-pointer"
                title="继续所有已暂停的任务"
              >
                <Play class="w-3 h-3" />
                <span>全部继续</span>
              </button>

              <!-- 全部重试 -->
              <button
                v-if="downloadManager.failedCount.value > 0"
                @click="downloadManager.retryAllFailed(activeSource?.id)"
                class="px-2 py-1 rounded-md bg-blue-50 text-blue-600 hover:bg-blue-100 text-[11px] font-medium transition-colors flex items-center gap-1 cursor-pointer"
                title="重新尝试所有失败的任务"
              >
                <RotateCw class="w-3 h-3" />
                <span>重试失败</span>
              </button>

              <!-- 停止全部进行中任务 -->
              <button
                v-if="downloadManager.activeCount.value > 0 || downloadManager.pausedCount.value > 0"
                @click="downloadManager.stopAll()"
                class="px-2 py-1 rounded-md bg-rose-50 text-rose-600 hover:bg-rose-100 text-[11px] font-medium transition-colors flex items-center gap-1 cursor-pointer"
                title="停止并取消所有正在进行与排队中的任务"
              >
                <Square class="w-3 h-3" />
                <span>停止全部</span>
              </button>

              <!-- 清空已完成 -->
              <button
                v-if="downloadManager.completedCount.value > 0"
                @click="downloadManager.clearCompleted()"
                class="px-2 py-1 rounded-md text-gray-600 hover:text-gray-600 hover:bg-gray-100 text-[11px] transition-colors cursor-pointer"
                title="清除所有已完成的记录"
              >
                清空完成
              </button>
            </div>
          </div>

          <!-- 4. 任务列表内容区 -->
          <div class="flex-1 overflow-y-auto p-4 space-y-2.5 bg-gray-50/50">
            <!-- 空状态 -->
            <div
              v-if="!filteredTasks.length"
              class="flex flex-col items-center justify-center py-24 text-center text-gray-600"
            >
              <div class="w-14 h-14 rounded-2xl bg-gray-100 text-gray-500 flex items-center justify-center mb-3">
                <Music2 class="w-7 h-7" />
              </div>
              <p class="text-xs font-medium text-gray-600 mb-1">
                {{ currentFilter === 'all' ? '暂无下载任务' : `暂无${currentFilterName}任务` }}
              </p>
              <p class="text-[11px] text-gray-600 max-w-xs">
                在「发现音乐」中点击单曲或榜单的下载按钮，歌曲将自动加入后台下载队列并保存在 NAS
              </p>
            </div>

            <!-- 任务卡片列表 -->
            <div
              v-for="task in filteredTasks"
              :key="task.id"
              class="bg-white rounded-xl border border-gray-100 p-3 shadow-2xs hover:border-gray-200 transition-all flex items-center gap-3 group"
            >
              <!-- 封面 -->
              <div class="w-10 h-10 rounded-lg overflow-hidden bg-gray-100 shrink-0 relative flex items-center justify-center shadow-2xs">
                <img
                  v-if="task.song.cover"
                  :src="task.song.cover"
                  alt="cover"
                  referrerpolicy="no-referrer"
                  class="w-full h-full object-cover"
                  @error="task.song.cover = ''"
                />
                <span v-else class="text-base">🎵</span>
              </div>

              <!-- 歌曲信息与状态 -->
              <div class="flex-1 min-w-0">
                <div class="flex items-center justify-between gap-1 mb-0.5">
                  <p class="text-xs font-semibold text-gray-800 truncate" :title="task.song.name">
                    {{ task.song.name }}
                  </p>
                  <!-- 平台标签 -->
                  <span class="text-[9px] px-1.5 py-0.2 rounded-full bg-gray-100 text-gray-500 font-mono shrink-0">
                    {{ task.song.source || task.platform || 'kw' }}
                  </span>
                </div>
                <p class="text-[11px] text-gray-600 truncate mb-1" :title="task.song.singer">
                  {{ task.song.singer }}{{ task.song.album ? ' · ' + task.song.album : '' }}
                </p>

                <!-- 状态徽标与提示 -->
                <div class="flex items-center gap-1.5 text-[10px]">
                  <!-- 等待中 -->
                  <span
                    v-if="task.status === 'pending'"
                    class="px-2 py-0.5 rounded-md bg-gray-100 text-gray-500 flex items-center gap-1"
                  >
                    <Clock class="w-3 h-3 text-gray-600" />
                    <span>排队等待中...</span>
                  </span>

                  <!-- 解析直链中 -->
                  <span
                    v-else-if="task.status === 'resolving'"
                    class="px-2 py-0.5 rounded-md bg-blue-50 text-blue-600 flex items-center gap-1"
                  >
                    <Loader2 class="w-3 h-3 animate-spin text-blue-500" />
                    <span>正在解析音源直链...</span>
                  </span>

                  <!-- 下载写入 NAS 中 -->
                  <span
                    v-else-if="task.status === 'downloading'"
                    class="px-2 py-0.5 rounded-md bg-emerald-50 text-emerald-600 flex items-center gap-1 font-medium"
                  >
                    <Loader2 class="w-3 h-3 animate-spin text-emerald-500" />
                    <span>正在下载写入 NAS...</span>
                  </span>

                  <!-- 已暂停 -->
                  <span
                    v-else-if="task.status === 'paused'"
                    class="px-2 py-0.5 rounded-md bg-amber-50 text-amber-600 flex items-center gap-1 font-medium"
                  >
                    <Pause class="w-3 h-3 text-amber-500" />
                    <span>已暂停下载</span>
                  </span>

                  <!-- 下载成功 -->
                  <span
                    v-else-if="task.status === 'success'"
                    class="px-2 py-0.5 rounded-md bg-emerald-50 text-emerald-600 flex items-center gap-1 font-medium"
                  >
                    <Check class="w-3 h-3 text-emerald-500" />
                    <span>已保存至 NAS 本地</span>
                  </span>

                  <!-- 下载失败 -->
                  <span
                    v-else-if="task.status === 'failed'"
                    class="px-2 py-0.5 rounded-md bg-rose-50 text-rose-600 flex items-center gap-1 max-w-[240px] truncate"
                    :title="task.error"
                  >
                    <AlertCircle class="w-3 h-3 text-rose-500 shrink-0" />
                    <span class="truncate">{{ task.error || '下载失败' }}</span>
                  </span>
                </div>
              </div>

              <!-- 右侧操作按钮 -->
              <div class="shrink-0 flex items-center gap-1">
                <!-- 排队或下载中：暂停按钮 -->
                <button
                  v-if="task.status === 'pending' || task.status === 'resolving' || task.status === 'downloading'"
                  @click="downloadManager.pauseTask(task.id)"
                  class="p-1.5 rounded-lg bg-amber-50 hover:bg-amber-100 text-amber-600 transition-colors"
                  title="暂停下载"
                >
                  <Pause class="w-3.5 h-3.5" />
                </button>

                <!-- 已暂停：继续按钮 -->
                <button
                  v-if="task.status === 'paused'"
                  @click="downloadManager.resumeTask(task.id, activeSource?.id)"
                  class="p-1.5 rounded-lg bg-emerald-50 hover:bg-emerald-100 text-emerald-600 transition-colors"
                  title="继续下载"
                >
                  <Play class="w-3.5 h-3.5" />
                </button>

                <!-- 排队中/下载中/已暂停：取消并停止按钮 -->
                <button
                  v-if="task.status === 'pending' || task.status === 'resolving' || task.status === 'downloading' || task.status === 'paused'"
                  @click="downloadManager.cancelTask(task.id)"
                  class="p-1.5 rounded-lg text-gray-600 hover:text-rose-600 hover:bg-rose-50 transition-colors"
                  title="停止并取消此任务"
                >
                  <Square class="w-3.5 h-3.5" />
                </button>

                <!-- 失败任务重试按钮 -->
                <button
                  v-if="task.status === 'failed'"
                  @click="downloadManager.retryTask(task.id, activeSource?.id)"
                  class="p-1.5 rounded-lg bg-emerald-50 hover:bg-emerald-100 text-emerald-600 transition-colors"
                  title="重新下载"
                >
                  <RotateCw class="w-3.5 h-3.5" />
                </button>

                <!-- 移除记录按钮 (已完成或失败) -->
                <button
                  v-if="task.status === 'success' || task.status === 'failed'"
                  @click="downloadManager.removeTask(task.id)"
                  class="p-1.5 rounded-lg text-gray-500 hover:text-gray-500 hover:bg-gray-100 transition-colors"
                  title="移除记录"
                >
                  <Trash2 class="w-3.5 h-3.5" />
                </button>
              </div>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>

    <!-- ── 下载设置弹窗 ── -->
    <Teleport to="body">
      <div
        v-if="downloadManager.showSettingsModal.value"
        class="fixed inset-0 z-[80] flex items-center justify-center p-4 bg-black/50 backdrop-blur-xs"
      >
        <div class="bg-white rounded-2xl max-w-md w-full p-5 shadow-2xl border border-gray-100 space-y-4 max-h-[calc(100vh-120px)] overflow-y-auto">
          <div class="flex items-center justify-between">
            <h3 class="text-sm font-bold text-gray-800 flex items-center gap-1.5">
              <Settings class="w-4 h-4 text-emerald-500" />
              <span>下载设置</span>
            </h3>
            <button
              @click="downloadManager.showSettingsModal.value = false"
              class="text-gray-600 hover:text-gray-600"
            >
              <X class="w-4 h-4" />
            </button>
          </div>

          <div class="space-y-4">
            <!-- 下载目录 -->
            <div>
              <label class="block text-xs font-semibold text-gray-700 mb-2 flex items-center gap-1">
                <Folder class="w-3.5 h-3.5 text-amber-500" />
                下载目录
              </label>
              <div class="flex gap-2">
                <div
                  class="flex-1 px-3 py-2 rounded-lg bg-gray-50 border border-gray-200 text-xs font-mono text-gray-700 truncate"
                  :title="downloadManager.currentDownloadDir.value"
                >
                  {{ downloadManager.currentDownloadDir.value }}
                </div>
                <button
                  @click="showFolderPicker = true"
                  class="px-3 py-2 rounded-lg bg-emerald-50 text-emerald-600 text-xs font-medium hover:bg-emerald-100 transition-colors flex items-center gap-1 shrink-0"
                  title="浏览选择已授权文件夹"
                >
                  <FolderOpen class="w-3.5 h-3.5" />
                  更改
                </button>
              </div>
              <p class="text-[10px] text-gray-600 mt-1">
                仅可选择飞牛「应用设置 → 访问权限」中已授权的文件夹，不可手动输入路径
              </p>
            </div>

            <!-- 下载品质 -->
            <div>
              <label class="block text-xs font-semibold text-gray-700 mb-2 flex items-center gap-1">
                <AudioLines class="w-3.5 h-3.5 text-emerald-500" />
                下载品质（默认）
              </label>
              <div class="grid grid-cols-2 gap-2">
                <button
                  v-for="opt in downloadManager.QUALITY_OPTIONS"
                  :key="opt.key"
                  @click="downloadManager.downloadQuality.value = opt.key"
                  :class="[
                    'p-3 rounded-xl border-2 text-left transition-all',
                    downloadManager.downloadQuality.value === opt.key
                      ? 'border-emerald-400 bg-emerald-50'
                      : 'border-gray-100 bg-gray-50 hover:border-gray-200'
                  ]"
                >
                  <div class="flex items-center justify-between mb-0.5">
                    <span class="text-xs font-bold text-gray-800">
                      {{ opt.name }}<span class="text-[9px] text-gray-600 font-normal ml-1">{{ opt.key.toUpperCase() }}</span>
                    </span>
                    <span v-if="downloadManager.downloadQuality.value === opt.key" class="text-emerald-500">
                      <Check class="w-3.5 h-3.5" />
                    </span>
                  </div>
                  <p class="text-[10px] text-gray-600">{{ opt.desc }}</p>
                </button>
              </div>
              <p class="text-[10px] text-gray-600 mt-1.5">
                注：实际可获取的品质取决于第三方音源脚本支持情况，单曲下载时会自动检测并显示文件大小。
              </p>
            </div>

            <!-- 嵌入选项 -->
            <div>
              <label class="block text-xs font-semibold text-gray-700 mb-2 flex items-center gap-1">
                <Tag class="w-3.5 h-3.5 text-emerald-500" />
                元数据嵌入
              </label>
              <div class="space-y-2">
                <label class="flex items-center justify-between p-2.5 rounded-xl bg-gray-50 border border-gray-100 cursor-pointer hover:bg-gray-100/60 transition-colors">
                  <div class="flex items-center gap-2">
                    <FileText class="w-4 h-4 text-blue-400" />
                    <div>
                      <p class="text-xs font-medium text-gray-700">嵌入歌词</p>
                      <p class="text-[10px] text-gray-600">将歌词写入音频文件内部标签</p>
                    </div>
                  </div>
                  <div
                    @click="downloadManager.embedLyrics.value = !downloadManager.embedLyrics.value"
                    :class="[
                      'w-9 h-5 rounded-full transition-colors relative cursor-pointer',
                      downloadManager.embedLyrics.value ? 'bg-emerald-500' : 'bg-gray-300'
                    ]"
                  >
                    <div
                      :class="[
                        'absolute top-0.5 w-4 h-4 rounded-full bg-white shadow transition-transform',
                        downloadManager.embedLyrics.value ? 'translate-x-4' : 'translate-x-0.5'
                      ]"
                    ></div>
                  </div>
                </label>

                <label class="flex items-center justify-between p-2.5 rounded-xl bg-gray-50 border border-gray-100 cursor-pointer hover:bg-gray-100/60 transition-colors">
                  <div class="flex items-center gap-2">
                    <Image class="w-4 h-4 text-purple-400" />
                    <div>
                      <p class="text-xs font-medium text-gray-700">嵌入封面</p>
                      <p class="text-[10px] text-gray-600">将专辑封面图片写入音频文件</p>
                    </div>
                  </div>
                  <div
                    @click="downloadManager.embedCover.value = !downloadManager.embedCover.value"
                    :class="[
                      'w-9 h-5 rounded-full transition-colors relative cursor-pointer',
                      downloadManager.embedCover.value ? 'bg-emerald-500' : 'bg-gray-300'
                    ]"
                  >
                    <div
                      :class="[
                        'absolute top-0.5 w-4 h-4 rounded-full bg-white shadow transition-transform',
                        downloadManager.embedCover.value ? 'translate-x-4' : 'translate-x-0.5'
                      ]"
                    ></div>
                  </div>
                </label>
              </div>
            </div>

            <!-- 命名格式 -->
            <div>
              <label class="block text-xs font-semibold text-gray-700 mb-2 flex items-center gap-1">
                <FileSignature class="w-3.5 h-3.5 text-emerald-500" />
                文件命名格式
              </label>
              <input
                v-model="downloadManager.namingFormat.value"
                type="text"
                placeholder="{singer} - {name}"
                class="w-full px-3 py-2 rounded-xl bg-gray-50 border border-gray-200 text-xs font-mono text-gray-800 focus:outline-none focus:border-emerald-400 focus:ring-2 focus:ring-emerald-400/20 mb-2"
              />
              <div class="flex flex-wrap gap-1.5 mb-2">
                <button
                  v-for="preset in downloadManager.namingPresets" :key="preset.format"
                  @click="downloadManager.namingFormat.value = preset.format"
                  :class="[
                    'px-2 py-1 rounded-lg text-[11px] transition-colors',
                    downloadManager.namingFormat.value === preset.format
                      ? 'bg-emerald-100 text-emerald-700 font-medium'
                      : 'bg-gray-100 text-gray-500 hover:bg-gray-200'
                  ]"
                >
                  {{ preset.label }}
                </button>
              </div>
              <div class="bg-gray-50 rounded-lg p-2.5 space-y-1">
                <p class="text-[10px] text-gray-600 font-medium">可用变量：</p>
                <div class="flex flex-wrap gap-x-3 gap-y-0.5">
                  <span class="text-[10px] font-mono text-emerald-600">{singer}</span>
                  <span class="text-[10px] text-gray-600">歌手</span>
                  <span class="text-[10px] font-mono text-emerald-600">{name}</span>
                  <span class="text-[10px] text-gray-600">歌曲名</span>
                  <span class="text-[10px] font-mono text-emerald-600">{album}</span>
                  <span class="text-[10px] text-gray-600">专辑</span>
                  <span class="text-[10px] font-mono text-emerald-600">{source}</span>
                  <span class="text-[10px] text-gray-600">平台</span>
                </div>
                <p class="text-[10px] text-gray-600 mt-1">
                  使用 <span class="font-mono">/</span> 可创建子目录，如 <span class="font-mono">{singer}/{album}/{name}</span>
                </p>
              </div>
            </div>

            <!-- 高级设置：端口 -->
            <div class="pt-3 border-t border-gray-100">
              <label class="block text-xs font-semibold text-gray-700 mb-2 flex items-center gap-1">
                <Server class="w-3.5 h-3.5 text-blue-500" />
                应用端口（高级设置）
              </label>
              <div class="flex gap-2 items-center">
                <input
                  v-model.number="downloadManager.appPort.value"
                  type="number"
                  min="1024"
                  max="65535"
                  placeholder="8899"
                  class="flex-1 px-3 py-2 rounded-xl bg-gray-50 border border-gray-200 text-xs font-mono text-gray-800 focus:outline-none focus:border-emerald-400 focus:ring-2 focus:ring-emerald-400/20"
                />
                <button
                  @click="downloadManager.appPort.value = 8899"
                  class="px-3 py-2 rounded-xl bg-gray-100 text-gray-500 text-xs font-medium hover:bg-gray-200 transition-colors shrink-0"
                >
                  恢复默认
                </button>
              </div>
              <div class="mt-2 p-2.5 rounded-lg bg-amber-50 border border-amber-100">
                <p class="text-[10px] text-amber-700 flex items-start gap-1.5">
                  <AlertCircle class="w-3 h-3 shrink-0 mt-0.5" />
                  <span>修改端口后需要<strong>重启应用</strong>才能生效。请在飞牛NAS应用管理中停止并重新启动LSmusic。请确保端口未被其他应用占用。</span>
                </p>
              </div>
            </div>

          </div>

          <div class="flex justify-end gap-2 pt-2 border-t border-gray-100">
            <button
              @click="downloadManager.showSettingsModal.value = false"
              class="px-4 py-1.5 rounded-xl text-xs text-gray-500 hover:bg-gray-100"
            >
              取消
            </button>
            <button
              @click="downloadManager.saveDownloadSettings()"
              :disabled="downloadManager.isSavingSettings.value"
              class="px-5 py-1.5 rounded-xl bg-emerald-500 hover:bg-emerald-600 text-white font-semibold text-xs shadow-sm disabled:opacity-50"
            >
              {{ downloadManager.isSavingSettings.value ? '保存中...' : '保存设置' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>
    <!-- ── 文件夹选择器弹窗 ── -->
    <FolderPickerModal
      v-model="showFolderPicker"
      :initial-path="downloadManager.currentDownloadDir.value"
      @select="onFolderSelected"
    />
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { downloadManager } from '../services/downloadManager'
import FolderPickerModal from './FolderPickerModal.vue'
import {
  Download, X, Folder, FolderOpen, RotateCw, Trash2, Check,
  AlertCircle, Loader2, Music2, Clock, Pause, Play, Square,
  Settings, Info, AudioLines, Tag, FileText, Image, FileSignature, Server
} from 'lucide-vue-next'

const props = defineProps({
  activeSource: Object
})

const showFolderPicker = ref(false)

function onFolderSelected(path) {
  downloadManager.currentDownloadDir.value = path
}

const currentFilter = ref('all') // 'all' | 'downloading' | 'paused' | 'completed' | 'failed'

const qualityLabel = computed(() => {
  return downloadManager.qualityOption(downloadManager.downloadQuality.value).name
})

const filterTabs = computed(() => [
  { id: 'all', name: '全部', count: downloadManager.totalCount.value },
  { id: 'downloading', name: '下载中', count: downloadManager.activeCount.value },
  { id: 'paused', name: '已暂停', count: downloadManager.pausedCount.value },
  { id: 'completed', name: '已完成', count: downloadManager.completedCount.value },
  { id: 'failed', name: '失败', count: downloadManager.failedCount.value },
])

const currentFilterName = computed(() => {
  const match = filterTabs.value.find(t => t.id === currentFilter.value)
  return match ? match.name : ''
})

const filteredTasks = computed(() => {
  const list = downloadManager.tasks.value
  if (currentFilter.value === 'downloading') {
    return list.filter(t => t.status === 'pending' || t.status === 'resolving' || t.status === 'downloading')
  }
  if (currentFilter.value === 'paused') {
    return list.filter(t => t.status === 'paused')
  }
  if (currentFilter.value === 'completed') {
    return list.filter(t => t.status === 'success')
  }
  if (currentFilter.value === 'failed') {
    return list.filter(t => t.status === 'failed')
  }
  return list
})
</script>
