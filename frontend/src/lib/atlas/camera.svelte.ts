// ── Sky camera: centre, zoom, canvas size and how they move ─────────────────
// Shared by the Sky Atlas and Coverage views: animated fly-to, inertial
// panning, anchored zoom and the keyboard/mouse gestures that drive them.

import { DragTracker, STOP_SPEED, coastStep } from "./inertia";
import type { FitResult } from "./fit";
import {
  angularSep,
  clampPpd,
  fromTangent,
  panFrom,
  toTangent,
  zoomAnchored,
  type Viewport,
} from "./projection";

/** Drag distance (px) below which a press counts as a click. */
const CLICK_SLOP_PX = 4;

function reducedMotion(): boolean {
  return window.matchMedia?.("(prefers-reduced-motion: reduce)").matches ?? false;
}

export class SkyCamera {
  ra = $state(180);
  dec = $state(0);
  ppd = $state(12);
  w = $state(800);
  h = $state(600);
  vp = $derived<Viewport>({ ra: this.ra, dec: this.dec, ppd: this.ppd, w: this.w, h: this.h });

  /** A drag is in progress (button held). */
  isPanning = $state(false);
  /** The current drag has moved past the click slop. */
  dragMoved = $state(false);

  private anim: number | null = null;
  private panStart = { x: 0, y: 0, ra: 0, dec: 0 };
  private drag = new DragTracker();

  /** Horizontal field of view in degrees. */
  get fovDeg(): number {
    return this.w / this.ppd;
  }

  stopAnim(): void {
    if (this.anim !== null) cancelAnimationFrame(this.anim);
    this.anim = null;
  }

  /** Flies to a view (or jumps, for long hops and reduced motion). */
  goTo(t: FitResult, animate = true): void {
    this.stopAnim();
    const start = { ra: this.ra, dec: this.dec, ppd: this.ppd };
    const off = toTangent(start.ra, start.dec, t.ra, t.dec);
    if (!animate || reducedMotion() || !off || angularSep(start.ra, start.dec, t.ra, t.dec) > 60) {
      this.ra = t.ra;
      this.dec = t.dec;
      this.ppd = t.ppd;
      return;
    }
    const t0 = performance.now();
    const dur = 380;
    const step = (now: number) => {
      const k = Math.min(1, (now - t0) / dur);
      const e = 1 - (1 - k) ** 3;
      const [ra, dec] = fromTangent(start.ra, start.dec, off[0] * e, off[1] * e);
      this.ra = ra;
      this.dec = dec;
      this.ppd = start.ppd * (t.ppd / start.ppd) ** e;
      this.anim = k < 1 ? requestAnimationFrame(step) : null;
    };
    this.anim = requestAnimationFrame(step);
  }

  /** Keeps the view gliding after a fast drag, slowing down until it stops. */
  private coast(v0: [number, number]): void {
    this.stopAnim();
    if (reducedMotion()) return;
    let v = v0;
    let last = performance.now();
    const step = (now: number) => {
      const r = coastStep(v, Math.min(now - last, 50));
      last = now;
      v = r.v;
      const c = panFrom(this.ra, this.dec, this.ppd, r.dx, r.dy);
      this.ra = c.ra;
      this.dec = c.dec;
      this.anim = Math.hypot(v[0], v[1]) > STOP_SPEED ? requestAnimationFrame(step) : null;
    };
    this.anim = requestAnimationFrame(step);
  }

  /** Zooms by `factor`, keeping the sky under canvas point (x, y) fixed. */
  zoomBy(factor: number, x = this.w / 2, y = this.h / 2): void {
    this.stopAnim();
    const old = this.ppd;
    const ppd = clampPpd(old * factor);
    const c = zoomAnchored({ ...this.vp, ppd }, old, x, y);
    this.ppd = ppd;
    this.ra = c.ra;
    this.dec = c.dec;
  }

  panBy(dx: number, dy: number): void {
    this.stopAnim();
    const c = panFrom(this.ra, this.dec, this.ppd, dx, dy);
    this.ra = c.ra;
    this.dec = c.dec;
  }

  /** Mouse wheel zoom about the pointer at canvas (x, y). */
  wheel(e: WheelEvent, x: number, y: number): void {
    e.preventDefault();
    this.zoomBy(Math.min(1.5, Math.max(0.66, Math.exp(-e.deltaY * 0.0015))), x, y);
  }

  /** Starts a drag on a primary-button press. Returns false for other buttons. */
  beginDrag(e: MouseEvent): boolean {
    if (e.button !== 0) return false;
    this.stopAnim();
    this.isPanning = true;
    this.dragMoved = false;
    this.panStart = { x: e.clientX, y: e.clientY, ra: this.ra, dec: this.dec };
    this.drag.reset(performance.now(), e.clientX, e.clientY);
    return true;
  }

  /** Follows the pointer while dragging. Returns true while a drag is panning the view. */
  dragTo(e: MouseEvent): boolean {
    if (!this.isPanning) return false;
    const dx = e.clientX - this.panStart.x;
    const dy = e.clientY - this.panStart.y;
    if (!this.dragMoved && Math.hypot(dx, dy) <= CLICK_SLOP_PX) return false;
    this.dragMoved = true;
    this.drag.add(performance.now(), e.clientX, e.clientY);
    const c = panFrom(this.panStart.ra, this.panStart.dec, this.ppd, dx, dy);
    this.ra = c.ra;
    this.dec = c.dec;
    return true;
  }

  /**
   * Ends a drag; a fast release keeps the view gliding. Returns "none" when no
   * drag was in progress, "click" when the press didn't move, else "drag".
   */
  endDrag(): "none" | "click" | "drag" {
    if (!this.isPanning) return "none";
    this.isPanning = false;
    if (!this.dragMoved) return "click";
    const v = this.drag.velocity(performance.now());
    if (v) this.coast(v);
    return "drag";
  }

  /** Handles the shared zoom (+ / −) and pan (arrow) keys. Returns whether it did. */
  navKey(key: string): boolean {
    const step = Math.min(this.w, this.h) / 6;
    switch (key) {
      case "+":
      case "=":
        this.zoomBy(1.4);
        return true;
      case "-":
      case "_":
        this.zoomBy(1 / 1.4);
        return true;
      case "ArrowLeft":
        this.panBy(step, 0);
        return true;
      case "ArrowRight":
        this.panBy(-step, 0);
        return true;
      case "ArrowUp":
        this.panBy(0, step);
        return true;
      case "ArrowDown":
        this.panBy(0, -step);
        return true;
      default:
        return false;
    }
  }
}
