import { applySkyState, getLiveSkyState, getSkyState } from "./controller";
import { isSkyScene, SKY_PRESETS, SKY_SCENES, type SkyState } from "./state";

type NumericField = Exclude<keyof SkyState, "scene">;

const FIELDS: readonly {
  key: NumericField;
  param: string;
  label: string;
  min: number;
  max: number;
  step: number;
  fallback: number;
  format: (value: number) => string;
}[] = [
  {
    key: "cloudCover",
    param: "cover",
    label: "Cloud cover",
    min: 0,
    max: 8,
    step: 0.5,
    fallback: 4,
    format: (v) => `${v}/8`,
  },
  {
    key: "precipitation1h",
    param: "precip",
    label: "Precipitation",
    min: 0,
    max: 20,
    step: 0.1,
    fallback: 0,
    format: (v) => `${v.toFixed(1)} mm/h`,
  },
  {
    key: "windSpeed",
    param: "wind",
    label: "Wind speed",
    min: 0,
    max: 25,
    step: 0.5,
    fallback: 3,
    format: (v) => `${v} m/s`,
  },
  {
    key: "windDirection",
    param: "dir",
    label: "Wind from",
    min: 0,
    max: 355,
    step: 5,
    fallback: 270,
    format: (v) => `${v}°`,
  },
];

function displayValue(state: SkyState, key: NumericField): number | null {
  const value = state[key];
  if (key === "cloudCover" && value != null && value > 8.5)
    return Math.round((value / 100) * 16) / 2;
  return value;
}

function readQuery(): SkyState | null {
  const params = new URLSearchParams(location.search);
  const scene = params.get("sky");
  if (!isSkyScene(scene)) return null;
  const state: SkyState = {
    scene,
    precipitation1h: null,
    cloudCover: null,
    windSpeed: null,
    windDirection: null,
  };
  for (const field of FIELDS) {
    const raw = params.get(field.param);
    const value = raw == null ? Number.NaN : Number(raw);
    if (Number.isFinite(value)) state[field.key] = value;
  }
  return state;
}

function writeQuery(state: SkyState | null): void {
  const url = new URL(location.href);
  url.searchParams.delete("sky");
  for (const field of FIELDS) url.searchParams.delete(field.param);
  if (state) {
    url.searchParams.set("sky", state.scene);
    for (const field of FIELDS) {
      const value = state[field.key];
      if (value != null) url.searchParams.set(field.param, String(value));
    }
  }
  history.replaceState(history.state, "", url);
}

const button =
  "rounded-md border border-line bg-surface-strong px-2 py-1 text-left text-[0.75rem] font-semibold hover:border-teal hover:text-ink";

export function mountSkyDevPanel(): void {
  const panel = document.createElement("details");
  panel.className =
    "fixed right-4 bottom-4 z-50 w-68 rounded-lg border border-line bg-surface p-3 text-[0.8rem] text-ink shadow-panel";
  panel.innerHTML = `
    <summary class="cursor-pointer font-extrabold">Sky preview</summary>
    <div class="mt-2.5 grid grid-cols-2 gap-1" data-presets></div>
    <label class="mt-3 flex items-center justify-between gap-2 font-semibold">Scene
      <select class="rounded-md border border-line bg-surface-strong px-1.5 py-1" data-scene>
        ${SKY_SCENES.map((scene) => `<option value="${scene}">${scene}</option>`).join("")}
      </select>
    </label>
    ${FIELDS.map(
      (field) => `
      <label class="mt-2 block font-semibold">
        <span class="flex justify-between">${field.label}<output class="font-normal text-muted" data-output="${field.key}"></output></span>
        <input class="w-full accent-teal" type="range" min="${field.min}" max="${field.max}" step="${field.step}" data-field="${field.key}" />
      </label>`,
    ).join("")}
    <button type="button" class="${button} mt-3 w-full text-center" data-reset>Reset to live data</button>
  `;

  const presets = panel.querySelector<HTMLElement>("[data-presets]");
  const scene = panel.querySelector<HTMLSelectElement>("[data-scene]");
  const inputs = new Map(
    FIELDS.map((field) => [
      field,
      {
        input: panel.querySelector<HTMLInputElement>(
          `[data-field="${field.key}"]`,
        ),
        output: panel.querySelector<HTMLOutputElement>(
          `[data-output="${field.key}"]`,
        ),
      },
    ]),
  );

  function sync(state: SkyState): void {
    if (scene) scene.value = state.scene;
    for (const [field, { input, output }] of inputs) {
      const value = displayValue(state, field.key) ?? field.fallback;
      if (input) input.value = String(value);
      if (output) output.textContent = field.format(value);
    }
  }

  function apply(state: SkyState, query: SkyState | null): void {
    applySkyState(state);
    writeQuery(query);
    sync(state);
  }

  function fromControls(): SkyState {
    const state = {
      ...getSkyState(),
      scene: isSkyScene(scene?.value) ? scene.value : getSkyState().scene,
    };
    for (const [field, { input }] of inputs) {
      if (input) state[field.key] = Number(input.value);
    }
    return state;
  }

  for (const preset of SKY_PRESETS) {
    const presetButton = document.createElement("button");
    presetButton.type = "button";
    presetButton.className = button;
    presetButton.textContent = preset.label;
    presetButton.addEventListener("click", () =>
      apply(preset.state, preset.state),
    );
    presets?.append(presetButton);
  }

  panel.addEventListener("input", () => {
    const state = fromControls();
    apply(state, state);
  });
  panel
    .querySelector("[data-reset]")
    ?.addEventListener("click", () => apply(getLiveSkyState(), null));

  const fromQuery = readQuery();
  if (fromQuery) {
    applySkyState(fromQuery);
    panel.open = true;
  }
  sync(getSkyState());
  document.body.append(panel);
}
