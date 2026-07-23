<script lang="ts" setup>
import { computed, onMounted, ref } from 'vue';

import { Page, useVbenDrawer } from 'shell/vben/common-ui';
import { $t } from 'shell/locales';

import {
  Button,
  Empty,
  Input,
  Modal,
  Select,
  SelectOption,
  Table,
  Tag,
  notification,
} from 'ant-design-vue';

import { useNetbootMachineStore } from '../../stores/netboot-machine.state';
import { useNetbootProfileStore } from '../../stores/netboot-profile.state';
import type { Machine, Profile, ProvisionState, UnknownBoot } from '../../types';
import MachineDrawer from './machine-drawer.vue';

const machineStore = useNetbootMachineStore();
const profileStore = useNetbootProfileStore();

const machines = ref<Machine[]>([]);
const unknownBoots = ref<UnknownBoot[]>([]);
const profiles = ref<Profile[]>([]);
const total = ref(0);
const loading = ref(false);

const page = ref(1);
const pageSize = ref(20);
const search = ref('');
const stateFilter = ref<string>('');

const [MachineDrawerComponent, machineDrawerApi] = useVbenDrawer({
  connectedComponent: MachineDrawer,
});

const profileNames = computed(() => {
  const byId = new Map<string, string>();
  for (const profile of profiles.value) {
    if (profile.id) byId.set(profile.id, profile.name ?? profile.id);
  }
  return byId;
});

// Machines mid-install are what an operator is usually watching, so they sort
// first, then failed, then the rest — regardless of upstream ordering.
const orderedMachines = computed(() => {
  const weight = (s?: ProvisionState) =>
    s === 'PROVISION_STATE_INSTALLING' ? 0 : s === 'PROVISION_STATE_FAILED' ? 1 : 2;
  return [...machines.value].sort(
    (a, b) => weight(a.provisionState) - weight(b.provisionState),
  );
});

const STATE_COLOR: Record<string, string> = {
  PROVISION_STATE_NEW: 'default',
  PROVISION_STATE_READY: 'blue',
  PROVISION_STATE_INSTALLING: 'gold',
  PROVISION_STATE_INSTALLED: 'green',
  PROVISION_STATE_FAILED: 'red',
};

function stateLabel(s?: ProvisionState) {
  return $t(`netboot.enum.provisionState.${s ?? 'PROVISION_STATE_UNSPECIFIED'}`);
}

async function load() {
  loading.value = true;
  try {
    const [machinePage, bootPage, profilePage] = await Promise.all([
      machineStore.listMachines(
        { page: page.value, pageSize: pageSize.value },
        { q: search.value || undefined, state: stateFilter.value || undefined },
      ),
      machineStore.listUnknownBoots({ page: 1, pageSize: 20 }),
      profileStore.listProfiles({ page: 1, pageSize: 200 }),
    ]);
    machines.value = machinePage.machines ?? [];
    total.value = machinePage.meta?.total ?? 0;
    unknownBoots.value = bootPage.boots ?? [];
    profiles.value = profilePage.profiles ?? [];
  } catch (e) {
    notification.error({ message: (e as Error).message || $t('netboot.common.loadFailed') });
  } finally {
    loading.value = false;
  }
}

function openCreate() {
  machineDrawerApi.setData({ mode: 'create' });
  machineDrawerApi.open();
}

function openEdit(machine: Machine) {
  machineDrawerApi.setData({ mode: 'edit', machine });
  machineDrawerApi.open();
}

// Promoting an unknown boot is the same drawer in a dedicated mode, pre-seeded
// with the MAC — no clumsy window.prompt for the name.
function registerUnknown(boot: UnknownBoot) {
  machineDrawerApi.setData({ mode: 'register-unknown', machine: { mac: boot.mac } });
  machineDrawerApi.open();
}

// Arming a machine erases its disks on the next boot, so it is always confirmed.
function provision(machine: Machine) {
  if (!machine.id) return;
  Modal.confirm({
    title: $t('netboot.page.machine.provision'),
    content: $t('netboot.page.machine.confirmProvision'),
    okType: 'danger',
    async onOk() {
      await machineStore.provisionMachine(machine.id!);
      notification.success({ message: $t('netboot.page.machine.provisionSuccess') });
      await load();
    },
  });
}

function cancelProvision(machine: Machine) {
  if (!machine.id) return;
  Modal.confirm({
    title: $t('netboot.page.machine.cancelProvision'),
    content: $t('netboot.page.machine.confirmCancel'),
    async onOk() {
      await machineStore.cancelProvision(machine.id!);
      notification.success({ message: $t('netboot.page.machine.cancelSuccess') });
      await load();
    },
  });
}

function remove(machine: Machine) {
  if (!machine.id) return;
  Modal.confirm({
    title: $t('netboot.page.machine.delete'),
    content: $t('netboot.page.machine.confirmDelete'),
    okType: 'danger',
    async onOk() {
      await machineStore.deleteMachine(machine.id!);
      notification.success({ message: $t('netboot.page.machine.deleteSuccess') });
      await load();
    },
  });
}

const machineColumns = [
  { title: $t('netboot.page.machine.name'), key: 'name' },
  { title: $t('netboot.page.machine.mac'), key: 'mac' },
  { title: $t('netboot.page.machine.firmware'), key: 'firmware' },
  { title: $t('netboot.page.machine.profile'), key: 'profile' },
  { title: $t('netboot.page.machine.reservationIp'), key: 'reservationIp' },
  { title: $t('netboot.page.machine.state'), key: 'state' },
  { title: $t('netboot.common.actions'), key: 'actions', width: 260 },
];

const bootColumns = [
  { title: $t('netboot.page.machine.mac'), key: 'mac' },
  { title: $t('netboot.page.machine.lastSeen'), key: 'lastSeen' },
  { title: $t('netboot.page.machine.attempts'), key: 'attempts' },
  { title: '', key: 'actions', width: 130 },
];

onMounted(load);
</script>

<template>
  <Page :title="$t('netboot.page.machine.title')">
    <div class="space-y-4">
      <!-- Toolbar -->
      <div class="flex flex-wrap items-center gap-2">
        <Input
          v-model:value="search"
          allow-clear
          class="w-64"
          :placeholder="$t('netboot.page.machine.search')"
          @press-enter="load"
        />
        <Select
          v-model:value="stateFilter"
          class="w-44"
          :placeholder="$t('netboot.page.machine.state')"
          allow-clear
          @change="load"
        >
          <SelectOption value="PROVISION_STATE_NEW">{{ stateLabel('PROVISION_STATE_NEW') }}</SelectOption>
          <SelectOption value="PROVISION_STATE_READY">{{ stateLabel('PROVISION_STATE_READY') }}</SelectOption>
          <SelectOption value="PROVISION_STATE_INSTALLING">
            {{ stateLabel('PROVISION_STATE_INSTALLING') }}
          </SelectOption>
          <SelectOption value="PROVISION_STATE_INSTALLED">
            {{ stateLabel('PROVISION_STATE_INSTALLED') }}
          </SelectOption>
          <SelectOption value="PROVISION_STATE_FAILED">{{ stateLabel('PROVISION_STATE_FAILED') }}</SelectOption>
        </Select>
        <Button :loading="loading" @click="load">{{ $t('netboot.common.refresh') }}</Button>
        <Button type="primary" @click="openCreate">{{ $t('netboot.page.machine.create') }}</Button>
      </div>

      <!-- Machines -->
      <Table
        :columns="machineColumns"
        :data-source="orderedMachines"
        :loading="loading"
        :pagination="{
          current: page,
          pageSize,
          total,
          showSizeChanger: false,
          onChange: (p: number) => {
            page = p;
            load();
          },
        }"
        row-key="id"
        size="middle"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'name'">
            <span class="font-medium">{{ record.name }}</span>
          </template>
          <template v-else-if="column.key === 'mac'">
            <span class="font-mono text-xs">{{ record.mac }}</span>
          </template>
          <template v-else-if="column.key === 'firmware'">
            {{ $t(`netboot.enum.firmware.${record.firmware ?? 'FIRMWARE_UNSPECIFIED'}`) }}
          </template>
          <template v-else-if="column.key === 'profile'">
            {{ record.profileId ? profileNames.get(record.profileId) : $t('netboot.common.none') }}
          </template>
          <template v-else-if="column.key === 'reservationIp'">
            <span class="font-mono text-xs">{{ record.reservationIp || '—' }}</span>
          </template>
          <template v-else-if="column.key === 'state'">
            <Tag :color="STATE_COLOR[record.provisionState] ?? 'default'">
              {{ stateLabel(record.provisionState) }}
            </Tag>
          </template>
          <template v-else-if="column.key === 'actions'">
            <Button type="link" size="small" @click="openEdit(record)">
              {{ $t('netboot.common.edit') }}
            </Button>
            <Button
              v-if="!record.activeSessionId"
              type="link"
              size="small"
              @click="provision(record)"
            >
              {{ $t('netboot.page.machine.provision') }}
            </Button>
            <Button v-else type="link" size="small" @click="cancelProvision(record)">
              {{ $t('netboot.page.machine.cancelProvision') }}
            </Button>
            <Button type="link" size="small" danger @click="remove(record)">
              {{ $t('netboot.common.delete') }}
            </Button>
          </template>
        </template>
      </Table>

      <!--
        Unknown boots are a first-class security signal: a MAC that requested
        boot but is registered to nobody is either a new machine to onboard or
        something that should not be on the provisioning segment. Shown only
        when present so it doesn't add noise to a clean inventory.
      -->
      <div v-if="unknownBoots.length > 0" class="rounded-lg border border-amber-300 dark:border-amber-700">
        <div class="flex items-center gap-2 border-b border-amber-200 bg-amber-50 px-4 py-2 dark:border-amber-800 dark:bg-amber-950">
          <span class="font-medium">{{ $t('netboot.page.machine.unknownBoots') }}</span>
          <Tag color="orange">{{ unknownBoots.length }}</Tag>
        </div>
        <Table
          :columns="bootColumns"
          :data-source="unknownBoots"
          :pagination="false"
          row-key="mac"
          size="small"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'mac'">
              <span class="font-mono text-xs">{{ record.mac }}</span>
            </template>
            <template v-else-if="column.key === 'lastSeen'">{{ record.lastSeen }}</template>
            <template v-else-if="column.key === 'attempts'">{{ record.attempts }}</template>
            <template v-else-if="column.key === 'actions'">
              <Button type="primary" size="small" ghost @click="registerUnknown(record)">
                {{ $t('netboot.page.machine.register') }}
              </Button>
            </template>
          </template>
          <template #emptyText>
            <Empty :image="Empty.PRESENTED_IMAGE_SIMPLE" />
          </template>
        </Table>
      </div>
    </div>

    <MachineDrawerComponent @saved="load" />
  </Page>
</template>
