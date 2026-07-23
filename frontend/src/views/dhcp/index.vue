<script lang="ts" setup>
import { onMounted, ref } from 'vue';

import { Page } from 'shell/vben/common-ui';
import { $t } from 'shell/locales';

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
const errorMessage = ref('');

async function load() {
  loading.value = true;
  errorMessage.value = '';
  try {
    const [configResult, leaseResult, conflictResult] = await Promise.all([
      store.getConfig(),
      store.listLeases({ page: 1, pageSize: 50 }),
      store.listConflicts({ page: 1, pageSize: 20 }),
    ]);
    config.value = configResult.config ?? {};
    subnets.value = (configResult.config?.subnets ?? []).map((subnet) => ({ ...subnet }));
    leaseTtl.value = configResult.config?.leaseTtlSeconds ?? 3600;
    leases.value = leaseResult.leases ?? [];
    conflicts.value = conflictResult.servers ?? [];
  } catch (error) {
    errorMessage.value = (error as Error).message || $t('netboot.common.loadFailed');
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
    .split(',')
    .map((entry) => entry.trim())
    .filter(Boolean);
}

async function save() {
  saving.value = true;
  errorMessage.value = '';
  try {
    const result = await store.updateConfig(leaseTtl.value, subnets.value);
    config.value = result.config ?? {};
  } catch (error) {
    errorMessage.value = (error as Error).message;
  } finally {
    saving.value = false;
  }
}

// Enabling and disabling the authoritative DHCP server changes the behaviour
// of an entire network segment, so both directions are confirmed.
async function toggle() {
  const enabling = !config.value.enabled;
  const prompt = enabling
    ? $t('netboot.page.dhcp.confirmEnable')
    : $t('netboot.page.dhcp.confirmDisable');
  if (!window.confirm(prompt)) return;

  const result = enabling ? await store.enable() : await store.disable();
  config.value = result.config ?? {};
}

onMounted(load);
</script>

<template>
  <Page :title="$t('netboot.page.dhcp.title')">
    <div class="space-y-6">
      <div v-if="errorMessage" class="rounded border border-red-300 bg-red-50 p-3 text-red-800">
        {{ errorMessage }}
      </div>

      <div class="flex items-center gap-3 rounded border p-4">
        <span class="text-sm">{{ $t('netboot.page.dhcp.status') }}:</span>
        <span
          class="rounded px-2 py-0.5 text-xs"
          :class="
            config.enabled
              ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200'
              : 'bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-200'
          "
        >
          {{ config.enabled ? $t('netboot.page.dhcp.enabled') : $t('netboot.page.dhcp.disabled') }}
        </span>
        <span class="text-xs text-gray-500">
          {{ $t('netboot.page.dhcp.version') }}: {{ config.version }}
        </span>
        <button
          class="ml-auto rounded px-3 py-1.5 text-white"
          :class="config.enabled ? 'bg-red-600' : 'bg-primary'"
          @click="toggle"
        >
          {{ config.enabled ? $t('netboot.page.dhcp.disable') : $t('netboot.page.dhcp.enable') }}
        </button>
        <button class="rounded border px-3 py-1.5" :disabled="loading" @click="load">
          {{ $t('netboot.common.refresh') }}
        </button>
      </div>

      <div class="rounded border p-4">
        <div class="mb-3 flex items-center gap-3">
          <label class="text-sm">{{ $t('netboot.page.dhcp.leaseTtl') }}</label>
          <input v-model.number="leaseTtl" type="number" class="w-32 rounded border px-3 py-1.5" />
          <button class="ml-auto rounded border px-3 py-1.5" @click="addSubnet">
            {{ $t('netboot.page.dhcp.addSubnet') }}
          </button>
          <button class="rounded bg-primary px-3 py-1.5 text-white" :disabled="saving" @click="save">
            {{ $t('netboot.common.save') }}
          </button>
        </div>

        <table class="w-full text-left text-sm">
          <thead class="bg-gray-50 dark:bg-gray-800">
            <tr>
              <th class="px-2 py-2">{{ $t('netboot.page.dhcp.network') }}</th>
              <th class="px-2 py-2">{{ $t('netboot.page.dhcp.rangeStart') }}</th>
              <th class="px-2 py-2">{{ $t('netboot.page.dhcp.rangeEnd') }}</th>
              <th class="px-2 py-2">{{ $t('netboot.page.dhcp.gateway') }}</th>
              <th class="px-2 py-2">{{ $t('netboot.page.dhcp.dns') }}</th>
              <th class="px-2 py-2"></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(subnet, index) in subnets" :key="index" class="border-t">
              <td class="px-2 py-1">
                <input v-model="subnet.network" class="w-full rounded border px-2 py-1 font-mono text-xs" />
              </td>
              <td class="px-2 py-1">
                <input v-model="subnet.rangeStart" class="w-full rounded border px-2 py-1 font-mono text-xs" />
              </td>
              <td class="px-2 py-1">
                <input v-model="subnet.rangeEnd" class="w-full rounded border px-2 py-1 font-mono text-xs" />
              </td>
              <td class="px-2 py-1">
                <input v-model="subnet.gateway" class="w-full rounded border px-2 py-1 font-mono text-xs" />
              </td>
              <td class="px-2 py-1">
                <input
                  :value="dnsText(subnet)"
                  class="w-full rounded border px-2 py-1 font-mono text-xs"
                  @input="setDns(subnet, ($event.target as HTMLInputElement).value)"
                />
              </td>
              <td class="px-2 py-1">
                <button class="text-red-600" @click="removeSubnet(index)">
                  {{ $t('netboot.page.dhcp.removeSubnet') }}
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <!--
        A second DHCP server on the provisioning segment races netbootd and
        silently breaks installs, so conflicts are shown prominently rather
        than buried in a diagnostics page.
      -->
      <div v-if="conflicts.length > 0" class="rounded border border-amber-400 bg-amber-50 p-4 dark:bg-amber-950">
        <div class="font-medium">{{ $t('netboot.page.dhcp.conflicts') }}</div>
        <p class="mb-2 text-xs">{{ $t('netboot.page.dhcp.conflictsHelp') }}</p>
        <table class="w-full text-left text-sm">
          <thead>
            <tr>
              <th class="px-2 py-1">{{ $t('netboot.page.dhcp.serverId') }}</th>
              <th class="px-2 py-1">{{ $t('netboot.page.machine.lastSeen') }}</th>
              <th class="px-2 py-1">{{ $t('netboot.page.dhcp.offersSeen') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="server in conflicts" :key="server.serverId" class="border-t">
              <td class="px-2 py-1 font-mono text-xs">{{ server.serverId }}</td>
              <td class="px-2 py-1 text-xs">{{ server.lastSeen }}</td>
              <td class="px-2 py-1">{{ server.offersSeen }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="rounded border">
        <div class="border-b bg-gray-50 px-3 py-2 font-medium dark:bg-gray-800">
          {{ $t('netboot.page.dhcp.leases') }}
        </div>
        <table class="w-full text-left text-sm">
          <thead>
            <tr>
              <th class="px-3 py-2">{{ $t('netboot.page.dhcp.ip') }}</th>
              <th class="px-3 py-2">{{ $t('netboot.page.machine.mac') }}</th>
              <th class="px-3 py-2">{{ $t('netboot.page.session.machine') }}</th>
              <th class="px-3 py-2">{{ $t('netboot.page.dhcp.expiresAt') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="lease in leases" :key="lease.ip" class="border-t">
              <td class="px-3 py-2 font-mono text-xs">{{ lease.ip }}</td>
              <td class="px-3 py-2 font-mono text-xs">{{ lease.mac }}</td>
              <td class="px-3 py-2">{{ lease.machineName || '—' }}</td>
              <td class="px-3 py-2 text-xs">{{ lease.expiresAt }}</td>
            </tr>
            <tr v-if="leases.length === 0">
              <td class="px-3 py-6 text-center text-gray-500" colspan="4">—</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </Page>
</template>
