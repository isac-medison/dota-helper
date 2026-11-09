import type { Hero } from "../api/heroes";

// Short names players use that the initials of the name don't give (one-word names, old names), or that
// mean this hero when two share the initials.
const NICKNAMES: Record<number, string[]> = {
  4: ["bs"], // Bloodseeker
  7: ["es"], // Earthshaker
  9: ["potm"], // Mirana
  21: ["wr", "windrunner"], // Windranger
  38: ["bm"], // Beastmaster
  41: ["void"], // Faceless Void (before Void Spirit)
  53: ["furion"], // Nature's Prophet
  54: ["ls", "naix"], // Lifestealer
  60: ["ns"], // Night Stalker (before Naga Siren)
  76: ["od"], // Outworld Devourer
  91: ["wisp"], // Io
  96: ["cent"], // Centaur Warrunner
  99: ["bb"], // Bristleback
  108: ["ul"], // Underlord
  109: ["tb"], // Terrorblade
  135: ["db"], // Dawnbreaker
};

const letters = (s: string) => s.toLowerCase().replace(/[^a-z]/g, "");
const initials = (name: string) =>
  name
    .split(/[\s-]+/)
    .map((word) => letters(word)[0] ?? "")
    .join("");

// Heroes the query names, best first: a nickname, the initials (qop, pa), the start of the name, then anywhere
// in it (spaces and punctuation don't count: "antimage" finds Anti-Mage).
export function searchHeroes(heroes: Hero[], query: string): Hero[] {
  const q = letters(query);
  if (!q) return query.trim() ? [] : heroes;
  const rank = (h: Hero) => {
    const name = letters(h.localizedName);
    const short = initials(h.localizedName);
    if (NICKNAMES[h.id]?.includes(q)) return 0;
    if (short === q) return 1;
    if (name.startsWith(q)) return 2;
    if (name.includes(q) || short.startsWith(q)) return 3;
    return 4;
  };
  return heroes
    .map((hero) => ({ hero, rank: rank(hero) }))
    .filter((x) => x.rank < 4)
    .sort((a, b) => a.rank - b.rank)
    .map((x) => x.hero);
}
