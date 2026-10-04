// Colors adapt to prefers-color-scheme; resolved once at load.
const dark = window.matchMedia("(prefers-color-scheme: dark)").matches;

export const colors = dark
  ? { bg: "#0e1621", bubble: "#182533", text: "#e9eef2", muted: "#7f91a4", accent: "#6ab3f3", chip: "#1f2f3f", error: "#ff7b7b" }
  : { bg: "#dfe7ee", bubble: "#ffffff", text: "#111418", muted: "#6b7885", accent: "#2481cc", chip: "#c3d0db", error: "#c0392b" };

export const space = { xs: 4, sm: 8, md: 12, lg: 16 };
export const radius = 12;
export const MAX_BUBBLE_WIDTH = 560;
export const FONT = "system-ui, -apple-system, 'Segoe UI', Roboto, sans-serif";
