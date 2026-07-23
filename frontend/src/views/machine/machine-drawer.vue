<script lang="ts" setup>
import { computed, ref } from 'vue';

import { useVbenDrawer } from 'shell/vben/common-ui';
import { $t } from 'shell/locales';

import {
  Alert,
  Button,
  Form,
  FormItem,
  Input,
  RadioButton,
  RadioGroup,
  Select,
  SelectOption,
  Textarea,
  notification,
} from 'ant-design-vue';

import { useNetbootMachineStore } from '../../stores/netboot-machine.state';
import { useNetbootProfileStore } from '../../stores/netboot-profile.state';
import type { Firmware, Machine, Profile } from '../../types';

// Local validation mirrors the backend's proto constraints so the operator
// gets immediate feedback instead of a round-trip 422.
const MAC_RE = /^([0-9a-fA-F]{2}[:-]){5}[0-9a-fA-F]{2}$/;
const HOSTNAME_RE = /^[a-zA-Z0-9][a-zA-Z0-9\-_.]*$/;
const IPV4_RE =
  /^(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}$/;
const CIDR_RE = new RegExp(`^${IPV4_RE.source.slice(1, -1)}/(3[0-2]|[12]?\\d)$`);

const machineStore = useNetbootMachineStore();
const profileStore = useNetbootProfileStore();

type DrawerMode = 'create' | 'edit' | 'register-unknown';
// How this machine gets its network: inherit the profile, the friendly
// two-NIC production form, or a raw netplan override.
type NetworkMode = 'inherit' | 'production' | 'advanced';

const emit = defineEmits<{ saved: [] }>();

const data = ref<{ mode: DrawerMode; machine?: Machine }>();
const loading = ref(false);
const profiles = ref<Profile[]>([]);

const formState = ref({
  mac: '',
  name: '',
  firmware: 'FIRMWARE_UEFI_X64' as Firmware,
  profileId: undefined as string | undefined,
  reservationIp: '',
  notes: '',
  networkMode: 'inherit' as NetworkMode,
  networkConfig: '',
  prodAddress: '',
  prodGateway: '',
  prodDns: '',
});

const mode = computed<DrawerMode>(() => data.value?.mode ?? 'create');
const isEdit = computed(() => mode.value === 'edit');
const isRegisterUnknown = computed(() => mode.value === 'register-unknown');
// The MAC identifies the machine on the wire; it can't change once known.
const macLocked = computed(() => isEdit.value || isRegisterUnknown.value);

const title = computed(() => {
  if (isEdit.value) return $t('netboot.page.machine.edit');
  if (isRegisterUnknown.value) return $t('netboot.page.machine.register');
  return $t('netboot.page.machine.create');
});

const firmwareOptions = [
  { value: 'FIRMWARE_UEFI_X64', label: $t('netboot.enum.firmware.FIRMWARE_UEFI_X64') },
  { value: 'FIRMWARE_BIOS', label: $t('netboot.enum.firmware.FIRMWARE_BIOS') },
  { value: 'FIRMWARE_UNSPECIFIED', label: $t('netboot.enum.firmware.FIRMWARE_UNSPECIFIED') },
];

const profileOptions = computed(() =>
  profiles.value.map((p) => ({
    value: p.id ?? '',
    label: `${p.name} (${p.ubuntuRelease})`,
  })),
);

function splitList(value: string): string[] {
  return value
    .split(/[,\s]+/)
    .map((entry) => entry.trim())
    .filter(Boolean);
}

// ---- validation rules (ant-design Form) -------------------------------------

const macRule = [
  { required: true, message: $t('netboot.rule.macRequired') },
  {
    validator: (_r: unknown, v: string) =>
      !v || MAC_RE.test(v.trim())
        ? Promise.resolve()
        : Promise.reject($t('netboot.rule.macInvalid')),
  },
];

const nameRule = [
  { required: true, message: $t('netboot.rule.nameRequired') },
  {
    validator: (_r: unknown, v: string) =>
      !v || HOSTNAME_RE.test(v.trim())
        ? Promise.resolve()
        : Promise.reject($t('netboot.rule.nameInvalid')),
  },
];

const ipRule = [
  {
    validator: (_r: unknown, v: string) =>
      !v || IPV4_RE.test(v.trim())
        ? Promise.resolve()
        : Promise.reject($t('netboot.rule.ipInvalid')),
  },
];

const cidrRule = computed(() =>
  formState.value.networkMode === 'production'
    ? [
        { required: true, message: $t('netboot.rule.cidrRequired') },
        {
          validator: (_r: unknown, v: string) =>
            !v || CIDR_RE.test(v.trim())
              ? Promise.resolve()
              : Promise.reject($t('netboot.rule.cidrInvalid')),
        },
      ]
    : [],
);

const advancedJsonRule = computed(() =>
  formState.value.networkMode === 'advanced' && formState.value.networkConfig.trim()
    ? [
        {
          validator: (_r: unknown, v: string) => {
            try {
              JSON.parse(v);
              return Promise.resolve();
            } catch {
              return Promise.reject($t('netboot.rule.jsonInvalid'));
            }
          },
        },
      ]
    : [],
);

// ---- lifecycle --------------------------------------------------------------

async function loadProfiles() {
  try {
    const resp = await profileStore.listProfiles({ page: 1, pageSize: 200 });
    profiles.value = resp.profiles ?? [];
  } catch {
    profiles.value = [];
  }
}

function resetForm() {
  formState.value = {
    mac: '',
    name: '',
    firmware: 'FIRMWARE_UEFI_X64',
    profileId: undefined,
    reservationIp: '',
    notes: '',
    networkMode: 'inherit',
    networkConfig: '',
    prodAddress: '',
    prodGateway: '',
    prodDns: '',
  };
}

// Reconstructs the edit form, choosing the network mode from the stored data:
// a configured production network wins over a raw override.
function hydrate(machine: Machine) {
  resetForm();
  formState.value.mac = machine.mac ?? '';
  formState.value.name = machine.name ?? '';
  formState.value.firmware = machine.firmware ?? 'FIRMWARE_UNSPECIFIED';
  formState.value.profileId = machine.profileId || undefined;
  formState.value.reservationIp = machine.reservationIp ?? '';
  formState.value.notes = machine.notes ?? '';

  const install = machine.installNetwork;
  if (install && install.address) {
    formState.value.networkMode = 'production';
    formState.value.prodAddress = install.address;
    formState.value.prodGateway = install.gateway ?? '';
    formState.value.prodDns = (install.dns ?? []).join(', ');
  } else if ((machine.networkConfig ?? '').trim()) {
    formState.value.networkMode = 'advanced';
    formState.value.networkConfig = machine.networkConfig ?? '';
  }
}

async function handleSubmit() {
  loading.value = true;
  try {
    // An empty address clears the production network; the field is always sent
    // in production mode so switching modes actually takes effect.
    const installNetwork =
      formState.value.networkMode === 'production'
        ? {
            address: formState.value.prodAddress.trim(),
            gateway: formState.value.prodGateway.trim(),
            dns: splitList(formState.value.prodDns),
          }
        : { address: '', gateway: '', dns: [] };

    const networkConfig =
      formState.value.networkMode === 'advanced'
        ? formState.value.networkConfig.trim()
        : '';

    if (isRegisterUnknown.value) {
      await machineStore.registerUnknown(
        formState.value.mac.trim().toLowerCase(),
        formState.value.name.trim(),
        formState.value.profileId,
      );
      notification.success({ message: $t('netboot.page.machine.createSuccess') });
    } else if (isEdit.value && data.value?.machine?.id) {
      await machineStore.updateMachine(data.value.machine.id, {
        name: formState.value.name.trim(),
        profileId: formState.value.profileId ?? '',
        reservationIp: formState.value.reservationIp.trim(),
        notes: formState.value.notes.trim(),
        networkConfig,
        installNetwork,
      });
      notification.success({ message: $t('netboot.page.machine.updateSuccess') });
    } else {
      await machineStore.createMachine({
        mac: formState.value.mac.trim().toLowerCase(),
        name: formState.value.name.trim(),
        firmware: formState.value.firmware,
        profileId: formState.value.profileId ?? '',
        reservationIp: formState.value.reservationIp.trim(),
        notes: formState.value.notes.trim(),
        networkConfig,
        installNetwork,
      });
      notification.success({ message: $t('netboot.page.machine.createSuccess') });
    }
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
  async onOpenChange(isOpen: boolean) {
    if (!isOpen) return;
    data.value = drawerApi.getData() as { mode: DrawerMode; machine?: Machine };
    await loadProfiles();
    if (data.value?.mode === 'edit' && data.value.machine) {
      hydrate(data.value.machine);
    } else {
      resetForm();
      if (data.value?.machine?.mac) formState.value.mac = data.value.machine.mac;
    }
  },
});
</script>

<template>
  <Drawer :title="title" class="w-[560px]" :footer="false">
    <Form layout="vertical" :model="formState" @finish="handleSubmit">
      <FormItem :label="$t('netboot.page.machine.mac')" name="mac" :rules="macRule">
        <Input
          v-model:value="formState.mac"
          :disabled="macLocked"
          placeholder="aa:bb:cc:dd:ee:ff"
          class="font-mono"
        />
      </FormItem>

      <FormItem :label="$t('netboot.page.machine.name')" name="name" :rules="nameRule">
        <Input v-model:value="formState.name" placeholder="node-01" :maxlength="255" />
      </FormItem>

      <FormItem
        v-if="!isEdit && !isRegisterUnknown"
        :label="$t('netboot.page.machine.firmware')"
        name="firmware"
      >
        <Select v-model:value="formState.firmware">
          <SelectOption v-for="o in firmwareOptions" :key="o.value" :value="o.value">
            {{ o.label }}
          </SelectOption>
        </Select>
      </FormItem>

      <FormItem
        :label="$t('netboot.page.machine.profile')"
        name="profileId"
        :rules="
          isRegisterUnknown
            ? [{ required: true, message: $t('netboot.rule.profileRequired') }]
            : []
        "
      >
        <Select
          v-model:value="formState.profileId"
          allow-clear
          show-search
          option-filter-prop="children"
          :placeholder="$t('netboot.common.none')"
        >
          <SelectOption v-for="o in profileOptions" :key="o.value" :value="o.value">
            {{ o.label }}
          </SelectOption>
        </Select>
      </FormItem>

      <!-- Registering an unknown boot only needs a name + profile; everything
           else (reservation, network) is edited afterwards. -->
      <template v-if="!isRegisterUnknown">
        <FormItem
          :label="$t('netboot.page.machine.reservationIp')"
          name="reservationIp"
          :rules="ipRule"
        >
          <Input
            v-model:value="formState.reservationIp"
            placeholder="10.0.0.50"
            class="font-mono"
          />
        </FormItem>

        <FormItem :label="$t('netboot.page.machine.installNetwork')">
          <RadioGroup v-model:value="formState.networkMode" button-style="solid" size="small">
            <RadioButton value="inherit">{{ $t('netboot.net.inherit') }}</RadioButton>
            <RadioButton value="production">{{ $t('netboot.net.production') }}</RadioButton>
            <RadioButton value="advanced">{{ $t('netboot.net.advanced') }}</RadioButton>
          </RadioGroup>
        </FormItem>

        <Alert
          v-if="formState.networkMode === 'production'"
          type="info"
          show-icon
          :message="$t('netboot.net.productionHelp')"
          class="mb-3"
        />

        <template v-if="formState.networkMode === 'production'">
          <FormItem
            :label="$t('netboot.page.machine.installAddress')"
            name="prodAddress"
            :rules="cidrRule"
          >
            <Input
              v-model:value="formState.prodAddress"
              placeholder="10.20.0.10/24"
              class="font-mono"
            />
          </FormItem>
          <FormItem
            :label="$t('netboot.page.machine.installGateway')"
            name="prodGateway"
            :rules="ipRule"
          >
            <Input
              v-model:value="formState.prodGateway"
              placeholder="10.20.0.1"
              class="font-mono"
            />
          </FormItem>
          <FormItem :label="$t('netboot.page.machine.installDns')" name="prodDns">
            <Input
              v-model:value="formState.prodDns"
              placeholder="1.1.1.1, 8.8.8.8"
              class="font-mono"
            />
          </FormItem>
        </template>

        <FormItem
          v-if="formState.networkMode === 'advanced'"
          :label="$t('netboot.page.machine.networkConfig')"
          name="networkConfig"
          :rules="advancedJsonRule"
        >
          <Textarea
            v-model:value="formState.networkConfig"
            :rows="5"
            class="font-mono text-xs"
            placeholder='{"version":2,"ethernets":{"en*":{"dhcp4":true}}}'
          />
        </FormItem>

        <FormItem :label="$t('netboot.page.machine.notes')" name="notes">
          <Textarea v-model:value="formState.notes" :rows="2" :maxlength="4096" />
        </FormItem>
      </template>

      <FormItem>
        <Button type="primary" html-type="submit" :loading="loading" block>
          {{ isRegisterUnknown ? $t('netboot.page.machine.register') : $t('netboot.common.save') }}
        </Button>
      </FormItem>
    </Form>
  </Drawer>
</template>
