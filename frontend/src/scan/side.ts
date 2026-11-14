// Which half of the top bar is my team: Radiant sits on the left, Dire on the right.
export type Side = "left" | "right";

const SIDE_KEY = "scanSide";

export function loadSide(): Side {
  try {
    return localStorage.getItem(SIDE_KEY) === "right" ? "right" : "left";
  } catch {
    return "left";
  }
}

export function saveSide(side: Side) {
  try {
    localStorage.setItem(SIDE_KEY, side);
  } catch {
    // not critical
  }
}
