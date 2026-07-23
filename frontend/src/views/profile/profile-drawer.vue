<script lang="ts" setup>
import { computed, ref } from 'vue';

import { useVbenDrawer } from 'shell/vben/common-ui';
import { $t } from 'shell/locales';

import {
  Button,
  Checkbox,
  Divider,
  Form,
  FormItem,
  Input,
  InputPassword,
  Select,
  SelectOption,
  Textarea,
  notification,
} from 'ant-design-vue';

import { useNetbootProfileStore } from '../../stores/netboot-profile.state';
import type { Profile, ProfileInput } from '../../types';

const profileStore = useNetbootProfileStore();

const emit = defineEmits<{ saved: [] }>();

const data = ref<{ mode: 'create' | 'edit'; profile?: Profile }>();
const loading = ref(false);

// Ubuntu releases the netbootd instance ships artifacts for. A plain select is
// friendlier (and less error-prone) than a free-text field.
const releaseOptions = [
  { value: 'noble', label: 'Ubuntu 24.04 (noble)' },
  { value: 'jammy', label: 'Ubuntu 22.04 (jammy)' },
];

// Line-oriented lists are edited as textareas (one entry per line) and split on
// save — far friendlier than a comma string for SSH keys and package lists.
const formState = ref({
  name: '',
  ubuntuRelease: 'noble',
  installUsername: '',
  password: '',
  clearPassword: false,
  keyboardLayout: '',
  locale: '',
  timezone: '',
  packages: '',
  sshAuthorizedKeys: '',
  defaultDns: '',
  storageLayout: '',
  networkConfig: '',
  lateCommands: '',
  kernelCmdlineExtra: '',
  userDataTemplate: '',
});

const isEdit = computed(() => data.value?.mode === 'edit');
const hasExistingPassword = computed(() => !!data.value?.profile?.hasPassword);
const title = computed(() =>
  isEdit.value ? $t('netboot.page.profile.edit') : $t('netboot.page.profile.create'),
);

const nameRule = [
  { required: true, message: $t('netboot.rule.nameRequired') },
  { max: 255, message: $t('netboot.rule.tooLong') },
];

function splitLines(value: string): string[] {
  return value
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean);
}

function splitCommas(value: string): string[] {
  return value
    .split(/[,\s]+/)
    .map((entry) => entry.trim())
    .filter(Boolean);
}

function resetForm() {
  formState.value = {
    name: '',
    ubuntuRelease: 'noble',
    installUsername: '',
    password: '',
    clearPassword: false,
    keyboardLayout: '',
    locale: '',
    timezone: '',
    packages: '',
    sshAuthorizedKeys: '',
    defaultDns: '',
    storageLayout: '',
    networkConfig: '',
    lateCommands: '',
    kernelCmdlineExtra: '',
    userDataTemplate: '',
  };
}

function hydrate(profile: Profile) {
  resetForm();
  formState.value.name = profile.name ?? '';
  formState.value.ubuntuRelease = profile.ubuntuRelease ?? 'noble';
  formState.value.installUsername = profile.installUsername ?? '';
  formState.value.keyboardLayout = profile.keyboardLayout ?? '';
  formState.value.locale = profile.locale ?? '';
  formState.value.timezone = profile.timezone ?? '';
  formState.value.packages = (profile.packages ?? []).join('\n');
  formState.value.sshAuthorizedKeys = (profile.sshAuthorizedKeys ?? []).join('\n');
  formState.value.defaultDns = (profile.defaultDns ?? []).join(', ');
  formState.value.storageLayout = profile.storageLayout ?? '';
  formState.value.networkConfig = profile.networkConfig ?? '';
  formState.value.lateCommands = (profile.lateCommands ?? []).join('\n');
  formState.value.kernelCmdlineExtra = profile.kernelCmdlineExtra ?? '';
  formState.value.userDataTemplate = profile.userDataTemplate ?? '';
}

async function handleSubmit() {
  loading.value = true;
  try {
    const payload: ProfileInput = {
      name: formState.value.name.trim(),
      ubuntuRelease: formState.value.ubuntuRelease,
      installUsername: formState.value.installUsername.trim(),
      keyboardLayout: formState.value.keyboardLayout.trim(),
      locale: formState.value.locale.trim(),
      timezone: formState.value.timezone.trim(),
      packages: splitLines(formState.value.packages),
      sshAuthorizedKeys: splitLines(formState.value.sshAuthorizedKeys),
      defaultDns: splitCommas(formState.value.defaultDns),
      storageLayout: formState.value.storageLayout.trim(),
      networkConfig: formState.value.networkConfig.trim(),
      lateCommands: splitLines(formState.value.lateCommands),
      kernelCmdlineExtra: formState.value.kernelCmdlineExtra.trim(),
      userDataTemplate: formState.value.userDataTemplate,
      clearPassword: formState.value.clearPassword,
    };
    // Only send the password when one was typed: an empty value means "keep the
    // existing password" upstream, so sending it explicitly would be ambiguous.
    if (formState.value.password) payload.password = formState.value.password;

    if (isEdit.value && data.value?.profile?.id) {
      await profileStore.updateProfile(data.value.profile.id, payload);
      notification.success({ message: $t('netboot.page.profile.updateSuccess') });
    } else {
      await profileStore.createProfile(payload);
      notification.success({ message: $t('netboot.page.profile.createSuccess') });
    }
    // Drop the plaintext from memory as soon as it has been sent.
    formState.value.password = '';
    drawerApi.close();
    emit('saved');
  } catch (e) {
    notification.error({ message: (e as Error).message || $t('netboot.common.loadFailed') });
  } finally {
    loading.value = false;
  }
}

const [Drawer, drawerApi] = useVbenDrawer({
  onCancel() {
    drawerApi.close();
  },
  onOpenChange(isOpen: boolean) {
    if (!isOpen) return;
    data.value = drawerApi.getData() as { mode: 'create' | 'edit'; profile?: Profile };
    if (data.value?.mode === 'edit' && data.value.profile) {
      hydrate(data.value.profile);
    } else {
      resetForm();
    }
  },
});
</script>

<template>
  <Drawer :title="title" class="w-[640px]" :footer="false">
    <Form layout="vertical" :model="formState" @finish="handleSubmit">
      <Divider orientation="left">{{ $t('netboot.section.identity') }}</Divider>

      <div class="grid grid-cols-2 gap-x-4">
        <FormItem :label="$t('netboot.page.profile.name')" name="name" :rules="nameRule">
          <Input v-model:value="formState.name" placeholder="ubuntu-noble-base" />
        </FormItem>
        <FormItem :label="$t('netboot.page.profile.ubuntuRelease')" name="ubuntuRelease">
          <Select v-model:value="formState.ubuntuRelease">
            <SelectOption v-for="o in releaseOptions" :key="o.value" :value="o.value">
              {{ o.label }}
            </SelectOption>
          </Select>
        </FormItem>
      </div>

      <Divider orientation="left">{{ $t('netboot.section.osLocale') }}</Divider>

      <div class="grid grid-cols-2 gap-x-4">
        <FormItem :label="$t('netboot.page.profile.installUsername')" name="installUsername">
          <Input v-model:value="formState.installUsername" placeholder="ubuntu" />
        </FormItem>
        <FormItem :label="$t('netboot.page.profile.keyboardLayout')" name="keyboardLayout">
          <Input v-model:value="formState.keyboardLayout" placeholder="us" />
        </FormItem>
        <FormItem :label="$t('netboot.page.profile.locale')" name="locale">
          <Input v-model:value="formState.locale" placeholder="en_US.UTF-8" />
        </FormItem>
        <FormItem :label="$t('netboot.page.profile.timezone')" name="timezone">
          <Input v-model:value="formState.timezone" placeholder="UTC" />
        </FormItem>
      </div>

      <FormItem
        :label="$t('netboot.page.profile.password')"
        name="password"
        :help="$t('netboot.page.profile.passwordHelp')"
      >
        <InputPassword
          v-model:value="formState.password"
          autocomplete="new-password"
          :placeholder="
            hasExistingPassword
              ? $t('netboot.page.profile.passwordSet')
              : $t('netboot.page.profile.passwordUnset')
          "
        />
      </FormItem>
      <FormItem v-if="isEdit && hasExistingPassword" name="clearPassword">
        <Checkbox v-model:checked="formState.clearPassword">
          {{ $t('netboot.page.profile.clearPassword') }}
        </Checkbox>
      </FormItem>

      <Divider orientation="left">{{ $t('netboot.section.access') }}</Divider>

      <FormItem
        :label="$t('netboot.page.profile.sshKeys')"
        name="sshAuthorizedKeys"
        :help="$t('netboot.hint.onePerLine')"
      >
        <Textarea
          v-model:value="formState.sshAuthorizedKeys"
          :rows="3"
          class="font-mono text-xs"
          placeholder="ssh-ed25519 AAAA... operator@example"
        />
      </FormItem>

      <FormItem
        :label="$t('netboot.page.profile.packages')"
        name="packages"
        :help="$t('netboot.hint.onePerLine')"
      >
        <Textarea
          v-model:value="formState.packages"
          :rows="3"
          class="font-mono text-xs"
          placeholder="curl&#10;vim&#10;htop"
        />
      </FormItem>

      <FormItem :label="$t('netboot.page.profile.defaultDns')" name="defaultDns">
        <Input v-model:value="formState.defaultDns" class="font-mono" placeholder="10.0.0.53, 1.1.1.1" />
      </FormItem>

      <Divider orientation="left">{{ $t('netboot.section.advanced') }}</Divider>

      <FormItem :label="$t('netboot.page.profile.storageLayout')" name="storageLayout">
        <Textarea
          v-model:value="formState.storageLayout"
          :rows="2"
          class="font-mono text-xs"
          placeholder='{"mode":"lvm"}'
        />
      </FormItem>
      <FormItem :label="$t('netboot.page.profile.networkConfig')" name="networkConfig">
        <Textarea
          v-model:value="formState.networkConfig"
          :rows="2"
          class="font-mono text-xs"
          placeholder='{"version":2}'
        />
      </FormItem>
      <FormItem
        :label="$t('netboot.page.profile.lateCommands')"
        name="lateCommands"
        :help="$t('netboot.hint.onePerLine')"
      >
        <Textarea v-model:value="formState.lateCommands" :rows="2" class="font-mono text-xs" />
      </FormItem>
      <FormItem :label="$t('netboot.page.profile.kernelCmdline')" name="kernelCmdlineExtra">
        <Input v-model:value="formState.kernelCmdlineExtra" class="font-mono" placeholder="console=ttyS0" />
      </FormItem>
      <FormItem :label="$t('netboot.page.profile.userDataTemplate')" name="userDataTemplate">
        <Textarea
          v-model:value="formState.userDataTemplate"
          :rows="5"
          class="font-mono text-xs"
          placeholder="#cloud-config"
        />
      </FormItem>

      <FormItem>
        <Button type="primary" html-type="submit" :loading="loading" block>
          {{ $t('netboot.common.save') }}
        </Button>
      </FormItem>
    </Form>
  </Drawer>
</template>
