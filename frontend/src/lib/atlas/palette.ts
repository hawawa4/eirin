// ── Atlas colour palettes, one per app theme ────────────────────────────────
// The canvas can't read CSS tokens cheaply on every frame, so each theme gets a
// full palette here. The red theme is for use at the telescope, so it keeps to
// warm reds and ambers like the rest of the red UI.

import type { Theme } from "../types";

export type RGB = readonly [number, number, number];

export interface AtlasPalette {
  /** Radial background gradient: centre, middle, edge. */
  bg: readonly [string, string, string];
  dust: RGB;
  grid: RGB;
  /** Equator and 0h meridian. */
  gridZero: RGB;
  raLabel: RGB;
  decLabel: RGB;
  /** Dark tone for label backdrops, marker edges and the compass disc. */
  ink: RGB;
  star: RGB;
  starLabel: RGB;
  dso: RGB;
  dsoLabel: RGB;
  frame: RGB;
  frameLabel: RGB;
  hovered: RGB;
  hoveredLabel: RGB;
  selected: RGB;
  selectedLabel: RGB;
  compass: RGB;
  compassText: RGB;
}

const BLUE: AtlasPalette = {
  bg: ["#0c1024", "#070912", "#04050a"],
  dust: [210, 220, 255],
  grid: [90, 115, 165],
  gridZero: [130, 155, 210],
  raLabel: [170, 195, 240],
  decLabel: [130, 225, 200],
  ink: [6, 8, 18],
  star: [230, 238, 255],
  starLabel: [205, 218, 255],
  dso: [255, 195, 90],
  dsoLabel: [255, 215, 135],
  frame: [100, 165, 255],
  frameLabel: [185, 218, 255],
  hovered: [120, 215, 255],
  hoveredLabel: [215, 238, 255],
  selected: [255, 210, 80],
  selectedLabel: [255, 228, 140],
  compass: [150, 180, 235],
  compassText: [205, 222, 255],
};

const RED: AtlasPalette = {
  bg: ["#1c0b09", "#0f0605", "#070202"],
  dust: [255, 170, 150],
  grid: [150, 66, 56],
  gridZero: [205, 100, 85],
  raLabel: [235, 150, 135],
  decLabel: [240, 178, 120],
  ink: [14, 5, 4],
  star: [255, 218, 208],
  starLabel: [245, 190, 180],
  dso: [255, 165, 90],
  dsoLabel: [255, 195, 135],
  frame: [224, 112, 96],
  frameLabel: [245, 178, 165],
  hovered: [255, 150, 128],
  hoveredLabel: [255, 218, 208],
  selected: [255, 200, 110],
  selectedLabel: [255, 226, 170],
  compass: [205, 120, 108],
  compassText: [245, 200, 190],
};

const GREY: AtlasPalette = {
  bg: ["#16161c", "#0d0d11", "#060608"],
  dust: [222, 222, 230],
  grid: [110, 110, 125],
  gridZero: [152, 152, 168],
  raLabel: [192, 192, 206],
  decLabel: [176, 198, 190],
  ink: [8, 8, 10],
  star: [236, 236, 242],
  starLabel: [212, 212, 222],
  dso: [230, 196, 132],
  dsoLabel: [236, 212, 162],
  frame: [155, 164, 184],
  frameLabel: [202, 208, 222],
  hovered: [212, 220, 236],
  hoveredLabel: [242, 244, 250],
  selected: [240, 205, 110],
  selectedLabel: [246, 226, 162],
  compass: [160, 165, 180],
  compassText: [222, 224, 232],
};

const PALETTES: Record<Theme, AtlasPalette> = { blue: BLUE, red: RED, grey: GREY };

export function atlasPalette(theme: Theme): AtlasPalette {
  return PALETTES[theme] ?? BLUE;
}

export function rgba(c: RGB, a = 1): string {
  return `rgba(${c[0]},${c[1]},${c[2]},${a})`;
}

/** CSS custom properties for the DOM parts of the atlas (legend swatches, panel accents). */
export function paletteCssVars(p: AtlasPalette): string {
  return [
    `--atlas-bg: ${p.bg[2]}`,
    `--atlas-backdrop: ${rgba(p.ink, 0.55)}`,
    `--atlas-frame: ${rgba(p.frame)}`,
    `--atlas-selected: ${rgba(p.selected)}`,
    `--atlas-hovered: ${rgba(p.hovered)}`,
    `--atlas-star: ${rgba(p.star)}`,
    `--atlas-dso: ${rgba(p.dso)}`,
  ].join("; ");
}
