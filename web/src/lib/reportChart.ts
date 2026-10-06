import { formatStat, type StatKind } from "./normals";

export type SeriesKey = "obs" | "n96" | "n91" | "past";

export type UnitPair = { metric: string; imperial: string };

export type LegendItem = {
  key: SeriesKey;
  label: string;
  mark: "line" | "dot" | "bar" | "band";
};

export type ChartNote = {
  index: number;
  value: number;
  key: SeriesKey;
  text: UnitPair;
  side: "above" | "below";
};

export type TooltipRow = { key: SeriesKey; label: string; value: UnitPair };

export type ChartTooltip = { title: string; rows: TooltipRow[] };

export type ChartTable = {
  caption: string;
  headers: string[];
  rows: (string | UnitPair)[][];
};

export type ChartSpec = {
  label: string;
  kind: StatKind;
  count: number;
  yMin: number;
  yMax: number;
  ticks: number[];
  xLabels: { index: number; text: string }[];
  legend: LegendItem[];
  bands: {
    key: SeriesKey;
    upper: (number | null)[];
    lower: (number | null)[];
  }[];
  lines: { key: SeriesKey; values: (number | null)[] }[];
  segments: {
    key: SeriesKey;
    from: number;
    to: number;
    value: number;
    text: string;
  }[];
  ranges: { key: SeriesKey; low: (number | null)[]; high: (number | null)[] }[];
  columns: { key: SeriesKey; values: (number | null)[]; strong?: boolean }[];
  dots: { key: SeriesKey; values: (number | null)[]; size: "sm" | "md" }[];
  notes: ChartNote[];
  tooltips: ChartTooltip[];
  table: ChartTable;
};

export type ReportDay = {
  date: string;
  temp_avg: number;
  temp_high: number;
  temp_low: number;
  precip_mm: number;
  precip_cum: number;
  feels_avg: number;
  feels_high: number;
  feels_low: number;
  sky?: string;
  n91: { temp_avg: number; precip_cum: number; feels_avg: number };
  n96: {
    temp_avg: number;
    temp_high: number;
    temp_low: number;
    precip_cum: number;
    feels_avg: number;
    feels_high: number;
    feels_low: number;
  };
};

export type ReportSeptember = {
  year: number;
  temp_avg: number;
  running30: number | null;
};

export type ReportStandardNormal = {
  period: string;
  start: number;
  end: number;
  temp_avg: number;
};

export type ReportNormal = {
  temp_avg: number;
  temp_high: number;
  temp_low: number;
  precip_mm: number;
  wet_days: number;
  wet_days_1mm: number;
  feels_avg: number;
};

export type ReportStation = {
  fmisid: number;
  name: string;
  temp_avg: number;
  abs_high: number;
  abs_low: number;
  precip_mm: number | null;
  gust_max: number;
  official_temp_avg: number | null;
  anomaly_official: number | null;
  official_precip_mm: number | null;
  anomaly_1996_2025: number | null;
  feels_avg: number | null;
};

export type SunDay = {
  date: string;
  sun_h: number;
  daylight_h: number;
  normal_h: number;
};

export type ReportSunshine = {
  normal_1991_2020: { sun_h: number; glob_mj: number };
  primary: {
    name: string;
    total_h: number;
    glob_mj: number;
    pct_possible: number;
    base_years: number[];
    base_mean_h: number;
    rank: number;
    of: number;
    hour_profile: number[];
    days: SunDay[];
  };
  city: {
    name: string;
    total_h: number;
    base_years: number[];
    base_mean_h: number;
    rank: number;
    of: number;
  };
};

export type ReportRank = { rank: number; of: number };

export type ReportDaytime = {
  hours: number[];
  window_hours: number;
  temp_avg: number;
  feels_avg: number;
  base_years: number[];
  base_temp_avg: number;
  base_feels_avg: number;
  temp_rank: ReportRank;
  feels_rank: ReportRank;
  rain: {
    base_years: number[];
    wet_hours: number;
    base_wet_hours: number;
    wet_hours_max: number[];
    wet_hours_rank: ReportRank;
  };
};

export type MonthReport = {
  observed: {
    temp_avg: number;
    feels_avg: number;
    precip_mm: number;
    wet_days_1mm: number;
  };
  normals: { "1991-2020": ReportNormal; "1996-2025": ReportNormal };
  rank_warmest: {
    rank: number;
    of: number;
    since: number;
    warmer_years: number[];
  };
  septembers: ReportSeptember[];
  standard_normals: ReportStandardNormal[];
  days: ReportDay[];
  stations: ReportStation[];
  sunshine: ReportSunshine;
  daytime: ReportDaytime;
};

export function ordinal(n: number): string {
  const suffix = n % 100 >= 11 && n % 100 <= 13 ? "th" : ["th", "st", "nd", "rd"][n % 10] ?? "th";
  return `${n}${suffix}`;
}

// Centre of slot `index` of `count`, as a percentage of the plot width.
export function xAt(index: number, count: number): number {
  return ((index + 0.5) / count) * 100;
}

// Height of `value` above the plot's base, as a percentage.
export function yAt(value: number, min: number, max: number): number {
  return ((value - min) / (max - min)) * 100;
}

function round(value: number): number {
  return Math.round(value * 100) / 100;
}

// An SVG path in the 0–100 viewBox (y down), broken at missing values.
export function linePath(
  values: (number | null)[],
  min: number,
  max: number,
): string {
  let path = "";
  let pen = false;
  values.forEach((value, index) => {
    if (value == null) {
      pen = false;
      return;
    }
    path += `${pen ? "L" : "M"}${round(xAt(index, values.length))} ${round(100 - yAt(value, min, max))}`;
    pen = true;
  });
  return path;
}

// A closed band between two curves over the slots where both are present.
export function bandPath(
  upper: (number | null)[],
  lower: (number | null)[],
  min: number,
  max: number,
): string {
  const points = upper.flatMap((high, index) => {
    const low = lower[index];
    return high == null || low == null ? [] : [{ index, high, low }];
  });
  if (points.length === 0) return "";
  const top = points.map(
    (p) =>
      `${round(xAt(p.index, upper.length))} ${round(100 - yAt(p.high, min, max))}`,
  );
  const bottom = points
    .map(
      (p) =>
        `${round(xAt(p.index, upper.length))} ${round(100 - yAt(p.low, min, max))}`,
    )
    .reverse();
  return `M${top.join("L")}L${bottom.join("L")}Z`;
}

// The value axis: padded past the data, ticks on multiples of `step`.
export function valueAxis(
  values: number[],
  step: number,
  options: { floor?: number } = {},
) {
  const pad = step * 0.2;
  const yMin = options.floor ?? Math.min(...values) - pad;
  const yMax = Math.max(...values) + pad;
  const ticks: number[] = [];
  for (let t = Math.ceil(yMin / step) * step; t <= yMax; t += step)
    ticks.push(round(t));
  return { yMin, yMax, ticks };
}

export function unitPair(
  kind: StatKind,
  value: number,
  decimals = 1,
): UnitPair {
  return {
    metric: formatStat(kind, value, "metric", decimals),
    imperial: formatStat(kind, value, "imperial", decimals),
  };
}

function rangePair(kind: StatKind, high: number, low: number): UnitPair {
  const a = unitPair(kind, high);
  const b = unitPair(kind, low);
  return {
    metric: `${a.metric} / ${b.metric}`,
    imperial: `${a.imperial} / ${b.imperial}`,
  };
}

const dayTitle = new Intl.DateTimeFormat("en-GB", {
  weekday: "short",
  day: "numeric",
  month: "long",
  timeZone: "UTC",
});

function dayLabel(date: string): string {
  return dayTitle.format(new Date(`${date}T00:00:00Z`));
}

function monthDayTicks(days: ReportDay[]): { index: number; text: string }[] {
  const month = new Intl.DateTimeFormat("en-GB", {
    month: "short",
    timeZone: "UTC",
  }).format(new Date(`${days[0].date}T00:00:00Z`));
  return [1, 5, 10, 15, 20, 25, 30]
    .filter((d) => d <= days.length)
    .map((d) => ({ index: d - 1, text: d === 1 ? `1 ${month}` : String(d) }));
}

function emptySpec(): Pick<
  ChartSpec,
  "bands" | "lines" | "segments" | "ranges" | "columns" | "dots" | "notes"
> {
  return {
    bands: [],
    lines: [],
    segments: [],
    ranges: [],
    columns: [],
    dots: [],
    notes: [],
  };
}

// Every September's mean temperature, the 30-year running mean ending each
// year, and the standard normals as flat segments over their periods.
export function septembersChart(
  septembers: ReportSeptember[],
  standardNormals: ReportStandardNormal[],
  year: number,
): ChartSpec {
  const count = septembers.length;
  const first = septembers[0].year;
  const values = septembers.map((s) => s.temp_avg);
  const { yMin, yMax, ticks } = valueAxis(values, 2);
  const current = septembers.findIndex((s) => s.year === year);
  const last = septembers.findLastIndex((s) => s.running30 != null);
  const runningEnd = septembers[last];
  return {
    ...emptySpec(),
    label: `September mean temperature at Kaisaniemi every year from ${first} to ${year}, with the 30-year running mean and the standard normals.`,
    kind: "temperature",
    count,
    yMin,
    yMax,
    ticks,
    xLabels: septembers
      .map((s, index) => ({ index, text: String(s.year) }))
      .filter((t) => Number(t.text) % 10 === 0 || t.index === 0),
    legend: [
      { key: "obs", label: String(year), mark: "dot" },
      { key: "past", label: "Earlier Septembers", mark: "dot" },
      { key: "n96", label: "30-year running mean", mark: "line" },
      { key: "n91", label: "Standard normal", mark: "line" },
    ],
    lines: [{ key: "n96", values: septembers.map((s) => s.running30) }],
    segments: standardNormals.map((n) => ({
      key: "n91" as const,
      from: n.start - first,
      to: n.end - first,
      value: n.temp_avg,
      text: n.period.replace("-", "–"),
    })),
    dots: [
      {
        key: "past",
        values: values.map((v, i) => (i === current ? null : v)),
        size: "sm",
      },
      {
        key: "obs",
        values: values.map((v, i) => (i === current ? v : null)),
        size: "md",
      },
    ],
    notes: [
      {
        index: current,
        value: values[current],
        key: "obs",
        side: "above",
        text: { metric: String(year), imperial: String(year) },
      },
      {
        index: last,
        value: runningEnd.running30 ?? 0,
        key: "n96",
        side: "below",
        text: unitPair("temperature", runningEnd.running30 ?? 0, 2),
      },
    ],
    tooltips: septembers.map((s) => ({
      title: `September ${s.year}`,
      rows: [
        {
          key: s.year === year ? "obs" : "past",
          label: "Mean",
          value: unitPair("temperature", s.temp_avg),
        },
        ...(s.running30 != null
          ? [
              {
                key: "n96" as const,
                label: `${s.year - 29}–${s.year} mean`,
                value: unitPair("temperature", s.running30, 2),
              },
            ]
          : []),
      ],
    })),
    table: {
      caption: "September mean temperature by year",
      headers: ["Year", "Mean", "30-year running mean"],
      rows: septembers.map((s) => [
        String(s.year),
        unitPair("temperature", s.temp_avg),
        s.running30 != null ? unitPair("temperature", s.running30, 2) : "",
      ]),
    },
  };
}

export type DailyMeasure = "temperature" | "feels";

type DailyValues = { avg: number; high: number; low: number };

const dailyMeasures: Record<
  DailyMeasure,
  {
    noun: string;
    obs: (d: ReportDay) => DailyValues;
    n96: (d: ReportDay) => DailyValues;
    n91: (d: ReportDay) => number;
  }
> = {
  temperature: {
    noun: "temperature",
    obs: (d) => ({ avg: d.temp_avg, high: d.temp_high, low: d.temp_low }),
    n96: (d) => ({
      avg: d.n96.temp_avg,
      high: d.n96.temp_high,
      low: d.n96.temp_low,
    }),
    n91: (d) => d.n91.temp_avg,
  },
  feels: {
    noun: "feels-like temperature",
    obs: (d) => ({ avg: d.feels_avg, high: d.feels_high, low: d.feels_low }),
    n96: (d) => ({
      avg: d.n96.feels_avg,
      high: d.n96.feels_high,
      low: d.n96.feels_low,
    }),
    n91: (d) => d.n91.feels_avg,
  },
};

// The month day by day: each day's low–high range and mean against the
// running normal's mean and range, and the standard normal's mean.
export function dailyChart(
  days: ReportDay[],
  measure: DailyMeasure = "temperature",
): ChartSpec {
  const { noun, ...field } = dailyMeasures[measure];
  const obs = days.map(field.obs);
  const n96 = days.map(field.n96);
  const n91 = days.map(field.n91);
  const all = [...obs, ...n96].flatMap((v) => [v.high, v.low]);
  const { yMin, yMax, ticks } = valueAxis(all, 5);
  const highest = obs.reduce((a, v, i) => (v.high > obs[a].high ? i : a), 0);
  const lowest = obs.reduce((a, v, i) => (v.low < obs[a].low ? i : a), 0);
  return {
    ...emptySpec(),
    label: `Daily mean, high and low ${noun} at Kaisaniemi through the month against the 1996–2025 and 1991–2020 daily normals.`,
    kind: "temperature",
    count: days.length,
    yMin,
    yMax,
    ticks,
    xLabels: monthDayTicks(days),
    legend: [
      { key: "obs", label: "Daily mean", mark: "dot" },
      { key: "obs", label: "Low to high", mark: "bar" },
      { key: "n96", label: "Normal mean, 1996–2025", mark: "line" },
      { key: "n96", label: "Normal low to high, 1996–2025", mark: "band" },
      { key: "n91", label: "Normal mean, 1991–2020", mark: "line" },
    ],
    bands: [
      {
        key: "n96",
        upper: n96.map((v) => v.high),
        lower: n96.map((v) => v.low),
      },
    ],
    lines: [
      { key: "n91", values: n91 },
      { key: "n96", values: n96.map((v) => v.avg) },
    ],
    ranges: [
      {
        key: "obs",
        low: obs.map((v) => v.low),
        high: obs.map((v) => v.high),
      },
    ],
    dots: [{ key: "obs", values: obs.map((v) => v.avg), size: "md" }],
    notes: [
      {
        index: highest,
        value: obs[highest].high,
        key: "obs",
        side: "above",
        text: unitPair("temperature", obs[highest].high),
      },
      {
        index: lowest,
        value: obs[lowest].low,
        key: "obs",
        side: "below",
        text: unitPair("temperature", obs[lowest].low),
      },
    ],
    tooltips: days.map((d, i) => ({
      title: dayLabel(d.date),
      rows: [
        {
          key: "obs",
          label: "Mean",
          value: unitPair("temperature", obs[i].avg),
        },
        {
          key: "obs",
          label: "High / low",
          value: rangePair("temperature", obs[i].high, obs[i].low),
        },
        {
          key: "n96",
          label: "Normal 1996–2025",
          value: unitPair("temperature", n96[i].avg),
        },
        {
          key: "n91",
          label: "Normal 1991–2020",
          value: unitPair("temperature", n91[i]),
        },
      ],
    })),
    table: {
      caption: `Daily ${noun} and normal means`,
      headers: [
        "Date",
        "Mean",
        "High",
        "Low",
        "Normal 1996–2025",
        "Normal 1991–2020",
      ],
      rows: days.map((d, i) => [
        dayLabel(d.date),
        unitPair("temperature", obs[i].avg),
        unitPair("temperature", obs[i].high),
        unitPair("temperature", obs[i].low),
        unitPair("temperature", n96[i].avg),
        unitPair("temperature", n91[i]),
      ]),
    },
  };
}

// Daily rain as columns and the month-to-date total against both normals'
// accumulated daily means.
export function rainChart(days: ReportDay[]): ChartSpec {
  const all = days.flatMap((d) => [
    d.precip_cum,
    d.n91.precip_cum,
    d.n96.precip_cum,
  ]);
  const { yMin, yMax, ticks } = valueAxis(all, 20, { floor: 0 });
  const wettest = days.reduce(
    (a, d, i) => (d.precip_mm > days[a].precip_mm ? i : a),
    0,
  );
  const last = days.length - 1;
  return {
    ...emptySpec(),
    label:
      "Daily rainfall and the month-to-date total at Kaisaniemi against the 1996–2025 and 1991–2020 normals.",
    kind: "precipitation",
    count: days.length,
    yMin,
    yMax,
    ticks,
    xLabels: monthDayTicks(days),
    legend: [
      { key: "obs", label: "Month to date", mark: "line" },
      { key: "obs", label: "Daily rain", mark: "bar" },
      { key: "n96", label: "Normal, 1996–2025", mark: "line" },
      { key: "n91", label: "Normal, 1991–2020", mark: "line" },
    ],
    lines: [
      { key: "n91", values: days.map((d) => d.n91.precip_cum) },
      { key: "n96", values: days.map((d) => d.n96.precip_cum) },
      { key: "obs", values: days.map((d) => d.precip_cum) },
    ],
    columns: [
      {
        key: "obs",
        values: days.map((d) => (d.precip_mm > 0 ? d.precip_mm : null)),
      },
    ],
    notes: [
      {
        index: last,
        value: days[last].precip_cum,
        key: "obs",
        side: "above",
        text: unitPair("precipitation", days[last].precip_cum),
      },
      {
        index: wettest,
        value: days[wettest].precip_mm,
        key: "obs",
        side: "above",
        text: unitPair("precipitation", days[wettest].precip_mm),
      },
    ],
    tooltips: days.map((d) => ({
      title: dayLabel(d.date),
      rows: [
        {
          key: "obs",
          label: "Rain",
          value: unitPair("precipitation", d.precip_mm),
        },
        {
          key: "obs",
          label: "Month to date",
          value: unitPair("precipitation", d.precip_cum),
        },
        {
          key: "n96",
          label: "Normal 1996–2025",
          value: unitPair("precipitation", d.n96.precip_cum),
        },
        {
          key: "n91",
          label: "Normal 1991–2020",
          value: unitPair("precipitation", d.n91.precip_cum),
        },
      ],
    })),
    table: {
      caption: "Daily rainfall and month-to-date totals",
      headers: [
        "Date",
        "Rain",
        "Month to date",
        "Normal 1996–2025",
        "Normal 1991–2020",
      ],
      rows: days.map((d) => [
        dayLabel(d.date),
        unitPair("precipitation", d.precip_mm),
        unitPair("precipitation", d.precip_cum),
        unitPair("precipitation", d.n96.precip_cum),
        unitPair("precipitation", d.n91.precip_cum),
      ]),
    },
  };
}

// Daily hours of sunshine against the day's length and the station's
// average for the date.
export function sunshineChart(
  days: SunDay[],
  station: string,
  baseYears: number[],
): ChartSpec {
  const sun = days.map((d) => d.sun_h);
  const daylight = days.map((d) => d.daylight_h);
  const normal = days.map((d) => d.normal_h);
  const { yMin, yMax, ticks } = valueAxis([...sun, ...daylight], 4, {
    floor: 0,
  });
  const sunniest = sun.reduce((a, v, i) => (v > sun[a] ? i : a), 0);
  const base = baseYears.join("–");
  return {
    ...emptySpec(),
    label: `Daily hours of sunshine at ${station} through the month against the length of the day and the ${base} average.`,
    kind: "hours",
    count: days.length,
    yMin,
    yMax,
    ticks,
    xLabels: monthDayTicks(days as unknown as ReportDay[]),
    legend: [
      { key: "obs", label: "Sunshine", mark: "bar" },
      { key: "n96", label: `Average, ${base}`, mark: "line" },
      { key: "past", label: "Daylight", mark: "band" },
    ],
    bands: [{ key: "past", upper: daylight, lower: days.map(() => 0) }],
    lines: [{ key: "n96", values: normal }],
    columns: [
      {
        key: "obs",
        values: sun.map((v) => (v >= 0.05 ? v : null)),
        strong: true,
      },
    ],
    notes: [
      {
        index: sunniest,
        value: sun[sunniest],
        key: "obs",
        side: "above",
        text: unitPair("hours", sun[sunniest]),
      },
    ],
    tooltips: days.map((d) => ({
      title: dayLabel(d.date),
      rows: [
        { key: "obs", label: "Sunshine", value: unitPair("hours", d.sun_h) },
        {
          key: "past",
          label: "Daylight",
          value: unitPair("hours", d.daylight_h),
        },
        {
          key: "n96",
          label: `Average ${base}`,
          value: unitPair("hours", d.normal_h),
        },
      ],
    })),
    table: {
      caption: "Daily sunshine, daylight and average",
      headers: ["Date", "Sunshine", "Daylight", `Average ${base}`],
      rows: days.map((d) => [
        dayLabel(d.date),
        unitPair("hours", d.sun_h),
        unitPair("hours", d.daylight_h),
        unitPair("hours", d.normal_h),
      ]),
    },
  };
}
