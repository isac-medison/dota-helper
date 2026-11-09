import { keepPreviousData, useQuery } from "@tanstack/react-query";
import type { components } from "./schema";

// Import Wails bindings for the Recommendations service
// (Adjust method names based on what you named them in services/recommendations.go)
import { Recommend, Threats } from "../../bindings/dota-helper/services/recommendations";

export type RecommendationRequest = Omit<components["schemas"]["RecommendationRequest"], "steamIds"> & {
  steamIds: (string | null)[] | null;
};
export type Recommendations = components["schemas"]["RecommendationsDto"];
export type Recommendation = components["schemas"]["Recommendation"];
export type Threat = components["schemas"]["Threat"];

// Keyed by the whole request, so every draft change refetches and going back hits the cache.
// dataVersion changes when the server's data does (new meta import, refreshed profile).
export function useRecommendations(request: RecommendationRequest, dataVersion: string) {
  return useQuery({
    queryKey: ["recommendations", request, dataVersion],
    queryFn: async () => {
      try {
        // Direct IPC call to Go
        const res = await Recommend(request as any);
        return res as unknown as Recommendations;
      } catch (e: any) {
        // Map Go errors to UI warnings
        if (typeof e === "string" && (e.includes("STRATZ") || e.includes("meta"))) {
          throw new Error("Мета STRATZ ще завантажується");
        }
        throw new Error(`Go Error: ${e}`);
      }
    },
    // Keep the old list on screen while the next one loads instead of flashing empty.
    placeholderData: keepPreviousData,
  });
}

// The enemy's best next picks: who to ban. Only fetched while banning or picking for the enemy.
export function useThreats(request: RecommendationRequest, dataVersion: string, enabled: boolean) {
  // Not for one of my positions, so switching my slot doesn't refetch.
  const forEnemy = { ...request, myPosition: null, openPositions: false };
  
  return useQuery({
    queryKey: ["threats", forEnemy, dataVersion],
    queryFn: async () => {
      try {
        // Direct IPC call to Go
        const res = await Threats(forEnemy as any);
        return res as unknown as Threat[];
      } catch (e: any) {
        throw new Error(`Go Error: ${e}`);
      }
    },
    enabled,
    placeholderData: keepPreviousData,
  });
}