<script lang="ts" setup>
import { onMounted, ref } from 'vue';

import { Page } from 'shell/vben/common-ui';
import { $t } from 'shell/locales';

import {
  Button,
  Empty,
  Modal,
  Select,
  SelectOption,
  Table,
  Tag,
  Timeline,
  TimelineItem,
  notification,
} from 'ant-design-vue';

import { useNetbootSessionStore } from '../../stores/netboot-session.state';
import type { ProvisioningEvent, ProvisioningSession, SessionState } from '../../types';

const store = useNetbootSessionStore();

const sessions = ref<ProvisioningSession[]>([]);
const total = ref(0);
const loading = ref(false);

const page = ref(1);
const pageSize = ref(20);
const stateFilter = ref('');

const timelineOpen = ref(false);
const timelineLoading = ref(false);
const timeline = ref<ProvisioningEvent[]>([]);
const evidence = ref('');
const activeSession = ref<ProvisioningSession | null>(null);

const STATE_COLOR: Record<string, string> = {
  SESSION_STATE_ACTIVE: 'blue',
  SESSION_STATE_COMPLETED: 'green',
  SESSION_STATE_FAILED: 'red',
  SESSION_STATE_STALE: 'default',
};

function stateLabel(s?: SessionState) {
  return $t(`netboot.enum.sessionState.${s ?? 'SESSION_STATE_UNSPECIFIED'}`);
}

// Timeline dots are colour-coded by outcome so a failed phase stands out.
function outcomeColor(outcome?: string): string {
  if (outcome === 'EVENT_OUTCOME_ERROR') return 'red';
  if (outcome === 'EVENT_OUTCOME_DENIED') return 'orange';
  return 'green';
}

function outcomeLabel(outcome?: string) {
  return $t(`netboot.enum.eventOutcome.${outcome ?? 'EVENT_OUTCOME_UNSPECIFIED'}`);
}

async function load() {
  loading.value = true;
  try {
    const result = await store.listSessions(
      { page: page.value, pageSize: pageSize.value },
      { state: stateFilter.value || undefined },
    );
    sessions.value = result.sessions ?? [];
    total.value = result.meta?.total ?? 0;
  } catch (e) {
    notification.error({ message: (e as Error).message || $t('netboot.common.loadFailed') });
  } finally {
    loading.value = false;
  }
}

async function openTimeline(session: ProvisioningSession) {
  if (!session.id) return;
  activeSession.value = session;
  timeline.value = [];
  evidence.value = '';
  timelineOpen.value = true;
  timelineLoading.value = true;
  try {
    const detail = await store.getSession(session.id);
    timeline.value = detail.timeline ?? [];
    evidence.value = detail.evidence ?? '';
  } catch (e) {
    notification.error({ message: (e as Error).message || $t('netboot.common.loadFailed') });
  } finally {
    timelineLoading.value = false;
  }
}

const columns = [
  { title: $t('netboot.page.session.machine'), key: 'machine' },
  { title: $t('netboot.page.session.profile'), key: 'profile' },
  { title: $t('netboot.page.session.state'), key: 'state' },
  { title: $t('netboot.page.session.startedAt'), key: 'startedAt' },
  { title: $t('netboot.page.session.endedAt'), key: 'endedAt' },
  { title: $t('netboot.page.session.failurePhase'), key: 'failurePhase' },
  { title: $t('netboot.common.actions'), key: 'actions', width: 140 },
];

onMounted(load);
</script>

<template>
  <Page :title="$t('netboot.page.session.title')">
    <div class="space-y-4">
      <div class="flex items-center gap-2">
        <Select
          v-model:value="stateFilter"
          class="w-44"
          :placeholder="$t('netboot.page.session.state')"
          allow-clear
          @change="load"
        >
          <SelectOption value="SESSION_STATE_ACTIVE">
            {{ stateLabel('SESSION_STATE_ACTIVE') }}
          </SelectOption>
          <SelectOption value="SESSION_STATE_COMPLETED">
            {{ stateLabel('SESSION_STATE_COMPLETED') }}
          </SelectOption>
          <SelectOption value="SESSION_STATE_FAILED">
            {{ stateLabel('SESSION_STATE_FAILED') }}
          </SelectOption>
          <SelectOption value="SESSION_STATE_STALE">
            {{ stateLabel('SESSION_STATE_STALE') }}
          </SelectOption>
        </Select>
        <Button :loading="loading" @click="load">{{ $t('netboot.common.refresh') }}</Button>
      </div>

      <Table
        :columns="columns"
        :data-source="sessions"
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
          <template v-if="column.key === 'machine'">
            <div class="font-medium">{{ record.machineName }}</div>
            <div class="font-mono text-xs text-gray-500">{{ record.machineMac }}</div>
          </template>
          <template v-else-if="column.key === 'profile'">
            {{ record.profileId }} <span class="text-gray-400">v{{ record.profileVersion }}</span>
          </template>
          <template v-else-if="column.key === 'state'">
            <Tag :color="STATE_COLOR[record.state] ?? 'default'">{{ stateLabel(record.state) }}</Tag>
          </template>
          <template v-else-if="column.key === 'startedAt'">
            <span class="text-xs">{{ record.startedAt }}</span>
          </template>
          <template v-else-if="column.key === 'endedAt'">
            <span class="text-xs">{{ record.endedAt || '—' }}</span>
          </template>
          <template v-else-if="column.key === 'failurePhase'">
            <span v-if="record.failurePhase" class="text-red-500">{{ record.failurePhase }}</span>
            <span v-else>—</span>
          </template>
          <template v-else-if="column.key === 'actions'">
            <Button type="link" size="small" @click="openTimeline(record)">
              {{ $t('netboot.page.session.viewTimeline') }}
            </Button>
          </template>
        </template>
      </Table>
    </div>

    <!-- Timeline modal: an ant Timeline colour-coded by outcome, plus the
         raw evidence bundle netbootd recorded. -->
    <Modal
      v-model:open="timelineOpen"
      :title="$t('netboot.page.session.timeline')"
      :footer="null"
      width="720px"
    >
      <div v-if="activeSession" class="mb-4 flex items-center gap-2">
        <span class="font-medium">{{ activeSession.machineName }}</span>
        <span class="font-mono text-xs text-gray-500">{{ activeSession.machineMac }}</span>
        <Tag :color="STATE_COLOR[activeSession.state] ?? 'default'">
          {{ stateLabel(activeSession.state) }}
        </Tag>
      </div>

      <div v-if="timelineLoading" class="py-8 text-center text-gray-500">…</div>

      <Timeline v-else-if="timeline.length > 0">
        <TimelineItem
          v-for="(event, index) in timeline"
          :key="index"
          :color="outcomeColor(event.outcome)"
        >
          <div class="flex items-center gap-2">
            <span class="font-medium">{{ event.phase }}</span>
            <Tag :color="outcomeColor(event.outcome)">
              {{ outcomeLabel(event.outcome) }}
            </Tag>
          </div>
          <div class="text-xs text-gray-500">{{ event.time }}</div>
          <pre
            v-if="event.detail"
            class="mt-1 overflow-x-auto rounded bg-gray-100 p-2 text-xs dark:bg-gray-800"
          >{{ event.detail }}</pre>
        </TimelineItem>
      </Timeline>

      <Empty v-else :description="$t('netboot.page.session.noEvents')" />

      <div v-if="evidence" class="mt-4">
        <div class="mb-1 text-sm font-medium">{{ $t('netboot.page.session.evidence') }}</div>
        <pre class="overflow-x-auto rounded bg-gray-100 p-3 text-xs dark:bg-gray-800">{{ evidence }}</pre>
      </div>
    </Modal>
  </Page>
</template>
