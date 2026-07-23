declare module 'shell/vben/stores' {
  import type { StoreDefinition } from 'pinia';
  export const useAccessStore: StoreDefinition;
  export const useUserStore: StoreDefinition;
}

declare module 'shell/vben/common-ui' {
  import type { Component } from 'vue';
  export const Page: Component;
  export function useVbenDrawer(options: any): [Component, any];
  export function useVbenModal(options: any): [Component, any];
  export type VbenFormProps = any;
}

declare module 'shell/vben/icons' {
  import type { Component } from 'vue';
  export const LucideActivity: Component;
  export const LucideAlertTriangle: Component;
  export const LucideCheckCircle: Component;
  export const LucideCopy: Component;
  export const LucideEye: Component;
  export const LucideFileCog: Component;
  export const LucideHardDrive: Component;
  export const LucideHelpCircle: Component;
  export const LucideNetwork: Component;
  export const LucidePackage: Component;
  export const LucidePencil: Component;
  export const LucidePlay: Component;
  export const LucidePlus: Component;
  export const LucideRefreshCw: Component;
  export const LucideServerCog: Component;
  export const LucideSquare: Component;
  export const LucideTrash: Component;
  export const LucideXCircle: Component;
}

declare module 'shell/vben/layouts' {
  import type { Component } from 'vue';
  export const BasicLayout: Component;
}

declare module 'shell/app-layout' {
  import type { Component } from 'vue';
  const component: Component;
  export default component;
}

declare module 'shell/adapter/vxe-table' {
  export function useVbenVxeGrid(options: any): any;
  export type VxeGridProps = any;
}

declare module 'shell/locales' {
  export function $t(key: string, ...args: any[]): string;
}
