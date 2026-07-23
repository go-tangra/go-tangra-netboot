<script lang="ts" setup>
import { createVNode, onMounted, ref } from 'vue';

import { Page, useVbenDrawer } from 'shell/vben/common-ui';
import { $t } from 'shell/locales';

import {
  Button,
  Input,
  Modal,
  Table,
  Tag,
  notification,
} from 'ant-design-vue';

import { useNetbootProfileStore } from '../../stores/netboot-profile.state';
import type { Profile } from '../../types';
import ProfileDrawer from './profile-drawer.vue';
import PreviewModal from './preview-modal.vue';

const store = useNetbootProfileStore();

const profiles = ref<Profile[]>([]);
const total = ref(0);
const loading = ref(false);

const page = ref(1);
const pageSize = ref(20);

const previewOpen = ref(false);
const previewUserData = ref('');
const previewCmdline = ref('');

const [ProfileDrawerComponent, profileDrawerApi] = useVbenDrawer({
  connectedComponent: ProfileDrawer,
});

async function load() {
  loading.value = true;
  try {
    const result = await store.listProfiles({ page: page.value, pageSize: pageSize.value });
    profiles.value = result.profiles ?? [];
    total.value = result.meta?.total ?? 0;
  } catch (e) {
    notification.error({ message: (e as Error).message || $t('netboot.common.loadFailed') });
  } finally {
    loading.value = false;
  }
}

function openCreate() {
  profileDrawerApi.setData({ mode: 'create' });
  profileDrawerApi.open();
}

function openEdit(profile: Profile) {
  profileDrawerApi.setData({ mode: 'edit', profile });
  profileDrawerApi.open();
}

// Clone prompts for the new name in a proper modal with an input, rather than
// a browser window.prompt.
function clone(profile: Profile) {
  if (!profile.id) return;
  const nameRef = ref(`${profile.name}-copy`);
  Modal.confirm({
    title: $t('netboot.page.profile.clone'),
    content: createVNode(Input, {
      value: nameRef.value,
      'onUpdate:value': (v: string) => (nameRef.value = v),
      placeholder: $t('netboot.page.profile.cloneName'),
    }),
    async onOk() {
      const newName = nameRef.value.trim();
      if (!newName) return;
      await store.cloneProfile(profile.id!, newName);
      notification.success({ message: $t('netboot.page.profile.cloneSuccess') });
      await load();
    },
  });
}

function remove(profile: Profile) {
  if (!profile.id) return;
  Modal.confirm({
    title: $t('netboot.page.profile.delete'),
    content: $t('netboot.page.profile.confirmDelete'),
    okType: 'danger',
    async onOk() {
      await store.deleteProfile(profile.id!);
      notification.success({ message: $t('netboot.page.profile.deleteSuccess') });
      await load();
    },
  });
}

// The rendered seed comes back with credentials already redacted upstream.
async function preview(profile: Profile) {
  if (!profile.id) return;
  try {
    const result = await store.previewProfile(profile.id);
    previewUserData.value = result.userData ?? '';
    previewCmdline.value = result.cmdline ?? '';
    previewOpen.value = true;
  } catch (e) {
    notification.error({ message: (e as Error).message || $t('netboot.common.loadFailed') });
  }
}

const columns = [
  { title: $t('netboot.page.profile.name'), key: 'name' },
  { title: $t('netboot.page.profile.ubuntuRelease'), key: 'ubuntuRelease' },
  { title: $t('netboot.page.profile.version'), key: 'version' },
  { title: $t('netboot.page.profile.installUsername'), key: 'installUsername' },
  { title: $t('netboot.page.profile.password'), key: 'password' },
  { title: $t('netboot.page.profile.assignedMachines'), key: 'assignedMachines' },
  { title: $t('netboot.common.actions'), key: 'actions', width: 280 },
];

onMounted(load);
</script>

<template>
  <Page :title="$t('netboot.page.profile.title')">
    <div class="space-y-4">
      <div class="flex items-center gap-2">
        <Button :loading="loading" @click="load">{{ $t('netboot.common.refresh') }}</Button>
        <Button type="primary" @click="openCreate">{{ $t('netboot.page.profile.create') }}</Button>
      </div>

      <Table
        :columns="columns"
        :data-source="profiles"
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
          <template v-else-if="column.key === 'version'">v{{ record.version }}</template>
          <template v-else-if="column.key === 'installUsername'">
            {{ record.installUsername || 'ubuntu' }}
          </template>
          <template v-else-if="column.key === 'password'">
            <Tag :color="record.hasPassword ? 'green' : 'default'">
              {{
                record.hasPassword
                  ? $t('netboot.page.profile.passwordSet')
                  : $t('netboot.page.profile.passwordUnset')
              }}
            </Tag>
          </template>
          <template v-else-if="column.key === 'assignedMachines'">
            {{ record.assignedMachines ?? 0 }}
          </template>
          <template v-else-if="column.key === 'actions'">
            <Button type="link" size="small" @click="openEdit(record)">
              {{ $t('netboot.common.edit') }}
            </Button>
            <Button type="link" size="small" @click="preview(record)">
              {{ $t('netboot.page.profile.preview') }}
            </Button>
            <Button type="link" size="small" @click="clone(record)">
              {{ $t('netboot.page.profile.clone') }}
            </Button>
            <Button type="link" size="small" danger @click="remove(record)">
              {{ $t('netboot.common.delete') }}
            </Button>
          </template>
        </template>
      </Table>
    </div>

    <ProfileDrawerComponent @saved="load" />
    <PreviewModal
      v-if="previewOpen"
      :user-data="previewUserData"
      :cmdline="previewCmdline"
      @close="previewOpen = false"
    />
  </Page>
</template>
