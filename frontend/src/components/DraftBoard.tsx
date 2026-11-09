import { useState, type Dispatch, type DragEvent } from "react";
import type { Hero } from "../api/heroes";
import {
  DRAG_HERO,
  DRAG_PARTY_SLOT,
  DRAG_PROFILE,
  emptySlotTarget,
  type DraftAction,
  type DraftState,
  type Slot,
  type Team,
} from "../draft/draft";
import { PartyMember } from "./Profiles";
import "./DraftBoard.css";

type Props = {
  state: DraftState;
  heroesById: Map<number, Hero>;
  // Steam IDs of my party by ally slot.
  party: (string | null)[];
  onPartyChange: (index: number, steamId: string | null) => void;
  onPartySwap: (from: number, to: number) => void;
  // Where the model thinks each enemy plays, from the last recommendations; for my team too when positions aren't set.
  enemyPositions: (number | null)[] | undefined;
  allyPositions: (number | null)[] | null | undefined;
  dispatch: Dispatch<DraftAction>;
};

const TEAM_LABELS: Record<Team, string> = {
  allies: "Твоя команда",
  enemies: "Вороги",
};

// ponytail: native HTML5 drag & drop — mouse only; add a click/keyboard swap if touch or a11y matters.
export function DraftBoard({
  state,
  heroesById,
  party,
  onPartyChange,
  onPartySwap,
  enemyPositions,
  allyPositions,
  dispatch,
}: Props) {
  const { draft, target, allyIndex } = state;
  const banMode = target.team === "bans";
  const [dragged, setDragged] = useState<Slot | null>(null);
  const [dropOver, setDropOver] = useState<Slot | "bans" | null>(null);

  function endDrag() {
    setDragged(null);
    setDropOver(null);
  }

  // Heroes go to any slot of either team (a misplaced pick moves to the other team), players only to my team.
  function accepts(e: DragEvent, team: Team, index: number) {
    const types = e.dataTransfer.types;
    if (types.includes(DRAG_HERO)) return true;
    if (types.includes(DRAG_PROFILE)) return team === "allies";
    return dragged !== null && !(dragged.team === team && dragged.index === index);
  }

  function drop(e: DragEvent, team: Team, index: number) {
    e.preventDefault();
    const heroId = Number(e.dataTransfer.getData(DRAG_HERO));
    const steamId = e.dataTransfer.getData(DRAG_PROFILE);
    const partySlot = e.dataTransfer.getData(DRAG_PARTY_SLOT);
    if (heroId) {
      dispatch({ type: "target", target: { team, index } });
      dispatch({ type: "pick", heroId });
    } else if (partySlot && team === "allies") {
      onPartySwap(Number(partySlot), index);
    } else if (steamId && team === "allies") {
      onPartyChange(index, steamId);
    } else if (dragged && !(dragged.team === team && dragged.index === index)) {
      dispatch({ type: "move", from: dragged, to: { team, index } });
    }
    endDrag();
  }

  // A hero dropped on the bans is banned without leaving the slot the cursor is on (in CM the order moves it).
  function dropBan(e: DragEvent) {
    e.preventDefault();
    const heroId = Number(e.dataTransfer.getData(DRAG_HERO));
    if (heroId) {
      dispatch({ type: "target", target: { team: "bans" } });
      dispatch({ type: "pick", heroId });
      if (!banMode && !state.cm) dispatch({ type: "target", target });
    }
    endDrag();
  }

  return (
    <section className="draft-board">
      {(["allies", "enemies"] as const).map((team) => (
        <div key={team} className={`draft-team draft-team--${team}`}>
          <h2>{TEAM_LABELS[team]}</h2>
          <div className="draft-slots">
            {draft[team].map((heroId, index) => {
              const hero = heroId === null ? undefined : heroesById.get(heroId);
              const active = target.team === team && target.index === index;
              const position = index + 1;
              const isMine = team === "allies" && allyIndex === index;
              const guessed = hero ? (team === "enemies" ? enemyPositions : allyPositions)?.[index] : null;
              const label =
                team === "allies" && !allyPositions ? `Pos ${position}` : guessed ? `≈ Pos ${guessed}` : team === "allies" ? "Союзник" : "Ворог";
              const isDropTarget = dropOver !== "bans" && dropOver?.team === team && dropOver.index === index;
              const classes = [
                "draft-slot",
                isMine && "draft-slot--mine",
                dragged?.team === team && dragged.index === index && "draft-slot--dragging",
                isDropTarget && "draft-slot--drop",
              ].filter(Boolean);
              return (
                <div
                  key={index}
                  className={classes.join(" ")}
                  draggable={hero !== undefined}
                  onDragStart={() => setDragged({ team, index })}
                  onDragEnd={endDrag}
                  onDragOver={(e) => {
                    if (!accepts(e, team, index)) return;
                    e.preventDefault();
                    setDropOver({ team, index });
                  }}
                  onDragLeave={() => setDropOver(null)}
                  onDrop={(e) => drop(e, team, index)}
                >
                  <span className="draft-slot__pos">
                    {label}
                    {isMine && " · поради"}
                  </span>
                  <button
                    type="button"
                    className="draft-slot__select"
                    aria-pressed={active}
                    aria-label={`Pos ${position}: ${hero?.localizedName ?? "порожньо"}`}
                    onClick={() => dispatch({ type: "target", target: { team, index } })}
                  >
                    {hero ? (
                      <img src={hero.iconUrl} alt="" draggable={false} />
                    ) : (
                      <span className="draft-slot__empty">+</span>
                    )}
                  </button>
                  {hero && (
                    <button
                      type="button"
                      className="draft-slot__clear"
                      aria-label={`Прибрати ${hero.localizedName}`}
                      onClick={() => dispatch({ type: "clear", team, index })}
                    >
                      ×
                    </button>
                  )}
                  {team === "allies" && (
                    <PartyMember slot={index} steamId={party[index]} onChange={(id) => onPartyChange(index, id)} />
                  )}
                </div>
              );
            })}
          </div>
        </div>
      ))}

      <div
        className={`draft-bans${dropOver === "bans" ? " draft-bans--drop" : ""}`}
        onDragOver={(e) => {
          if (!e.dataTransfer.types.includes(DRAG_HERO)) return;
          e.preventDefault();
          setDropOver("bans");
        }}
        onDragLeave={() => setDropOver(null)}
        onDrop={dropBan}
      >
        <button
          type="button"
          className="draft-bans__toggle"
          aria-pressed={banMode}
          title="Або перетягни героя сюди"
          onClick={() =>
            dispatch({
              type: "target",
              target: banMode ? emptySlotTarget(draft, "allies") : { team: "bans" },
            })
          }
        >
          {banMode ? "Бани: вибирай героїв ✓" : "+ Бани"}
        </button>
        {draft.bans.map((heroId) => {
          const hero = heroesById.get(heroId);
          return (
            <button
              key={heroId}
              type="button"
              className="draft-ban"
              title={`Прибрати бан: ${hero?.localizedName}`}
              onClick={() => dispatch({ type: "unban", heroId })}
            >
              <img src={hero?.iconUrl} alt={hero?.localizedName} />
            </button>
          );
        })}
      </div>
    </section>
  );
}
