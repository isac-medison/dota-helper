import { useEffect, useState } from "react";
import { resolveId } from "../api/lobby";
import { useProfileLabel } from "../api/players";
import { DRAG_PARTY_SLOT, DRAG_PROFILE } from "../draft/draft";
import "./Profiles.css";

type SteamIdInputProps = {
  initial: string;
  className: string;
  // null = the field was emptied.
  onSubmit: (steamId: string | null) => void;
  onCancel: () => void;
};

// A pasted profile link (https://steamcommunity.com/profiles/7656…/) is cut down to the Steam ID in it.
function idFromLink(text: string) {
  return /steamcommunity\.com\/profiles\/(\d+)/i.exec(text)?.[1] ?? text;
}

// A link with a custom name has no ID in it: Steam is asked whose profile it is.
const CUSTOM_LINK = /steamcommunity\.com\/id\/[^/?#\s]+/i;

export function SteamIdInput({ initial, className, onSubmit, onCancel }: SteamIdInputProps) {
  const [value, setValue] = useState(initial);
  // The custom link Steam knows no profile for.
  const [unknown, setUnknown] = useState<string | null>(null);
  const looking = CUSTOM_LINK.test(value) && unknown !== value;
  const invalid = value !== "" && !looking && !/^\d+$/.test(value);

  useEffect(() => {
    if (!looking) return;
    let current = true;
    resolveId(value)
      .catch(() => null)
      .then((id) => {
        if (!current) return;
        if (id) setValue(id);
        else setUnknown(value);
      });
    return () => {
      current = false;
    };
  }, [looking, value]);

  const submit = () => {
    if (!invalid && !looking) onSubmit(value || null);
  };
  return (
    <input
      className={className}
      autoFocus
      placeholder="Steam ID / Friend ID / посилання на профіль"
      value={value}
      aria-invalid={invalid}
      aria-busy={looking}
      title={
        looking
          ? "Питаю у Steam, чий це профіль…"
          : unknown === value
            ? "Steam не знає профілю за цим посиланням"
            : invalid
              ? "Steam ID64, Friend ID або посилання на профіль Steam"
              : undefined
      }
      onChange={(e) => setValue(idFromLink(e.target.value.trim()))}
      onBlur={submit}
      onKeyDown={(e) => {
        if (e.key === "Enter") submit();
        if (e.key === "Escape") onCancel();
      }}
    />
  );
}

type PartyMemberProps = {
  // The ally slot index, so the player can be dragged to another position.
  slot: number;
  steamId: string | null;
  onChange: (steamId: string | null) => void;
};

// Who plays this ally slot: their profile gives the comfort when recommending for it.
// Dragged onto another ally slot, the two players swap positions.
export function PartyMember({ slot, steamId, onChange }: PartyMemberProps) {
  const [editing, setEditing] = useState(false);
  const { name, avatarUrl } = useProfileLabel(steamId);

  if (editing) {
    return (
      <SteamIdInput
        initial={steamId ?? ""}
        className="party-member__input"
        onSubmit={(id) => {
          onChange(id);
          setEditing(false);
        }}
        onCancel={() => setEditing(false)}
      />
    );
  }

  const member = (
    <button
      type="button"
      className="party-member"
      data-empty={steamId === null}
      draggable={steamId !== null}
      onDragStart={(e) => {
        // The slot around is draggable too (its hero); this drag is the player's.
        e.stopPropagation();
        e.dataTransfer.setData(DRAG_PROFILE, steamId!);
        e.dataTransfer.setData(DRAG_PARTY_SLOT, String(slot));
        e.dataTransfer.effectAllowed = "move";
      }}
      title={
        steamId === null
          ? "Steam ID гравця на цій позиції (або перетягни збережений профіль)"
          : `Steam ID ${steamId} — клік: змінити, перетягни: на іншу позицію`
      }
      onClick={() => setEditing(true)}
    >
      {avatarUrl && <img src={avatarUrl} alt="" />}
      <span>{steamId === null ? "+ гравець" : name}</span>
    </button>
  );
  if (steamId === null) return member;

  return (
    <div className="party-slot">
      {member}
      <button
        type="button"
        className="saved-profile__remove"
        aria-label={`Прибрати ${name} з Pos ${slot + 1}`}
        title="Прибрати гравця з позиції (у збережених профілях лишиться)"
        onClick={() => onChange(null)}
      >
        ×
      </button>
    </div>
  );
}

type SavedProfilesProps = {
  steamIds: string[];
  onAdd: (steamId: string) => void;
  onRemove: (steamId: string) => void;
  // Click = put on the slot recommendations are for.
  onUse: (steamId: string) => void;
};

// Friends I play with: drag one onto an ally slot, or click to put them on the active one.
export function SavedProfiles({ steamIds, onAdd, onRemove, onUse }: SavedProfilesProps) {
  const [adding, setAdding] = useState(false);

  return (
    <section className="saved-profiles">
      <h2>Профілі</h2>
      <div className="saved-profiles__list">
        {steamIds.map((id) => (
          <SavedProfile key={id} steamId={id} onRemove={() => onRemove(id)} onUse={() => onUse(id)} />
        ))}
        {adding ? (
          <SteamIdInput
            initial=""
            className="party-member__input saved-profiles__input"
            onSubmit={(id) => {
              if (id) onAdd(id);
              setAdding(false);
            }}
            onCancel={() => setAdding(false)}
          />
        ) : (
          <button type="button" className="party-member" data-empty onClick={() => setAdding(true)}>
            + профіль
          </button>
        )}
      </div>
    </section>
  );
}

type SavedProfileProps = {
  steamId: string;
  onRemove: () => void;
  onUse: () => void;
};

function SavedProfile({ steamId, onRemove, onUse }: SavedProfileProps) {
  const { name, avatarUrl } = useProfileLabel(steamId);
  return (
    <div
      className="saved-profile"
      draggable
      onDragStart={(e) => {
        e.dataTransfer.setData(DRAG_PROFILE, steamId);
        e.dataTransfer.effectAllowed = "copy";
      }}
    >
      <button
        type="button"
        className="party-member"
        title={`Steam ID ${steamId}: клік — на позицію з порадами, або перетягни на будь-яку`}
        onClick={onUse}
      >
        {avatarUrl && <img src={avatarUrl} alt="" draggable={false} />}
        <span>{name}</span>
      </button>
      <button type="button" className="saved-profile__remove" aria-label={`Видалити ${name}`} onClick={onRemove}>
        ×
      </button>
    </div>
  );
}
