import './styles/tailwind.css';

import enUS from './locales/en-US.json';
import routes from './routes';
import type { TangraModule } from './sdk';
import { useNetbootArtifactStore } from './stores/netboot-artifact.state';
import { useNetbootDhcpStore } from './stores/netboot-dhcp.state';
import { useNetbootMachineStore } from './stores/netboot-machine.state';
import { useNetbootProfileStore } from './stores/netboot-profile.state';
import { useNetbootSessionStore } from './stores/netboot-session.state';

// This default export is the federated contract: the shell imports
// 'netboot/module' and hands the result to registerModule().
const netbootModule: TangraModule = {
  id: 'netboot',
  version: '1.0.0',
  routes,
  stores: {
    'netboot-machine': useNetbootMachineStore,
    'netboot-profile': useNetbootProfileStore,
    'netboot-dhcp': useNetbootDhcpStore,
    'netboot-session': useNetbootSessionStore,
    'netboot-artifact': useNetbootArtifactStore,
  },
  locales: {
    'en-US': enUS,
  },
};

export default netbootModule;
