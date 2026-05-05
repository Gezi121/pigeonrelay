<template>
  <Teleport to="body">
    <Transition name="modal">
      <div v-if="modelValue" class="fixed inset-0 z-50 overflow-y-auto">
        <div class="flex min-h-screen items-center justify-center px-4 pt-4 pb-20 text-center sm:p-0">
          <div class="fixed inset-0 bg-slate-900/40 backdrop-blur-sm transition-opacity" @click="close"></div>

          <div class="modal-card relative transform overflow-hidden rounded-2xl bg-white/90 backdrop-blur-xl border border-white/50 text-left shadow-2xl transition-all sm:my-8 sm:w-full sm:max-w-lg">
            <div class="px-6 py-5 border-b border-slate-100">
              <div class="flex items-center justify-between">
                <h3 class="text-lg font-bold text-slate-800">{{ title }}</h3>
                <button @click="close" class="text-slate-400 hover:text-slate-600 transition-colors p-1 rounded-lg hover:bg-slate-100">
                  <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                  </svg>
                </button>
              </div>
            </div>

            <div class="px-6 py-5">
              <slot></slot>
            </div>

            <div v-if="$slots.footer" class="bg-slate-50/50 px-6 py-4 flex items-center justify-end gap-3 border-t border-slate-100">
              <slot name="footer"></slot>
            </div>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup>
import { watch } from 'vue'

const props = defineProps({
  modelValue: { type: Boolean, default: false },
  title: { type: String, required: true }
})

const emit = defineEmits(['update:modelValue', 'close'])

function close() {
  emit('update:modelValue', false)
  emit('close')
}

// Lock body scroll when modal is open
watch(() => props.modelValue, (val) => {
  if (val) {
    document.body.style.overflow = 'hidden'
  } else {
    document.body.style.overflow = ''
  }
})
</script>
