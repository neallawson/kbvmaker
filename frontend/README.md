# kbvmaker client

Svelte client for Montage 1. The browser and the Wails window both use it. The API is `kbvmaker serve`.

```
pnpm install
pnpm test
pnpm dev
pnpm build
```

`pnpm dev` is the UI only. Point it at a running `kbvmaker serve` when the page should load a real document. `pnpm build` writes `build/`, which `kbvmaker serve -static build` can host. kbvmaker-gui does not read that directory. `./scripts/build` builds this client and the GUI together.
