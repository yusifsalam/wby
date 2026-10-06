const NUMBER = /[−+-]?\d+(?:\.\d+)?/;

function easeOut(t: number): number {
  return 1 - (1 - t) ** 3;
}

// Counts the first number in each text node of [data-count] elements up from
// zero, keeping its sign, decimals and surrounding text.
function countUp(root: Element, reduce: boolean): void {
  if (reduce) return;
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
      const t = Math.min(1, Math.max(0, (now - start) / duration));
      for (const n of nodes)
        n.node.textContent = `${n.before}${(n.value * easeOut(t)).toFixed(n.decimals)}${n.after}`;
      if (t < 1) requestAnimationFrame(frame);
    };
    requestAnimationFrame(frame);
  }
}

// Shifts parallax stages' layers by depth as the stage moves through the
// viewport (the stylesheet turns --py into a per-layer translate).
function startParallax(root: ParentNode): void {
  const stages = [...root.querySelectorAll<HTMLElement>(".cl-parallax")];
  if (stages.length === 0) return;
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
      if (queued) return;
      queued = true;
      requestAnimationFrame(update);
    },
    { passive: true },
  );
  update();
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
// animations while it is off screen.
export function startCollage(root: ParentNode = document): void {
  const scenes = [...root.querySelectorAll<HTMLElement>("[data-cl]")];
  if (scenes.length === 0) return;
  const reduce = !document.documentElement.classList.contains("cl-motion");
  if (!("IntersectionObserver" in window)) {
    for (const el of scenes) el.classList.add("is-in");
    return;
  }
  const enter = new IntersectionObserver(
    (entries) => {
      for (const entry of entries) {
        if (!entry.isIntersecting) continue;
        entry.target.classList.add("is-in");
        countUp(entry.target, reduce);
        enter.unobserve(entry.target);
      }
    },
    { threshold: 0.3, rootMargin: "0px 0px -8% 0px" },
  );
  for (const el of scenes) enter.observe(el);
  if (reduce) return;
  startParallax(root);
  startReplay(root);
  if (!("getAnimations" in Element.prototype)) return;
  const paused = new WeakMap<Element, Animation[]>();
  const visible = new IntersectionObserver((entries) => {
    for (const entry of entries) {
      const el = entry.target;
      if (entry.isIntersecting) {
        for (const animation of paused.get(el) ?? []) animation.play();
        paused.delete(el);
      } else if (el.classList.contains("is-in")) {
        const running = el
          .getAnimations({ subtree: true })
          .filter((a) => a.playState === "running");
        for (const animation of running) animation.pause();
        paused.set(el, running);
      }
    }
  });
  for (const el of scenes) visible.observe(el);
}
