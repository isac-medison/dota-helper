import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { components } from "./schema";

// Import Wails bindings for the Players service
import { Get } from "../../bindings/dota-helper/services/players";

export type Player = components["schemas"]["PlayerDto"];

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
      ? player.data.name
      : player.data === null
        ? "не знайдено"
        : "помилка профілю";
  return { name, avatarUrl: player.data?.avatarUrl };
}