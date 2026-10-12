// @ts-check

import { satteri } from "@astrojs/markdown-satteri";
import mdx from "@astrojs/mdx";
import node from "@astrojs/node";

import tailwindcss from "@tailwindcss/vite";
import { defineConfig } from "astro/config";
import { reportSections } from "./src/lib/reportSections.ts";

try {
  process.loadEnvFile(".env");
} catch {
  // No .env file — the real environment is the only source.
}

// https://astro.build/config
export default defineConfig({
  site: process.env.SITE_URL,
  output: "server",

  adapter: node({
    mode: "standalone",
  }),

  markdown: {
    processor: satteri({ mdastPlugins: [reportSections] }),
  },

  vite: {
    plugins: [tailwindcss()],
  },

  integrations: [mdx()],
});
