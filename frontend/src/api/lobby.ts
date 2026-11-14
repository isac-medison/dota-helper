import type { LobbyPoint, LobbyStep } from "../../bindings/dota-helper/models/models";
import { Cancel, Capture, ResolveIds, Run } from "../../bindings/dota-helper/services/lobbyscan";

export type { LobbyPoint, LobbyStep };

export const STEP_KINDS = [
  { value: "click", label: "Клік" },
  { value: "rightClick", label: "Правий клік" },
  { value: "doubleClick", label: "Подвійний клік" },
  { value: "keys", label: "Клавіші" },
  { value: "wait", label: "Пауза" },
] as const;

export const isClick = (step: LobbyStep) => step.kind !== "keys" && step.kind !== "wait";

// Where to click, recorded once by hand: it depends on the screen and on the game's UI.
export type Calibration = {
  // The ten players in the top bar, left team first. Empty until recorded.
  players: LobbyPoint[];
  // What to do for each player, starting with the click on the player.
  steps: LobbyStep[];
  // The player the steps were recorded on: relative steps are offsets from this point.
  origin: LobbyPoint | null;
  // The corner of the other window (a Steam one) when the steps in it were recorded: window steps are offsets from it.
  windowOrigin: LobbyPoint | null;
  // How long the cursor rests on a point before each click, in milliseconds.
  hoverMs: number;
};

const KEY = "lobbyScan";
export const DEFAULT_HOVER = 250;
const EMPTY: Calibration = { players: [], steps: [], origin: null, windowOrigin: null, hoverMs: DEFAULT_HOVER };

// Storage can be blocked; then the calibration lasts until the page reloads.
export function loadCalibration(): Calibration {
  try {
    const saved = JSON.parse(localStorage.getItem(KEY) ?? "null");
    if (saved && Array.isArray(saved.players) && Array.isArray(saved.steps)) return { ...EMPTY, ...saved };
  } catch {
    // start over
  }
  return EMPTY;
}

export function saveCalibration(calibration: Calibration) {
  try {
    localStorage.setItem(KEY, JSON.stringify(calibration));
  } catch {
    // not critical
  }
}

// Five players sit evenly spaced in each half of the top bar, so the ends of a team give the rest.
export function spread(first: LobbyPoint, last: LobbyPoint): LobbyPoint[] {
  return Array.from({ length: 5 }, (_, i) => ({
    x: Math.round(first.x + ((last.x - first.x) * i) / 4),
    y: Math.round(first.y + ((last.y - first.y) * i) / 4),
  }));
}

// A point recorded for the flow, whether it takes a right click, and the corner of the window active then.
export type RecordedClick = LobbyPoint & { right: boolean; window: LobbyPoint | null };

// What a click step's numbers are counted from: the screen, the player's point, or the active window's corner.
export type Anchor = "screen" | "player" | "window";
export const ANCHORS: { value: Anchor; label: string; hint: string }[] = [
  { value: "screen", label: "точка на екрані", hint: "однаково для всіх" },
  { value: "player", label: "біля гравця", hint: "біля гравця" },
  { value: "window", label: "у активному вікні", hint: "рухається з вікном" },
];

export const anchorOf = (step: LobbyStep): Anchor => (step.window ? "window" : step.relative ? "player" : "screen");

type Origins = { origin: LobbyPoint | null; windowOrigin: LobbyPoint | null };
const ZERO: LobbyPoint = { x: 0, y: 0 };
const baseOf = (anchor: Anchor, { origin, windowOrigin }: Origins) =>
  (anchor === "player" ? origin : anchor === "window" ? windowOrigin : null) ?? ZERO;

// The same place on the screen (as it was when recording), counted from another anchor.
export function withAnchor(step: LobbyStep, to: Anchor, origins: Origins): LobbyStep {
  const from = baseOf(anchorOf(step), origins);
  const base = baseOf(to, origins);
  return { ...step, relative: to === "player", window: to === "window", x: step.x + from.x - base.x, y: step.y + from.y - base.y };
}

// The window the later clicks were recorded in, when it is another one than the first click was in (the game).
export function otherWindow(points: RecordedClick[]): LobbyPoint | null {
  const home = points[0]?.window;
  const other = points.findLast((p) => p.window && home && (p.window.x !== home.x || p.window.y !== home.y));
  return other?.window ?? null;
}

// What opens next to a player (a menu under the cursor) is close to them and moves with them;
// a window that opens in the same place for everyone is further away.
const NEAR_PLAYER = 350;

// The first recorded click is the player. Later ones made in another window than the game become
// offsets from that window's corner, so they follow it when it is moved; ones near the player become
// offsets from the player; the rest stay fixed points on the screen. All can be switched by hand afterwards.
export function stepsFrom(points: RecordedClick[]): LobbyStep[] {
  const origin = points[0];
  const home = origin.window;
  return points.map((p, i) => {
    const inWindow = p.window !== null && home !== null && (p.window.x !== home.x || p.window.y !== home.y);
    const relative = !inWindow && Math.hypot(p.x - origin.x, p.y - origin.y) <= NEAR_PLAYER;
    const base = inWindow ? p.window! : relative ? origin : ZERO;
    return {
      kind: p.right ? "rightClick" : "click",
      x: p.x - base.x,
      y: p.y - base.y,
      relative,
      window: inWindow,
      keys: "",
      // Starting pauses; the click that opens the profile window usually needs a longer one, set by hand.
      delayMs: i === 0 ? 500 : relative ? 700 : 600,
    };
  });
}

// Wails rejects with the Go error; its text is what the user needs to see.
export function errorText(e: unknown) {
  return e instanceof Error ? e.message : String(e);
}

// Waits for F8 (a point), F7 (a point for a right click) or F9 (done) pressed anywhere, the game included.
export async function capture(): Promise<RecordedClick | null> {
  const c = await Capture();
  return c && c.key !== "done" ? { x: c.x, y: c.y, right: c.key === "right", window: c.window ?? null } : null;
}

// Clicks through the players and returns the ID found for each (null = none). Starts on F8 in the game.
export async function scanPlayers(players: LobbyPoint[], steps: LobbyStep[], hoverMs: number) {
  const result = await Run({ players, steps, noHotkey: false, dryRun: false, hoverMs });
  const raw = result?.raw ?? [];
  const ids = (await ResolveIds(raw)) ?? [];
  return {
    aborted: result?.aborted ?? false,
    raw,
    ids: players.map((_, i) => ids[i] || null),
  };
}

// Moves the cursor over every point the clicks would land on, without clicking. Starts on F8 in the game.
// Nothing opens during it, so window steps are shown where the window was when they were recorded.
export async function showPoints(players: LobbyPoint[], steps: LobbyStep[], windowOrigin: LobbyPoint | null) {
  const fixed = steps.map((s) => (s.window ? withAnchor(s, "screen", { origin: null, windowOrigin }) : s));
  const result = await Run({ players, steps: fixed, noHotkey: false, dryRun: true, hoverMs: 0 });
  return result?.aborted ?? false;
}

// Where a click step lands on the screen for a given player (a window step: with the window where it was recorded).
export function stepPoint(step: LobbyStep, player: LobbyPoint, windowOrigin: LobbyPoint | null): LobbyPoint {
  const base = baseOf(anchorOf(step), { origin: player, windowOrigin });
  return { x: base.x + step.x, y: base.y + step.y };
}

// The Steam ID behind a profile link with a custom name (steamcommunity.com/id/…), asked from Steam; null = none.
export async function resolveId(text: string) {
  const [id] = (await ResolveIds([text])) ?? [];
  return id || null;
}

export const cancelScan =() => Cancel();
