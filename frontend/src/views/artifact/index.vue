<script lang="ts" setup>
import { onMounted, ref } from 'vue';

import { Page } from 'shell/vben/common-ui';
import { $t } from 'shell/locales';

import { useNetbootArtifactStore } from '../../stores/netboot-artifact.state';
import type { BootArtifact, Transfer } from '../../types';

const store = useNetbootArtifactStore();

const artifacts = ref<BootArtifact[]>([]);
const transfers = ref<Transfer[]>([]);
const loading = ref(false);
const errorMessage = ref('');

async function load() {
  loading.value = true;
  errorMessage.value = '';
  try {
    const [artifactResult, transferResult] = await Promise.all([
      store.listArtifacts({ page: 1, pageSize: 100 }),
      store.listTransfers({ page: 1, pageSize: 50 }),
    ]);
    artifacts.value = artifactResult.artifacts ?? [];
    transfers.value = transferResult.transfers ?? [];
  } catch (error) {
    errorMessage.value = (error as Error).message || $t('netboot.common.loadFailed');
  } finally {
    loading.value = false;
  }
}

async function remove(artifact: BootArtifact) {
  if (!artifact.id) return;
  if (!window.confirm($t('netboot.page.artifact.confirmDelete'))) return;
  await store.deleteArtifact(artifact.id);
  await load();
}

/** Renders a byte count in the largest unit that keeps it readable. */
function formatSize(bytes?: number): string {
  if (!bytes) return '—';
  const units = ['B', 'KiB', 'MiB', 'GiB'];
  let value = bytes;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit += 1;
  }
  return `${value.toFixed(unit === 0 ? 0 : 1)} ${units[unit]}`;
}

onMounted(load);
</script>

<template>
  <Page :title="$t('netboot.page.artifact.title')">
    <div class="space-y-4">
      <div v-if="errorMessage" class="rounded border border-red-300 bg-red-50 p-3 text-red-800">
        {{ errorMessage }}
      </div>

      <div class="flex items-center gap-2">
        <button class="rounded border px-3 py-1.5" :disabled="loading" @click="load">
          {{ $t('netboot.common.refresh') }}
        </button>
        <!-- Uploads bypass this module entirely; see ArtifactService in the backend. -->
        <span class="text-xs text-gray-500">{{ $t('netboot.page.artifact.uploadHelp') }}</span>
      </div>

      <div class="overflow-x-auto rounded border">
        <table class="w-full text-left text-sm">
          <thead class="bg-gray-50 dark:bg-gray-800">
            <tr>
              <th class="px-3 py-2">{{ $t('netboot.page.artifact.filename') }}</th>
              <th class="px-3 py-2">{{ $t('netboot.page.artifact.kind') }}</th>
              <th class="px-3 py-2">{{ $t('netboot.page.artifact.ubuntuRelease') }}</th>
              <th class="px-3 py-2">{{ $t('netboot.page.artifact.size') }}</th>
              <th class="px-3 py-2">{{ $t('netboot.page.artifact.sha256') }}</th>
              <th class="px-3 py-2">{{ $t('netboot.common.actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="artifact in artifacts" :key="artifact.id" class="border-t">
              <td class="px-3 py-2 font-mono text-xs">{{ artifact.filename }}</td>
              <td class="px-3 py-2">
                {{ $t(`netboot.enum.artifactKind.${artifact.kind ?? 'ARTIFACT_KIND_UNSPECIFIED'}`) }}
              </td>
              <td class="px-3 py-2">{{ artifact.ubuntuRelease || '—' }}</td>
              <td class="px-3 py-2">{{ formatSize(artifact.sizeBytes) }}</td>
              <td class="px-3 py-2 font-mono text-xs">{{ artifact.sha256?.slice(0, 12) }}…</td>
              <td class="px-3 py-2">
                <button class="text-red-600" @click="remove(artifact)">
                  {{ $t('netboot.common.delete') }}
                </button>
              </td>
            </tr>
            <tr v-if="!loading && artifacts.length === 0">
              <td class="px-3 py-6 text-center text-gray-500" colspan="6">—</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="rounded border">
        <div class="border-b bg-gray-50 px-3 py-2 font-medium dark:bg-gray-800">
          {{ $t('netboot.page.artifact.transfers') }}
        </div>
        <table class="w-full text-left text-sm">
          <thead>
            <tr>
              <th class="px-3 py-2">{{ $t('netboot.page.artifact.time') }}</th>
              <th class="px-3 py-2">{{ $t('netboot.page.artifact.clientIp') }}</th>
              <th class="px-3 py-2">{{ $t('netboot.page.artifact.filename') }}</th>
              <th class="px-3 py-2">{{ $t('netboot.page.artifact.protocol') }}</th>
              <th class="px-3 py-2">{{ $t('netboot.page.artifact.bytesSent') }}</th>
              <th class="px-3 py-2">{{ $t('netboot.page.artifact.success') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(transfer, index) in transfers" :key="index" class="border-t">
              <td class="px-3 py-2 text-xs">{{ transfer.time }}</td>
              <td class="px-3 py-2 font-mono text-xs">{{ transfer.clientIp }}</td>
              <td class="px-3 py-2 font-mono text-xs">{{ transfer.filename }}</td>
              <td class="px-3 py-2">
                {{
                  $t(
                    `netboot.enum.transferProtocol.${
                      transfer.protocol ?? 'TRANSFER_PROTOCOL_UNSPECIFIED'
                    }`,
                  )
                }}
              </td>
              <td class="px-3 py-2">{{ formatSize(transfer.bytesSent) }}</td>
              <td class="px-3 py-2">
                <span :class="transfer.success ? 'text-green-600' : 'text-red-600'">
                  {{ transfer.success ? $t('netboot.common.yes') : transfer.error }}
                </span>
              </td>
            </tr>
            <tr v-if="transfers.length === 0">
              <td class="px-3 py-6 text-center text-gray-500" colspan="6">—</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </Page>
</template>
