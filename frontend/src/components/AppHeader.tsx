import { useEffect, useState } from "react";
import { useMetaStatus, useRefreshMeta } from "../api/meta";
import { useRefreshPlayer, type Player } from "../api/players";
import "./AppHeader.css";

const relative = new Intl.RelativeTimeFormat("uk", { numeric: "auto" });

function timeAgo(iso: string, now: number) {
  const minutes = Math.round((new Date(iso).getTime() - now) / 60_000);
  if (minutes > -1) return "щойно";
  if (minutes > -60) return relative.format(minutes, "minute");
  if (minutes > -60 * 24) return relative.format(Math.round(minutes / 60), "hour");
  return relative.format(Math.round(minutes / 60 / 24), "day");
}

// Re-renders every minute so "5 хвилин тому" doesn't freeze.
function useNow() {
  const [now, setNow] = useState(Date.now);
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), 60_000);
    return () => clearInterval(id);
  }, []);
  return now;
}

type Props = {
  player: Player | null | undefined;
  steamId: string | null;
};

export function AppHeader({ player, steamId }: Props) {
  const now = useNow();
  const { data: meta } = useMetaStatus();
  const refreshMeta = useRefreshMeta();
  const refreshPlayer = useRefreshPlayer(steamId);
  const importing = meta?.importing || refreshMeta.isPending;
  const cooldownUntil = meta?.manualRefreshAvailableAtUtc ? new Date(meta.manualRefreshAvailableAtUtc) : null;
  const onCooldown = cooldownUntil !== null && cooldownUntil.getTime() > now;

  return (
    <header className="app-header">
      <h1>DotaPickAnalyzer</h1>

      <dl className="freshness">
        <div>
          <dt>Мета STRATZ</dt>
          <dd>
            {meta?.updatedAtUtc ? (
              <span title={new Date(meta.updatedAtUtc).toLocaleString()}>
                {timeAgo(meta.updatedAtUtc, now)} · {meta.matchupCount.toLocaleString()} матчапів ·
                авто раз на добу
              </span>
            ) : (
              <span>ще не завантажена</span>
            )}
            {meta?.refreshEnabled && (
              <button
                type="button"
                disabled={importing || onCooldown}
                onClick={() => refreshMeta.mutate()}
                title={
                  onCooldown
                    ? `Щойно оновлювали, знову можна о ${cooldownUntil.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}`
                    : undefined
                }
              >
                {importing ? "Оновлюється… (до кількох хвилин)" : "Оновити"}
              </button>
            )}
            {meta && !meta.refreshEnabled && <span>оновлення вимкнене: немає Stratz:Token</span>}
            {meta?.lastImportFailed && !importing && (
              <span className="freshness__error">минуле оновлення не вдалося (деталі в лозі API)</span>
            )}
          </dd>
        </div>

        {player && (
          <div>
            <dt>Профіль</dt>
            <dd>
              <span title={new Date(player.fetchedAtUtc).toLocaleString()}>
                {player.name} · {timeAgo(player.fetchedAtUtc, now)}
              </span>
              <button
                type="button"
                disabled={refreshPlayer.isPending}
                onClick={() => refreshPlayer.mutate()}
                title="Підтягнути свіжі матчі зі STRATZ"
              >
                {refreshPlayer.isPending ? "Оновлюється…" : "Оновити"}
              </button>
              {refreshPlayer.error && <span className="freshness__error">не вдалося</span>}
            </dd>
          </div>
        )}
      </dl>
    </header>
  );
}
