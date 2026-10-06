// @ts-check

import mdx from "@astrojs/mdx";
import node from "@astrojs/node";

import tailwindcss from "@tailwindcss/vite";
import { defineConfig } from "astro/config";

try {
  process.loadEnvFile(".env");
} catch {
  // No .env file — the real environment is the only source.
}

// https://astro.build/config
export default defineConfig({
  output: "server",

  adapter: node({
    mode: "standalone",
  }),

  vite: {
    plugins: [tailwindcss()],
  },

  integrations: [mdx()],
});
