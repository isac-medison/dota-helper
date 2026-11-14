import { useMemo, useState } from "react";
import type { Hero } from "../api/heroes";
import { rankName, usePlayer, visibleName } from "../api/players";
import { SteamIdInput } from "./Profiles";
import "./Lobby.css";

const TOP_HEROES = 6;

type Props = {
  heroesById: Map<number, Hero>;
  // Steam IDs of my party by ally slot, and of the enemy players in no particular order.
  allies: (string | null)[];
  enemies: (string | null)[];
  // Heroes already picked or banned.
  used: Set<number>;
  onEnemyChange: (index: number, steamId: string | null) => void;
  onBan: (heroId: number) => void;
};

// What every known player of the match plays most: the enemy's heroes are a click away from a ban.
export function Lobby({ heroesById, allies, enemies, used, onEnemyChange, onBan }: Props) {
  const [adding, setAdding] = useState(false);
  const free = enemies.indexOf(null);

  return (
    <section className="lobby">
      <h2>Гравці матчу</h2>

      <div className="lobby__team lobby__team--enemies">
        <h3>Вороги</h3>
        <ul>
          {enemies.map(
            (id, i) =>
              id !== null && (
                <PlayerRow
                  key={i}
                  steamId={id}
                  heroesById={heroesById}
                  used={used}
                  onHero={onBan}
                  heroTitle="Забанити"
                  onRemove={() => onEnemyChange(i, null)}
                />
              ),
          )}
        </ul>
        {free >= 0 &&
          (adding ? (
            <SteamIdInput
              initial=""
              className="party-member__input lobby__input"
              onSubmit={(id) => {
                if (id) onEnemyChange(free, id);
                setAdding(false);
              }}
              onCancel={() => setAdding(false)}
            />
          ) : (
            <button
              type="button"
              className="party-member"
              data-empty
              title="Steam ID або Friend ID ворожого гравця — або зберіть їх сканом гравців"
              onClick={() => setAdding(true)}
            >
              + ворог
            </button>
          ))}
      </div>

      {allies.some((id) => id !== null) && (
        <div className="lobby__team lobby__team--allies">
          <h3>Союзники</h3>
          <ul>
            {allies.map(
              (id, i) => id !== null && <PlayerRow key={i} steamId={id} heroesById={heroesById} used={used} />,
            )}
          </ul>
        </div>
      )}
    </section>
  );
}

type RowProps = {
  steamId: string;
  heroesById: Map<number, Hero>;
  used: Set<number>;
  onHero?: (heroId: number) => void;
  heroTitle?: string;
  onRemove?: () => void;
};

function PlayerRow({ steamId, heroesById, used, onHero, heroTitle, onRemove }: RowProps) {
  const player = usePlayer(steamId);

  // Over all positions: where the enemy plays a hero isn't known before they pick it.
  const top = useMemo(() => {
    const byHero = new Map<number, { heroId: number; games: number; wins: number }>();
    for (const h of player.data?.heroes ?? []) {
      const total = byHero.get(h.heroId) ?? { heroId: h.heroId, games: 0, wins: 0 };
      total.games += h.games;
      total.wins += h.wins;
      byHero.set(h.heroId, total);
    }
    return [...byHero.values()].sort((a, b) => b.games - a.games).slice(0, TOP_HEROES);
  }, [player.data]);

  const note = player.isPending
    ? "завантаження…"
    : player.error
      ? "не вдалося завантажити"
      : player.data === null
        ? "STRATZ не знає гравця"
        : top.length === 0
          ? "профіль закритий"
          : null;

  return (
    <li className="lobby-player">
      <div className="lobby-player__who" title={`Steam ID ${steamId}`}>
        {player.data?.avatarUrl && <img src={player.data.avatarUrl} alt="" />}
        <div>
          <strong>{player.data ? visibleName(player.data) : steamId}</strong>
          <small>{player.data ? rankName(player.data.rankTier) : note}</small>
        </div>
      </div>

      {player.data && note ? (
        <span className="lobby-player__note">{note}</span>
      ) : (
        <ul className="lobby-player__heroes">
          {top.map((h) => {
            const hero = heroesById.get(h.heroId);
            const content = (
              <>
                {hero && <img src={hero.iconUrl} alt={hero.localizedName} />}
                <span>
                  {h.games} · {Math.round((h.wins / h.games) * 100)}%
                </span>
              </>
            );
            return (
              <li key={h.heroId} title={hero?.localizedName}>
                {onHero ? (
                  <button
                    type="button"
                    disabled={used.has(h.heroId)}
                    title={`${heroTitle}: ${hero?.localizedName ?? h.heroId}`}
                    onClick={() => onHero(h.heroId)}
                  >
                    {content}
                  </button>
                ) : (
                  <div data-used={used.has(h.heroId)}>{content}</div>
                )}
              </li>
            );
          })}
        </ul>
      )}

      {onRemove && (
        <button type="button" className="lobby-player__remove" aria-label="Прибрати гравця" onClick={onRemove}>
          ×
        </button>
      )}
    </li>
  );
}
