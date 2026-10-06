import { describe, expect, it } from "vitest";
import {
  bandPath,
  dailyChart,
  linePath,
  type ReportDay,
  rainChart,
  septembersChart,
  sunshineChart,
  valueAxis,
  xAt,
  yAt,
} from "./reportChart";

function day(n: number, overrides: Partial<ReportDay> = {}): ReportDay {
  return {
    date: `2026-09-${String(n).padStart(2, "0")}`,
    temp_avg: 12,
    temp_high: 15,
    temp_low: 9,
    precip_mm: 0,
    precip_cum: 0,
    feels_avg: 10,
    feels_high: 13,
    feels_low: 7,
    n91: { temp_avg: 11, precip_cum: n * 2, feels_avg: 9 },
    n96: {
      temp_avg: 11.5,
      temp_high: 14,
      temp_low: 8,
      precip_cum: n * 2,
      feels_avg: 9.5,
      feels_high: 12,
      feels_low: 6,
    },
    ...overrides,
  };
}

describe("geometry", () => {
  it("centres slots across the plot", () => {
    expect(xAt(0, 4)).toBe(12.5);
    expect(xAt(3, 4)).toBe(87.5);
  });

  it("maps values onto the plot height", () => {
    expect(yAt(5, 0, 10)).toBe(50);
    expect(yAt(0, 0, 10)).toBe(0);
  });

  it("breaks lines at missing values", () => {
    expect(linePath([0, 10, null, 5], 0, 10)).toBe("M12.5 100L37.5 0M87.5 50");
  });

  it("closes a band over the slots where both edges exist", () => {
    expect(bandPath([10, 10, null], [0, 0, 0], 0, 10)).toBe(
      "M16.67 0L50 0L50 100L16.67 100Z",
    );
    expect(bandPath([null], [null], 0, 10)).toBe("");
  });

  it("pads the axis and ticks on the step", () => {
    expect(valueAxis([5.3, 20.1], 5)).toEqual({
      yMin: 4.3,
      yMax: 21.1,
      ticks: [5, 10, 15, 20],
    });
    expect(valueAxis([3, 89.8], 20, { floor: 0 })).toEqual({
      yMin: 0,
      yMax: 93.8,
      ticks: [0, 20, 40, 60, 80],
    });
  });
});

describe("septembersChart", () => {
  const septembers = [
    { year: 1990, temp_avg: 11, running30: 11 },
    { year: 1991, temp_avg: 13, running30: 11.2 },
    { year: 1992, temp_avg: 14, running30: null },
  ];
  const spec = septembersChart(
    septembers,
    [{ period: "1961-1990", start: 1961, end: 1990, temp_avg: 11 }],
    1992,
  );

  it("highlights the report year and greys the rest", () => {
    expect(spec.dots.find((d) => d.key === "obs")?.values).toEqual([
      null,
      null,
      14,
    ]);
    expect(spec.dots.find((d) => d.key === "past")?.values).toEqual([
      11,
      13,
      null,
    ]);
  });

  it("labels the running mean where it ends", () => {
    expect(spec.notes[1]).toMatchObject({
      index: 1,
      value: 11.2,
      side: "below",
    });
    expect(spec.notes[1].text.metric).toBe("11.20°");
  });

  it("places standard normals relative to the first year", () => {
    expect(spec.segments[0]).toMatchObject({
      from: -29,
      to: 0,
      value: 11,
      text: "1961–1990",
    });
  });

  it("names the 30 years in each tooltip", () => {
    expect(spec.tooltips[1].rows[1].label).toBe("1962–1991 mean");
    expect(spec.tooltips[2].rows).toHaveLength(1);
  });
});

describe("dailyChart", () => {
  const days = [
    day(1, { temp_high: 20.1, feels_low: 2.8 }),
    day(2, { temp_low: 5.3, feels_high: 18 }),
    day(3),
  ];

  it("labels the month's temperature extremes", () => {
    const spec = dailyChart(days);
    expect(spec.notes.map((n) => [n.index, n.text.metric])).toEqual([
      [0, "20.1°"],
      [1, "5.3°"],
    ]);
  });

  it("converts tooltip values for imperial", () => {
    const spec = dailyChart(days);
    expect(spec.tooltips[0].rows[0].value).toEqual({
      metric: "12.0°",
      imperial: "53.6°",
    });
    expect(spec.tooltips[0].title).toBe("Tue 1 September");
  });

  it("plots feels-like against the feels-like normals", () => {
    const spec = dailyChart(days, "feels");
    expect(spec.notes.map((n) => [n.index, n.text.metric])).toEqual([
      [1, "18.0°"],
      [0, "2.8°"],
    ]);
    expect(spec.dots[0].values).toEqual([10, 10, 10]);
    expect(spec.lines.map((l) => [l.key, l.values[0]])).toEqual([
      ["n91", 9],
      ["n96", 9.5],
    ]);
    expect(spec.bands[0]).toMatchObject({
      upper: [12, 12, 12],
      lower: [6, 6, 6],
    });
    expect(spec.label).toContain("feels-like temperature");
  });
});

describe("rainChart", () => {
  const days = [
    day(1, { precip_mm: 4, precip_cum: 4 }),
    day(2, { precip_cum: 4 }),
    day(3, { precip_mm: 16.1, precip_cum: 20.1 }),
  ];
  const spec = rainChart(days);

  it("draws columns only on wet days", () => {
    expect(spec.columns[0].values).toEqual([4, null, 16.1]);
  });

  it("starts the axis at zero", () => {
    expect(spec.yMin).toBe(0);
    expect(spec.ticks[0]).toBe(0);
  });

  it("labels the month total and the wettest day", () => {
    expect(spec.notes.map((n) => [n.index, n.text.metric])).toEqual([
      [2, "20.1 mm"],
      [2, "16.1 mm"],
    ]);
  });
});

describe("sunshineChart", () => {
  const days = [
    { date: "2026-09-01", sun_h: 2.7, daylight_h: 14.3, normal_h: 5.8 },
    { date: "2026-09-02", sun_h: 0, daylight_h: 14.2, normal_h: 5.9 },
    { date: "2026-09-03", sun_h: 11.8, daylight_h: 14.1, normal_h: 5.9 },
  ];
  const spec = sunshineChart(days, "Helsinki-Vantaa", [1999, 2025]);

  it("leaves sunless days without a column", () => {
    expect(spec.columns[0].values).toEqual([2.7, null, 11.8]);
  });

  it("shades the daylight from the axis up", () => {
    expect(spec.bands[0]).toMatchObject({
      key: "past",
      upper: [14.3, 14.2, 14.1],
      lower: [0, 0, 0],
    });
    expect(spec.yMin).toBe(0);
  });

  it("labels the sunniest day in hours", () => {
    expect(spec.notes[0]).toMatchObject({
      index: 2,
      text: { metric: "11.8 h", imperial: "11.8 h" },
    });
    expect(spec.legend[1].label).toBe("Average, 1999–2025");
  });
});
