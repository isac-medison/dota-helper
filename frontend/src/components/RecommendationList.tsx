import type { Hero } from "../api/heroes";
import type { Recommendation } from "../api/recommendations";
import "./RecommendationList.css";

type Props = {
  items: Recommendation[];
  heroesById: Map<number, Hero>;
  onPick: (heroId: number) => void;
  // Hero → the free position it's scored on, when positions aren't set (always for the enemy).
  positions?: Map<number, number>;
  // The enemy's picks (bans): scores are then the enemy's gain.
  forEnemy?: boolean;
};

const COMPONENTS = [
  { key: "counter", label: "Контр", title: "Проти ворогів" },
  { key: "synergy", label: "Синергія", title: "З союзниками" },
  { key: "meta", label: "Мета", title: "Вінрейт на позиції в цьому бракеті" },
  { key: "lineup", label: "Склад", title: "Чи лягають герої команди на різні позиції" },
  { key: "lane", label: "Лінія", title: "Проти ворогів на твоїй лінії (потрібна позиція)" },
  { key: "partner", label: "Пара", title: "З союзником на твоїй лінії: Pos 1 + 5, Pos 3 + 4 (потрібна позиція)" },
  { key: "comfort", label: "Комфорт", title: "Твої ігри на герої" },
] as const;

// Smaller per-hero effects are noise next to the totals above.
const MIN_REASON_POINTS = 0.3;
const MAX_REASONS = 3;

function signed(value: number) {
  return `${value >= 0 ? "+" : "−"}${Math.abs(value).toFixed(1)}`;
}

// The biggest single-hero effects behind Counter, Synergy, Lane and Partner: "проти Medusa +3.1 · з Lion −0.8".
function reasons(r: Recommendation, heroesById: Map<number, Hero>) {
  return [
    ...(r.vsEnemies ?? []).map((i) => ({ ...i, prefix: "проти" })),
    ...(r.withAllies ?? []).map((i) => ({ ...i, prefix: "з" })),
    ...(r.inLane ?? []).map((i) => ({ ...i, prefix: "на лінії проти" })),
    ...(r.lanePartners ?? []).map((i) => ({ ...i, prefix: "в парі з" })),
  ]
    .filter((i) => Math.abs(i.points) >= MIN_REASON_POINTS)
    .sort((a, b) => Math.abs(b.points) - Math.abs(a.points))
    .slice(0, MAX_REASONS)
    .map((i) => ({
      key: `${i.prefix}-${i.heroId}`,
      text: `${i.prefix} ${heroesById.get(i.heroId)?.localizedName ?? `#${i.heroId}`} ${signed(i.points)}`,
      sign: Math.sign(i.points),
    }));
}

export function RecommendationList({ items, heroesById, onPick, positions, forEnemy }: Props) {
  const components = forEnemy ? COMPONENTS.filter((c) => c.key !== "comfort") : COMPONENTS;
  
  return (
    <ol className="recommendations">
      {/* ADD THE ?? [] FALLBACK HERE */}
      {(items ?? []).map((r) => {
        const hero = heroesById.get(r.heroId);
        const why = reasons(r, heroesById);
        const risks = forEnemy ? [] : (r.risks ?? []).filter((i) => i.points >= MIN_REASON_POINTS);
        const position = positions?.get(r.heroId);
        
        return (
          <li key={r.heroId}>
            <button type="button" onClick={() => onPick(r.heroId)} title={forEnemy ? "Забанити / взяти" : "Пікнути"}>
              {hero && <img src={hero.iconUrl} alt="" />}
              <span className="recommendations__name">{hero?.localizedName ?? `#${r.heroId}`}</span>
              {forEnemy ? (
                <strong
                  className="recommendations__score"
                  title={`На скільки п.п. цей пік підняв би шанс ворога (≈${Math.round(r.winChance * 100)}% з ним)`}
                >
                  Pos {position} · загроза {signed(r.score)}
                </strong>
              ) : (
                <strong
                  className="recommendations__score"
                  title={`Оцінка шансу на перемогу з цим піком (ще не відомі герої рахуються нейтральними). Внесок піку: ${signed(r.score)} п.п.`}
                >
                  {position && `Pos ${position} · `}≈{Math.round(r.winChance * 100)}%
                </strong>
              )}
              <span className="recommendations__parts">
                {components.map((c) => (
                  <span key={c.key} title={c.title} data-sign={Math.sign(Math.round(r[c.key] * 10))}>
                    {c.label} {signed(r[c.key])}
                  </span>
                ))}
              </span>
              {why.length > 0 && (
                <span className="recommendations__why">
                  {why.map((w) => (
                    <span key={w.key} data-sign={w.sign}>
                      {w.text}
                    </span>
                  ))}
                </span>
              )}
              {risks.length > 0 && (
                <span
                  className="recommendations__risk"
                  title="Найсильніші контрпіки, які ворог ще може взяти на вільну позицію. На оцінку не впливають."
                >
                  ризик:
                  {risks.map((i) => {
                    const counter = heroesById.get(i.heroId);
                    const name = counter?.localizedName ?? `#${i.heroId}`;
                    return (
                      <span key={i.heroId} title={`${name} ${signed(-i.points)}`}>
                        {counter && <img src={counter.iconUrl} alt={name} />}
                        {signed(-i.points)}
                      </span>
                    );
                  })}
                </span>
              )}
            </button>
          </li>
        );
      })}
    </ol>
  );
}
