export type City = {
  slug: string;
  name: string;
  latitude: number;
  longitude: number;
};

export const cities = [
  {
    slug: "helsinki",
    name: "Helsinki",
    latitude: 60.1699,
    longitude: 24.9384,
  },
  { slug: "espoo", name: "Espoo", latitude: 60.2055, longitude: 24.6559 },
  { slug: "turku", name: "Turku", latitude: 60.4518, longitude: 22.2666 },
  { slug: "tampere", name: "Tampere", latitude: 61.4978, longitude: 23.761 },
  { slug: "lahti", name: "Lahti", latitude: 60.9827, longitude: 25.6615 },
  {
    slug: "jyvaskyla",
    name: "Jyväskylä",
    latitude: 62.2426,
    longitude: 25.7473,
  },
  { slug: "kuopio", name: "Kuopio", latitude: 62.8924, longitude: 27.677 },
  { slug: "vaasa", name: "Vaasa", latitude: 63.0951, longitude: 21.6165 },
  { slug: "oulu", name: "Oulu", latitude: 65.0121, longitude: 25.4651 },
  {
    slug: "rovaniemi",
    name: "Rovaniemi",
    latitude: 66.5039,
    longitude: 25.7294,
  },
] as const satisfies readonly City[];

export function findCityBySlug(slug: string): City | undefined {
  return cities.find((city) => city.slug === slug);
}
