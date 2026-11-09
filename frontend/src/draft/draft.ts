export const TEAM_SIZE = 5;

// Drag & drop payloads: a hero from the list, a profile's Steam ID, and the ally slot a dragged player sits in.
export const DRAG_HERO = "application/x-dota-hero";
export const DRAG_PROFILE = "application/x-steam-id";
export const DRAG_PARTY_SLOT = "application/x-party-slot";

export type Position = 1 | 2 | 3 | 4 | 5;
export type Team = "allies" | "enemies";
export type Slot = { team: Team; index: number };
export type Target = Slot | { team: "bans" };

// Slot index = position − 1: allies[0] is Pos 1 (carry), allies[4] is Pos 5 (hard support).
export type Draft = {
  allies: (number | null)[];
  enemies: (number | null)[];
  bans: number[];
};

// One team as a screenshot showed it. complete = every portrait was recognized, so a hero that isn't there left.
export type ScannedTeam = { heroes: number[]; complete: boolean };

export type DraftState = {
  draft: Draft;
  target: Target;
  // The ally slot recommendations are for: the last ally slot the target was on.
  allyIndex: number;
  // Captains Mode: the target follows the pick order. first = we pick first.
  cm: { first: boolean } | null;
};

// Captains Mode order since 7.34 (24 steps): f/s = ban by the team picking first/second, F/S = their pick.
// ponytail: hard-coded; check it against the game when Valve changes CM.
const CM_ORDER = "ffssfss" + "FS" + "ffs" + "SFFSSF" + "fssf" + "FS";
export const CM_STEPS = CM_ORDER.length;

export type CmTurn = { step: number; ours: boolean; ban: boolean };

// Whose turn it is, counted from the bans and picks made so far; null when not in CM or the draft is done.
export function cmTurn({ draft, cm }: DraftState): CmTurn | null {
  const step = draft.bans.length + [...draft.allies, ...draft.enemies].filter((id) => id !== null).length;
  if (!cm || step >= CM_STEPS) return null;
  const turn = CM_ORDER[step];
  return { step, ours: (turn.toLowerCase() === "f") === cm.first, ban: turn === turn.toLowerCase() };
}

export type DraftAction =
  | { type: "target"; target: Target }
  | { type: "pick"; heroId: number }
  | { type: "clear"; team: Team; index: number }
  | { type: "move"; from: Slot; to: Slot }
  | { type: "arrange"; allies: (number | null)[] }
  | { type: "scan"; allies: ScannedTeam; enemies: ScannedTeam }
  | { type: "unban"; heroId: number }
  | { type: "cm"; cm: DraftState["cm"] }
  | { type: "reset" };

export const initialDraftState: DraftState = {
  draft: {
    allies: Array(TEAM_SIZE).fill(null),
    enemies: Array(TEAM_SIZE).fill(null),
    bans: [],
  },
  target: { team: "allies", index: 0 },
  allyIndex: 0,
  cm: null,
};

// After these the target jumps to the next CM turn; picking a slot by hand or swapping heroes doesn't move it.
const CM_FOLLOWS: DraftAction["type"][] = ["pick", "clear", "unban", "scan", "cm", "reset"];

export function usedHeroIds(draft: Draft): Set<number> {
  return new Set(
    [...draft.allies, ...draft.enemies, ...draft.bans].filter(
      (id): id is number => id !== null,
    ),
  );
}

export function draftReducer(state: DraftState, action: DraftAction): DraftState {
  let next = reduce(state, action);
  if (next.cm && CM_FOLLOWS.includes(action.type)) next = { ...next, target: cmTarget(next) ?? next.target };
  return next.target.team === "allies" && next.target.index !== next.allyIndex
    ? { ...next, allyIndex: next.target.index }
    : next;
}

function reduce(state: DraftState, action: DraftAction): DraftState {
  const { draft, target } = state;

  switch (action.type) {
    case "target":
      return { ...state, target: action.target };

    case "pick": {
      if (usedHeroIds(draft).has(action.heroId)) return state;

      if (target.team === "bans") {
        return {
          ...state,
          draft: { ...draft, bans: [...draft.bans, action.heroId] },
        };
      }

      const next = {
        ...draft,
        [target.team]: draft[target.team].with(target.index, action.heroId),
      };
      return { ...state, draft: next, target: emptySlotTarget(next, target.team, target.index) };
    }

    case "clear":
      return {
        ...state,
        draft: {
          ...draft,
          [action.team]: draft[action.team].with(action.index, null),
        },
        target: { team: action.team, index: action.index },
      };

    // Swap two slots (the target may be empty): reassigns positions, or fixes a hero put on the wrong team.
    case "move": {
      const { from, to } = action;
      const hero = draft[from.team][from.index];
      const swapped = { ...draft, [from.team]: draft[from.team].with(from.index, draft[to.team][to.index]) };
      return { ...state, draft: { ...swapped, [to.team]: swapped[to.team].with(to.index, hero) } };
    }

    // My team's heroes put on new slots at once (a suggested swap).
    case "arrange":
      return { ...state, draft: { ...draft, allies: action.allies } };

    // Heroes already on their team stay in their slots (positions set by hand survive a rescan),
    // new ones go into empty slots, and a hero the screenshot shows elsewhere leaves where it was.
    case "scan": {
      const seen = new Set([...action.allies.heroes, ...action.enemies.heroes]);
      const merge = (slots: (number | null)[], scanned: ScannedTeam) => {
        const kept = slots.map((id) =>
          id !== null && (scanned.heroes.includes(id) || (!scanned.complete && !seen.has(id))) ? id : null,
        );
        for (const id of scanned.heroes.filter((h) => !kept.includes(h))) {
          const empty = kept.indexOf(null);
          if (empty >= 0) kept[empty] = id;
        }
        return kept;
      };
      return {
        ...state,
        draft: {
          allies: merge(draft.allies, action.allies),
          enemies: merge(draft.enemies, action.enemies),
          bans: draft.bans.filter((id) => !seen.has(id)),
        },
      };
    }

    case "unban":
      return {
        ...state,
        draft: { ...draft, bans: draft.bans.filter((id) => id !== action.heroId) },
      };

    case "cm":
      return { ...state, cm: action.cm };

    case "reset":
      return state.cm ? { ...initialDraftState, cm: state.cm } : initialDraftState;
  }
}

// Our pick goes to the slot recommendations are for if it's empty, theirs to their first empty slot.
function cmTarget(state: DraftState): Target | null {
  const turn = cmTurn(state);
  if (!turn) return null;
  if (turn.ban) return { team: "bans" };
  if (!turn.ours) return emptySlotTarget(state.draft, "enemies");
  return state.draft.allies[state.allyIndex] === null
    ? { team: "allies", index: state.allyIndex }
    : emptySlotTarget(state.draft, "allies");
}

// First empty slot of the team after `after` (wrapping around). If the team is full,
// stays on `after` (or slot 0) so the next pick replaces that hero.
export function emptySlotTarget(draft: Draft, team: Team, after = -1): Target {
  const slots = draft[team];
  for (let step = 1; step <= slots.length; step++) {
    const index = (after + step) % slots.length;
    if (slots[index] === null) return { team, index };
  }
  return { team, index: Math.max(after, 0) };
}
