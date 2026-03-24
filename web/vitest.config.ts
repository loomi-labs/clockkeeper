import { defineConfig } from "vitest/config";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));

export default defineConfig({
  plugins: [svelte()],
  resolve: {
    alias: {
      "$app/navigation": resolve(
        __dirname,
        "src/lib/__mocks__/app-navigation.ts",
      ),
      "$app/stores": resolve(__dirname, "src/lib/__mocks__/app-stores.ts"),
      "~": resolve(__dirname, "src"),
    },
  },
  test: {
    environment: "jsdom",
    include: ["src/**/*.test.ts"],
  },
});
