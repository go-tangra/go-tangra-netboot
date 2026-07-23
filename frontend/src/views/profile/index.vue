<script lang="ts" setup>
import { onMounted, ref } from 'vue';

import { Page } from 'shell/vben/common-ui';
import { $t } from 'shell/locales';

import { useNetbootProfileStore } from '../../stores/netboot-profile.state';
import type { Profile } from '../../types';
import ProfileDrawer from './profile-drawer.vue';
import PreviewModal from './preview-modal.vue';

const store = useNetbootProfileStore();

const profiles = ref<Profile[]>([]);
const total = ref(0);
const loading = ref(false);
const errorMessage = ref('');

const page = ref(1);
const pageSize = ref(20);

const drawerOpen = ref(false);
const editing = ref<Profile | null>(null);

const previewOpen = ref(false);
const previewUserData = ref('');
const previewCmdline = ref('');

async function load() {
  loading.value = true;
  errorMessage.value = '';
  try {
    const result = await store.listProfiles({ page: page.value, pageSize: pageSize.value });
    profiles.value = result.profiles ?? [];
    total.value = result.meta?.total ?? 0;
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

function openEdit(profile: Profile) {
  editing.value = profile;
  drawerOpen.value = true;
}

async function onSaved() {
  drawerOpen.value = false;
  await load();
}

async function clone(profile: Profile) {
  if (!profile.id) return;
  const newName = window.prompt($t('netboot.page.profile.cloneName'), `${profile.name}-copy`);
  if (!newName) return;
  await store.cloneProfile(profile.id, newName);
  await load();
}

async function remove(profile: Profile) {
  if (!profile.id) return;
  if (!window.confirm($t('netboot.page.profile.confirmDelete'))) return;
  await store.deleteProfile(profile.id);
  await load();
}

// The rendered seed comes back with credentials already redacted by the
// netboot server; nothing here needs to sanitise it further.
async function preview(profile: Profile) {
  if (!profile.id) return;
  const result = await store.previewProfile(profile.id);
  previewUserData.value = result.userData ?? '';
  previewCmdline.value = result.cmdline ?? '';
  previewOpen.value = true;
}

onMounted(load);
</script>

<template>
  <Page :title="$t('netboot.page.profile.title')">
    <div class="space-y-4">
      <div v-if="errorMessage" class="rounded border border-red-300 bg-red-50 p-3 text-red-800">
        {{ errorMessage }}
      </div>

      <div class="flex items-center gap-2">
        <button class="rounded border px-3 py-1.5" :disabled="loading" @click="load">
          {{ $t('netboot.common.refresh') }}
        </button>
        <button class="rounded bg-primary px-3 py-1.5 text-white" @click="openCreate">
          {{ $t('netboot.page.profile.create') }}
        </button>
      </div>

      <div class="overflow-x-auto rounded border">
        <table class="w-full text-left text-sm">
          <thead class="bg-gray-50 dark:bg-gray-800">
            <tr>
              <th class="px-3 py-2">{{ $t('netboot.page.profile.name') }}</th>
              <th class="px-3 py-2">{{ $t('netboot.page.profile.ubuntuRelease') }}</th>
              <th class="px-3 py-2">{{ $t('netboot.page.profile.version') }}</th>
              <th class="px-3 py-2">{{ $t('netboot.page.profile.installUsername') }}</th>
              <th class="px-3 py-2">{{ $t('netboot.page.profile.password') }}</th>
              <th class="px-3 py-2">{{ $t('netboot.page.profile.assignedMachines') }}</th>
              <th class="px-3 py-2">{{ $t('netboot.common.actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="profile in profiles" :key="profile.id" class="border-t">
              <td class="px-3 py-2 font-medium">{{ profile.name }}</td>
              <td class="px-3 py-2">{{ profile.ubuntuRelease }}</td>
              <td class="px-3 py-2">{{ profile.version }}</td>
              <td class="px-3 py-2">{{ profile.installUsername || 'ubuntu' }}</td>
              <td class="px-3 py-2 text-xs">
                {{
                  profile.hasPassword
                    ? $t('netboot.page.profile.passwordSet')
                    : $t('netboot.page.profile.passwordUnset')
                }}
              </td>
              <td class="px-3 py-2">{{ profile.assignedMachines ?? 0 }}</td>
              <td class="space-x-2 px-3 py-2">
                <button class="text-primary" @click="openEdit(profile)">
                  {{ $t('netboot.common.edit') }}
                </button>
                <button class="text-primary" @click="preview(profile)">
                  {{ $t('netboot.page.profile.preview') }}
                </button>
                <button class="text-primary" @click="clone(profile)">
                  {{ $t('netboot.page.profile.clone') }}
                </button>
                <button class="text-red-600" @click="remove(profile)">
                  {{ $t('netboot.common.delete') }}
                </button>
              </td>
            </tr>
            <tr v-if="!loading && profiles.length === 0">
              <td class="px-3 py-6 text-center text-gray-500" colspan="7">—</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="flex items-center justify-between text-sm">
        <span>{{ total }}</span>
        <div class="space-x-2">
          <button class="rounded border px-2 py-1" :disabled="page <= 1" @click="page -= 1; load()">
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
    </div>

    <ProfileDrawer
      v-if="drawerOpen"
      :profile="editing"
      @close="drawerOpen = false"
      @saved="onSaved"
    />
    <PreviewModal
      v-if="previewOpen"
      :user-data="previewUserData"
      :cmdline="previewCmdline"
      @close="previewOpen = false"
    />
  </Page>
</template>
