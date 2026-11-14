import { useState } from "react";
import {
  ANCHORS,
  anchorOf,
  cancelScan,
  capture,
  errorText,
  isClick,
  loadCalibration,
  otherWindow,
  saveCalibration,
  scanPlayers,
  showPoints,
  spread,
  stepPoint,
  stepsFrom,
  STEP_KINDS,
  withAnchor,
  type Anchor,
  type Calibration,
  type LobbyPoint,
  type LobbyStep,
  type RecordedClick,
} from "../api/lobby";
import { loadSide } from "../scan/side";
import "./LobbyScan.css";

type Scope = "all" | "enemies" | "allies";
type Busy = "players" | "steps" | "scan" | "test";

type Props = {
  // Steam IDs found for my team and for the enemy. An empty list = that team wasn't scanned or gave nothing.
  onFound: (allies: string[], enemies: string[]) => void;
};

const PLAYERS = 10;
const ENDS = ["1-й гравець зліва", "5-й гравець зліва", "1-й гравець справа", "5-й гравець справа"];

// Collects the Steam IDs of the players of a match by clicking through their profiles in the game,
// the way it's done by hand. Where to click is recorded once (calibration): F8 in the game marks a point.
export function LobbyScan({ onFound }: Props) {
  const [calibration, setCalibration] = useState(loadCalibration);
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState<Busy | null>(null);
  const [status, setStatus] = useState<string | null>(null);
  const [scope, setScope] = useState<Scope>("all");
  const [testPlayer, setTestPlayer] = useState(0);

  const { players, steps, origin, windowOrigin, hoverMs } = calibration;
  const ready = players.length === PLAYERS && steps.length > 0;

  function update(change: (old: Calibration) => Calibration) {
    setCalibration((old) => {
      const next = change(old);
      saveCalibration(next);
      return next;
    });
  }

  async function during(what: Busy, work: () => Promise<void>) {
    setBusy(what);
    try {
      await work();
    } catch (e) {
      setStatus(errorText(e));
    } finally {
      setBusy(null);
    }
  }

  const recordPlayers = () =>
    during("players", async () => {
      const ends: LobbyPoint[] = [];
      while (ends.length < ENDS.length) {
        setStatus(
          `Точка ${ends.length + 1}/${ENDS.length}: у Dota наведи курсор на ім'я — ${ENDS[ends.length]} — і натисни F8. F9 — скасувати.`,
        );
        const point = await capture();
        if (!point) return setStatus("Запис точок скасовано.");
        ends.push(point);
      }
      update((old) => ({ ...old, players: [...spread(ends[0], ends[1]), ...spread(ends[2], ends[3])] }));
      setStatus("Точки гравців записано.");
    });

  const recordSteps = () =>
    during("steps", async () => {
      const points: RecordedClick[] = [];
      for (;;) {
        setStatus(
          points.length === 0
            ? "У Dota наведи курсор на ім'я гравця й натисни F8, потім клікни сам. Так само перед кожним наступним кліком: F8 — лівий, F7 — правий. F9 — кінець."
            : `Записано кліків: ${points.length}. Перед наступним: F8 — лівий, F7 — правий. F9 — кінець.`,
        );
        const point = await capture();
        if (!point) break;
        points.push(point);
      }
      if (points.length === 0) return setStatus("Нічого не записано.");
      update((old) => ({ ...old, steps: stepsFrom(points), origin: { x: points[0].x, y: points[0].y }, windowOrigin: otherWindow(points) }));
      setStatus(`Записано кліків: ${points.length}. Перевір кроки нижче й запусти тест.`);
    });

  const addClick = () =>
    during("steps", async () => {
      setStatus("У Dota наведи курсор на місце кліку й натисни F8 (лівий) або F7 (правий). F9 — скасувати.");
      const point = await capture();
      if (!point) return setStatus("Скасовано.");
      const step = { kind: point.right ? "rightClick" : "click", x: point.x, y: point.y, relative: false, window: false, keys: "", delayMs: 600 };
      update((old) => ({ ...old, steps: [...old.steps, step] }));
      setStatus("Клік додано в кінець.");
    });

  const test = () =>
    during("test", async () => {
      setStatus(`Тест на гравці ${testPlayer + 1}: перейди в Dota й натисни F8. F8 ще раз — зупинка.`);
      const result = await scanPlayers([players[testPlayer]], steps, hoverMs);
      const [id] = result.ids;
      const [raw] = result.raw;
      setStatus(
        id
          ? `Тест: знайдено ID ${id}.`
          : raw
            ? `Тест: скопійовано «${raw.slice(0, 60)}», але ID у цьому немає.`
            : "Тест: у буфер обміну нічого не потрапило — перевір точки, кроки й паузи.",
      );
    });

  // Nothing is clicked: the cursor only visits the points, so it's safe to watch anywhere.
  const show = (everyPlayer: boolean) =>
    during("test", async () => {
      const clicks = steps.filter(isClick);
      if (clicks.length === 0) return setStatus("У кроках немає жодного кліку.");
      setStatus("Перейди в Dota й натисни F8 — проведу курсором по точках без кліків. F8 ще раз — зупинка.");
      const aborted = everyPlayer
        ? await showPoints(players, [clicks[0]], windowOrigin)
        : await showPoints([players[testPlayer]], clicks, windowOrigin);
      setStatus(aborted ? "Показ зупинено." : "Показ закінчено. Якщо курсор ставав не туди — поправ точки або координати кроків.");
    });

  const scan = () =>
    during("scan", async () => {
      const mine = loadSide();
      const isAlly = (i: number) => (i < PLAYERS / 2 ? "left" : "right") === mine;
      const wanted = players.map((_, i) => i).filter((i) => scope === "all" || isAlly(i) === (scope === "allies"));

      setStatus("Перейди в Dota й натисни F8 — почну клікати. F8 ще раз — зупинка.");
      const result = await scanPlayers(
        wanted.map((i) => players[i]),
        steps,
        hoverMs,
      );
      const found = (allies: boolean) => wanted.flatMap((i, k) => (isAlly(i) === allies && result.ids[k] ? [result.ids[k]] : []));
      const [allies, enemies] = [found(true), found(false)];
      onFound(allies, enemies);

      const total = allies.length + enemies.length;
      const repeated = total - new Set([...allies, ...enemies]).size;
      setStatus(
        `${result.aborted ? "Зупинено. " : ""}Знайдено ID: ${total} з ${wanted.length} (союзники ${allies.length}, вороги ${enemies.length}).` +
          (total < wanted.length && !result.aborted ? " Де не знайдено — профіль не відкрився або ID не скопіювався." : "") +
          (repeated > 0 ? ` Той самий ID повторився ${repeated} раз(ів): схоже, для різних гравців відкривався один профіль — перевір кроки тестом.` : ""),
      );
    });

  function changeStep(index: number, change: Partial<LobbyStep>) {
    update((old) => ({ ...old, steps: old.steps.map((s, i) => (i === index ? { ...s, ...change } : s)) }));
  }

  // The point stays where it was on the screen when the steps were recorded; only what it is counted from changes.
  function changeAnchor(index: number, anchor: Anchor) {
    changeStep(index, withAnchor(steps[index], anchor, { origin, windowOrigin }));
  }

  function moveStep(index: number, by: number) {
    update((old) => {
      const to = index + by;
      if (to < 0 || to >= old.steps.length) return old;
      const next = [...old.steps];
      [next[index], next[to]] = [next[to], next[index]];
      return { ...old, steps: next };
    });
  }

  function addStep(step: LobbyStep) {
    update((old) => ({ ...old, steps: [...old.steps, step] }));
  }

  return (
    <section className="lobby-scan">
      <div className="lobby-scan__controls">
        <button
          type="button"
          disabled={!ready || busy !== null}
          title={ready ? "Проклікати профілі гравців у Dota й зібрати їхні Steam ID" : "Спершу зроби калібровку"}
          onClick={scan}
        >
          Сканувати гравців
        </button>
        <select value={scope} aria-label="Кого сканувати" disabled={busy !== null} onChange={(e) => setScope(e.target.value as Scope)}>
          <option value="all">усі 10</option>
          <option value="enemies">лише вороги</option>
          <option value="allies">лише союзники</option>
        </select>
        <button type="button" aria-pressed={open} onClick={() => setOpen(!open)}>
          Калібровка{ready ? "" : " — потрібна"}
        </button>
        {busy && (
          <button type="button" className="lobby-scan__cancel" onClick={() => cancelScan()}>
            Скасувати
          </button>
        )}
      </div>

      {status && (
        <p className="hint" data-busy={busy !== null}>
          {status}
        </p>
      )}

      {open && (
        <div className="lobby-scan__calibration">
          <div className="lobby-scan__part">
            <h3>1. Де гравці</h3>
            <p className="hint">
              На екрані драфту наведи курсор на ім'я гравця й натисни F8 — чотири рази: перший і п'ятий зліва, перший і
              п'ятий справа. Решту порахую сам.
            </p>
            <div className="lobby-scan__row">
              <button type="button" disabled={busy !== null} onClick={recordPlayers}>
                {players.length === PLAYERS ? "Записати заново" : "Записати точки"}
              </button>
              {players.length === PLAYERS && (
                <span className="lobby-scan__points" title={players.map((p, i) => `${i + 1}: ${p.x}, ${p.y}`).join("\n")}>
                  ✓ {players[0].x},{players[0].y} … {players[9].x},{players[9].y}
                </span>
              )}
            </div>
          </div>

          <div className="lobby-scan__part">
            <h3>2. Що клікати на кожному</h3>
            <p className="hint">
              Пройди шлях на одному гравці: перед кожним кліком наведи курсор, натисни F8 (лівий клік) або F7 (правий),
              потім клікни сам — ім'я, пункт меню, ID, закриття. F9 — кінець. Якщо ID копіюється клавішами, додай крок
              «Клавіші».
            </p>
            <div className="lobby-scan__row">
              <button type="button" disabled={busy !== null} onClick={recordSteps}>
                {steps.length > 0 ? "Записати заново" : "Записати кроки"}
              </button>
              <button type="button" disabled={busy !== null} onClick={addClick}>
                + Клік (F8 / F7)
              </button>
              <button
                type="button"
                title="Клік на точці гравця без запису в грі — тип і координати задаються вручну"
                onClick={() => addStep({ kind: "click", x: 0, y: 0, relative: true, window: false, keys: "", delayMs: 600 })}
              >
                + Клік вручну
              </button>
              <button type="button" onClick={() => addStep({ kind: "keys", x: 0, y: 0, relative: false, window: false, keys: "ctrl+c", delayMs: 300 })}>
                + Клавіші
              </button>
              <button type="button" onClick={() => addStep({ kind: "wait", x: 0, y: 0, relative: false, window: false, keys: "", delayMs: 500 })}>
                + Пауза
              </button>
            </div>

            <label className="lobby-scan__row" title="Скільки курсор стоїть на точці перед кожним кліком, щоб гра встигла побачити, що під ним. Якщо кліки не спрацьовують — збільш.">
              Наведення перед кліком
              <input
                type="number"
                className="lobby-scan__number"
                min={50}
                step={50}
                value={hoverMs}
                onChange={(e) => update((old) => ({ ...old, hoverMs: Math.max(0, Number(e.target.value) || 0) }))}
              />
              мс
            </label>

            {steps.length > 0 && (
              <ol className="lobby-scan__steps">
                {steps.map((step, i) => (
                  <li key={i}>
                    <div className="lobby-scan__step-head">
                      <select value={step.kind} aria-label={`Тип кроку ${i + 1}`} onChange={(e) => changeStep(i, { kind: e.target.value })}>
                        {STEP_KINDS.map((k) => (
                          <option key={k.value} value={k.value}>
                            {k.label}
                          </option>
                        ))}
                      </select>

                      {isClick(step) && (
                        <select
                          value={anchorOf(step)}
                          aria-label={`Прив'язка кроку ${i + 1}`}
                          title="Від чого рахуються координати: місце на екрані; точка гравця (меню під курсором); кут активного вікна (вікно Steam, яке можна пересунути)"
                          onChange={(e) => changeAnchor(i, e.target.value as Anchor)}
                        >
                          {ANCHORS.map((a) => (
                            <option key={a.value} value={a.value}>
                              {a.label}
                            </option>
                          ))}
                        </select>
                      )}
                      {step.kind === "keys" && (
                        <input
                          className="lobby-scan__keys"
                          aria-label={`Клавіші кроку ${i + 1}`}
                          placeholder="ctrl+c, esc"
                          value={step.keys}
                          onChange={(e) => changeStep(i, { keys: e.target.value })}
                        />
                      )}
                      <span className="lobby-scan__step-tools">
                        <button type="button" aria-label="Вище" disabled={i === 0} onClick={() => moveStep(i, -1)}>
                          ↑
                        </button>
                        <button type="button" aria-label="Нижче" disabled={i === steps.length - 1} onClick={() => moveStep(i, 1)}>
                          ↓
                        </button>
                        <button
                          type="button"
                          aria-label={`Копіювати крок ${i + 1}`}
                          title="Копія кроку одразу під ним"
                          onClick={() => update((old) => ({ ...old, steps: old.steps.toSpliced(i + 1, 0, { ...old.steps[i] }) }))}
                        >
                          ⧉
                        </button>
                        <button
                          type="button"
                          aria-label={`Видалити крок ${i + 1}`}
                          onClick={() => update((old) => ({ ...old, steps: old.steps.filter((_, k) => k !== i) }))}
                        >
                          ×
                        </button>
                      </span>
                    </div>

                    <div className="lobby-scan__step-params">
                      {isClick(step) && (
                        <>
                          <span>{step.window ? "зсув від кута вікна" : step.relative ? "зсув від гравця" : "точка на екрані"}</span>
                          <input
                            type="number"
                            className="lobby-scan__number"
                            aria-label={`X кроку ${i + 1}`}
                            value={step.x}
                            onChange={(e) => changeStep(i, { x: Number(e.target.value) || 0 })}
                          />
                          <input
                            type="number"
                            className="lobby-scan__number"
                            aria-label={`Y кроку ${i + 1}`}
                            value={step.y}
                            onChange={(e) => changeStep(i, { y: Number(e.target.value) || 0 })}
                          />
                        </>
                      )}

                      <label className="lobby-scan__delay" title="Пауза після кроку, щоб гра встигла відкрити те, що клікнули">
                        пауза
                        <input
                          type="number"
                          className="lobby-scan__number"
                          min={0}
                          step={100}
                          value={step.delayMs}
                          onChange={(e) => changeStep(i, { delayMs: Math.max(0, Number(e.target.value) || 0) })}
                        />
                        мс
                      </label>
                    </div>
                  </li>
                ))}
              </ol>
            )}
          </div>

          <div className="lobby-scan__part">
            <h3>3. Перевірка</h3>
            <div className="lobby-scan__row">
              <button type="button" disabled={!ready || busy !== null} onClick={test}>
                Тест на одному гравці
              </button>
              <select value={testPlayer} aria-label="Гравець для тесту" onChange={(e) => setTestPlayer(Number(e.target.value))}>
                {Array.from({ length: PLAYERS }, (_, i) => (
                  <option key={i} value={i}>
                    {i < 5 ? `зліва ${i + 1}` : `справа ${i - 4}`}
                  </option>
                ))}
              </select>
              <span className="hint">Починає після F8 у Dota; перевір і крайніх гравців справа.</span>
            </div>
            <div className="lobby-scan__row">
              <button
                type="button"
                disabled={!ready || busy !== null}
                title="Курсор пройде по всіх точках кроків для вибраного гравця, нічого не клікаючи"
                onClick={() => show(false)}
              >
                Показати курсором: кроки
              </button>
              <button
                type="button"
                disabled={!ready || busy !== null}
                title="Курсор пройде по першій точці кожного з 10 гравців, нічого не клікаючи"
                onClick={() => show(true)}
              >
                Показати курсором: 10 гравців
              </button>
            </div>
            {players.length === PLAYERS && <ClickMap players={players} steps={steps} windowOrigin={windowOrigin} selected={testPlayer} onSelect={setTestPlayer} />}
          </div>
        </div>
      )}
    </section>
  );
}

type MapProps = {
  players: LobbyPoint[];
  steps: LobbyStep[];
  windowOrigin: LobbyPoint | null;
  // The player whose steps are drawn.
  selected: number;
  onSelect: (player: number) => void;
};

// The screen in miniature: the ten player points, and where each click lands for the selected player.
function ClickMap({ players, steps, windowOrigin, selected, onSelect }: MapProps) {
  const clicks = steps.flatMap((step, i) => (isClick(step) ? [{ step, number: i + 1, ...stepPoint(step, players[selected], windowOrigin) }] : []));

  // Points are in physical pixels; the screen is at least as big as the furthest of them.
  const all = [...players, ...clicks];
  const width = Math.max(Math.round(window.screen.width * window.devicePixelRatio), ...all.map((p) => p.x + 40));
  const height = Math.max(Math.round(window.screen.height * window.devicePixelRatio), ...all.map((p) => p.y + 40));
  const unit = width / 200;
  const offScreen = clicks.filter((c) => c.x < 0 || c.y < 0);

  return (
    <div className="lobby-scan__map">
      <svg viewBox={`0 0 ${width} ${height}`} role="img" aria-label="Де будуть кліки на екрані">
        <rect className="lobby-scan__screen" width={width} height={height} />
        {clicks.length > 1 && <polyline className="lobby-scan__path" points={clicks.map((c) => `${c.x},${c.y}`).join(" ")} strokeWidth={unit / 3} />}
        {players.map((p, i) => (
          <circle
            key={i}
            className="lobby-scan__player"
            data-side={i < PLAYERS / 2 ? "left" : "right"}
            data-selected={i === selected}
            cx={p.x}
            cy={p.y}
            r={unit * 1.6}
            strokeWidth={unit / 2}
            onClick={() => onSelect(i)}
          >
            <title>{`Гравець ${i + 1}: ${p.x}, ${p.y}`}</title>
          </circle>
        ))}
        {clicks.map((c) => (
          <g key={c.number} className="lobby-scan__click" data-right={c.step.kind === "rightClick"}>
            <circle cx={c.x} cy={c.y} r={unit} strokeWidth={unit / 3} />
            <text x={c.x + unit * 1.6} y={c.y + unit * 1.2} fontSize={unit * 3.2}>
              {/* Clicks on one spot share a label, written at the first of them. */}
              {clicks.find((o) => o.x === c.x && o.y === c.y) === c &&
                clicks
                  .filter((o) => o.x === c.x && o.y === c.y)
                  .map((o) => o.number)
                  .join("·")}
            </text>
            <title>{`Крок ${c.number}: ${c.x}, ${c.y}`}</title>
          </g>
        ))}
      </svg>

      <ol className="lobby-scan__coords">
        {clicks.map((c) => (
          <li key={c.number}>
            <b>{c.number}</b>
            <span>{STEP_KINDS.find((k) => k.value === c.step.kind)?.label}</span>
            <code>
              {c.x}, {c.y}
            </code>
            <small>{ANCHORS.find((a) => a.value === anchorOf(c.step))?.hint}</small>
          </li>
        ))}
      </ol>
      <p className="hint">
        Екран {width}×{height}. Кроки показано для гравця {selected + 1} — клікни іншу точку гравця, щоб побачити для нього.
        {offScreen.length > 0 && ` Крок ${offScreen.map((c) => c.number).join(", ")} виходить за екран.`}
      </p>
    </div>
  );
}
