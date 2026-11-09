import { useState } from "react";
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

function SteamIdInput({ initial, className, onSubmit, onCancel }: SteamIdInputProps) {
  const [value, setValue] = useState(initial);
  const invalid = value !== "" && !/^\d+$/.test(value);
  const submit = () => {
    if (!invalid) onSubmit(value || null);
  };
  return (
    <input
      className={className}
      autoFocus
      inputMode="numeric"
      placeholder="Steam ID / Friend ID"
      value={value}
      aria-invalid={invalid}
      title={invalid ? "Тільки цифри: Steam ID64 або Friend ID" : undefined}
      onChange={(e) => setValue(e.target.value.trim())}
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
