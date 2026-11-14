type CacheOptions = {
  ttlMs: number;
  staleMs: number;
  now?: () => number;
  onRefreshError?: (key: string, error: unknown) => void;
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
  private readonly loading = new Map<string, Promise<CacheEntry<T>>>();
  private readonly ttlMs: number;
  private readonly staleMs: number;
  private readonly now: () => number;
  private readonly onRefreshError: (key: string, error: unknown) => void;

  constructor({
    ttlMs,
    staleMs,
    now = Date.now,
    onRefreshError = (key, error) =>
      console.error(`cache refresh failed for ${key}:`, error),
  }: CacheOptions) {
    this.ttlMs = ttlMs;
    this.staleMs = staleMs;
    this.now = now;
    this.onRefreshError = onRefreshError;
  }

  // An entry that `usable` rejects is loaded again as if it were absent.
  async get(
    key: string,
    loader: () => Promise<T>,
    usable: (data: T) => boolean = () => true,
  ): Promise<CacheResult<T>> {
    const stored = this.entries.get(key);
    const entry = stored && usable(stored.data) ? stored : undefined;
    const age = entry ? this.now() - entry.updatedAt : Number.POSITIVE_INFINITY;

    if (entry && age <= this.ttlMs) {
      return {
        state: "fresh",
        data: entry.data,
        updatedAt: entry.updatedAt,
      };
    }

    if (entry && age <= this.ttlMs + this.staleMs) {
      const refresh = this.refresh(key, entry, loader);
      return {
        state: "stale",
        data: entry.data,
        updatedAt: entry.updatedAt,
        refresh,
      };
    }

    const loaded = await this.load(key, loader);
    return { state: "fresh", data: loaded.data, updatedAt: loaded.updatedAt };
  }

  // Concurrent requests for a key with nothing to serve share one load.
  private load(key: string, loader: () => Promise<T>): Promise<CacheEntry<T>> {
    const pending = this.loading.get(key);
    if (pending) {
      return pending;
    }
    const load = loader()
      .then((data) => {
        const entry = { data, updatedAt: this.now() };
        this.entries.set(key, entry);
        return entry;
      })
      .finally(() => {
        this.loading.delete(key);
      });
    this.loading.set(key, load);
    return load;
  }

  private refresh(
    key: string,
    entry: CacheEntry<T>,
    loader: () => Promise<T>,
  ): Promise<void> {
    if (entry.refresh) {
      return entry.refresh;
    }

    entry.refresh = loader()
      .then((data) => {
        this.entries.set(key, { data, updatedAt: this.now() });
      })
      .catch((error) => {
        this.onRefreshError(key, error);
        this.entries.set(key, {
          data: entry.data,
          updatedAt: entry.updatedAt,
        });
      });

    return entry.refresh;
  }
}

// Remembers that a key failed, so callers can skip it until `ms` has passed.
export class Cooldown {
  private readonly until = new Map<string, number>();

  constructor(
    private readonly ms: number,
    private readonly now: () => number = Date.now,
  ) {}

  active(key: string): boolean {
    const until = this.until.get(key);
    if (until === undefined) {
      return false;
    }
    if (this.now() < until) {
      return true;
    }
    this.until.delete(key);
    return false;
  }

  start(key: string): void {
    this.until.set(key, this.now() + this.ms);
  }
}
