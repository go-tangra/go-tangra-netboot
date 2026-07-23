<script lang="ts" setup>
import { onMounted, ref } from 'vue';

import { Page } from 'shell/vben/common-ui';
import { $t } from 'shell/locales';

import { Alert, Button, Card, Modal, Table, Tag, notification } from 'ant-design-vue';

import { useNetbootArtifactStore } from '../../stores/netboot-artifact.state';
import type { BootArtifact, Transfer } from '../../types';

const store = useNetbootArtifactStore();

const artifacts = ref<BootArtifact[]>([]);
const transfers = ref<Transfer[]>([]);
const loading = ref(false);

const KIND_COLOR: Record<string, string> = {
  ARTIFACT_KIND_KERNEL: 'blue',
  ARTIFACT_KIND_INITRD: 'purple',
  ARTIFACT_KIND_IPXE_BIN: 'geekblue',
  ARTIFACT_KIND_OTHER: 'default',
};

async function load() {
  loading.value = true;
  try {
    const [artifactResult, transferResult] = await Promise.all([
      store.listArtifacts({ page: 1, pageSize: 100 }),
      store.listTransfers({ page: 1, pageSize: 50 }),
    ]);
    artifacts.value = artifactResult.artifacts ?? [];
    transfers.value = transferResult.transfers ?? [];
  } catch (e) {
    notification.error({ message: (e as Error).message || $t('netboot.common.loadFailed') });
  } finally {
    loading.value = false;
  }
}

function remove(artifact: BootArtifact) {
  if (!artifact.id) return;
  Modal.confirm({
    title: $t('netboot.common.delete'),
    content: $t('netboot.page.artifact.confirmDelete'),
    okType: 'danger',
    async onOk() {
      await store.deleteArtifact(artifact.id!);
      notification.success({ message: $t('netboot.page.artifact.deleteSuccess') });
      await load();
    },
  });
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

const artifactColumns = [
  { title: $t('netboot.page.artifact.filename'), key: 'filename' },
  { title: $t('netboot.page.artifact.kind'), key: 'kind' },
  { title: $t('netboot.page.artifact.ubuntuRelease'), key: 'ubuntuRelease' },
  { title: $t('netboot.page.artifact.size'), key: 'size' },
  { title: $t('netboot.page.artifact.sha256'), key: 'sha256' },
  { title: $t('netboot.common.actions'), key: 'actions', width: 120 },
];

const transferColumns = [
  { title: $t('netboot.page.artifact.time'), key: 'time' },
  { title: $t('netboot.page.artifact.clientIp'), key: 'clientIp' },
  { title: $t('netboot.page.artifact.filename'), key: 'filename' },
  { title: $t('netboot.page.artifact.protocol'), key: 'protocol' },
  { title: $t('netboot.page.artifact.bytesSent'), key: 'bytesSent' },
  { title: $t('netboot.page.artifact.success'), key: 'success' },
];

onMounted(load);
</script>

<template>
  <Page :title="$t('netboot.page.artifact.title')">
    <div class="space-y-4">
      <div class="flex items-center gap-2">
        <Button :loading="loading" @click="load">{{ $t('netboot.common.refresh') }}</Button>
      </div>

      <!-- Uploads bypass this module entirely; the view is read-only + delete. -->
      <Alert type="info" show-icon :message="$t('netboot.page.artifact.uploadHelp')" />

      <Card size="small" :title="$t('netboot.page.artifact.title')">
        <Table
          :columns="artifactColumns"
          :data-source="artifacts"
          :loading="loading"
          :pagination="false"
          row-key="id"
          size="middle"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'filename'">
              <span class="font-mono text-xs">{{ record.filename }}</span>
            </template>
            <template v-else-if="column.key === 'kind'">
              <Tag :color="KIND_COLOR[record.kind] ?? 'default'">
                {{ $t(`netboot.enum.artifactKind.${record.kind ?? 'ARTIFACT_KIND_UNSPECIFIED'}`) }}
              </Tag>
            </template>
            <template v-else-if="column.key === 'ubuntuRelease'">
              {{ record.ubuntuRelease || '—' }}
            </template>
            <template v-else-if="column.key === 'size'">{{ formatSize(record.sizeBytes) }}</template>
            <template v-else-if="column.key === 'sha256'">
              <span class="font-mono text-xs">{{ record.sha256?.slice(0, 12) }}…</span>
            </template>
            <template v-else-if="column.key === 'actions'">
              <Button type="link" size="small" danger @click="remove(record)">
                {{ $t('netboot.common.delete') }}
              </Button>
            </template>
          </template>
        </Table>
      </Card>

      <Card size="small" :title="$t('netboot.page.artifact.transfers')">
        <Table
          :columns="transferColumns"
          :data-source="transfers"
          :loading="loading"
          :pagination="false"
          row-key="time"
          size="middle"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'time'">
              <span class="text-xs">{{ record.time }}</span>
            </template>
            <template v-else-if="column.key === 'clientIp'">
              <span class="font-mono text-xs">{{ record.clientIp }}</span>
            </template>
            <template v-else-if="column.key === 'filename'">
              <span class="font-mono text-xs">{{ record.filename }}</span>
            </template>
            <template v-else-if="column.key === 'protocol'">
              <Tag>
                {{
                  $t(
                    `netboot.enum.transferProtocol.${
                      record.protocol ?? 'TRANSFER_PROTOCOL_UNSPECIFIED'
                    }`,
                  )
                }}
              </Tag>
            </template>
            <template v-else-if="column.key === 'bytesSent'">{{ formatSize(record.bytesSent) }}</template>
            <template v-else-if="column.key === 'success'">
              <Tag :color="record.success ? 'green' : 'red'">
                {{ record.success ? $t('netboot.common.yes') : (record.error || $t('netboot.common.no')) }}
              </Tag>
            </template>
          </template>
        </Table>
      </Card>
    </div>
  </Page>
</template>
