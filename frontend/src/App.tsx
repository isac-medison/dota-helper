import { useMemo, useReducer, useRef, useState } from "react";
import { useHeroes } from "./api/heroes";
import { useMetaStatus } from "./api/meta";
import { usePlayer } from "./api/players";
import { useRecommendations, useThreats } from "./api/recommendations";
import { AppHeader } from "./components/AppHeader";
import { DraftBoard } from "./components/DraftBoard";
import { HeroPicker } from "./components/HeroPicker";
import { PlayerCard } from "./components/PlayerCard";
import { SavedProfiles } from "./components/Profiles";
import { ScreenScan } from "./components/ScreenScan";
import { SwapAdvice } from "./components/SwapAdvice";
import { RecommendationList } from "./components/RecommendationList";
import {
  CM_STEPS,
  cmTurn,
  draftReducer,
  initialDraftState,
  TEAM_SIZE,
  usedHeroIds,
  type DraftAction,
  type Position,
} from "./draft/draft";
import "./App.css";

const PARTY_KEY = "partySteamIds";
const PROFILES_KEY = "savedProfiles";
// Values match the API's Bracket enum.
const BRACKETS = [
  { value: 1, label: "Herald–Guardian" },
  { value: 2, label: "Crusader–Archon" },
  { value: 3, label: "Legend–Ancient" },
  { value: 4, label: "Divine–Immortal" },
];

// Storage can be blocked (private mode, disabled site data); the page works without it.
function load(key: string): unknown[] {
  try {
    const saved = JSON.parse(localStorage.getItem(key) ?? "[]");
    return Array.isArray(saved) ? saved : [];
  } catch {
    return [];
  }
}

function save(key: string, value: unknown) {
  try {
    localStorage.setItem(key, JSON.stringify(value));
  } catch {
    // not critical
  }
}

// Steam IDs by ally slot.
function loadParty(): (string | null)[] {
  const saved = load(PARTY_KEY);
  return Array.from({ length: TEAM_SIZE }, (_, i) => (typeof saved[i] === "string" ? (saved[i] as string) : null));
}

function loadProfiles(): string[] {
  const saved = load(PROFILES_KEY).filter((id): id is string => typeof id === "string");
  // Before the profile list existed, IDs only lived in the party slots.
  return saved.length > 0 ? saved : loadParty().filter((id): id is string => id !== null);
}

export default function App() {
  const [bracket, setBracket] = useState<number | null>(null);

  const { data: heroes, isPending, error } = useHeroes();
  const { data: meta } = useMetaStatus();
  const [state, dispatch] = useReducer(draftReducer, initialDraftState);
  const [party, setParty] = useState(loadParty);
  const [profiles, setProfiles] = useState(loadProfiles);
  // Recommendations are for the ally slot the cursor was last on, with that slot's player's comfort.
  const myPosition = (state.allyIndex + 1) as Position;
  const steamId = party[state.allyIndex];
  const player = usePlayer(steamId);
  // null = take it from the party's average rank.
  // The party hasn't split positions yet: slots are just slots, positions are guessed from the heroes.
  const [openPositions, setOpenPositions] = useState(false);
  const request = { ...state.draft, myPosition, steamIds: party, bracket, openPositions };
  const dataVersion = `${meta?.updatedAtUtc}|${player.data?.fetchedAtUtc}`;
  const recommendations = useRecommendations(request, dataVersion);
  // Banning or filling an enemy slot: show what the enemy wants instead of my picks.
  const forEnemy = state.target.team !== "allies";
  const threats = useThreats(request, dataVersion, forEnemy);
  const threatPositions = useMemo(
    () => new Map(threats.data?.map((t) => [t.recommendation.heroId, t.position])),
    [threats.data],
  );
  const pickPositions = useMemo(() => {
    const data = recommendations.data;
    return data?.positions ? new Map(data.items.map((r, i) => [r.heroId, data.positions![i]])) : undefined;
  }, [recommendations.data]);
  const searchRef = useRef<HTMLInputElement>(null);

  const heroesById = useMemo(
    () => new Map((heroes ?? []).map((h) => [h.id, h])),
    [heroes],
  );
  const used = useMemo(() => usedHeroIds(state.draft), [state.draft]);
  const turn = cmTurn(state);

  if (isPending) return <p>Завантаження героїв…</p>;
  if (error) return <p>Помилка: {error.message}</p>;

  // A recommendation is for its ally slot, so it goes there rather than wherever the cursor is.
  function pickRecommended(heroId: number) {
    dispatch({ type: "target", target: { team: "allies", index: state.allyIndex } });
    dispatch({ type: "pick", heroId });
  }

  function changeProfiles(next: string[]) {
    setProfiles(next);
    save(PROFILES_KEY, next);
  }

  // One player sits in one slot: putting them somewhere else moves them. New IDs are remembered.
  function changeParty(index: number, id: string | null) {
    const next = party.map((old, i) => (i === index ? id : old === id ? null : old));
    setParty(next);
    save(PARTY_KEY, next);
    if (id !== null && !profiles.includes(id)) changeProfiles([...profiles, id]);
  }

  // A suggested swap moves heroes and players together; recommendations follow my player if they moved.
  function applySwap(allies: (number | null)[], steamIds: (string | null)[]) {
    dispatch({ type: "arrange", allies });
    setParty(steamIds);
    save(PARTY_KEY, steamIds);
    const me = steamId ? steamIds.indexOf(steamId) : -1;
    if (me >= 0 && me !== state.allyIndex) dispatch({ type: "target", target: { team: "allies", index: me } });
  }

  function swapParty(from: number, to: number) {
    const next = party.with(from, party[to]).with(to, party[from]);
    setParty(next);
    save(PARTY_KEY, next);
  }

  // Clicking the board returns focus to search, so you can type the next hero right away.
  function boardDispatch(action: DraftAction) {
    dispatch(action);
    searchRef.current?.focus({ preventScroll: true });
  }

  return (
    <>
      <AppHeader player={player.data} steamId={steamId} />

      <main className="draft-page">
        <div className="draft-page__heroes">
          <HeroPicker
            heroes={heroes}
            used={used}
            onPick={(heroId) => dispatch({ type: "pick", heroId })}
            searchRef={searchRef}
          />
        </div>

        <div className="draft-page__draft">
                      <label className="bracket-select">
              Бракет
              <select
                value={bracket ?? ""}
                onChange={(e) => setBracket(e.target.value ? Number(e.target.value) : null)}
              >
                <option value="">Авто (з рангу паті)</option>
                {BRACKETS.map((b) => (
                  <option key={b.value} value={b.value}>
                    {b.label}
                  </option>
                ))}
              </select>
            </label>
          <div className="draft-settings">
           <label className="bracket-select">
              Позиції
              <select value={openPositions ? "open" : ""} onChange={(e) => setOpenPositions(e.target.value === "open")}>
                <option value="">Розписані (слот = позиція)</option>
                <option value="open">Не розписані — пікаємо, ролі потім</option>
              </select>
            </label>
            <button type="button" onClick={() => dispatch({ type: "reset" })}>
              Очистити драфт
            </button>

            {state.cm && (
              <strong className="cm-turn" data-ours={turn?.ours}>
                {turn
                  ? `Крок ${turn.step + 1}/${CM_STEPS}: ${turn.ours ? "наш" : "їхній"} ${turn.ban ? "бан" : "пік"}`
                  : "Драфт завершено"}
              </strong>
            )}
          </div>

          <ScreenScan heroes={heroes} heroesById={heroesById} dispatch={dispatch} />

          <DraftBoard
            state={state}
            heroesById={heroesById}
            party={party}
            onPartyChange={changeParty}
            onPartySwap={swapParty}
            enemyPositions={recommendations.data?.enemyPositions}
            allyPositions={openPositions ? (recommendations.data?.allyPositions ?? []) : null}
            dispatch={boardDispatch}
          />

          {/* Not while the next answer loads: an old swap would undo the latest change. */}
          {recommendations.data?.swap && !recommendations.isPlaceholderData && (
            <SwapAdvice
              swap={recommendations.data.swap}
              allies={state.draft.allies}
              party={party}
              heroesById={heroesById}
              onApply={() => applySwap(recommendations.data.swap!.allies, recommendations.data.swap!.steamIds)}
            />
          )}

          <SavedProfiles
            steamIds={profiles}
            onAdd={(id) => !profiles.includes(id) && changeProfiles([...profiles, id])}
            onRemove={(id) => changeProfiles(profiles.filter((p) => p !== id))}
            onUse={(id) => changeParty(state.allyIndex, id)}
          />

          {player.isFetching && <p>Завантаження профілю…</p>}
          {player.error && <p>Не вдалося завантажити профіль: {player.error.message}</p>}
          {player.data === null && <p>STRATZ не знає гравця на Pos {myPosition} — перевір Steam ID.</p>}
          {player.data && (
            <PlayerCard player={player.data} heroesById={heroesById} myPosition={myPosition} />
          )}
        </div>

        {forEnemy ? (
          <section className="draft-page__recommendations">
            <h2>{state.target.team === "bans" ? "Кого банити" : "Небезпечні піки ворога"}</h2>
            <p className="hint">Що ворогу дав би кожен пік на його вільних позиціях. Клік — у бани / ворожий слот.</p>
            {threats.error ? (
              <p>{threats.error.message}</p>
            ) : threats.data ? (
              <RecommendationList
                items={threats.data.map((t) => t.recommendation)}
                heroesById={heroesById}
                onPick={(heroId) => boardDispatch({ type: "pick", heroId })}
                positions={threatPositions}
                forEnemy
              />
            ) : (
              <p>Рахую…</p>
            )}
          </section>
        ) : (
          <section className="draft-page__recommendations">
            <h2>
              {openPositions ? "Рекомендації на вільну позицію" : `Рекомендації на Pos ${myPosition}`}
              {recommendations.data && (
                <small>
                  {BRACKETS.find((b) => b.value === recommendations.data.bracket)?.label}
                  {!recommendations.data.comfortUsed
                    ? " · без комфорту"
                    : openPositions
                      ? " · з урахуванням ігор паті (хто з вас найкращий на герої)"
                      : player.data
                        ? ` · з урахуванням ігор ${player.data.name}`
                        : ""}
                </small>
              )}
            </h2>
            <p className="hint">Клікни слот своєї команди, щоб отримати поради для нього, або бани / ворожий слот — кого забрати у ворога.</p>
            {recommendations.error ? (
              <p>{recommendations.error.message}</p>
            ) : recommendations.data ? (
              <RecommendationList
                items={recommendations.data.items}
                heroesById={heroesById}
                onPick={pickRecommended}
                positions={pickPositions}
              />
            ) : (
              <p>Рахую…</p>
            )}
          </section>
        )}
      </main>
    </>
  );
}
