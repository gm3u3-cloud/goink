import { describe, it, expect, beforeEach, vi } from "vitest";
import {
  useFontScaleStore,
  FONT_SCALE_MIN,
  FONT_SCALE_MAX,
  FONT_SCALE_DEFAULT,
} from "./useFontScaleStore";

const STORAGE_KEY = "goink-font-scale";

beforeEach(() => {
  localStorage.removeItem(STORAGE_KEY);
  useFontScaleStore.setState({ scale: FONT_SCALE_DEFAULT });
  document.documentElement.style.fontSize = "";
});

describe("useFontScaleStore", () => {
  it("defaults to 100%", () => {
    expect(useFontScaleStore.getState().scale).toBe(FONT_SCALE_DEFAULT);
  });

  it("sets scale and applies it to the root font-size", () => {
    useFontScaleStore.getState().setScale(110);
    expect(useFontScaleStore.getState().scale).toBe(110);
    expect(document.documentElement.style.fontSize).toBe("110%");
  });

  it("clamps below the minimum and above the maximum", () => {
    useFontScaleStore.getState().setScale(40);
    expect(useFontScaleStore.getState().scale).toBe(FONT_SCALE_MIN);
    expect(document.documentElement.style.fontSize).toBe("75%");

    useFontScaleStore.getState().setScale(400);
    expect(useFontScaleStore.getState().scale).toBe(FONT_SCALE_MAX);
    expect(document.documentElement.style.fontSize).toBe("150%");
  });

  it("snaps to the 5% step", () => {
    useFontScaleStore.getState().setScale(97);
    expect(useFontScaleStore.getState().scale).toBe(95);
    useFontScaleStore.getState().setScale(92);
    expect(useFontScaleStore.getState().scale).toBe(90);
  });

  it("persists the scale to localStorage", () => {
    useFontScaleStore.getState().setScale(125);
    const raw = localStorage.getItem(STORAGE_KEY);
    expect(raw).not.toBeNull();
    expect(JSON.parse(raw as string).state.scale).toBe(125);
  });

  it("resets to the default and applies it", () => {
    useFontScaleStore.getState().setScale(130);
    useFontScaleStore.getState().reset();
    expect(useFontScaleStore.getState().scale).toBe(FONT_SCALE_DEFAULT);
    expect(document.documentElement.style.fontSize).toBe("100%");
  });

  it("rehydrates a persisted scale on fresh load", async () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ state: { scale: 120 }, version: 0 }),
    );
    vi.resetModules();
    const { useFontScaleStore: fresh } = await import("./useFontScaleStore");
    expect(fresh.getState().scale).toBe(120);
    expect(document.documentElement.style.fontSize).toBe("120%");
  });
});
