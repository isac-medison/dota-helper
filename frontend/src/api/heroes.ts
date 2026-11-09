import { useQuery } from "@tanstack/react-query";
import type { components } from "./schema";

// Import Wails bindings for the Heroes service
import { GetHeroes } from "../../bindings/dota-helper/services/heroes";

export type Hero = components["schemas"]["HeroDto"];

export function useHeroes() {
  return useQuery({
    queryKey: ["heroes"],
    queryFn: async (): Promise<Hero[]> => {
      // Direct IPC call to Go
      const heroes = await GetHeroes();
      return heroes as unknown as Hero[];
    },
    staleTime: Infinity,
  });
}