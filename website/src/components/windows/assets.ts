// The drawn windows' pictures, all from src/assets/ui (exported by Pluso, see its README.md).
// This is the one place that knows file names. A missing file never fails the build:
// an app icon falls back to a grey rounded square, an image to an empty box.
// Symbols are drawn as CSS masks; their file names are listed in sym.css.
import type { ImageMetadata } from "astro";

const icons = import.meta.glob<ImageMetadata>("../../assets/ui/icons/*.png", {
  eager: true,
  import: "default",
});
const images = import.meta.glob<ImageMetadata>("../../assets/ui/*.png", {
  eager: true,
  import: "default",
});

/** What the windows call each app or file icon → its file in src/assets/ui/icons (without .png). */
export const ICON = {
  wechat: "wechat",
  safari: "safari",
  vscode: "vscode",
  notes: "notes",
  calendar: "calendar",
  finder: "finder",
  weread: "weread",
  wireless: "wireless-diagnostics",
  chatgpt: "chatgpt",
  pdf: "pdf",
} as const;
export type IconKey = keyof typeof ICON;

/** Other pictures → their file in src/assets/ui (without .png). */
export const IMAGE = {
  landscape: "landscape-546x364",
} as const;

export const iconFile = (k: IconKey): ImageMetadata | undefined =>
  icons[`../../assets/ui/icons/${ICON[k]}.png`];

export const imageFile = (k: keyof typeof IMAGE): ImageMetadata | undefined =>
  images[`../../assets/ui/${IMAGE[k]}.png`];
