import { useEffect, useEffectEvent, useRef, useState, type Dispatch } from "react";
import type { Hero } from "../api/heroes";
import { DRAG_HERO, type DraftAction } from "../draft/draft";
import { loadIconTemplates, scan, taughtTemplates, teach, type ScannedSlot } from "../scan/scan";
import "./ScreenScan.css";

const SIDE_KEY = "scanSide";
const AUTO_MS = 3000;
type Side = "left" | "right";

type Props = {
  heroes: Hero[];
  heroesById: Map<number, Hero>;
  dispatch: Dispatch<DraftAction>;
};

function loadSide(): Side {
  try {
    return localStorage.getItem(SIDE_KEY) === "right" ? "right" : "left";
  } catch {
    return "left";
  }
}

// Fills the draft from a screenshot of the game: pasted (Print Screen / Win+Shift+S, then Ctrl+V) or grabbed
// from a shared window. A portrait it can't place (an arcana, a persona) is taught by dropping the hero on it.
export function ScreenScan({ heroes, heroesById, dispatch }: Props) {
  const [side, setSide] = useState(loadSide);
  const [slots, setSlots] = useState<ScannedSlot[] | null>(null);
  const [status, setStatus] = useState<string | null>(null);
  const [dropOver, setDropOver] = useState<number | null>(null);
  const [auto, setAuto] = useState(false);
  const video = useRef<HTMLVideoElement | null>(null);
  // What the last auto scan saw: an unchanged screen isn't applied again, so manual edits stay until it changes.
  const lastSeen = useRef("");

  function apply(next: ScannedSlot[], mySide: Side) {
    const team = (part: ScannedSlot[]) => ({
      heroes: part.flatMap((s) => (s.heroId === null ? [] : [s.heroId])),
      complete: part.every((s) => s.status !== "unknown"),
    });
    const [left, right] = [team(next.slice(0, 5)), team(next.slice(5))];
    dispatch({ type: "scan", allies: mySide === "left" ? left : right, enemies: mySide === "left" ? right : left });
  }

  async function run(source: CanvasImageSource, width: number, height: number, quiet = false) {
    if (!quiet) setStatus("Розпізнаю…");
    try {
      const icons = await loadIconTemplates(heroes.map((h) => h.id));
      const next = scan(source, width, height, [...icons, ...taughtTemplates()]);
      const seen = next.map((s) => s.heroId ?? s.status).join();
      if (quiet && seen === lastSeen.current) return;
      lastSeen.current = seen;
      setSlots(next);
      apply(next, side);
      const found = next.filter((s) => s.status === "hero").length;
      const unknown = next.filter((s) => s.status === "unknown").length;
      setStatus(
        `Знайдено героїв: ${found}` +
          (unknown > 0 ? `. Не впізнано: ${unknown} — перетягни на портрет правильного героя, і я запам'ятаю.` : ""),
      );
    } catch (e) {
      setStatus(e instanceof Error ? e.message : String(e));
    }
  }

  const onPaste = useEffectEvent(async (e: ClipboardEvent) => {
    const file = Array.from(e.clipboardData?.files ?? []).find((f) => f.type.startsWith("image/"));
    if (!file) return;
    e.preventDefault();
    const image = await createImageBitmap(file);
    await run(image, image.width, image.height);
    image.close();
  });

  useEffect(() => {
    const handler = (e: ClipboardEvent) => onPaste(e);
    window.addEventListener("paste", handler);
    return () => window.removeEventListener("paste", handler);
  }, []);

  const sharing = () => (video.current?.srcObject as MediaStream | null)?.active === true;

  // The first time asks which window to share (pick Dota), later just takes a frame.
  async function capture(quiet = false) {
    try {
      if (!sharing()) {
        const stream = await navigator.mediaDevices.getDisplayMedia({ video: true, audio: false });
        const v = document.createElement("video");
        v.muted = true;
        v.srcObject = stream;
        await v.play();
        video.current = v;
      }
      const v = video.current!;
      await run(v, v.videoWidth, v.videoHeight, quiet);
      return true;
    } catch (e) {
      // Closing the share dialog isn't an error worth showing.
      if (!(e instanceof DOMException && e.name === "NotAllowedError")) setStatus(String(e));
      return false;
    }
  }

  async function toggleAuto() {
    if (auto) return setAuto(false);
    lastSeen.current = "";
    if (await capture(true)) setAuto(true);
  }

  const onTick = useEffectEvent(() => {
    if (sharing()) return void capture(true);
    setAuto(false);
    setStatus("Доступ до вікна зупинено — автоскан вимкнено.");
  });

  useEffect(() => {
    if (!auto) return;
    const id = setInterval(onTick, AUTO_MS);
    return () => clearInterval(id);
  }, [auto]);

  function changeSide(next: Side) {
    setSide(next);
    try {
      localStorage.setItem(SIDE_KEY, next);
    } catch {
      // not critical
    }
    if (slots) apply(slots, next);
  }

  function teachSlot(index: number, heroId: number) {
    if (!slots) return;
    teach(heroId, slots[index].features);
    const next = slots.map((s, i): ScannedSlot => (i === index ? { ...s, status: "hero", heroId } : s));
    setSlots(next);
    apply(next, side);
    setStatus(`Запам'ятав: ${heroesById.get(heroId)?.localizedName}`);
  }

  return (
    <section className="screen-scan">
      <div className="screen-scan__controls">
        <button type="button" onClick={() => capture()} title="Перший раз вибери вікно Dota (режим «вікно без рамки»)">
          Сканувати екран
        </button>
        <button
          type="button"
          className="screen-scan__auto"
          aria-pressed={auto}
          title={`Сканувати вікно Dota кожні ${AUTO_MS / 1000} с, поки не вимкнеш`}
          onClick={toggleAuto}
        >
          Автоскан {auto ? "увімк." : "вимк."}
        </button>
        <div className="screen-scan__side" role="group" aria-label="Моя сторона">
          Я за
          {(["left", "right"] as const).map((s) => (
            <button
              key={s}
              type="button"
              aria-pressed={side === s}
              title={s === "left" ? "Моя команда ліворуч у верхній смузі" : "Моя команда праворуч у верхній смузі"}
              onClick={() => changeSide(s)}
            >
              {s === "left" ? "Radiant" : "Dire"}
            </button>
          ))}
        </div>
        <span className="hint">або Ctrl+V зі скріном усього екрана</span>
      </div>

      {slots && (
        <div className="screen-scan__slots">
          {slots.map((s, i) => {
            const hero = s.heroId === null ? undefined : heroesById.get(s.heroId);
            const canTeach = s.status !== "empty";
            return (
              <figure
                key={i}
                className={`screen-scan__slot${dropOver === i ? " screen-scan__slot--drop" : ""}`}
                data-status={s.status}
                title={canTeach ? "Перетягни сюди правильного героя, щоб навчити" : undefined}
                onDragOver={(e) => {
                  if (!canTeach || !e.dataTransfer.types.includes(DRAG_HERO)) return;
                  e.preventDefault();
                  setDropOver(i);
                }}
                onDragLeave={() => setDropOver(null)}
                onDrop={(e) => {
                  e.preventDefault();
                  setDropOver(null);
                  const heroId = Number(e.dataTransfer.getData(DRAG_HERO));
                  if (heroId) teachSlot(i, heroId);
                }}
              >
                <img src={s.thumb} alt="" draggable={false} />
                <figcaption>{s.status === "empty" ? "—" : (hero?.localizedName ?? "?")}</figcaption>
              </figure>
            );
          })}
        </div>
      )}

      {status && <p className="hint">{status}</p>}
    </section>
  );
}
