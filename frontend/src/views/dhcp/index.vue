<script lang="ts" setup>
import { onMounted, ref } from 'vue';

import { Page } from 'shell/vben/common-ui';
import { $t } from 'shell/locales';

import {
  Alert,
  Badge,
  Button,
  Card,
  Empty,
  Input,
  InputNumber,
  Modal,
  Table,
  notification,
} from 'ant-design-vue';

import { useNetbootDhcpStore } from '../../stores/netboot-dhcp.state';
import type { DhcpConfig, DhcpSubnet, ForeignServer, Lease } from '../../types';

const store = useNetbootDhcpStore();

const config = ref<DhcpConfig>({});
const subnets = ref<DhcpSubnet[]>([]);
const leaseTtl = ref(3600);
const leases = ref<Lease[]>([]);
const conflicts = ref<ForeignServer[]>([]);

const loading = ref(false);
const saving = ref(false);
const toggling = ref(false);

async function load() {
  loading.value = true;
  try {
    const [configResult, leaseResult, conflictResult] = await Promise.all([
      store.getConfig(),
      store.listLeases({ page: 1, pageSize: 50 }),
      store.listConflicts({ page: 1, pageSize: 20 }),
    ]);
    config.value = configResult.config ?? {};
    subnets.value = (configResult.config?.subnets ?? []).map((s) => ({ ...s }));
    leaseTtl.value = configResult.config?.leaseTtlSeconds ?? 3600;
    leases.value = leaseResult.leases ?? [];
    conflicts.value = conflictResult.servers ?? [];
  } catch (e) {
    notification.error({ message: (e as Error).message || $t('netboot.common.loadFailed') });
  } finally {
    loading.value = false;
  }
}

function addSubnet() {
  subnets.value.push({ network: '', rangeStart: '', rangeEnd: '', gateway: '', dns: [] });
}

function removeSubnet(index: number) {
  subnets.value.splice(index, 1);
}

function dnsText(subnet: DhcpSubnet): string {
  return (subnet.dns ?? []).join(', ');
}

function setDns(subnet: DhcpSubnet, value: string) {
  subnet.dns = value
    .split(/[,\s]+/)
    .map((entry) => entry.trim())
    .filter(Boolean);
}

async function save() {
  saving.value = true;
  try {
    const result = await store.updateConfig(leaseTtl.value, subnets.value);
    config.value = result.config ?? {};
    notification.success({ message: $t('netboot.page.dhcp.updateSuccess') });
  } catch (e) {
    notification.error({ message: (e as Error).message || $t('netboot.common.loadFailed') });
  } finally {
    saving.value = false;
  }
}

// Toggling the authoritative DHCP server reshapes a whole network segment, so
// both directions are confirmed.
function toggle() {
  const enabling = !config.value.enabled;
  Modal.confirm({
    title: enabling ? $t('netboot.page.dhcp.enable') : $t('netboot.page.dhcp.disable'),
    content: enabling
      ? $t('netboot.page.dhcp.confirmEnable')
      : $t('netboot.page.dhcp.confirmDisable'),
    okType: enabling ? 'primary' : 'danger',
    async onOk() {
      toggling.value = true;
      try {
        const result = enabling ? await store.enable() : await store.disable();
        config.value = result.config ?? {};
        notification.success({
          message: enabling
            ? $t('netboot.page.dhcp.enableSuccess')
            : $t('netboot.page.dhcp.disableSuccess'),
        });
      } catch (e) {
        notification.error({ message: (e as Error).message || $t('netboot.common.loadFailed') });
      } finally {
        toggling.value = false;
      }
    },
  });
}

const leaseColumns = [
  { title: $t('netboot.page.dhcp.ip'), key: 'ip' },
  { title: $t('netboot.page.machine.mac'), key: 'mac' },
  { title: $t('netboot.page.session.machine'), key: 'machine' },
  { title: $t('netboot.page.dhcp.expiresAt'), key: 'expiresAt' },
];

const conflictColumns = [
  { title: $t('netboot.page.dhcp.serverId'), key: 'serverId' },
  { title: $t('netboot.page.machine.lastSeen'), key: 'lastSeen' },
  { title: $t('netboot.page.dhcp.offersSeen'), key: 'offersSeen' },
];

onMounted(load);
</script>

<template>
  <Page :title="$t('netboot.page.dhcp.title')">
    <div class="space-y-4">
      <!-- Server status + toggle -->
      <Card size="small">
        <div class="flex flex-wrap items-center gap-3">
          <span class="text-sm">{{ $t('netboot.page.dhcp.status') }}:</span>
          <Badge
            :status="config.enabled ? 'success' : 'default'"
            :text="config.enabled ? $t('netboot.page.dhcp.enabled') : $t('netboot.page.dhcp.disabled')"
          />
          <span class="text-xs text-gray-500">
            {{ $t('netboot.page.dhcp.version') }}: {{ config.version ?? 0 }}
          </span>
          <div class="ml-auto flex items-center gap-2">
            <Button
              :type="config.enabled ? 'default' : 'primary'"
              :danger="config.enabled"
              :loading="toggling"
              @click="toggle"
            >
              {{ config.enabled ? $t('netboot.page.dhcp.disable') : $t('netboot.page.dhcp.enable') }}
            </Button>
            <Button :loading="loading" @click="load">{{ $t('netboot.common.refresh') }}</Button>
          </div>
        </div>
      </Card>

      <!-- Foreign DHCP servers (rogue server warning) -->
      <Alert v-if="conflicts.length > 0" type="warning" show-icon>
        <template #message>{{ $t('netboot.page.dhcp.conflicts') }}</template>
        <template #description>
          <div class="mb-2">{{ $t('netboot.page.dhcp.conflictsHelp') }}</div>
          <Table
            :columns="conflictColumns"
            :data-source="conflicts"
            :pagination="false"
            row-key="serverId"
            size="small"
          >
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'serverId'">
                <span class="font-mono text-xs">{{ record.serverId }}</span>
              </template>
              <template v-else-if="column.key === 'lastSeen'">
                <span class="text-xs">{{ record.lastSeen }}</span>
              </template>
              <template v-else-if="column.key === 'offersSeen'">{{ record.offersSeen }}</template>
            </template>
          </Table>
        </template>
      </Alert>

      <!-- Config editor -->
      <Card size="small" :title="$t('netboot.page.dhcp.subnets')">
        <template #extra>
          <div class="flex items-center gap-2">
            <span class="text-sm">{{ $t('netboot.page.dhcp.leaseTtl') }}</span>
            <InputNumber v-model:value="leaseTtl" :min="60" :max="604800" class="w-32" />
            <Button @click="addSubnet">{{ $t('netboot.page.dhcp.addSubnet') }}</Button>
            <Button type="primary" :loading="saving" @click="save">
              {{ $t('netboot.common.save') }}
            </Button>
          </div>
        </template>

        <div v-if="subnets.length === 0">
          <Empty :description="$t('netboot.page.dhcp.noSubnets')" />
        </div>
        <div
          v-for="(subnet, index) in subnets"
          :key="index"
          class="mb-2 grid grid-cols-12 items-center gap-2"
        >
          <Input
            v-model:value="subnet.network"
            class="col-span-3 font-mono text-xs"
            :placeholder="$t('netboot.page.dhcp.network')"
          />
          <Input
            v-model:value="subnet.rangeStart"
            class="col-span-2 font-mono text-xs"
            :placeholder="$t('netboot.page.dhcp.rangeStart')"
          />
          <Input
            v-model:value="subnet.rangeEnd"
            class="col-span-2 font-mono text-xs"
            :placeholder="$t('netboot.page.dhcp.rangeEnd')"
          />
          <Input
            v-model:value="subnet.gateway"
            class="col-span-2 font-mono text-xs"
            :placeholder="$t('netboot.page.dhcp.gateway')"
          />
          <Input
            :value="dnsText(subnet)"
            class="col-span-2 font-mono text-xs"
            :placeholder="$t('netboot.page.dhcp.dns')"
            @update:value="(v: string) => setDns(subnet, v)"
          />
          <Button danger type="text" size="small" class="col-span-1" @click="removeSubnet(index)">
            {{ $t('netboot.page.dhcp.removeSubnet') }}
          </Button>
        </div>
      </Card>

      <!-- Active leases -->
      <Card size="small" :title="$t('netboot.page.dhcp.leases')">
        <Table
          :columns="leaseColumns"
          :data-source="leases"
          :loading="loading"
          :pagination="false"
          row-key="ip"
          size="middle"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'ip'">
              <span class="font-mono text-xs">{{ record.ip }}</span>
            </template>
            <template v-else-if="column.key === 'mac'">
              <span class="font-mono text-xs">{{ record.mac }}</span>
            </template>
            <template v-else-if="column.key === 'machine'">
              {{ record.machineName || '—' }}
            </template>
            <template v-else-if="column.key === 'expiresAt'">
              <span class="text-xs">{{ record.expiresAt }}</span>
            </template>
          </template>
        </Table>
      </Card>
    </div>
  </Page>
</template>
