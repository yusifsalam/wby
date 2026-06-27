import { describe, expect, it } from "vitest";
import { cities, findCityBySlug } from "./cities";

describe("cities", () => {
  it("contains the fixed geographic-spread city set", () => {
    expect(cities.map((city) => city.slug)).toEqual([
      "helsinki",
      "espoo",
      "turku",
      "tampere",
      "lahti",
      "jyvaskyla",
      "kuopio",
      "vaasa",
      "oulu",
      "rovaniemi",
    ]);
  });

  it("uses stable ASCII slugs for city pages", () => {
    expect(findCityBySlug("jyvaskyla")).toMatchObject({
      name: "Jyväskylä",
      latitude: 62.2426,
      longitude: 25.7473,
    });
  });
});
