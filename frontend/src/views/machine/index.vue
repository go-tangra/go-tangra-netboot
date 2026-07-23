<script lang="ts" setup>
import { computed, onMounted, ref } from 'vue';

import { Page } from 'shell/vben/common-ui';
import { $t } from 'shell/locales';

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
const errorMessage = ref('');

const page = ref(1);
const pageSize = ref(20);
const search = ref('');
const stateFilter = ref<string>('');

const drawerOpen = ref(false);
const editing = ref<Machine | null>(null);

const profileNames = computed(() => {
  const byId = new Map<string, string>();
  for (const profile of profiles.value) {
    if (profile.id) byId.set(profile.id, profile.name ?? profile.id);
  }
  return byId;
});

/**
 * Machines mid-install are the ones an operator is usually watching, so they
 * are surfaced first regardless of the upstream ordering.
 */
const orderedMachines = computed(() => {
  const weight = (state?: ProvisionState) =>
    state === 'PROVISION_STATE_INSTALLING' ? 0 : state === 'PROVISION_STATE_FAILED' ? 1 : 2;
  return [...machines.value].sort((a, b) => weight(a.provisionState) - weight(b.provisionState));
});

async function load() {
  loading.value = true;
  errorMessage.value = '';
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
  } catch (error) {
    errorMessage.value = (error as Error).message || $t('netboot.common.loadFailed');
  } finally {
    loading.value = false;
  }
}

function openCreate() {
  editing.value = null;
  drawerOpen.value = true;
}

function openEdit(machine: Machine) {
  editing.value = machine;
  drawerOpen.value = true;
}

async function onSaved() {
  drawerOpen.value = false;
  await load();
}

// Provisioning erases the target's disks on its next boot, so it is always
// confirmed explicitly rather than being a one-click action.
async function provision(machine: Machine) {
  if (!machine.id) return;
  if (!window.confirm($t('netboot.page.machine.confirmProvision'))) return;
  await machineStore.provisionMachine(machine.id);
  await load();
}

async function cancelProvision(machine: Machine) {
  if (!machine.id) return;
  if (!window.confirm($t('netboot.page.machine.confirmCancel'))) return;
  await machineStore.cancelProvision(machine.id);
  await load();
}

async function remove(machine: Machine) {
  if (!machine.id) return;
  if (!window.confirm($t('netboot.page.machine.confirmDelete'))) return;
  await machineStore.deleteMachine(machine.id);
  await load();
}

async function registerUnknown(boot: UnknownBoot) {
  if (!boot.mac) return;
  const name = window.prompt($t('netboot.page.machine.name'), boot.mac.replace(/:/g, '-'));
  if (!name) return;
  await machineStore.registerUnknown(boot.mac, name);
  await load();
}

function stateLabel(state?: ProvisionState) {
  return $t(`netboot.enum.provisionState.${state ?? 'PROVISION_STATE_UNSPECIFIED'}`);
}

function stateClass(state?: ProvisionState) {
  switch (state) {
    case 'PROVISION_STATE_INSTALLING':
      return 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200';
    case 'PROVISION_STATE_INSTALLED':
      return 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200';
    case 'PROVISION_STATE_FAILED':
      return 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200';
    default:
      return 'bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-200';
  }
}

onMounted(load);
</script>

<template>
  <Page :title="$t('netboot.page.machine.title')">
    <div class="space-y-4">
      <div v-if="errorMessage" class="rounded border border-red-300 bg-red-50 p-3 text-red-800">
        {{ errorMessage }}
      </div>

      <div class="flex flex-wrap items-center gap-2">
        <input
          v-model="search"
          class="rounded border px-3 py-1.5"
          :placeholder="$t('netboot.page.machine.search')"
          @keyup.enter="load"
        />
        <select v-model="stateFilter" class="rounded border px-3 py-1.5" @change="load">
          <option value="">{{ $t('netboot.page.machine.state') }}</option>
          <option value="PROVISION_STATE_NEW">{{ stateLabel('PROVISION_STATE_NEW') }}</option>
          <option value="PROVISION_STATE_READY">{{ stateLabel('PROVISION_STATE_READY') }}</option>
          <option value="PROVISION_STATE_INSTALLING">
            {{ stateLabel('PROVISION_STATE_INSTALLING') }}
          </option>
          <option value="PROVISION_STATE_INSTALLED">
            {{ stateLabel('PROVISION_STATE_INSTALLED') }}
          </option>
          <option value="PROVISION_STATE_FAILED">{{ stateLabel('PROVISION_STATE_FAILED') }}</option>
        </select>
        <button class="rounded border px-3 py-1.5" :disabled="loading" @click="load">
          {{ $t('netboot.common.refresh') }}
        </button>
        <button class="rounded bg-primary px-3 py-1.5 text-white" @click="openCreate">
          {{ $t('netboot.page.machine.create') }}
        </button>
      </div>

      <div class="overflow-x-auto rounded border">
        <table class="w-full text-left text-sm">
          <thead class="bg-gray-50 dark:bg-gray-800">
            <tr>
              <th class="px-3 py-2">{{ $t('netboot.page.machine.name') }}</th>
              <th class="px-3 py-2">{{ $t('netboot.page.machine.mac') }}</th>
              <th class="px-3 py-2">{{ $t('netboot.page.machine.firmware') }}</th>
              <th class="px-3 py-2">{{ $t('netboot.page.machine.profile') }}</th>
              <th class="px-3 py-2">{{ $t('netboot.page.machine.reservationIp') }}</th>
              <th class="px-3 py-2">{{ $t('netboot.page.machine.state') }}</th>
              <th class="px-3 py-2">{{ $t('netboot.common.actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="machine in orderedMachines" :key="machine.id" class="border-t">
              <td class="px-3 py-2 font-medium">{{ machine.name }}</td>
              <td class="px-3 py-2 font-mono text-xs">{{ machine.mac }}</td>
              <td class="px-3 py-2">
                {{ $t(`netboot.enum.firmware.${machine.firmware ?? 'FIRMWARE_UNSPECIFIED'}`) }}
              </td>
              <td class="px-3 py-2">
                {{ machine.profileId ? profileNames.get(machine.profileId) : $t('netboot.common.none') }}
              </td>
              <td class="px-3 py-2 font-mono text-xs">{{ machine.reservationIp || '—' }}</td>
              <td class="px-3 py-2">
                <span class="rounded px-2 py-0.5 text-xs" :class="stateClass(machine.provisionState)">
                  {{ stateLabel(machine.provisionState) }}
                </span>
              </td>
              <td class="space-x-2 px-3 py-2">
                <button class="text-primary" @click="openEdit(machine)">
                  {{ $t('netboot.common.edit') }}
                </button>
                <button
                  v-if="!machine.activeSessionId"
                  class="text-amber-600"
                  @click="provision(machine)"
                >
                  {{ $t('netboot.page.machine.provision') }}
                </button>
                <button v-else class="text-amber-600" @click="cancelProvision(machine)">
                  {{ $t('netboot.page.machine.cancelProvision') }}
                </button>
                <button class="text-red-600" @click="remove(machine)">
                  {{ $t('netboot.common.delete') }}
                </button>
              </td>
            </tr>
            <tr v-if="!loading && orderedMachines.length === 0">
              <td class="px-3 py-6 text-center text-gray-500" colspan="7">—</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="flex items-center justify-between text-sm">
        <span>{{ total }}</span>
        <div class="space-x-2">
          <button
            class="rounded border px-2 py-1"
            :disabled="page <= 1"
            @click="page -= 1; load()"
          >
            ‹
          </button>
          <span>{{ page }}</span>
          <button
            class="rounded border px-2 py-1"
            :disabled="page * pageSize >= total"
            @click="page += 1; load()"
          >
            ›
          </button>
        </div>
      </div>

      <!--
        Unknown boots are a security signal as much as an inventory gap: a MAC
        requesting boot that nobody registered is either a new machine or
        something that should not be on the provisioning segment.
      -->
      <div v-if="unknownBoots.length > 0" class="rounded border">
        <div class="border-b bg-gray-50 px-3 py-2 font-medium dark:bg-gray-800">
          {{ $t('netboot.page.machine.unknownBoots') }}
        </div>
        <table class="w-full text-left text-sm">
          <thead>
            <tr>
              <th class="px-3 py-2">{{ $t('netboot.page.machine.mac') }}</th>
              <th class="px-3 py-2">{{ $t('netboot.page.machine.lastSeen') }}</th>
              <th class="px-3 py-2">{{ $t('netboot.page.machine.attempts') }}</th>
              <th class="px-3 py-2">{{ $t('netboot.common.actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="boot in unknownBoots" :key="boot.mac" class="border-t">
              <td class="px-3 py-2 font-mono text-xs">{{ boot.mac }}</td>
              <td class="px-3 py-2">{{ boot.lastSeen }}</td>
              <td class="px-3 py-2">{{ boot.attempts }}</td>
              <td class="px-3 py-2">
                <button class="text-primary" @click="registerUnknown(boot)">
                  {{ $t('netboot.page.machine.register') }}
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <MachineDrawer
      v-if="drawerOpen"
      :machine="editing"
      :profiles="profiles"
      @close="drawerOpen = false"
      @saved="onSaved"
    />
  </Page>
</template>
