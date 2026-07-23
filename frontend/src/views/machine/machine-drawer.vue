<script lang="ts" setup>
import { reactive, ref } from 'vue';

import { $t } from 'shell/locales';

import { useNetbootMachineStore } from '../../stores/netboot-machine.state';
import type { Machine, Profile } from '../../types';

const props = defineProps<{ machine: Machine | null; profiles: Profile[] }>();
const emit = defineEmits<{ close: []; saved: [] }>();

const store = useNetbootMachineStore();
const saving = ref(false);
const errorMessage = ref('');

const isEdit = !!props.machine?.id;

const form = reactive({
  mac: props.machine?.mac ?? '',
  name: props.machine?.name ?? '',
  firmware: props.machine?.firmware ?? 'FIRMWARE_UNSPECIFIED',
  profileId: props.machine?.profileId ?? '',
  reservationIp: props.machine?.reservationIp ?? '',
  notes: props.machine?.notes ?? '',
  networkConfig: props.machine?.networkConfig ?? '',
  installAddress: props.machine?.installNetwork?.address ?? '',
  installGateway: props.machine?.installNetwork?.gateway ?? '',
  installDns: (props.machine?.installNetwork?.dns ?? []).join(', '),
});

function splitList(value: string): string[] {
  return value
    .split(',')
    .map((entry) => entry.trim())
    .filter(Boolean);
}

async function save() {
  saving.value = true;
  errorMessage.value = '';
  try {
    // An empty address is the API's way of clearing the production network,
    // so the object is always sent rather than omitted when the field is blank.
    const installNetwork = {
      address: form.installAddress,
      gateway: form.installGateway,
      dns: splitList(form.installDns),
    };

    if (isEdit && props.machine?.id) {
      await store.updateMachine(props.machine.id, {
        name: form.name,
        profileId: form.profileId,
        reservationIp: form.reservationIp,
        notes: form.notes,
        networkConfig: form.networkConfig,
        installNetwork,
      });
    } else {
      await store.createMachine({
        mac: form.mac,
        name: form.name,
        firmware: form.firmware as Machine['firmware'],
        profileId: form.profileId,
        reservationIp: form.reservationIp,
        notes: form.notes,
        networkConfig: form.networkConfig,
        installNetwork,
      });
    }
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
    <div class="h-full w-[32rem] overflow-y-auto bg-white p-6 dark:bg-gray-900">
      <h2 class="mb-4 text-lg font-semibold">
        {{ isEdit ? $t('netboot.page.machine.edit') : $t('netboot.page.machine.create') }}
      </h2>

      <div v-if="errorMessage" class="mb-3 rounded border border-red-300 bg-red-50 p-2 text-red-800">
        {{ errorMessage }}
      </div>

      <div class="space-y-3">
        <label class="block">
          <span class="text-sm">{{ $t('netboot.page.machine.mac') }}</span>
          <!-- The MAC identifies the machine on the wire and is immutable. -->
          <input
            v-model="form.mac"
            class="mt-1 w-full rounded border px-3 py-1.5 font-mono"
            :disabled="isEdit"
            placeholder="aa:bb:cc:dd:ee:ff"
          />
        </label>

        <label class="block">
          <span class="text-sm">{{ $t('netboot.page.machine.name') }}</span>
          <input v-model="form.name" class="mt-1 w-full rounded border px-3 py-1.5" />
        </label>

        <label v-if="!isEdit" class="block">
          <span class="text-sm">{{ $t('netboot.page.machine.firmware') }}</span>
          <select v-model="form.firmware" class="mt-1 w-full rounded border px-3 py-1.5">
            <option value="FIRMWARE_UNSPECIFIED">
              {{ $t('netboot.enum.firmware.FIRMWARE_UNSPECIFIED') }}
            </option>
            <option value="FIRMWARE_BIOS">{{ $t('netboot.enum.firmware.FIRMWARE_BIOS') }}</option>
            <option value="FIRMWARE_UEFI_X64">
              {{ $t('netboot.enum.firmware.FIRMWARE_UEFI_X64') }}
            </option>
          </select>
        </label>

        <label class="block">
          <span class="text-sm">{{ $t('netboot.page.machine.profile') }}</span>
          <select v-model="form.profileId" class="mt-1 w-full rounded border px-3 py-1.5">
            <option value="">{{ $t('netboot.common.none') }}</option>
            <option v-for="profile in props.profiles" :key="profile.id" :value="profile.id">
              {{ profile.name }}
            </option>
          </select>
        </label>

        <label class="block">
          <span class="text-sm">{{ $t('netboot.page.machine.reservationIp') }}</span>
          <input v-model="form.reservationIp" class="mt-1 w-full rounded border px-3 py-1.5 font-mono" />
        </label>

        <fieldset class="rounded border p-3">
          <legend class="px-1 text-sm">{{ $t('netboot.page.machine.installNetwork') }}</legend>
          <label class="block">
            <span class="text-xs">{{ $t('netboot.page.machine.installAddress') }}</span>
            <input
              v-model="form.installAddress"
              class="mt-1 w-full rounded border px-3 py-1.5 font-mono"
              placeholder="10.0.0.10/24"
            />
          </label>
          <label class="mt-2 block">
            <span class="text-xs">{{ $t('netboot.page.machine.installGateway') }}</span>
            <input v-model="form.installGateway" class="mt-1 w-full rounded border px-3 py-1.5 font-mono" />
          </label>
          <label class="mt-2 block">
            <span class="text-xs">{{ $t('netboot.page.machine.installDns') }}</span>
            <input v-model="form.installDns" class="mt-1 w-full rounded border px-3 py-1.5 font-mono" />
          </label>
        </fieldset>

        <label class="block">
          <span class="text-sm">{{ $t('netboot.page.machine.networkConfig') }}</span>
          <textarea
            v-model="form.networkConfig"
            class="mt-1 w-full rounded border px-3 py-1.5 font-mono text-xs"
            rows="4"
          />
        </label>

        <label class="block">
          <span class="text-sm">{{ $t('netboot.page.machine.notes') }}</span>
          <textarea v-model="form.notes" class="mt-1 w-full rounded border px-3 py-1.5" rows="2" />
        </label>
      </div>

      <div class="mt-6 flex justify-end gap-2">
        <button class="rounded border px-4 py-1.5" @click="emit('close')">
          {{ $t('netboot.common.cancel') }}
        </button>
        <button
          class="rounded bg-primary px-4 py-1.5 text-white"
          :disabled="saving"
          @click="save"
        >
          {{ $t('netboot.common.save') }}
        </button>
      </div>
    </div>
  </div>
</template>
