import { defineStore } from 'pinia';

import { MachineService, type ListMachinesResponse, type MachineResponse } from '../api/services';
import type { Machine, Paging } from '../types';

export const useNetbootMachineStore = defineStore('netboot-machine', () => {
  async function listMachines(
    paging?: Paging,
    formValues?: { state?: string; profileId?: string; q?: string } | null,
  ): Promise<ListMachinesResponse> {
    return await MachineService.list(paging, formValues);
  }

  async function getMachine(id: string): Promise<MachineResponse> {
    return await MachineService.get(id);
  }

  async function createMachine(data: Partial<Machine>): Promise<MachineResponse> {
    return await MachineService.create(data);
  }

  async function updateMachine(id: string, data: Partial<Machine>): Promise<MachineResponse> {
    return await MachineService.update(id, data);
  }

  async function deleteMachine(id: string): Promise<void> {
    return await MachineService.delete(id);
  }

  /** Arms the machine: its next boot reinstalls from the assigned profile. */
  async function provisionMachine(id: string): Promise<MachineResponse> {
    return await MachineService.provision(id);
  }

  async function cancelProvision(id: string): Promise<MachineResponse> {
    return await MachineService.cancelProvision(id);
  }

  async function listUnknownBoots(paging?: Paging) {
    return await MachineService.listUnknownBoots(paging);
  }

  async function registerUnknown(mac: string, name: string, profileId?: string) {
    return await MachineService.registerUnknown({ mac, name, profileId });
  }

  function $reset() {}

  return {
    $reset,
    listMachines,
    getMachine,
    createMachine,
    updateMachine,
    deleteMachine,
    provisionMachine,
    cancelProvision,
    listUnknownBoots,
    registerUnknown,
  };
});
