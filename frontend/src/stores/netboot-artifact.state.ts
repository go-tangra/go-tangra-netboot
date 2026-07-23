import { defineStore } from 'pinia';

import { ArtifactService, type ListArtifactsResponse } from '../api/services';
import type { Paging } from '../types';

export const useNetbootArtifactStore = defineStore('netboot-artifact', () => {
  async function listArtifacts(paging?: Paging): Promise<ListArtifactsResponse> {
    return await ArtifactService.list(paging);
  }

  async function getArtifact(id: string) {
    return await ArtifactService.get(id);
  }

  async function deleteArtifact(id: string): Promise<void> {
    return await ArtifactService.delete(id);
  }

  async function listTransfers(paging?: Paging, filename?: string) {
    return await ArtifactService.listTransfers(paging, filename);
  }

  function $reset() {}

  return { $reset, listArtifacts, getArtifact, deleteArtifact, listTransfers };
});
