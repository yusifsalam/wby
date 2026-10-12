import { defineCollection } from "astro:content";
import { glob } from "astro/loaders";
import { z } from "astro/zod";

const posts = defineCollection({
  loader: glob({ pattern: "*/*/*.mdx", base: "./src/content/posts" }),
  schema: ({ image }) =>
    z.object({
      title: z.string(),
      kicker: z.string(),
      description: z.string(),
      published: z.iso.date(),
      image: image().optional(),
      draft: z.boolean().default(false),
    }),
});

export const collections = { posts };
