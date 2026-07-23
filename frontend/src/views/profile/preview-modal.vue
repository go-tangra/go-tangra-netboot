<script lang="ts" setup>
import { $t } from 'shell/locales';

defineProps<{ userData: string; cmdline: string }>();
const emit = defineEmits<{ close: [] }>();
</script>

<template>
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/40" @click.self="emit('close')">
    <div class="max-h-[80vh] w-[52rem] overflow-y-auto rounded bg-white p-6 dark:bg-gray-900">
      <h2 class="mb-4 text-lg font-semibold">{{ $t('netboot.page.profile.previewTitle') }}</h2>

      <div class="mb-4">
        <div class="mb-1 text-sm font-medium">{{ $t('netboot.page.profile.previewCmdline') }}</div>
        <pre class="overflow-x-auto rounded bg-gray-100 p-3 font-mono text-xs dark:bg-gray-800">{{ cmdline }}</pre>
      </div>

      <!--
        Rendered by the netboot server with credentials already redacted, so
        this is safe to display to an operator who may edit the profile.
      -->
      <pre class="overflow-x-auto rounded bg-gray-100 p-3 font-mono text-xs dark:bg-gray-800">{{ userData }}</pre>

      <div class="mt-4 flex justify-end">
        <button class="rounded border px-4 py-1.5" @click="emit('close')">
          {{ $t('netboot.common.close') }}
        </button>
      </div>
    </div>
  </div>
</template>
