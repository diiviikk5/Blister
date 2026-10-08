import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";

export default defineConfig({
  plugins: [svelte()],
  server: { port: 5199, strictPort: true },
  build: { target: "es2022", chunkSizeWarningLimit: 900 },
});
