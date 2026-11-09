import type { Hero } from "../api/heroes";
import type { Recommendations } from "../api/recommendations";
import { useProfileLabel } from "../api/players";
import "./SwapAdvice.css";

type Swap = NonNullable<Recommendations["swap"]>;

type Props = {
  swap: Swap;
  allies: (number | null)[];
  party: (string | null)[];
  heroesById: Map<number, Hero>;
  onApply: () => void;
};

function PlayerName({ steamId }: { steamId: string }) {
  return <>{useProfileLabel(steamId).name}</>;
}

// "Better like this": only the slots that change, with their new hero and player.
export function SwapAdvice({ swap, allies, party, heroesById, onApply }: Props) {
  const changed = (swap.allies ?? [])
    .map((heroId, i) => ({ i, heroId, steamId: swap.steamIds[i] ?? null }))
    .filter(({ i, heroId, steamId }) => heroId !== allies[i] || steamId !== party[i]);

  return (
    <section className="swap-advice">
      <h2>
        Краще переставити <small>+{swap.gain.toFixed(1)} п.п.</small>
      </h2>
      <ul>
        {changed.map(({ i, heroId, steamId }) => {
          const hero = heroId === null ? undefined : heroesById.get(heroId);
          return (
            <li key={i}>
              <strong>Pos {i + 1}:</strong> {hero ? hero.localizedName : "без героя"}
              {steamId && (
                <>
                  {" · "}
                  <PlayerName steamId={steamId} />
                </>
              )}
            </li>
          );
        })}
      </ul>
      <button type="button" onClick={onApply}>
        Переставити
      </button>
    </section>
  );
}
