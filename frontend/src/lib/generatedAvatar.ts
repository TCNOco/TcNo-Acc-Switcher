/**
 * Deterministic abstract avatars.
 *
 * Two jobs, one generator: the stand-in shown in streamer mode instead of a real
 * profile picture, and the placeholder for an account that never had one. The same
 * seed always paints the same image, so an account keeps its face across restarts
 * with nothing cached on disk — but the seed carries a machine-local salt, so the
 * same SteamID on someone else's computer produces a completely different picture
 * and nobody can regenerate a streamer's avatar to confirm who they are watching.
 *
 * Everything below is seeded: there is not a single Math.random call, by design.
 */

/** Painted size. Tiles display at 64–100px; 128 keeps it crisp on hi-dpi. */
const RENDER_SIZE = 128;

/** Avatars are cheap to redraw but not free, and a list re-renders constantly. */
const CACHE_LIMIT = 512;

const cache = new Map<string, string>();

// ---- Seeded randomness ------------------------------------------------------

/** cyrb128: string to four well-mixed 32-bit words. */
function seedWords(text: string): [number, number, number, number] {
  let h1 = 1779033703;
  let h2 = 3144134277;
  let h3 = 1013904242;
  let h4 = 2773480762;
  for (let i = 0; i < text.length; i++) {
    const k = text.charCodeAt(i);
    h1 = h2 ^ Math.imul(h1 ^ k, 597399067);
    h2 = h3 ^ Math.imul(h2 ^ k, 2869860233);
    h3 = h4 ^ Math.imul(h3 ^ k, 951274213);
    h4 = h1 ^ Math.imul(h4 ^ k, 2716044179);
  }
  h1 = Math.imul(h3 ^ (h1 >>> 18), 597399067);
  h2 = Math.imul(h4 ^ (h2 >>> 22), 2869860233);
  h3 = Math.imul(h1 ^ (h3 >>> 17), 951274213);
  h4 = Math.imul(h2 ^ (h4 >>> 19), 2716044179);
  return [(h1 ^ h2 ^ h3 ^ h4) >>> 0, (h2 ^ h1) >>> 0, (h3 ^ h1) >>> 0, (h4 ^ h1) >>> 0];
}

/**
 * sfc32: small, fast, and good enough that neighbouring seeds look unrelated.
 * Exported because it is the determinism contract the whole feature rests on, and
 * the painting itself cannot be asserted without a canvas.
 */
export function makeRng(text: string): Rng {
  let [a, b, c, d] = seedWords(text);
  const next = (): number => {
    a >>>= 0; b >>>= 0; c >>>= 0; d >>>= 0;
    let t = (a + b) | 0;
    a = b ^ (b >>> 9);
    b = (c + (c << 3)) | 0;
    c = (c << 21) | (c >>> 11);
    d = (d + 1) | 0;
    t = (t + d) | 0;
    c = (c + t) | 0;
    return (t >>> 0) / 4294967296;
  };
  // sfc32 needs a warm-up or the first few outputs correlate with the seed.
  for (let i = 0; i < 12; i++) next();
  return {
    next,
    range: (min, max) => min + next() * (max - min),
    int: (min, max) => Math.floor(min + next() * (max - min + 1)),
    pick: <T>(items: readonly T[]): T => items[Math.floor(next() * items.length) % items.length],
    chance: (p) => next() < p,
    sign: () => (next() < 0.5 ? -1 : 1),
  };
}

interface Rng {
  next(): number;
  range(min: number, max: number): number;
  int(min: number, max: number): number;
  pick<T>(items: readonly T[]): T;
  chance(p: number): boolean;
  sign(): number;
}

// ---- Colour -----------------------------------------------------------------

/** Hue offsets that keep a set of colours looking chosen rather than collided. */
const HARMONIES: readonly (readonly number[])[] = [
  [0, 18, -18, 36], // analogous
  [0, 180, 155, 205], // complementary pair, softened
  [0, 120, 240, 60], // triad
  [0, 150, 210, 30], // split complementary
  [0, 90, 180, 270], // tetrad
  [0, 8, -8, 14], // near-monochrome, carried by lightness instead
];

/** Kept as components, not strings, so gradient stops can vary the alpha. */
type Colour = readonly [h: number, s: number, l: number];

interface Palette {
  bgFrom: Colour;
  bgTo: Colour;
  inks: Colour[];
  accent: Colour;
  glow: Colour;
  /** Pale ground with dark ink. Roughly one avatar in six, for range. */
  light: boolean;
}

function css([h, s, l]: Colour, a = 1): string {
  return `hsla(${((h % 360) + 360) % 360} ${s}% ${l}% / ${a})`;
}

function buildPalette(rng: Rng): Palette {
  const hue = rng.int(0, 359);
  const harmony = rng.pick(HARMONIES);
  const light = rng.chance(0.17);
  // Ink lightness is chosen against the ground rather than absolutely, so every
  // shape reads at 64px. An all-dark set was the first version's failure: it
  // turned a wall of avatars into a wall of murk.
  const groundL = light ? rng.range(72, 88) : rng.range(12, 27);
  const groundS = light ? rng.range(14, 38) : rng.range(30, 62);
  const inkL: [number, number] = light ? [26, 52] : [55, 82];
  return {
    bgFrom: [hue + rng.range(-14, 14), groundS, groundL],
    bgTo: [
      hue + rng.pick(harmony) + rng.range(-12, 12),
      groundS * rng.range(0.7, 1.15),
      groundL * rng.range(light ? 0.92 : 0.5, light ? 1.06 : 0.95),
    ],
    inks: harmony.map(
      (offset): Colour => [
        hue + offset + rng.range(-8, 8),
        rng.range(62, 95),
        rng.range(inkL[0], inkL[1]),
      ],
    ),
    accent: [hue + rng.pick([120, 150, 180, 210]), rng.range(78, 97), light ? rng.range(22, 40) : rng.range(62, 84)],
    glow: [hue + rng.range(-50, 50), rng.range(65, 90), light ? rng.range(60, 75) : rng.range(48, 66)],
    light,
  };
}

// ---- Drawing helpers --------------------------------------------------------

type Ctx = CanvasRenderingContext2D;

function polygon(ctx: Ctx, cx: number, cy: number, r: number, sides: number, rot: number): void {
  ctx.beginPath();
  for (let i = 0; i < sides; i++) {
    const angle = rot + (i / sides) * Math.PI * 2;
    const x = cx + Math.cos(angle) * r;
    const y = cy + Math.sin(angle) * r;
    if (i === 0) ctx.moveTo(x, y);
    else ctx.lineTo(x, y);
  }
  ctx.closePath();
}

function ring(ctx: Ctx, cx: number, cy: number, r: number, from: number, to: number, width: number): void {
  ctx.beginPath();
  ctx.lineWidth = width;
  ctx.lineCap = "round";
  ctx.arc(cx, cy, r, from, to);
  ctx.stroke();
}

function dot(ctx: Ctx, cx: number, cy: number, r: number): void {
  ctx.beginPath();
  ctx.arc(cx, cy, r, 0, Math.PI * 2);
  ctx.fill();
}

/** Rounded rect without depending on roundRect being present. */
function roundedRect(ctx: Ctx, x: number, y: number, w: number, h: number, r: number): void {
  const radius = Math.max(0, Math.min(r, Math.min(w, h) / 2));
  ctx.beginPath();
  ctx.moveTo(x + radius, y);
  ctx.arcTo(x + w, y, x + w, y + h, radius);
  ctx.arcTo(x + w, y + h, x, y + h, radius);
  ctx.arcTo(x, y + h, x, y, radius);
  ctx.arcTo(x, y, x + w, y, radius);
  ctx.closePath();
}

// ---- Compositions -----------------------------------------------------------

/**
 * Each archetype is a different way of filling the square. Picking one per seed is
 * what makes a wall of avatars look like a set of distinct pictures rather than one
 * picture with the colours shuffled.
 */
type Composition = (ctx: Ctx, rng: Rng, p: Palette, s: number) => void;

const orbits: Composition = (ctx, rng, p, s) => {
  const cx = s * rng.range(0.35, 0.65);
  const cy = s * rng.range(0.35, 0.65);
  const rings = rng.int(3, 6);
  for (let i = 0; i < rings; i++) {
    const r = s * (0.12 + (i / rings) * rng.range(0.32, 0.46));
    const from = rng.range(0, Math.PI * 2);
    ctx.strokeStyle = css(rng.pick(p.inks));
    ctx.globalAlpha = rng.range(0.45, 0.9);
    ring(ctx, cx, cy, r, from, from + rng.range(0.7, Math.PI * 2), s * rng.range(0.012, 0.05));
    if (rng.chance(0.65)) {
      const angle = from + rng.range(0, 1.5);
      ctx.fillStyle = css(p.accent);
      dot(ctx, cx + Math.cos(angle) * r, cy + Math.sin(angle) * r, s * rng.range(0.02, 0.05));
    }
  }
  ctx.globalAlpha = 1;
};

const shards: Composition = (ctx, rng, p, s) => {
  const count = rng.int(4, 8);
  for (let i = 0; i < count; i++) {
    ctx.globalAlpha = rng.range(0.18, 0.5);
    ctx.fillStyle = css(rng.pick(p.inks));
    polygon(
      ctx,
      s * rng.range(0.1, 0.9),
      s * rng.range(0.1, 0.9),
      s * rng.range(0.18, 0.48),
      rng.int(3, 6),
      rng.range(0, Math.PI * 2),
    );
    if (rng.chance(0.72)) {
      ctx.fill();
    } else {
      ctx.lineWidth = s * rng.range(0.012, 0.03);
      ctx.strokeStyle = css(rng.pick(p.inks));
      ctx.stroke();
    }
  }
  ctx.globalAlpha = 1;
};

const bauhaus: Composition = (ctx, rng, p, s) => {
  const cells = rng.int(3, 4);
  const step = s / cells;
  for (let gx = 0; gx < cells; gx++) {
    for (let gy = 0; gy < cells; gy++) {
      if (rng.chance(0.3)) continue;
      const x = gx * step;
      const y = gy * step;
      ctx.globalAlpha = rng.range(0.55, 0.95);
      ctx.fillStyle = css(rng.pick(p.inks));
      switch (rng.int(0, 4)) {
        case 0: {
          // Quarter circle tucked into one corner of the cell.
          const corner = rng.int(0, 3);
          ctx.beginPath();
          ctx.moveTo(x + (corner === 1 || corner === 2 ? step : 0), y + (corner >= 2 ? step : 0));
          ctx.arc(
            x + (corner === 1 || corner === 2 ? step : 0),
            y + (corner >= 2 ? step : 0),
            step,
            (corner * Math.PI) / 2,
            ((corner + 1) * Math.PI) / 2,
          );
          ctx.closePath();
          ctx.fill();
          break;
        }
        case 1:
          dot(ctx, x + step / 2, y + step / 2, step * rng.range(0.22, 0.45));
          break;
        case 2: {
          const thin = step * rng.range(0.18, 0.4);
          const vertical = rng.chance(0.5);
          ctx.fillRect(
            vertical ? x + (step - thin) / 2 : x,
            vertical ? y : y + (step - thin) / 2,
            vertical ? thin : step,
            vertical ? step : thin,
          );
          break;
        }
        case 3:
          roundedRect(ctx, x + step * 0.12, y + step * 0.12, step * 0.76, step * 0.76, step * rng.range(0.05, 0.35));
          ctx.fill();
          break;
        default:
          ctx.beginPath();
          ctx.moveTo(x, y + step);
          ctx.lineTo(x + step, y + step);
          ctx.lineTo(rng.chance(0.5) ? x : x + step, y);
          ctx.closePath();
          ctx.fill();
      }
    }
  }
  ctx.globalAlpha = 1;
};

const strata: Composition = (ctx, rng, p, s) => {
  ctx.save();
  ctx.translate(s / 2, s / 2);
  ctx.rotate(rng.range(-Math.PI / 3, Math.PI / 3));
  ctx.translate(-s, -s);
  let y = 0;
  const span = s * 2;
  while (y < span) {
    const thickness = s * rng.range(0.04, 0.22);
    ctx.globalAlpha = rng.range(0.3, 0.85);
    ctx.fillStyle = css(rng.pick(p.inks));
    ctx.fillRect(0, y, span, thickness);
    if (rng.chance(0.35)) {
      ctx.fillStyle = css(p.accent);
      ctx.globalAlpha = rng.range(0.5, 0.95);
      ctx.fillRect(rng.range(0, span * 0.7), y, span * rng.range(0.08, 0.3), thickness);
    }
    y += thickness + s * rng.range(0.01, 0.09);
  }
  ctx.restore();
  ctx.globalAlpha = 1;
};

const bloom: Composition = (ctx, rng, p, s) => {
  const petals = rng.int(3, 8);
  const cx = s / 2;
  const cy = s / 2;
  const layers = rng.int(1, 3);
  for (let layer = 0; layer < layers; layer++) {
    const reach = s * rng.range(0.2, 0.46);
    const width = s * rng.range(0.06, 0.2);
    const spin = rng.range(0, Math.PI * 2);
    const colour = css(rng.pick(p.inks));
    const triangular = rng.chance(0.4);
    ctx.globalAlpha = rng.range(0.35, 0.8);
    ctx.fillStyle = colour;
    for (let i = 0; i < petals; i++) {
      ctx.save();
      ctx.translate(cx, cy);
      ctx.rotate(spin + (i / petals) * Math.PI * 2);
      if (triangular) {
        ctx.beginPath();
        ctx.moveTo(0, -reach);
        ctx.lineTo(width, 0);
        ctx.lineTo(-width, 0);
        ctx.closePath();
        ctx.fill();
      } else {
        ctx.beginPath();
        ctx.ellipse(0, -reach * 0.6, width, reach * 0.6, 0, 0, Math.PI * 2);
        ctx.fill();
      }
      ctx.restore();
    }
  }
  ctx.globalAlpha = 1;
};

const mesh: Composition = (ctx, rng, p, s) => {
  const blobs = rng.int(4, 6);
  for (let i = 0; i < blobs; i++) {
    const x = s * rng.range(-0.05, 1.05);
    const y = s * rng.range(-0.05, 1.05);
    const r = s * rng.range(0.3, 0.66);
    const grad = ctx.createRadialGradient(x, y, 0, x, y, r);
    const colour = rng.pick(p.inks);
    grad.addColorStop(0, css(colour, rng.range(0.75, 1)));
    grad.addColorStop(0.5, css(colour, rng.range(0.3, 0.55)));
    grad.addColorStop(1, css(colour, 0));
    ctx.fillStyle = grad;
    ctx.fillRect(0, 0, s, s);
  }
  // Hard edges over the gradients, or the whole thing reads as a smudge.
  for (let i = 0; i < rng.int(2, 4); i++) {
    ctx.globalAlpha = rng.range(0.75, 1);
    ctx.strokeStyle = css(rng.chance(0.5) ? p.accent : rng.pick(p.inks));
    ctx.lineWidth = s * rng.range(0.016, 0.045);
    ctx.beginPath();
    ctx.arc(s * rng.range(0.2, 0.8), s * rng.range(0.2, 0.8), s * rng.range(0.12, 0.38), 0, Math.PI * 2);
    ctx.stroke();
  }
  ctx.globalAlpha = 1;
};

const rays: Composition = (ctx, rng, p, s) => {
  const spokes = rng.int(5, 14);
  const cx = s * rng.range(0.1, 0.9);
  const cy = s * rng.range(0.1, 0.9);
  const spin = rng.range(0, Math.PI * 2);
  const spread = (Math.PI * 2) / spokes;
  for (let i = 0; i < spokes; i++) {
    ctx.globalAlpha = rng.range(0.45, 0.9);
    ctx.fillStyle = css(rng.pick(p.inks));
    const from = spin + i * spread;
    ctx.beginPath();
    ctx.moveTo(cx, cy);
    ctx.arc(cx, cy, s * 1.5, from, from + spread * rng.range(0.3, 0.85));
    ctx.closePath();
    ctx.fill();
  }
  ctx.globalAlpha = rng.range(0.7, 1);
  ctx.fillStyle = css(p.accent);
  dot(ctx, cx, cy, s * rng.range(0.05, 0.14));
  ctx.globalAlpha = 1;
};

const target: Composition = (ctx, rng, p, s) => {
  const cx = s * rng.range(0.3, 0.7);
  const cy = s * rng.range(0.3, 0.7);
  const bands = rng.int(4, 8);
  const reach = s * rng.range(0.6, 0.95);
  for (let i = bands; i > 0; i--) {
    ctx.globalAlpha = rng.range(0.7, 1);
    ctx.fillStyle = css(rng.pick(p.inks));
    dot(ctx, cx, cy, (i / bands) * reach);
  }
  if (rng.chance(0.5)) {
    // Slice the rings so the result is not just a bullseye every time.
    ctx.globalCompositeOperation = "destination-out";
    ctx.beginPath();
    ctx.moveTo(cx, cy);
    const from = rng.range(0, Math.PI * 2);
    ctx.arc(cx, cy, s * 1.5, from, from + rng.range(0.5, 2.2));
    ctx.closePath();
    ctx.fill();
    ctx.globalCompositeOperation = "source-over";
  }
  ctx.globalAlpha = 1;
};

const confetti: Composition = (ctx, rng, p, s) => {
  const count = rng.int(14, 30);
  for (let i = 0; i < count; i++) {
    ctx.globalAlpha = rng.range(0.5, 1);
    ctx.fillStyle = css(rng.pick(p.inks));
    const x = s * rng.range(0.05, 0.95);
    const y = s * rng.range(0.05, 0.95);
    const size = s * rng.range(0.03, 0.11);
    switch (rng.int(0, 2)) {
      case 0:
        dot(ctx, x, y, size);
        break;
      case 1:
        ctx.save();
        ctx.translate(x, y);
        ctx.rotate(rng.range(0, Math.PI));
        ctx.fillRect(-size, -size * rng.range(0.2, 0.6), size * 2, size * rng.range(0.4, 1.2));
        ctx.restore();
        break;
      default:
        polygon(ctx, x, y, size * 1.3, 3, rng.range(0, Math.PI * 2));
        ctx.fill();
    }
  }
  ctx.globalAlpha = 1;
};

const weave: Composition = (ctx, rng, p, s) => {
  const bars = rng.int(3, 6);
  const draw = (vertical: boolean): void => {
    for (let i = 0; i < bars; i++) {
      const thickness = s * rng.range(0.05, 0.16);
      const at = s * rng.range(0, 1) - thickness / 2;
      ctx.globalAlpha = rng.range(0.55, 0.95);
      ctx.fillStyle = css(rng.pick(p.inks));
      if (vertical) ctx.fillRect(at, -s * 0.1, thickness, s * 1.2);
      else ctx.fillRect(-s * 0.1, at, s * 1.2, thickness);
    }
  };
  draw(true);
  draw(false);
  ctx.globalAlpha = 1;
};

const glyph: Composition = (ctx, rng, p, s) => {
  // Mirrored block grid: the identicon lineage, but with the palette and the
  // rounding doing the work instead of flat squares.
  const cells = rng.int(4, 6);
  const step = s / cells;
  const half = Math.ceil(cells / 2);
  const radius = step * rng.pick([0, 0.15, 0.5]);
  for (let gx = 0; gx < half; gx++) {
    for (let gy = 0; gy < cells; gy++) {
      if (rng.chance(0.42)) continue;
      ctx.globalAlpha = rng.range(0.6, 1);
      ctx.fillStyle = css(rng.pick(p.inks));
      for (const x of [gx * step, (cells - 1 - gx) * step]) {
        roundedRect(ctx, x + step * 0.06, gy * step + step * 0.06, step * 0.88, step * 0.88, radius);
        ctx.fill();
      }
    }
  }
  ctx.globalAlpha = 1;
};

const prism: Composition = (ctx, rng, p, s) => {
  const layers = rng.int(4, 8);
  const sides = rng.int(3, 6);
  const cx = s * rng.range(0.42, 0.58);
  const cy = s * rng.range(0.42, 0.58);
  const twist = rng.range(0.05, 0.5) * rng.sign();
  for (let i = layers; i > 0; i--) {
    const r = (i / layers) * s * rng.range(0.5, 0.72);
    ctx.globalAlpha = rng.range(0.3, 0.75);
    polygon(ctx, cx, cy, r, sides, twist * i + rng.range(-0.05, 0.05));
    if (i % 2 === 0 || rng.chance(0.4)) {
      ctx.fillStyle = css(rng.pick(p.inks));
      ctx.fill();
    } else {
      ctx.strokeStyle = css(rng.pick(p.inks));
      ctx.lineWidth = s * rng.range(0.01, 0.03);
      ctx.stroke();
    }
  }
  ctx.globalAlpha = 1;
};

const COMPOSITIONS: readonly Composition[] = [
  orbits, shards, bauhaus, strata, bloom, mesh, glyph, prism, rays, target, confetti, weave,
];

// ---- Grain ------------------------------------------------------------------

let grainTile: HTMLCanvasElement | null = null;

/**
 * One 64px noise tile for the whole app, built once. Per-pixel noise over every
 * avatar would be the most expensive thing on the page; a tiled pattern offset per
 * seed is visually the same and costs one drawImage.
 */
function getGrainTile(): HTMLCanvasElement | null {
  if (grainTile) return grainTile;
  const size = 64;
  const canvas = document.createElement("canvas");
  canvas.width = size;
  canvas.height = size;
  const ctx = canvas.getContext("2d");
  if (!ctx) return null;
  const image = ctx.createImageData(size, size);
  const rng = makeRng("tcno-grain");
  for (let i = 0; i < image.data.length; i += 4) {
    const value = 110 + rng.next() * 76;
    image.data[i] = value;
    image.data[i + 1] = value;
    image.data[i + 2] = value;
    image.data[i + 3] = 255;
  }
  ctx.putImageData(image, 0, 0);
  grainTile = canvas;
  return grainTile;
}

// ---- Render -----------------------------------------------------------------

let scratch: HTMLCanvasElement | null = null;

function getScratch(): HTMLCanvasElement | null {
  if (typeof document === "undefined") return null;
  if (!scratch) {
    scratch = document.createElement("canvas");
    scratch.width = RENDER_SIZE;
    scratch.height = RENDER_SIZE;
  }
  return scratch;
}

function paintBackground(ctx: Ctx, rng: Rng, p: Palette, s: number): void {
  switch (rng.int(0, 2)) {
    case 0: {
      const angle = rng.range(0, Math.PI * 2);
      const grad = ctx.createLinearGradient(
        s / 2 - (Math.cos(angle) * s) / 2, s / 2 - (Math.sin(angle) * s) / 2,
        s / 2 + (Math.cos(angle) * s) / 2, s / 2 + (Math.sin(angle) * s) / 2,
      );
      grad.addColorStop(0, css(p.bgFrom));
      grad.addColorStop(1, css(p.bgTo));
      ctx.fillStyle = grad;
      break;
    }
    case 1: {
      const grad = ctx.createRadialGradient(
        s * rng.range(0.2, 0.8), s * rng.range(0.2, 0.8), 0,
        s / 2, s / 2, s * rng.range(0.7, 1.1),
      );
      grad.addColorStop(0, css(p.bgFrom));
      grad.addColorStop(1, css(p.bgTo));
      ctx.fillStyle = grad;
      break;
    }
    default: {
      ctx.fillStyle = css(p.bgTo);
      ctx.fillRect(0, 0, s, s);
      // Wedge of the lighter tone, for a background with a direction to it.
      ctx.fillStyle = css(p.bgFrom);
      ctx.beginPath();
      ctx.moveTo(0, s * rng.range(0, 1));
      ctx.lineTo(s, s * rng.range(0, 1));
      ctx.lineTo(s, rng.chance(0.5) ? 0 : s);
      ctx.lineTo(0, rng.chance(0.5) ? 0 : s);
      ctx.closePath();
      ctx.fill();
      return;
    }
  }
  ctx.fillRect(0, 0, s, s);
}

/** The squiggles, kept to the 1–3 that read as deliberate. */
function paintFilaments(ctx: Ctx, rng: Rng, p: Palette, s: number): void {
  const count = rng.int(1, 3);
  for (let i = 0; i < count; i++) {
    ctx.globalAlpha = rng.range(p.light ? 0.35 : 0.2, p.light ? 0.7 : 0.5);
    ctx.strokeStyle = css(rng.chance(0.5) ? p.accent : rng.pick(p.inks));
    ctx.lineWidth = s * rng.range(0.004, 0.014);
    ctx.beginPath();
    ctx.moveTo(s * rng.range(-0.1, 1.1), s * rng.range(-0.1, 1.1));
    ctx.bezierCurveTo(
      s * rng.range(0, 1), s * rng.range(0, 1),
      s * rng.range(0, 1), s * rng.range(0, 1),
      s * rng.range(-0.1, 1.1), s * rng.range(-0.1, 1.1),
    );
    ctx.stroke();
  }
  ctx.globalAlpha = 1;
}

/**
 * Two passes at different scales: a coarse one that carries the visible tooth and
 * a fine one that breaks up the gradients. Both reuse the single cached tile —
 * per-pixel noise per avatar would be the most expensive thing on the page.
 */
function paintGrain(ctx: Ctx, rng: Rng, s: number): void {
  const tile = getGrainTile();
  if (!tile) return;
  const pattern = ctx.createPattern(tile, "repeat");
  if (!pattern) return;
  const passes: [number, number, GlobalCompositeOperation][] = [
    [1, rng.range(0.3, 0.5), "overlay"],
    [rng.range(2, 3.5), rng.range(0.12, 0.24), "soft-light"],
  ];
  for (const [scale, alpha, mode] of passes) {
    ctx.save();
    ctx.globalCompositeOperation = mode;
    ctx.globalAlpha = alpha;
    ctx.scale(scale, scale);
    ctx.translate(rng.range(-64, 0), rng.range(-64, 0));
    ctx.fillStyle = pattern;
    ctx.fillRect(0, 0, s / scale + 128, s / scale + 128);
    ctx.restore();
  }
}

function paintVignette(ctx: Ctx, p: Palette, s: number): void {
  const grad = ctx.createRadialGradient(s / 2, s / 2, s * 0.34, s / 2, s / 2, s * 0.78);
  grad.addColorStop(0, "rgba(0,0,0,0)");
  // A pale ground darkened at the edges just looks dirty; lift it instead.
  grad.addColorStop(1, p.light ? "rgba(255,255,255,0.22)" : "rgba(0,0,0,0.26)");
  ctx.fillStyle = grad;
  ctx.fillRect(0, 0, s, s);
}

function render(seed: string): string {
  const canvas = getScratch();
  const ctx = canvas?.getContext("2d");
  if (!canvas || !ctx) return "";

  const s = RENDER_SIZE;
  const rng = makeRng(seed);
  const palette = buildPalette(rng);

  ctx.setTransform(1, 0, 0, 1, 0, 0);
  ctx.globalCompositeOperation = "source-over";
  ctx.globalAlpha = 1;
  ctx.clearRect(0, 0, s, s);

  paintBackground(ctx, rng, palette, s);

  // A soft off-centre glow under the composition gives the flat shapes some depth.
  // Kept weak: turned up, it swallows whatever is drawn on top of it.
  const glow = ctx.createRadialGradient(
    s * rng.range(0.15, 0.85), s * rng.range(0.15, 0.85), 0,
    s / 2, s / 2, s * rng.range(0.6, 1),
  );
  glow.addColorStop(0, css(palette.glow, rng.range(0.16, 0.3)));
  glow.addColorStop(1, css(palette.glow, 0));
  ctx.fillStyle = glow;
  ctx.fillRect(0, 0, s, s);

  ctx.save();
  if (rng.chance(0.35)) {
    // Whole-composition rotation, clipped back to the square.
    ctx.beginPath();
    ctx.rect(0, 0, s, s);
    ctx.clip();
    ctx.translate(s / 2, s / 2);
    ctx.rotate(rng.range(-0.5, 0.5));
    ctx.translate(-s / 2, -s / 2);
  }
  // Blend modes are picked per ground. "screen" and "soft-light" over a pale
  // ground erase the composition entirely, which is how the first version ended
  // up with blank tiles.
  ctx.globalCompositeOperation = palette.light
    ? rng.pick(["source-over", "source-over", "source-over", "multiply"])
    : rng.pick(["source-over", "source-over", "source-over", "screen", "overlay"]);
  rng.pick(COMPOSITIONS)(ctx, rng, palette, s);
  ctx.restore();

  ctx.globalCompositeOperation = "source-over";
  paintFilaments(ctx, rng, palette, s);
  paintGrain(ctx, rng, s);
  paintVignette(ctx, palette, s);

  // WebP holds grain at a fraction of PNG's size, and these are never transparent.
  const webp = canvas.toDataURL("image/webp", 0.86);
  return webp.startsWith("data:image/webp") ? webp : canvas.toDataURL("image/jpeg", 0.88);
}

/**
 * Data URL for the avatar belonging to `seed`. Cached: the same seed re-renders on
 * every list update otherwise.
 */
export function generatedAvatar(seed: string): string {
  const cached = cache.get(seed);
  if (cached !== undefined) return cached;
  const url = render(seed);
  if (url) {
    if (cache.size >= CACHE_LIMIT) {
      // Oldest insertion first — Map preserves insertion order.
      const oldest = cache.keys().next();
      if (!oldest.done) cache.delete(oldest.value);
    }
    cache.set(seed, url);
  }
  return url;
}

/**
 * Builds the seed and renders. `accountKey` is the account's login name or platform
 * ID; `salt` is the machine-local value from the backend, without which the same
 * account would look identical on every computer.
 */
export function accountAvatar(salt: string, platformKey: string, accountKey: string): string {
  const key = `${salt} ${platformKey} ${accountKey}`.trim();
  if (!accountKey.trim()) return "";
  return generatedAvatar(key);
}

/** Exposed for tests and for the CSS preview page. */
export function resetGeneratedAvatarCache(): void {
  cache.clear();
}
