<script setup lang="ts">
import { ref, watch } from 'vue'
import { searchUsers } from '@/api/users'
import type { User } from '@/api/auth'

const props = defineProps<{ modelValue: string; placeholder?: string }>()
const emit = defineEmits<{ 'update:modelValue': [value: string]; select: [user: User] }>()

const query = ref('')
const results = ref<User[]>([])
const open = ref(false)
const loading = ref(false)
let timer: ReturnType<typeof setTimeout> | undefined

watch(
  () => props.modelValue,
  (v) => {
    if (!v) query.value = ''
  },
)

async function search() {
  const q = query.value.trim()
  emit('update:modelValue', q)
  if (q.length < 2) {
    results.value = []
    open.value = false
    return
  }
  loading.value = true
  try {
    const res = await searchUsers(q)
    results.value = res.users
    open.value = res.users.length > 0
  } finally {
    loading.value = false
  }
}

function onInput() {
  clearTimeout(timer)
  timer = setTimeout(search, 250)
}

function pick(user: User) {
  query.value = user.username
  emit('update:modelValue', user.username)
  emit('select', user)
  open.value = false
}
</script>

<template>
  <div class="user-search">
    <input
      v-model="query"
      class="gl-input"
      type="text"
      :placeholder="placeholder || '按用户名、姓名或邮箱搜索'"
      autocomplete="off"
      @input="onInput"
      @focus="onInput"
    />
    <ul v-if="open" class="user-search__results panel">
      <li v-for="u in results" :key="u.id">
        <button type="button" @click="pick(u)">
          <strong>{{ u.name || u.username }}</strong>
          <span class="muted">@{{ u.username }} · {{ u.email }}</span>
        </button>
      </li>
    </ul>
    <span v-if="loading" class="user-search__loading muted">搜索中…</span>
  </div>
</template>

<style scoped lang="scss">
.user-search {
  position: relative;
}

.user-search__results {
  position: absolute;
  top: calc(100% + 4px);
  left: 0;
  right: 0;
  list-style: none;
  margin: 0;
  padding: 4px 0;
  z-index: 50;
  max-height: 240px;
  overflow-y: auto;
}

.user-search__results li button {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 2px;
  width: 100%;
  padding: 8px 12px;
  border: none;
  background: none;
  text-align: left;
  cursor: pointer;
  font: inherit;

  &:hover {
    background: var(--gl-background-color-subtle);
  }
}

.user-search__loading {
  position: absolute;
  right: 10px;
  top: 50%;
  transform: translateY(-50%);
  font-size: 12px;
}
</style>
