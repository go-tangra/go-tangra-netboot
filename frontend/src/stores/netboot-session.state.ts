import { defineStore } from 'pinia';

import { SessionService, type ListSessionsResponse, type SessionResponse } from '../api/services';
import type { Paging } from '../types';

export const useNetbootSessionStore = defineStore('netboot-session', () => {
  async function listSessions(
    paging?: Paging,
    formValues?: { machineId?: string; state?: string } | null,
  ): Promise<ListSessionsResponse> {
    return await SessionService.list(paging, formValues);
  }

  async function getSession(id: string): Promise<SessionResponse> {
    return await SessionService.get(id);
  }

  function $reset() {}

  return { $reset, listSessions, getSession };
});
