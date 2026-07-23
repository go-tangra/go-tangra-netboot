import { defineStore } from 'pinia';

import { DhcpService, type DhcpConfigResponse } from '../api/services';
import type { DhcpSubnet, Paging } from '../types';

export const useNetbootDhcpStore = defineStore('netboot-dhcp', () => {
  async function getConfig(): Promise<DhcpConfigResponse> {
    return await DhcpService.getConfig();
  }

  async function updateConfig(
    leaseTtlSeconds: number,
    subnets: DhcpSubnet[],
  ): Promise<DhcpConfigResponse> {
    return await DhcpService.updateConfig({ leaseTtlSeconds, subnets });
  }

  /** Starts the authoritative DHCP server on the provisioning segment. */
  async function enable(): Promise<DhcpConfigResponse> {
    return await DhcpService.enable();
  }

  async function disable(): Promise<DhcpConfigResponse> {
    return await DhcpService.disable();
  }

  async function listLeases(paging?: Paging) {
    return await DhcpService.listLeases(paging);
  }

  async function listConflicts(paging?: Paging) {
    return await DhcpService.listConflicts(paging);
  }

  function $reset() {}

  return { $reset, getConfig, updateConfig, enable, disable, listLeases, listConflicts };
});
