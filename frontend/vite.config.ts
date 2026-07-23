import { federation } from '@module-federation/vite';
import vue from '@vitejs/plugin-vue';
import { defineConfig } from 'vite';

// The module is published as a federated remote. In development it is served
// straight from Vite; in production the built bundle is embedded into the Go
// binary and served from /modules/netboot/ by internal/server/http.go, which
// is why the base path differs between the two modes.
export default defineConfig(({ command }) => ({
  base: command === 'serve' ? '/' : '/modules/netboot/',
  plugins: [
    vue(),
    federation({
      name: 'netboot',
      filename: 'remoteEntry.js',
      remotes: {
        shell: {
          type: 'module',
          name: 'shell',
          entry:
            command === 'serve'
              ? 'http://localhost:5666/remoteEntry.js'
              : '/remoteEntry.js',
        },
      },
      exposes: {
        './module': './src/index.ts',
      },
      // Singletons: two copies of Vue or Pinia in one page would give the
      // module its own reactivity graph and detach it from the shell.
      shared: {
        vue: { singleton: true, requiredVersion: '^3.5.13' },
        'vue-router': { singleton: true, requiredVersion: '^4.5.0' },
        pinia: { singleton: true, requiredVersion: '^2.2.2' },
        'ant-design-vue': { singleton: true, requiredVersion: '^4.2.6' },
      },
      dts: false,
    }),
  ],
  server: {
    port: 3014,
    strictPort: true,
    origin: 'http://localhost:3014',
    cors: true,
  },
  build: {
    target: 'esnext',
    minify: true,
  },
}));
