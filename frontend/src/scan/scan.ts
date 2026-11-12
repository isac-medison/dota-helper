import { GetIcon } from "../../bindings/dota-helper/services/heroes";

// Finds the ten hero portraits in the top bar of a Dota screenshot and tells which hero each one is,
// by comparing a coarse 16×8 colour grid of the portrait with every hero's icon (and portraits taught by hand,
// for arcanas and personas, whose portraits differ from the icons).

// The top bar is anchored top-centre and scales with the screen. Measured on a 16:9 shot, in screen units
// (the height of a 16:9 screen) from the centre: left edges of the slots, Radiant (left) first.
const LEFTS = [218, 348, 476, 605, 733, 1143, 1272, 1400, 1530, 1658].map((x) => (x - 1000) / 1125);
// Portraits hang under the player-colour strip at the top of the screen.
const TOP = 6.5 / 1125;
const SLOT_W = 120 / 1125;
const SLOT_H = 67.5 / 1125;
// The part of a portrait away from its slanted edges, as a share of the slot (and of the icon).
const INNER = { x0: 0.15, x1: 0.85, y0: 0.08, y1: 0.92 };
const GW = 16;
const GH = 8;
// A hero is the one whose distance is at most this share of the next hero's.
const MAX_RATIO = 0.6;
// An empty slot is a flat dark gradient; any portrait varies far more (≈14 vs ≥35 on real shots).
const EMPTY_STD = 22;
// How far the measured geometry may be off, in screen units (a captured window can sit a few pixels off),
// tried in steps of half of it.
const SLACK = 0.006;
// Closer than this, two grids are the same portrait (the same hero is 7–15 from its icon, other heroes ≥26).
const SAME_PORTRAIT = 5;
// Portraits taught before the strip above them was accounted for were cut too high, so they start afresh.
const TEMPLATES_KEY = "heroTemplates2";

export type Template = { heroId: number; features: number[] };

export type ScannedSlot = {
  status: "hero" | "empty" | "unknown";
  heroId: number | null;
  // The portrait's grid, kept so it can be taught as a template.
  features: number[];
  // The slot as a small image, to show what was seen.
  thumb: string;
};

// A piece of the image read back once: reading pixels from a canvas is far slower than averaging them here.
type Pixels = { data: Uint8ClampedArray; x: number; y: number; w: number; h: number };

function read(source: CanvasImageSource, sx: number, sy: number, sw: number, sh: number): Pixels {
  const [x, y, w, h] = [Math.floor(sx), Math.floor(sy), Math.ceil(sw) + 1, Math.ceil(sh) + 1];
  const g = new OffscreenCanvas(w, h).getContext("2d")!;
  g.drawImage(source, x, y, w, h, 0, 0, w, h);
  return { data: g.getImageData(0, 0, w, h).data, x, y, w, h };
}

// The mean colour of each cell of a GW×GH grid laid over the rectangle.
function grid(p: Pixels, sx: number, sy: number, sw: number, sh: number): number[] {
  const edge = (from: number, size: number, cells: number, max: number) =>
    Array.from({ length: cells + 1 }, (_, i) => Math.min(max, Math.max(0, Math.round(from + (i * size) / cells))));
  const xs = edge(sx - p.x, sw, GW, p.w);
  const ys = edge(sy - p.y, sh, GH, p.h);
  const features: number[] = [];
  for (let cy = 0; cy < GH; cy++)
    for (let cx = 0; cx < GW; cx++) {
      let r = 0, g = 0, b = 0, n = 0;
      for (let y = ys[cy]; y < Math.max(ys[cy + 1], ys[cy] + 1); y++)
        for (let x = xs[cx]; x < Math.max(xs[cx + 1], xs[cx] + 1); x++) {
          const i = (y * p.w + x) * 4;
          r += p.data[i];
          g += p.data[i + 1];
          b += p.data[i + 2];
          n++;
        }
      features.push(r / n, g / n, b / n);
    }
  return features;
}

function distance(a: number[], b: number[]) {
  let sum = 0;
  for (let i = 0; i < a.length; i++) sum += Math.abs(a[i] - b[i]);
  return sum / a.length;
}

function spread(features: number[]) {
  const mean = features.reduce((s, v) => s + v, 0) / features.length;
  return Math.sqrt(features.reduce((s, v) => s + (v - mean) ** 2, 0) / features.length);
}

function load(src: string) {
  return new Promise<HTMLImageElement>((resolve, reject) => {
    const img = new Image();
    img.onload = () => resolve(img);
    img.onerror = () => reject(new Error(`Не вдалося завантажити ${src}`));
    img.src = src;
  });
}

async function iconTemplate(heroId: number): Promise<Template> {
  // Through Go rather than straight from the CDN: a cross-origin image would taint the canvas.
  const png = await GetIcon(heroId);
  if (!png) throw new Error(`Немає іконки героя ${heroId}`);
  const img = await load(`data:image/png;base64,${png}`);
  const { x0, x1, y0, y1 } = INNER;
  const w = img.naturalWidth;
  const h = img.naturalHeight;
  return { heroId, features: grid(read(img, 0, 0, w, h), x0 * w, y0 * h, (x1 - x0) * w, (y1 - y0) * h) };
}

// A burst of all ~130 icons at once through the tunnel (and on to the CDN) fails some of them.
const ICON_REQUESTS = 6;

// Every hero's icon as a template; loaded once per page. An icon that fails doesn't stop the scan (that hero
// just shows as "?"): the next scan tries the missing ones again, the rest come from the browser's cache.
let iconTemplates: Promise<Template[]> | null = null;
export function loadIconTemplates(heroIds: number[]): Promise<Template[]> {
  iconTemplates ??= (async () => {
    const loaded: Template[] = [];
    let next = 0;
    const worker = async () => {
      while (next < heroIds.length) {
        const heroId = heroIds[next++];
        try {
          loaded.push(await iconTemplate(heroId));
        } catch {
          // that hero shows as "?" until a later scan
        }
      }
    };
    await Promise.all(Array.from({ length: ICON_REQUESTS }, worker));
    if (loaded.length < heroIds.length) iconTemplates = null;
    if (loaded.length === 0) throw new Error("Не вдалося завантажити іконки героїв");
    return loaded;
  })();
  return iconTemplates;
}

// Portraits taught by hand. Storage can be blocked; then teaching lasts until the page reloads.
let memoryTemplates: Template[] = [];
export function taughtTemplates(): Template[] {
  try {
    const saved = JSON.parse(localStorage.getItem(TEMPLATES_KEY) ?? "[]");
    return Array.isArray(saved) ? saved : memoryTemplates;
  } catch {
    return memoryTemplates;
  }
}

// Teaching the same portrait again replaces what was taught for it before (a mistake being fixed).
export function teach(heroId: number, features: number[]) {
  const others = taughtTemplates().filter((t) => distance(t.features, features) >= SAME_PORTRAIT);
  memoryTemplates = [...others, { heroId, features: features.map(Math.round) }];
  try {
    localStorage.setItem(TEMPLATES_KEY, JSON.stringify(memoryTemplates));
  } catch {
    // not critical
  }
}

function scanSlot(source: CanvasImageSource, centre: number, unit: number, left: number, templates: Template[]): ScannedSlot {
  const sx = centre + left * unit;
  const sw = SLOT_W * unit;
  const sh = SLOT_H * unit;
  const { x0, x1, y0, y1 } = INNER;
  const [cx, cy, cw, ch] = [sx + x0 * sw, TOP * unit + y0 * sh, (x1 - x0) * sw, (y1 - y0) * sh];
  const slack = SLACK * unit;
  const pixels = read(source, cx - slack, cy - slack, cw + 2 * slack, ch + 2 * slack);
  const crop = (dx: number, dy: number) => grid(pixels, cx + dx, cy + dy, cw, ch);

  const thumbCanvas = document.createElement("canvas");
  thumbCanvas.width = 96;
  thumbCanvas.height = 54;
  thumbCanvas.getContext("2d")!.drawImage(source, sx, 0, sw, sh, 0, 0, 96, 54);
  const thumb = thumbCanvas.toDataURL();
  const shifts = [-1, -0.5, 0, 0.5, 1].map((d) => d * slack);
  const crops = shifts.flatMap((dx) => shifts.map((dy) => crop(dx, dy)));
  const features = crops[crops.length >> 1];
  // A shift may catch the frame around an empty slot, so the flattest one decides.
  if (Math.min(...crops.map(spread)) < EMPTY_STD) return { status: "empty", heroId: null, features, thumb };

  // Each hero's closest template over a few small shifts.
  const best = new Map<number, number>();
  for (const f of crops)
    for (const t of templates) {
      const d = distance(f, t.features);
      if (d < (best.get(t.heroId) ?? Infinity)) best.set(t.heroId, d);
    }
  const [first, second] = [...best].sort((a, b) => a[1] - b[1]);
  return first && (!second || first[1] <= MAX_RATIO * second[1])
    ? { status: "hero", heroId: first[0], features, thumb }
    : { status: "unknown", heroId: null, features, thumb };
}

// The ten slots, Radiant first. The bar scales with the height on wide screens; on narrower ones (16:10, 4:3)
// it may follow the width instead, so both are tried and the one that recognizes more wins.
export function scan(source: CanvasImageSource, width: number, height: number, templates: Template[]): ScannedSlot[] {
  const units = [...new Set([height, Math.min(height, (width * 9) / 16)])];
  const tries = units.map((unit) => LEFTS.map((left) => scanSlot(source, width / 2, unit, left, templates)));
  const score = (slots: ScannedSlot[]) => slots.filter((s) => s.status !== "unknown").length;
  return tries.reduce((a, b) => (score(b) > score(a) ? b : a));
}
