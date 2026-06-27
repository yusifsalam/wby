type CacheOptions = {
  ttlMs: number;
  staleMs: number;
  now?: () => number;
};

type CacheEntry<T> = {
  data: T;
  updatedAt: number;
  refresh?: Promise<void>;
};

export type CacheResult<T> = {
  state: "fresh" | "stale";
  data: T;
  updatedAt: number;
  refresh?: Promise<void>;
};

export class WeatherCache<T> {
  private readonly entries = new Map<string, CacheEntry<T>>();
  private readonly ttlMs: number;
  private readonly staleMs: number;
  private readonly now: () => number;

  constructor({ ttlMs, staleMs, now = Date.now }: CacheOptions) {
    this.ttlMs = ttlMs;
    this.staleMs = staleMs;
    this.now = now;
  }

  async get(key: string, loader: () => Promise<T>): Promise<CacheResult<T>> {
    const entry = this.entries.get(key);
    const currentTime = this.now();

    if (!entry) {
      const data = await loader();
      const loadedEntry = { data, updatedAt: this.now() };
      this.entries.set(key, loadedEntry);
      return { state: "fresh", data: loadedEntry.data, updatedAt: loadedEntry.updatedAt };
    }

    const age = currentTime - entry.updatedAt;
    if (age <= this.ttlMs) {
      return { state: "fresh", data: entry.data, updatedAt: entry.updatedAt };
    }

    if (age <= this.ttlMs + this.staleMs) {
      const refresh = this.refresh(key, entry, loader);
      return { state: "stale", data: entry.data, updatedAt: entry.updatedAt, refresh };
    }

    const data = await loader();
    const loadedEntry = { data, updatedAt: this.now() };
    this.entries.set(key, loadedEntry);
    return { state: "fresh", data: loadedEntry.data, updatedAt: loadedEntry.updatedAt };
  }

  private refresh(key: string, entry: CacheEntry<T>, loader: () => Promise<T>): Promise<void> {
    if (entry.refresh) {
      return entry.refresh;
    }

    entry.refresh = loader()
      .then((data) => {
        this.entries.set(key, { data, updatedAt: this.now() });
      })
      .catch(() => {
        this.entries.set(key, { data: entry.data, updatedAt: entry.updatedAt });
      });

    return entry.refresh;
  }
}
