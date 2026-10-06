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
        v-if="visible"
        class="fixed inset-0 z-[90] flex items-center justify-center p-4 bg-black/50 backdrop-blur-sm"
        @click.self="close"
      >
        <Transition
          enter-active-class="transition-all duration-250 ease-out"
          enter-from-class="opacity-0 scale-95 translate-y-4"
          enter-to-class="opacity-100 scale-100 translate-y-0"
          leave-active-class="transition-all duration-150 ease-in"
          leave-from-class="opacity-100 scale-100"
          leave-to-class="opacity-0 scale-95"
        >
          <div class="bg-white rounded-2xl w-full max-w-md shadow-2xl border border-gray-100 overflow-hidden">
            <!-- 标题栏 -->
            <div class="px-5 py-4 bg-gradient-to-r from-emerald-50 to-teal-50 border-b border-gray-100">
              <div class="flex items-center justify-between">
                <h3 class="text-sm font-bold text-gray-800 flex items-center gap-2">
                  <Folder class="w-4 h-4 text-emerald-600" />
                  选择下载目录
                </h3>
                <button @click="close" class="text-gray-600 hover:text-gray-600">
                  <X class="w-4 h-4" />
                </button>
              </div>
              <!-- 当前路径（只读展示，不可手输） -->
              <div class="mt-3 flex items-center gap-2">
                <div class="flex-1 px-3 py-1.5 bg-white rounded-lg border border-gray-200 text-xs text-gray-600 font-mono truncate">
                  {{ currentPath }}
                </div>
                <button
                  @click="refresh"
                  class="p-1.5 rounded-lg border border-gray-200 hover:border-emerald-300 text-gray-500 hover:text-emerald-600 transition-all"
                  title="刷新"
                >
                  <RefreshCw class="w-3.5 h-3.5" />
                </button>
              </div>
              <p class="mt-1.5 text-[10px] text-emerald-700/70">
                仅能浏览并选择飞牛「应用设置 → 访问权限」中已授权的文件夹
              </p>
            </div>

            <!-- 目录列表 -->
            <div class="max-h-80 overflow-y-auto">
              <div v-if="loading" class="flex items-center justify-center py-12">
                <RefreshCw class="w-6 h-6 text-emerald-500 animate-spin" />
              </div>

              <!-- 无授权目录 -->
              <div v-else-if="isRootLevel && dirs.length === 0" class="flex flex-col items-center justify-center py-10 px-6 text-gray-600">
                <Folder class="w-10 h-10 mb-2 opacity-50" />
                <p class="text-xs mb-1.5">未检测到可访问的文件夹</p>
                <p class="text-[10px] text-gray-600 max-w-[260px] text-center leading-relaxed">
                  请在飞牛桌面打开「应用设置 → 访问权限」，添加需要授权给 LSmusic 的文件夹（如「音乐」「LSmusic」），然后点击右上角刷新重试
                </p>
                <button
                  @click="refresh"
                  class="mt-3 px-3 py-1.5 rounded-lg bg-emerald-50 hover:bg-emerald-100 text-emerald-600 text-[11px] font-medium transition-colors"
                >
                  重新检测
                </button>
              </div>

              <!-- 空子目录 -->
              <div v-else-if="dirs.length === 0" class="flex flex-col items-center justify-center py-12 text-gray-600">
                <Folder class="w-10 h-10 mb-2 opacity-50" />
                <p class="text-xs mb-1">该目录下没有可访问的子文件夹</p>
                <p class="text-[10px] text-gray-500 max-w-[240px] text-center">
                  点击下方「确认选择」即可将当前目录设为下载目录
                </p>
              </div>

              <!-- 目录列表 -->
              <div v-else class="p-2">
                <button
                  v-for="dir in dirs"
                  :key="dir.path"
                  class="w-full flex items-center gap-3 px-3 py-2.5 rounded-lg hover:bg-emerald-50 transition-colors text-left group"
                  @click="enterDir(dir.path)"
                >
                  <Folder class="w-5 h-5 text-amber-400 shrink-0" />
                  <div class="flex-1 min-w-0">
                    <div class="flex items-center gap-1.5">
                      <span class="text-sm text-gray-700 truncate group-hover:text-emerald-700">{{ dir.name }}</span>
                      <span
                        v-if="dir.writable === false"
                        class="text-[9px] px-1 py-0.5 rounded bg-gray-100 text-gray-600 font-medium shrink-0"
                        title="该文件夹为只读，不可设为下载目录"
                      >只读</span>
                    </div>
                    <p class="text-[10px] text-gray-600 font-mono truncate">{{ dir.path }}</p>
                  </div>
                  <button
                    @click.stop="selectDir(dir.path)"
                    class="px-2.5 py-1 rounded-lg bg-emerald-50 hover:bg-emerald-500 hover:text-white text-emerald-600 text-[11px] font-medium transition-all shrink-0"
                    :disabled="dir.writable === false"
                  >
                    选择此目录
                  </button>
                  <ChevronRight class="w-4 h-4 text-gray-500 group-hover:text-emerald-400 shrink-0" />
                </button>
              </div>
            </div>

            <!-- 底部操作栏 -->
            <div class="px-5 py-3 bg-gray-50 border-t border-gray-100 flex items-center justify-between gap-3">
              <button
                v-if="hasParent && !isRootLevel"
                @click="goParent"
                class="px-3 py-1.5 rounded-lg bg-white border border-gray-200 hover:bg-gray-100 text-gray-600 text-[11px] font-medium flex items-center gap-1 transition-colors shrink-0"
                title="返回上一级"
              >
                <ArrowUp class="w-3 h-3" />
                返回上一级
              </button>
              <span v-else class="text-[10px] text-gray-600">已到达授权目录顶层</span>

              <div class="flex items-center gap-2 shrink-0 ml-auto">
                <span v-if="!isRootLevel && !currentWritable" class="text-[10px] text-amber-500">当前文件夹为只读，不可设为下载目录</span>
                <button
                  @click="close"
                  class="px-4 py-2 text-xs font-medium text-gray-600 bg-white border border-gray-200 rounded-lg hover:bg-gray-50 transition-colors"
                >
                  取消
                </button>
                <button
                  @click="confirm"
                  :disabled="!canSelect"
                  class="px-4 py-2 text-xs font-medium text-white bg-emerald-500 rounded-lg hover:bg-emerald-600 transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
                >
                  确认选择
                </button>
              </div>
            </div>
          </div>
        </Transition>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { Folder, X, ArrowUp, RefreshCw, ChevronRight } from 'lucide-vue-next'

const props = defineProps({
  modelValue: { type: Boolean, default: false },
  initialPath: { type: String, default: '/' }
})

const emit = defineEmits(['update:modelValue', 'select'])

const visible = ref(false)
const currentPath = ref('/')
const parentPath = ref('')
const hasParent = ref(false)
const dirs = ref([])
const loading = ref(false)
const isRootLevel = ref(true)
const currentWritable = ref(true) // 当前所在目录是否可写（只读目录不可设为下载目录）

// 根级视图展示的是授权目录本身，不可直接“确认”根路径，必须进入或点选具体目录；
// 且当前目录必须可写才能确认
const canSelect = computed(() => {
  if (loading.value) return false
  if (isRootLevel.value) return false
  if (!currentWritable.value) return false
  return currentPath.value !== '/' && currentPath.value !== ''
})

watch(() => props.modelValue, (val) => {
  visible.value = val
  if (val) {
    loadRoots()
  }
})

watch(visible, (val) => {
  emit('update:modelValue', val)
})

// 顶层：只展示授权目录（应用设置 → 访问权限）；force=true 时强制重新扫描
async function loadRoots(force = false) {
  loading.value = true
  try {
    const res = await fetch(`/api/fs/roots${force ? '?refresh=1' : ''}`)
    const data = await res.json()
    if (data.code === 200) {
      const roots = data.data.roots || []
      dirs.value = roots.map(r => ({
        name: r.name,
        path: r.path,
        writable: r.writable !== false,
      }))
      currentPath.value = '/'
      parentPath.value = ''
      hasParent.value = false
      isRootLevel.value = true
      currentWritable.value = true
    } else {
      dirs.value = []
    }
  } catch (e) {
    dirs.value = []
  } finally {
    loading.value = false
  }
}

async function loadDirs(path) {
  loading.value = true
  try {
    const res = await fetch(`/api/fs/list?path=${encodeURIComponent(path)}`)
    const data = await res.json()
    if (data.code === 200) {
      dirs.value = data.data.dirs || []
      currentPath.value = data.data.path || path
      parentPath.value = data.data.parent || ''
      hasParent.value = !!data.data.has_parent
      isRootLevel.value = data.data.root_level === true
      // 顶层选择界面无“当前目录”概念，确认按钮本就禁用；此处避免误判
      currentWritable.value = isRootLevel.value ? true : data.data.writable !== false
    } else {
      // 越权或出错：回到授权根列表
      loadRoots()
    }
  } catch (e) {
    dirs.value = []
  } finally {
    loading.value = false
  }
}

function enterDir(path) {
  loadDirs(path)
}

function goParent() {
  if (hasParent.value && parentPath.value) {
    loadDirs(parentPath.value)
  }
}

function refresh() {
  if (isRootLevel.value) {
    // 强制重新扫描授权目录（用户新授权后立即生效）
    loadRoots(true)
  } else {
    loadDirs(currentPath.value)
  }
}

function selectDir(path) {
  emit('select', path)
  visible.value = false
}

function confirm() {
  if (!canSelect.value) return
  emit('select', currentPath.value)
  visible.value = false
}

function close() {
  visible.value = false
}
</script>
