<template>
  <div class="relative">
    <div v-if="icon" class="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none text-slate-400">
      <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" :d="icon" />
      </svg>
    </div>
    <input
      :type="type"
      :value="modelValue"
      @input="$emit('update:modelValue', $event.target.value)"
      :placeholder="placeholder"
      :required="required"
      :disabled="disabled"
      class="input-glass"
      :class="[
        icon ? 'pl-10' : 'pl-4',
        error ? 'border-danger focus:border-danger focus:ring-danger/20' : '',
        className
      ]"
      v-bind="$attrs"
    />
    <p v-if="error" class="mt-1.5 text-xs text-danger absolute">{{ error }}</p>
  </div>
</template>

<script setup>
defineProps({
  modelValue: { type: [String, Number], default: '' },
  type: { type: String, default: 'text' },
  placeholder: { type: String, default: '' },
  icon: { type: String, default: '' },
  error: { type: String, default: '' },
  required: { type: Boolean, default: false },
  disabled: { type: Boolean, default: false },
  className: { type: String, default: '' }
})

defineEmits(['update:modelValue'])
</script>

<script>
export default { inheritAttrs: false }
</script>
