import { useState, type Ref } from "react";
import type { Hero } from "../api/heroes";
import { DRAG_HERO } from "../draft/draft";
import { searchHeroes } from "../draft/search";
import "./HeroPicker.css";

type Props = {
  heroes: Hero[];
  used: Set<number>;
  onPick: (heroId: number) => void;
  searchRef?: Ref<HTMLInputElement>;
};

export function HeroPicker({ heroes, used, onPick, searchRef }: Props) {
  const [query, setQuery] = useState("");

  const visible = searchHeroes(heroes, query);

  function pick(heroId: number) {
    onPick(heroId);
    setQuery("");
  }

  return (
    <section className="hero-picker">
      <input
        ref={searchRef}
        type="search"
        className="hero-search"
        placeholder="Пошук героя… (Enter — вибрати першого)"
        aria-label="Пошук героя"
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        onKeyDown={(e) => {
          if (e.key !== "Enter") return;
          const first = visible.find((h) => !used.has(h.id));
          if (first) pick(first.id);
        }}
        autoFocus
      />
      <ul className="hero-grid">
        {visible.map((hero) => (
          <li key={hero.id}>
            <button
              type="button"
              disabled={used.has(hero.id)}
              title={`${hero.roles.join(", ")} · клік або перетягни на слот`}
              onClick={() => pick(hero.id)}
              draggable={!used.has(hero.id)}
              onDragStart={(e) => {
                e.dataTransfer.setData(DRAG_HERO, String(hero.id));
                e.dataTransfer.effectAllowed = "copy";
              }}
            >
              <img src={hero.iconUrl} alt="" loading="lazy" draggable={false} />
              <span>{hero.localizedName}</span>
            </button>
          </li>
        ))}
      </ul>
    </section>
  );
}
