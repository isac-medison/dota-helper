import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { components } from "./schema";

// Import Wails bindings for the Players service
import { Get } from "../../bindings/dota-helper/services/players";

export type Player = components["schemas"]["PlayerDto"];

const MEDALS = ["Herald", "Guardian", "Crusader", "Archon", "Legend", "Ancient", "Divine", "Immortal"];

// Rank tier is medal * 10 + stars: 54 = Legend 4, 80 = Immortal.
export function rankName(rankTier: number | null) {
  const medal = rankTier ? MEDALS[Math.floor(rankTier / 10) - 1] : undefined;
  if (!medal) return "Без рангу";
  const stars = rankTier! % 10;
  return stars > 0 ? `${medal} ${stars}` : medal;
}

// Zero-width and filler characters (by code point): with spaces, what a nickname made to look empty consists of.
const INVISIBLE: [number, number][] = [
  [0x00ad, 0x00ad],
  [0x034f, 0x034f],
  [0x115f, 0x1160],
  [0x180e, 0x180e],
  [0x200b, 0x200f],
  [0x2028, 0x202f],
  [0x2060, 0x206f],
  [0x2800, 0x2800],
  [0x3164, 0x3164],
  [0xfe00, 0xfe0f],
  [0xfeff, 0xfeff],
  [0xffa0, 0xffa0],
];

function isInvisible(char: string) {
  const code = char.codePointAt(0)!;
  return char.trim() === "" || INVISIBLE.some(([from, to]) => code >= from && code <= to);
}

// A name there is something to read in; for an invisible one, the account ID.
export function visibleName(player: { name: string; accountId: number }) {
  return [...player.name].every(isInvisible) ? `#${player.accountId}` : player.name;
}

const STEAM_ID64_BASE = 76561197960265728n;

// The same player as a Steam ID (64-bit) and as a friend ID compare equal by their account ID.
export function accountId(steamId: string) {
  try {
    const id = BigInt(steamId);
    return String(id >= STEAM_ID64_BASE ? id - STEAM_ID64_BASE : id);
  } catch {
    return steamId;
  }
}

async function fetchPlayer(steamId: string, refresh: boolean): Promise<Player | null> {
  try {
    // Direct IPC call to Go
    const player = await Get(steamId, refresh);
    return player as unknown as Player;
  } catch (e: any) {
    // Wails passes Go errors as rejected promises (strings).
    // This replaces the old 404 check.
    if (typeof e === "string" && e.includes("not found")) {
      return null;
    }
    throw new Error(`Go Error: ${e}`);
  }
}

// null data = STRATZ doesn't know this account.
export function usePlayer(steamId: string | null) {
  return useQuery({
    queryKey: ["player", steamId],
    queryFn: () => fetchPlayer(steamId!, false),
    enabled: steamId !== null,
    // The API caches profiles for a day anyway.
    staleTime: 60 * 60 * 1000,
    retry: false,
  });
}

// Skips the API's day-long cache, e.g. right after a match.
export function useRefreshPlayer(steamId: string | null) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => fetchPlayer(steamId!, true),
    onSuccess: (player) => queryClient.setQueryData(["player", steamId], player),
  });
}

// The player's name and avatar once the profile is loaded.
export function useProfileLabel(steamId: string | null) {
  const player = usePlayer(steamId);
  const name = player.isPending
    ? "…"
    : player.data
      ? visibleName(player.data)
      : player.data === null
        ? "не знайдено"
        : "помилка профілю";
  return { name, avatarUrl: player.data?.avatarUrl };
}