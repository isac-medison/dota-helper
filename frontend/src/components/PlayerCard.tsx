import type { Hero } from "../api/heroes";
import type { Player } from "../api/players";
import type { Position } from "../draft/draft";
import "./PlayerCard.css";

const MEDALS = ["Herald", "Guardian", "Crusader", "Archon", "Legend", "Ancient", "Divine", "Immortal"];
const TOP_HEROES = 6;

type Props = {
  player: Player;
  heroesById: Map<number, Hero>;
  myPosition: Position | null;
};

// Rank tier is medal * 10 + stars: 54 = Legend 4, 80 = Immortal.
function rankName(rankTier: number | null) {
  const medal = rankTier ? MEDALS[Math.floor(rankTier / 10) - 1] : undefined;
  if (!medal) return "Без рангу";
  const stars = rankTier! % 10;
  return stars > 0 ? `${medal} ${stars}` : medal;
}

export function PlayerCard({ player, heroesById, myPosition }: Props) {
  // Heroes arrive sorted by games.
  const top = player.heroes.filter((h) => h.position === myPosition).slice(0, TOP_HEROES);

  return (
    <section className="player-card">
      <div className="player-card__head">
        {player.avatarUrl && <img src={player.avatarUrl} alt="" />}
        <div>
          <strong>{player.name}</strong>
          <small>{rankName(player.rankTier)}</small>
        </div>
      </div>

      {player.heroes.length === 0 ? (
        <p className="player-card__hint">
          Немає матчів: профіль закритий або давно не грав. Увімкни «Expose Public Match Data» в
          налаштуваннях Dota 2 — рекомендації будуть без comfort.
        </p>
      ) : myPosition === null ? (
        <p className="player-card__hint">Обери позицію, щоб побачити свої герої на ній.</p>
      ) : (
        <div>
          <small>Топ на Pos {myPosition}</small>
          {top.length === 0 ? (
            <p className="player-card__hint">На цій позиції останнім часом не грав.</p>
          ) : (
            <ul className="player-card__heroes">
              {top.map((h) => {
                const hero = heroesById.get(h.heroId);
                return (
                  <li key={`${h.heroId}-${h.position}`} title={hero?.localizedName}>
                    {hero && <img src={hero.iconUrl} alt={hero.localizedName} />}
                    <span>
                      {h.games} · {Math.round((h.wins / h.games) * 100)}%
                    </span>
                  </li>
                );
              })}
            </ul>
          )}
        </div>
      )}
    </section>
  );
}
