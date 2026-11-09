import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { components } from "./schema";

// Import Wails bindings for the Meta service
import { GetStatus, RequestRefresh } from "../../bindings/dota-helper/services/meta";

export type MetaStatus = components["schemas"]["MetaStatusDto"];

export function useMetaStatus() {
  return useQuery({
    queryKey: ["meta-status"],
    queryFn: async (): Promise<MetaStatus> => {
      // Direct IPC call to Go
      const res = await GetStatus();
      return res as unknown as MetaStatus;
    },
    // Poll while an import runs (or before the first one lands) so the page notices when it's done.
    refetchInterval: (query) => {
      const data = query.state.data;
      if (data?.importing) return 3_000;
      return data?.updatedAtUtc ? false : 30_000;
    },
  });
}

export function useRefreshMeta() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async () => {
      try {
        // Direct IPC call to Go
        await RequestRefresh();
      } catch (e: any) {
        // Go returns errors as strings. 
        // If your Go code throws a specific error for "already importing", catch it here.
        // Otherwise, Wails handles it cleanly.
        if (typeof e === "string" && (e.includes("already") || e.includes("importing"))) {
          return; // Equivalent to ignoring 409
        }
        throw e;
      }
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: ["meta-status"] }),
  });
}