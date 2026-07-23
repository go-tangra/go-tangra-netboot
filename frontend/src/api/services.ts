/**
 * Netboot Module Service Functions
 *
 * Typed wrappers over the module's REST surface, which the admin gateway
 * transcodes onto netboot.service.v1. Base URL: /admin/v1/modules/netboot/v1
 */

import { buildQuery, netbootApi, type RequestOptions } from './client';
import type {
  BootArtifact,
  DhcpConfig,
  DhcpSubnet,
  ForeignServer,
  Lease,
  Machine,
  NetbootStats,
  PageMeta,
  Paging,
  Profile,
  ProfileInput,
  ProvisioningEvent,
  ProvisioningSession,
  Transfer,
  UnknownBoot,
  UpstreamCheck,
} from '../types';

interface Paged {
  meta?: PageMeta;
}

export interface ListMachinesResponse extends Paged {
  machines?: Machine[];
}
export interface ListUnknownBootsResponse extends Paged {
  boots?: UnknownBoot[];
}
export interface ListProfilesResponse extends Paged {
  profiles?: Profile[];
}
export interface ListLeasesResponse extends Paged {
  leases?: Lease[];
}
export interface ListForeignServersResponse extends Paged {
  servers?: ForeignServer[];
}
export interface ListSessionsResponse extends Paged {
  sessions?: ProvisioningSession[];
}
export interface ListArtifactsResponse extends Paged {
  artifacts?: BootArtifact[];
}
export interface ListTransfersResponse extends Paged {
  transfers?: Transfer[];
}

export interface MachineResponse {
  machine?: Machine;
}
export interface ProfileResponse {
  profile?: Profile;
}
export interface DhcpConfigResponse {
  config?: DhcpConfig;
}
export interface SessionResponse {
  session?: ProvisioningSession;
  timeline?: ProvisioningEvent[];
  evidence?: string;
}
export interface ArtifactResponse {
  artifact?: BootArtifact;
}
export interface PreviewProfileResponse {
  userData?: string;
  cmdline?: string;
}

export interface MachineFilters {
  state?: string;
  profileId?: string;
  q?: string;
}

export const MachineService = {
  list: (paging?: Paging, filters?: MachineFilters | null, options?: RequestOptions) =>
    netbootApi.get<ListMachinesResponse>(
      `/machines${buildQuery({
        page: paging?.page,
        pageSize: paging?.pageSize,
        state: filters?.state,
        profileId: filters?.profileId,
        q: filters?.q,
      })}`,
      options,
    ),

  get: (id: string, options?: RequestOptions) =>
    netbootApi.get<MachineResponse>(`/machines/${encodeURIComponent(id)}`, options),

  create: (data: Partial<Machine>, options?: RequestOptions) =>
    netbootApi.post<MachineResponse>('/machines', data, options),

  update: (id: string, data: Partial<Machine>, options?: RequestOptions) =>
    netbootApi.patch<MachineResponse>(
      `/machines/${encodeURIComponent(id)}`,
      { ...data, id },
      options,
    ),

  delete: (id: string, options?: RequestOptions) =>
    netbootApi.delete<void>(`/machines/${encodeURIComponent(id)}`, options),

  provision: (id: string, options?: RequestOptions) =>
    netbootApi.post<MachineResponse>(
      `/machines/${encodeURIComponent(id)}/provision`,
      { id },
      options,
    ),

  cancelProvision: (id: string, options?: RequestOptions) =>
    netbootApi.post<MachineResponse>(
      `/machines/${encodeURIComponent(id)}/cancel`,
      { id },
      options,
    ),

  listUnknownBoots: (paging?: Paging, options?: RequestOptions) =>
    netbootApi.get<ListUnknownBootsResponse>(
      `/machines/unknown${buildQuery({ page: paging?.page, pageSize: paging?.pageSize })}`,
      options,
    ),

  registerUnknown: (
    data: { mac: string; name: string; profileId?: string },
    options?: RequestOptions,
  ) => netbootApi.post<MachineResponse>('/machines/register-unknown', data, options),
};

export const ProfileService = {
  list: (paging?: Paging, options?: RequestOptions) =>
    netbootApi.get<ListProfilesResponse>(
      `/profiles${buildQuery({ page: paging?.page, pageSize: paging?.pageSize })}`,
      options,
    ),

  get: (id: string, options?: RequestOptions) =>
    netbootApi.get<ProfileResponse>(`/profiles/${encodeURIComponent(id)}`, options),

  create: (profile: ProfileInput, options?: RequestOptions) =>
    netbootApi.post<ProfileResponse>('/profiles', { profile }, options),

  update: (id: string, profile: ProfileInput, options?: RequestOptions) =>
    netbootApi.put<ProfileResponse>(
      `/profiles/${encodeURIComponent(id)}`,
      { id, profile },
      options,
    ),

  clone: (id: string, newName: string, options?: RequestOptions) =>
    netbootApi.post<ProfileResponse>(
      `/profiles/${encodeURIComponent(id)}/clone`,
      { id, newName },
      options,
    ),

  delete: (id: string, options?: RequestOptions) =>
    netbootApi.delete<void>(`/profiles/${encodeURIComponent(id)}`, options),

  preview: (id: string, machineId?: string, options?: RequestOptions) =>
    netbootApi.post<PreviewProfileResponse>(
      `/profiles/${encodeURIComponent(id)}/preview`,
      { id, machineId },
      options,
    ),
};

export const DhcpService = {
  getConfig: (options?: RequestOptions) =>
    netbootApi.get<DhcpConfigResponse>('/dhcp/config', options),

  updateConfig: (
    data: { leaseTtlSeconds: number; subnets: DhcpSubnet[] },
    options?: RequestOptions,
  ) => netbootApi.put<DhcpConfigResponse>('/dhcp/config', data, options),

  enable: (options?: RequestOptions) =>
    netbootApi.post<DhcpConfigResponse>('/dhcp/enable', {}, options),

  disable: (options?: RequestOptions) =>
    netbootApi.post<DhcpConfigResponse>('/dhcp/disable', {}, options),

  listLeases: (paging?: Paging, options?: RequestOptions) =>
    netbootApi.get<ListLeasesResponse>(
      `/dhcp/leases${buildQuery({ page: paging?.page, pageSize: paging?.pageSize })}`,
      options,
    ),

  listConflicts: (paging?: Paging, options?: RequestOptions) =>
    netbootApi.get<ListForeignServersResponse>(
      `/dhcp/conflicts${buildQuery({ page: paging?.page, pageSize: paging?.pageSize })}`,
      options,
    ),
};

export const SessionService = {
  list: (
    paging?: Paging,
    filters?: { machineId?: string; state?: string } | null,
    options?: RequestOptions,
  ) =>
    netbootApi.get<ListSessionsResponse>(
      `/sessions${buildQuery({
        page: paging?.page,
        pageSize: paging?.pageSize,
        machineId: filters?.machineId,
        state: filters?.state,
      })}`,
      options,
    ),

  get: (id: string, options?: RequestOptions) =>
    netbootApi.get<SessionResponse>(`/sessions/${encodeURIComponent(id)}`, options),
};

export const ArtifactService = {
  list: (paging?: Paging, options?: RequestOptions) =>
    netbootApi.get<ListArtifactsResponse>(
      `/artifacts${buildQuery({ page: paging?.page, pageSize: paging?.pageSize })}`,
      options,
    ),

  get: (id: string, options?: RequestOptions) =>
    netbootApi.get<ArtifactResponse>(`/artifacts/${encodeURIComponent(id)}`, options),

  delete: (id: string, options?: RequestOptions) =>
    netbootApi.delete<void>(`/artifacts/${encodeURIComponent(id)}`, options),

  listTransfers: (paging?: Paging, filename?: string, options?: RequestOptions) =>
    netbootApi.get<ListTransfersResponse>(
      `/artifacts/transfers${buildQuery({
        page: paging?.page,
        pageSize: paging?.pageSize,
        filename,
      })}`,
      options,
    ),
};

export const SystemService = {
  stats: (options?: RequestOptions) =>
    netbootApi.get<NetbootStats>('/stats', options),

  checkUpstream: (options?: RequestOptions) =>
    netbootApi.get<UpstreamCheck>('/upstream/check', options),

  info: (options?: RequestOptions) =>
    netbootApi.get<Record<string, string>>('/info', options),
};
