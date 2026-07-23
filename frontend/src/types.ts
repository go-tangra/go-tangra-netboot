export type Paging = { page?: number; pageSize?: number } | undefined;

// Enum unions mirroring netboot/service/v1. The REST transcoder renders proto
// enums as their names, so these are the exact strings the API returns.

export type Firmware =
  | 'FIRMWARE_UNSPECIFIED'
  | 'FIRMWARE_BIOS'
  | 'FIRMWARE_UEFI_X64'
  | 'FIRMWARE_UNKNOWN';

export type ProvisionState =
  | 'PROVISION_STATE_UNSPECIFIED'
  | 'PROVISION_STATE_NEW'
  | 'PROVISION_STATE_READY'
  | 'PROVISION_STATE_INSTALLING'
  | 'PROVISION_STATE_INSTALLED'
  | 'PROVISION_STATE_FAILED';

export type SessionState =
  | 'SESSION_STATE_UNSPECIFIED'
  | 'SESSION_STATE_ACTIVE'
  | 'SESSION_STATE_COMPLETED'
  | 'SESSION_STATE_FAILED'
  | 'SESSION_STATE_STALE';

export type EventOutcome =
  | 'EVENT_OUTCOME_UNSPECIFIED'
  | 'EVENT_OUTCOME_OK'
  | 'EVENT_OUTCOME_ERROR'
  | 'EVENT_OUTCOME_DENIED';

export type ArtifactKind =
  | 'ARTIFACT_KIND_UNSPECIFIED'
  | 'ARTIFACT_KIND_KERNEL'
  | 'ARTIFACT_KIND_INITRD'
  | 'ARTIFACT_KIND_IPXE_BIN'
  | 'ARTIFACT_KIND_OTHER';

export type TransferProtocol =
  | 'TRANSFER_PROTOCOL_UNSPECIFIED'
  | 'TRANSFER_PROTOCOL_TFTP'
  | 'TRANSFER_PROTOCOL_HTTP';

export interface PageMeta {
  total?: number;
  page?: number;
  pageSize?: number;
}

export interface InstallNetwork {
  address?: string;
  gateway?: string;
  dns?: string[];
}

export interface Machine {
  id?: string;
  mac?: string;
  name?: string;
  firmware?: Firmware;
  profileId?: string;
  reservationIp?: string;
  provisionState?: ProvisionState;
  notes?: string;
  createTime?: string;
  updateTime?: string;
  activeSessionId?: string;
  networkConfig?: string;
  installNetwork?: InstallNetwork;
}

export interface UnknownBoot {
  mac?: string;
  lastSeen?: string;
  attempts?: number;
}

export interface Profile {
  id?: string;
  name?: string;
  version?: number;
  ubuntuRelease?: string;
  storageLayout?: string;
  networkConfig?: string;
  packages?: string[];
  sshAuthorizedKeys?: string[];
  userDataTemplate?: string;
  lateCommands?: string[];
  kernelCmdlineExtra?: string;
  createTime?: string;
  updateTime?: string;
  assignedMachines?: number;
  keyboardLayout?: string;
  keyboardVariant?: string;
  locale?: string;
  timezone?: string;
  installUsername?: string;
  /** The stored password is never returned; only whether one is set. */
  hasPassword?: boolean;
  defaultDns?: string[];
}

export interface ProfileInput {
  name: string;
  ubuntuRelease: string;
  storageLayout?: string;
  networkConfig?: string;
  packages?: string[];
  sshAuthorizedKeys?: string[];
  userDataTemplate?: string;
  lateCommands?: string[];
  kernelCmdlineExtra?: string;
  keyboardLayout?: string;
  keyboardVariant?: string;
  locale?: string;
  timezone?: string;
  installUsername?: string;
  /** Write-only: sent on save, never returned by the API. */
  password?: string;
  clearPassword?: boolean;
  defaultDns?: string[];
}

export interface DhcpSubnet {
  id?: string;
  network: string;
  rangeStart: string;
  rangeEnd: string;
  gateway?: string;
  dns?: string[];
}

export interface DhcpConfig {
  enabled?: boolean;
  version?: number;
  leaseTtlSeconds?: number;
  subnets?: DhcpSubnet[];
  updateTime?: string;
}

export interface Lease {
  ip?: string;
  mac?: string;
  machineId?: string;
  machineName?: string;
  expiresAt?: string;
}

export interface ForeignServer {
  serverId?: string;
  lastSeen?: string;
  offersSeen?: number;
}

export interface ProvisioningSession {
  id?: string;
  machineId?: string;
  machineName?: string;
  machineMac?: string;
  profileId?: string;
  profileVersion?: number;
  state?: SessionState;
  startedAt?: string;
  endedAt?: string;
  failurePhase?: string;
}

export interface ProvisioningEvent {
  time?: string;
  sessionId?: string;
  machineMac?: string;
  phase?: string;
  outcome?: EventOutcome;
  detail?: string;
}

export interface BootArtifact {
  id?: string;
  kind?: ArtifactKind;
  ubuntuRelease?: string;
  filename?: string;
  sizeBytes?: number;
  sha256?: string;
  uploadedBy?: string;
  createTime?: string;
  updateTime?: string;
}

export interface Transfer {
  time?: string;
  clientIp?: string;
  filename?: string;
  bytesSent?: number;
  success?: boolean;
  error?: string;
  protocol?: TransferProtocol;
}

export interface NetbootStats {
  totalMachines?: number;
  installingMachines?: number;
  installedMachines?: number;
  failedMachines?: number;
  totalProfiles?: number;
  activeSessions?: number;
  unknownBoots?: number;
  activeLeases?: number;
  dhcpEnabled?: boolean;
}

export interface UpstreamCheck {
  connected?: boolean;
  endpoint?: string;
  tls?: boolean;
  authenticated?: boolean;
  latencyMs?: number;
  message?: string;
}
