// Run: npm test  (plain Node, no test framework)
import { cmTurn, draftReducer, emptySlotTarget, initialDraftState, usedHeroIds } from "./draft.ts";
import { searchHeroes } from "./search.ts";

function check(condition: boolean, message: string) {
  if (!condition) throw new Error(`FAILED: ${message}`);
}

let s = initialDraftState;

s = draftReducer(s, { type: "pick", heroId: 1 });
check(s.draft.allies[0] === 1, "pick goes into the active slot");
check(s.target.team === "allies" && s.target.index === 1, "target advances to the next empty slot");

check(draftReducer(s, { type: "pick", heroId: 1 }) === s, "same hero cannot be picked twice");

s = draftReducer(s, { type: "target", target: { team: "enemies", index: 4 } });
s = draftReducer(s, { type: "pick", heroId: 2 });
check(s.draft.enemies[4] === 2, "pick into a chosen enemy slot");
check(s.target.team === "enemies" && s.target.index === 0, "target wraps around to the first empty slot");

s = draftReducer(s, { type: "target", target: { team: "bans" } });
s = draftReducer(s, { type: "pick", heroId: 3 });
s = draftReducer(s, { type: "pick", heroId: 4 });
check(s.draft.bans.join() === "3,4", "bans accumulate while ban mode is active");

s = draftReducer(s, { type: "unban", heroId: 3 });
check(s.draft.bans.join() === "4", "unban removes the hero");

s = draftReducer(s, { type: "clear", team: "allies", index: 0 });
check(s.draft.allies[0] === null, "clear empties the slot");
check(!usedHeroIds(s.draft).has(1), "cleared hero is available again");

let full = initialDraftState;
for (const heroId of [10, 11, 12, 13, 14]) full = draftReducer(full, { type: "pick", heroId });
full = draftReducer(full, { type: "pick", heroId: 15 });
check(full.draft.allies.join() === "10,11,12,13,15", "pick into a full team replaces the active slot");

let moved = draftReducer(initialDraftState, { type: "pick", heroId: 20 });
moved = draftReducer(moved, { type: "pick", heroId: 21 });
const ally = (index: number) => ({ team: "allies" as const, index });
const enemy = (index: number) => ({ team: "enemies" as const, index });
moved = draftReducer(moved, { type: "move", from: ally(0), to: ally(1) });
check(moved.draft.allies.slice(0, 2).join() === "21,20", "move swaps two heroes");
moved = draftReducer(moved, { type: "move", from: ally(0), to: ally(4) });
check(moved.draft.allies[0] === null && moved.draft.allies[4] === 21, "move into an empty slot");
moved = draftReducer(moved, { type: "move", from: ally(4), to: enemy(2) });
check(moved.draft.allies[4] === null && moved.draft.enemies[2] === 21, "move a hero to the other team");
moved = draftReducer(moved, { type: "move", from: enemy(2), to: ally(1) });
check(moved.draft.allies[1] === 21 && moved.draft.enemies[2] === 20, "swap heroes across teams");

const arranged = draftReducer(moved, { type: "arrange", allies: [null, null, 21, null, 20] });
check(arranged.draft.allies.join() === ",,21,,20" && arranged.draft.enemies === moved.draft.enemies, "arrange sets my team's slots");

const team = (heroes: number[], complete = true) => ({ heroes, complete });
let scanned = draftReducer(initialDraftState, { type: "scan", allies: team([50, 51]), enemies: team([52]) });
check(scanned.draft.allies.join() === "50,51,,," && scanned.draft.enemies.join() === "52,,,,", "scan fills empty slots");
scanned = draftReducer(scanned, { type: "move", from: ally(0), to: ally(4) });
scanned = draftReducer(scanned, { type: "scan", allies: team([50, 51, 53]), enemies: team([52]) });
check(scanned.draft.allies.join() === "53,51,,,50", "rescan keeps heroes where I put them");
scanned = draftReducer(scanned, { type: "scan", allies: team([51, 53]), enemies: team([52, 50]) });
check(scanned.draft.allies.join() === "53,51,,," && scanned.draft.enemies.join() === "52,50,,,", "a hero moves to the team it's on");
scanned = draftReducer(scanned, { type: "scan", allies: team([], false), enemies: team([52]) });
check(scanned.draft.allies.join() === "53,51,,,", "unrecognized portraits don't clear my team");
check(
  draftReducer({ ...scanned, draft: { ...scanned.draft, bans: [54] } }, { type: "scan", allies: team([54], false), enemies: team([52], false) })
    .draft.bans.length === 0,
  "a picked hero isn't banned",
);

const afterBans = emptySlotTarget({ ...s.draft, allies: [7, null, 8, null, null] }, "allies");
check(afterBans.team === "allies" && afterBans.index === 1, "leaving ban mode targets the first empty ally slot");

check(draftReducer(s, { type: "reset" }) === initialDraftState, "reset returns the empty draft");

let forWhom = draftReducer(initialDraftState, { type: "target", target: { team: "allies", index: 2 } });
check(forWhom.allyIndex === 2, "recommendations follow the chosen ally slot");
forWhom = draftReducer(forWhom, { type: "target", target: { team: "enemies", index: 0 } });
forWhom = draftReducer(forWhom, { type: "pick", heroId: 30 });
check(forWhom.allyIndex === 2, "picking enemies keeps recommending for the same ally slot");

let cm = draftReducer(initialDraftState, { type: "cm", cm: { first: true } });
check(cm.target.team === "bans" && cmTurn(cm)?.ours === true, "CM starts with our ban when we pick first");
for (const heroId of [40, 41, 42, 43, 44, 45, 46]) cm = draftReducer(cm, { type: "pick", heroId });
check(cm.draft.bans.length === 7, "the first ban phase is 7 bans");
check(cm.target.team === "allies" && cm.target.index === 0, "then our first pick goes to my slot");
cm = draftReducer(cm, { type: "pick", heroId: 47 });
check(cm.target.team === "enemies" && cm.target.index === 0, "then their pick");
cm = draftReducer(cm, { type: "clear", team: "allies", index: 0 });
check(cm.target.team === "allies" && cm.target.index === 0, "undoing a pick gives the turn back");
cm = draftReducer(cm, { type: "reset" });
check(cm.cm?.first === true && cm.target.team === "bans", "reset keeps Captains Mode");
const second = draftReducer(initialDraftState, { type: "cm", cm: { first: false } });
check(cmTurn(second)?.ours === false, "picking second, the first ban is theirs");

const hero = (id: number, localizedName: string) => ({ id, localizedName, iconUrl: "", roles: [] });
const heroes = [
  hero(1, "Anti-Mage"), hero(120, "Pangolier"), hero(44, "Phantom Assassin"), hero(12, "Phantom Lancer"),
  hero(39, "Queen of Pain"), hero(27, "Shadow Shaman"), hero(54, "Lifestealer"),
];
const first = (q: string) => searchHeroes(heroes, q)[0]?.localizedName;
check(first("qop") === "Queen of Pain", "initials find a hero");
check(first("pa") === "Phantom Assassin", "initials come before a name that only starts with them");
check(first("pl") === "Phantom Lancer" && first("am") === "Anti-Mage", "initials of hyphenated names");
check(first("naix") === "Lifestealer", "nicknames");
check(first("anti mage") === "Anti-Mage" && first("shaman") === "Shadow Shaman", "spaces and punctuation don't count");
check(searchHeroes(heroes, "").length === heroes.length && searchHeroes(heroes, "йщз").length === 0, "empty and non-Latin queries");

console.log("draft checks passed");
