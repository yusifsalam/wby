const NUMBER = /[−+-]?\d+(?:\.\d+)?/;

function easeOut(t: number): number {
  return 1 - (1 - t) ** 3;
}

function motion(): boolean {
  return document.documentElement.classList.contains("cl-motion");
}

// Calls back when the page's .cl-motion class is switched on or off.
function onMotionChange(callback: (on: boolean) => void): void {
  let on = motion();
  new MutationObserver(() => {
    if (motion() === on) return;
    on = motion();
    callback(on);
  }).observe(document.documentElement, { attributeFilter: ["class"] });
}

// Counts the first number in each text node of [data-count] elements up from
// zero, keeping its sign, decimals and surrounding text.
function countUp(root: Element): void {
  if (!motion()) return;
  for (const el of root.querySelectorAll<HTMLElement>("[data-count]")) {
    const walker = document.createTreeWalker(el, NodeFilter.SHOW_TEXT);
    const nodes: {
      node: Text;
      before: string;
      after: string;
      value: number;
      decimals: number;
      sign: string;
    }[] = [];
    for (let node = walker.nextNode(); node; node = walker.nextNode()) {
      const text = node.textContent ?? "";
      const match = text.match(NUMBER);
      if (!match || match.index == null) continue;
      const raw = match[0];
      const sign = /^[−+-]/.test(raw) ? raw[0] : "";
      const digits = raw.slice(sign.length);
      nodes.push({
        node: node as Text,
        before: text.slice(0, match.index) + sign,
        after: text.slice(match.index + raw.length),
        value: Number(digits),
        decimals: digits.split(".")[1]?.length ?? 0,
        sign,
      });
    }
    const delay = Number(el.dataset.countDelay ?? 0);
    const duration = 1100;
    const start = performance.now() + delay;
    const frame = (now: number) => {
      const t = motion()
        ? Math.min(1, Math.max(0, (now - start) / duration))
        : 1;
      for (const n of nodes)
        n.node.textContent = `${n.before}${(n.value * easeOut(t)).toFixed(n.decimals)}${n.after}`;
      if (t < 1) requestAnimationFrame(frame);
    };
    requestAnimationFrame(frame);
  }
}

// Shifts parallax stages' layers by depth as the stage moves through the
// viewport (the stylesheet turns --py into a per-layer translate). Returns the
// update, for when motion is switched back on.
function startParallax(root: ParentNode): () => void {
  const stages = [...root.querySelectorAll<HTMLElement>(".cl-parallax")];
  if (stages.length === 0) return () => {};
  let queued = false;
  const update = () => {
    queued = false;
    for (const stage of stages) {
      const rect = stage.getBoundingClientRect();
      if (rect.bottom < 0 || rect.top > innerHeight) continue;
      const offset = innerHeight / 2 - (rect.top + rect.height / 2);
      stage.style.setProperty(
        "--py",
        Math.max(-800, Math.min(800, offset)).toFixed(1),
      );
    }
  };
  addEventListener(
    "scroll",
    () => {
      if (queued || !motion()) return;
      queued = true;
      requestAnimationFrame(update);
    },
    { passive: true },
  );
  if (motion()) update();
  return update;
}

// Clicking a doodle plays it again.
function startReplay(root: ParentNode): void {
  for (const doodle of root.querySelectorAll<HTMLElement>(".cl-doodle")) {
    const stage = doodle.querySelector<HTMLElement>("[data-cl]");
    if (!stage) continue;
    doodle.addEventListener("click", () => {
      stage.classList.remove("is-in");
      void stage.offsetWidth;
      stage.classList.add("is-in");
    });
  }
}

// Starts each collage scene when it scrolls into view and pauses its running
// animations while it is off screen. Switching motion back on replays the
// scenes in view and rewinds the rest to play as they scroll in again.
export function startCollage(root: ParentNode = document): void {
  const scenes = [...root.querySelectorAll<HTMLElement>("[data-cl]")];
  if (scenes.length === 0) return;
  if (!("IntersectionObserver" in window)) {
    for (const el of scenes) el.classList.add("is-in");
    return;
  }
  const enter = new IntersectionObserver(
    (entries) => {
      for (const entry of entries) {
        if (!entry.isIntersecting) continue;
        entry.target.classList.add("is-in");
        countUp(entry.target);
        enter.unobserve(entry.target);
      }
    },
    { threshold: 0.3, rootMargin: "0px 0px -8% 0px" },
  );
  for (const el of scenes) enter.observe(el);
  const parallax = startParallax(root);
  startReplay(root);
  const canPause = "getAnimations" in Element.prototype;
  const inView = new Set<Element>();
  let paused = new WeakMap<Element, Animation[]>();
  const visible = new IntersectionObserver((entries) => {
    for (const entry of entries) {
      const el = entry.target;
      if (entry.isIntersecting) {
        inView.add(el);
        for (const animation of paused.get(el) ?? []) animation.play();
        paused.delete(el);
      } else {
        inView.delete(el);
        if (!canPause || !el.classList.contains("is-in")) continue;
        const running = el
          .getAnimations({ subtree: true })
          .filter((a) => a.playState === "running");
        for (const animation of running) animation.pause();
        paused.set(el, running);
      }
    }
  });
  for (const el of scenes) visible.observe(el);
  onMotionChange((on) => {
    paused = new WeakMap();
    if (!on) return;
    parallax();
    for (const el of scenes) {
      if (!el.classList.contains("is-in")) continue;
      if (inView.has(el)) {
        countUp(el);
      } else {
        el.classList.remove("is-in");
        enter.observe(el);
      }
    }
  });
}
