import { defineCollection } from "astro:content";
import { glob } from "astro/loaders";
import { z } from "astro/zod";

const posts = defineCollection({
  loader: glob({ pattern: "*/*/*.mdx", base: "./src/content/posts" }),
  schema: z.object({
    title: z.string(),
    kicker: z.string(),
    description: z.string(),
    station: z.string(),
    published: z.iso.date(),
    draft: z.boolean().default(false),
  }),
});

export const collections = { posts };
