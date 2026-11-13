import { getRelativeLocaleUrl } from "astro:i18n";
import en from "./en.json";
import zh from "./zh.json";

export type Locale = "en" | "zh";
export type Copy = typeof en;

const copies: Record<Locale, Copy> = { en, zh };

/** The copy for one language. scripts/check-i18n.mjs keeps both files the same shape. */
export function t(locale: Locale): Copy {
  return copies[locale];
}

export function otherLocale(locale: Locale): Locale {
  return locale === "en" ? "zh" : "en";
}

/** "/" or "/zh/", plus a page path like "support/". */
export function url(locale: Locale, path = ""): string {
  return getRelativeLocaleUrl(locale, path);
}
