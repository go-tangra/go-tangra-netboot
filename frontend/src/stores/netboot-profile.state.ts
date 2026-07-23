import { defineStore } from 'pinia';

import { ProfileService, type ListProfilesResponse, type ProfileResponse } from '../api/services';
import type { Paging, ProfileInput } from '../types';

export const useNetbootProfileStore = defineStore('netboot-profile', () => {
  async function listProfiles(paging?: Paging): Promise<ListProfilesResponse> {
    return await ProfileService.list(paging);
  }

  async function getProfile(id: string): Promise<ProfileResponse> {
    return await ProfileService.get(id);
  }

  async function createProfile(profile: ProfileInput): Promise<ProfileResponse> {
    return await ProfileService.create(profile);
  }

  async function updateProfile(id: string, profile: ProfileInput): Promise<ProfileResponse> {
    return await ProfileService.update(id, profile);
  }

  async function cloneProfile(id: string, newName: string): Promise<ProfileResponse> {
    return await ProfileService.clone(id, newName);
  }

  async function deleteProfile(id: string): Promise<void> {
    return await ProfileService.delete(id);
  }

  /** Renders the autoinstall seed. Credentials are redacted server-side. */
  async function previewProfile(id: string, machineId?: string) {
    return await ProfileService.preview(id, machineId);
  }

  function $reset() {}

  return {
    $reset,
    listProfiles,
    getProfile,
    createProfile,
    updateProfile,
    cloneProfile,
    deleteProfile,
    previewProfile,
  };
});
