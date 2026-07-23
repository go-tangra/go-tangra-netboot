<script lang="ts" setup>
import { reactive, ref } from 'vue';

import { $t } from 'shell/locales';

import { useNetbootProfileStore } from '../../stores/netboot-profile.state';
import type { Profile, ProfileInput } from '../../types';

const props = defineProps<{ profile: Profile | null }>();
const emit = defineEmits<{ close: []; saved: [] }>();

const store = useNetbootProfileStore();
const saving = ref(false);
const errorMessage = ref('');

const isEdit = !!props.profile?.id;

const form = reactive({
  name: props.profile?.name ?? '',
  ubuntuRelease: props.profile?.ubuntuRelease ?? 'noble',
  storageLayout: props.profile?.storageLayout ?? '',
  networkConfig: props.profile?.networkConfig ?? '',
  packages: (props.profile?.packages ?? []).join('\n'),
  sshAuthorizedKeys: (props.profile?.sshAuthorizedKeys ?? []).join('\n'),
  userDataTemplate: props.profile?.userDataTemplate ?? '',
  lateCommands: (props.profile?.lateCommands ?? []).join('\n'),
  kernelCmdlineExtra: props.profile?.kernelCmdlineExtra ?? '',
  keyboardLayout: props.profile?.keyboardLayout ?? '',
  locale: props.profile?.locale ?? '',
  timezone: props.profile?.timezone ?? '',
  installUsername: props.profile?.installUsername ?? '',
  // Write-only. The existing password is never loaded into the form because
  // the API does not return it; leaving this blank keeps it unchanged.
  password: '',
  clearPassword: false,
  defaultDns: (props.profile?.defaultDns ?? []).join(', '),
});

function splitLines(value: string): string[] {
  return value
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean);
}

function splitCommas(value: string): string[] {
  return value
    .split(',')
    .map((entry) => entry.trim())
    .filter(Boolean);
}

async function save() {
  saving.value = true;
  errorMessage.value = '';
  try {
    const payload: ProfileInput = {
      name: form.name,
      ubuntuRelease: form.ubuntuRelease,
      storageLayout: form.storageLayout,
      networkConfig: form.networkConfig,
      packages: splitLines(form.packages),
      sshAuthorizedKeys: splitLines(form.sshAuthorizedKeys),
      userDataTemplate: form.userDataTemplate,
      lateCommands: splitLines(form.lateCommands),
      kernelCmdlineExtra: form.kernelCmdlineExtra,
      keyboardLayout: form.keyboardLayout,
      locale: form.locale,
      timezone: form.timezone,
      installUsername: form.installUsername,
      clearPassword: form.clearPassword,
      defaultDns: splitCommas(form.defaultDns),
    };
    // Only send the password when one was typed: an empty value means
    // "leave it alone" upstream, and sending it explicitly is ambiguous.
    if (form.password) {
      payload.password = form.password;
    }

    if (isEdit && props.profile?.id) {
      await store.updateProfile(props.profile.id, payload);
    } else {
      await store.createProfile(payload);
    }
    // Drop the plaintext from memory as soon as it has been sent.
    form.password = '';
    emit('saved');
  } catch (error) {
    errorMessage.value = (error as Error).message;
  } finally {
    saving.value = false;
  }
}
</script>

<template>
  <div class="fixed inset-0 z-50 flex justify-end bg-black/30" @click.self="emit('close')">
    <div class="h-full w-[40rem] overflow-y-auto bg-white p-6 dark:bg-gray-900">
      <h2 class="mb-4 text-lg font-semibold">
        {{ isEdit ? $t('netboot.page.profile.edit') : $t('netboot.page.profile.create') }}
      </h2>

      <div v-if="errorMessage" class="mb-3 rounded border border-red-300 bg-red-50 p-2 text-red-800">
        {{ errorMessage }}
      </div>

      <div class="space-y-3">
        <label class="block">
          <span class="text-sm">{{ $t('netboot.page.profile.name') }}</span>
          <input v-model="form.name" class="mt-1 w-full rounded border px-3 py-1.5" />
        </label>

        <label class="block">
          <span class="text-sm">{{ $t('netboot.page.profile.ubuntuRelease') }}</span>
          <input v-model="form.ubuntuRelease" class="mt-1 w-full rounded border px-3 py-1.5" />
        </label>

        <div class="grid grid-cols-2 gap-3">
          <label class="block">
            <span class="text-sm">{{ $t('netboot.page.profile.installUsername') }}</span>
            <input v-model="form.installUsername" class="mt-1 w-full rounded border px-3 py-1.5" />
          </label>
          <label class="block">
            <span class="text-sm">{{ $t('netboot.page.profile.keyboardLayout') }}</span>
            <input v-model="form.keyboardLayout" class="mt-1 w-full rounded border px-3 py-1.5" />
          </label>
          <label class="block">
            <span class="text-sm">{{ $t('netboot.page.profile.locale') }}</span>
            <input v-model="form.locale" class="mt-1 w-full rounded border px-3 py-1.5" />
          </label>
          <label class="block">
            <span class="text-sm">{{ $t('netboot.page.profile.timezone') }}</span>
            <input v-model="form.timezone" class="mt-1 w-full rounded border px-3 py-1.5" />
          </label>
        </div>

        <fieldset class="rounded border p-3">
          <legend class="px-1 text-sm">{{ $t('netboot.page.profile.password') }}</legend>
          <p class="mb-2 text-xs text-gray-500">{{ $t('netboot.page.profile.passwordHelp') }}</p>
          <input
            v-model="form.password"
            type="password"
            autocomplete="new-password"
            class="w-full rounded border px-3 py-1.5"
          />
          <label v-if="isEdit && props.profile?.hasPassword" class="mt-2 flex items-center gap-2 text-sm">
            <input v-model="form.clearPassword" type="checkbox" />
            {{ $t('netboot.page.profile.clearPassword') }}
          </label>
        </fieldset>

        <label class="block">
          <span class="text-sm">{{ $t('netboot.page.profile.sshKeys') }}</span>
          <textarea
            v-model="form.sshAuthorizedKeys"
            class="mt-1 w-full rounded border px-3 py-1.5 font-mono text-xs"
            rows="3"
          />
        </label>

        <label class="block">
          <span class="text-sm">{{ $t('netboot.page.profile.packages') }}</span>
          <textarea v-model="form.packages" class="mt-1 w-full rounded border px-3 py-1.5 font-mono text-xs" rows="3" />
        </label>

        <label class="block">
          <span class="text-sm">{{ $t('netboot.page.profile.storageLayout') }}</span>
          <textarea
            v-model="form.storageLayout"
            class="mt-1 w-full rounded border px-3 py-1.5 font-mono text-xs"
            rows="3"
          />
        </label>

        <label class="block">
          <span class="text-sm">{{ $t('netboot.page.profile.networkConfig') }}</span>
          <textarea
            v-model="form.networkConfig"
            class="mt-1 w-full rounded border px-3 py-1.5 font-mono text-xs"
            rows="3"
          />
        </label>

        <label class="block">
          <span class="text-sm">{{ $t('netboot.page.profile.defaultDns') }}</span>
          <input v-model="form.defaultDns" class="mt-1 w-full rounded border px-3 py-1.5 font-mono" />
        </label>

        <label class="block">
          <span class="text-sm">{{ $t('netboot.page.profile.lateCommands') }}</span>
          <textarea
            v-model="form.lateCommands"
            class="mt-1 w-full rounded border px-3 py-1.5 font-mono text-xs"
            rows="3"
          />
        </label>

        <label class="block">
          <span class="text-sm">{{ $t('netboot.page.profile.kernelCmdline') }}</span>
          <input
            v-model="form.kernelCmdlineExtra"
            class="mt-1 w-full rounded border px-3 py-1.5 font-mono text-xs"
          />
        </label>

        <label class="block">
          <span class="text-sm">{{ $t('netboot.page.profile.userDataTemplate') }}</span>
          <textarea
            v-model="form.userDataTemplate"
            class="mt-1 w-full rounded border px-3 py-1.5 font-mono text-xs"
            rows="6"
          />
        </label>
      </div>

      <div class="mt-6 flex justify-end gap-2">
        <button class="rounded border px-4 py-1.5" @click="emit('close')">
          {{ $t('netboot.common.cancel') }}
        </button>
        <button class="rounded bg-primary px-4 py-1.5 text-white" :disabled="saving" @click="save">
          {{ $t('netboot.common.save') }}
        </button>
      </div>
    </div>
  </div>
</template>
