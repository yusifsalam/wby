// Collects a report's <SectionHead id title> elements at compile time into
// the `sections` frontmatter that render() returns as remarkPluginFrontmatter.
import type { MdastPluginDefinition } from "satteri";

export type ReportSection = { id: string; title: string };

export const reportSections: MdastPluginDefinition = {
  name: "report-sections",
  mdxJsxFlowElement(node, ctx) {
    if (node.name !== "SectionHead") return;
    const attr = (name: string) =>
      node.attributes.find(
        (a) => a.type === "mdxJsxAttribute" && a.name === name,
      )?.value;
    const id = attr("id");
    const title = attr("title");
    const frontmatter = ctx.data.astro?.frontmatter;
    if (!frontmatter || typeof id !== "string" || typeof title !== "string")
      return;
    frontmatter.sections = [...(frontmatter.sections ?? []), { id, title }];
  },
};
