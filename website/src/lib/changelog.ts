// Reads the repository's CHANGELOG.md (Keep a Changelog, written in Chinese and
// English) into releases whose text is split by language.
//
//   ## [0.1.0] - 2026-11-12
//
//   第一个版本。First release.
//
//   ### 新增 Added
//
//   - 中文一行。
//     The English line.
//
// A bullet's lines are split where the lines with Chinese in them end; a heading
// or one-line paragraph is split after its last Chinese character. Text that
// can't be split (only one language) is shown in both languages.
// All strings in the result are HTML: escaped, with minimal inline markdown.

export type Lang = "en" | "zh";
export type Bilingual = Record<Lang, string>;

export interface Group {
  /** The `###` heading; null for text before the first `###`. */
  heading: Bilingual | null;
  items: Bilingual[];
  paragraphs: Bilingual[];
}

export interface Release {
  version: string;
  /** YYYY-MM-DD, or null (e.g. Unreleased). */
  date: string | null;
  /** Paragraphs between the version heading and the first `###`. */
  intro: Bilingual[];
  groups: Group[];
}

// Han characters, CJK punctuation and full-width forms.
const CJK = /[\u2E80-\u2FFF\u3000-\u303F\u3040-\u30FF\u3400-\u4DBF\u4E00-\u9FFF\uF900-\uFAFF\uFE30-\uFE4F\uFF00-\uFFEF]/;
const CJK_ALL = new RegExp(CJK.source, "gu");
const SENTENCE_END = /[。！？）」』]/;

const hasCJK = (s: string) => CJK.test(s);

/** A line is Chinese when it has at least as many Chinese characters as Latin words. */
function isChinese(line: string): boolean {
  const cjk = line.match(CJK_ALL)?.length ?? 0;
  const words = line.match(/[A-Za-z]+/g)?.length ?? 0;
  return cjk > 0 && cjk >= words;
}

const VERSION = /^##\s+\[?([^\]\s]+)\]?(?:\s*[-–—]\s*(\d{4}-\d{2}-\d{2}))?/;
const LINK_DEF = /^\s*\[[^\]]+\]:\s*\S+/;
const BULLET = /^ ?[-*+]\s+(.*)$/;

const both = (html: string): Bilingual => ({ en: html, zh: html });

/** Escapes HTML, then renders `code`, \-escapes, **bold** and [links](https://…). */
export function inline(md: string): string {
  const slots: string[] = [];
  const keep = (html: string) => `\u0000${slots.push(html) - 1}\u0000`;
  const esc = (s: string) =>
    s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");

  let s = md
    // Backslash escapes: \` \* \_ \[ etc. become the literal character.
    .replace(/\\([\\`*_{}\[\]()#+\-.!<>|~])/g, (_, ch: string) => keep(esc(ch)))
    // Code spans.
    .replace(/(`+)([\s\S]*?[^`])\1(?!`)/g, (_, _ticks, code: string) => keep(`<code>${esc(code.trim())}</code>`));
  s = esc(s)
    .replace(/\[([^\]]+)\]\((https?:\/\/[^\s)]+)\)/g, (_, text: string, href: string) => `<a href="${href}">${text}</a>`)
    .replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>");
  return s.replace(/\u0000(\d+)\u0000/g, (_, i: string) => slots[Number(i)]);
}

/** Joins wrapped lines: no space between two Chinese characters, one space otherwise. */
function joinLines(lines: string[]): string {
  return lines.reduce((acc, line) => {
    if (!acc) return line;
    const glue = hasCJK(acc.slice(-1)) && hasCJK(line.charAt(0)) ? "" : " ";
    return acc + glue + line;
  }, "");
}

/**
 * Splits "中文部分 English part" after the last Chinese character.
 * `strict` (paragraphs, one-line bullets) only splits after a Chinese sentence
 * end like 。, so "支持 Intel 芯片" or "支持 macOS" stay whole.
 * Headings ("新增 Added") may split at a space.
 */
function splitText(text: string, strict: boolean): Bilingual | null {
  const matches = [...text.matchAll(CJK_ALL)];
  if (!matches.length) return null;
  const last = matches[matches.length - 1];
  const cut = (last.index ?? 0) + last[0].length;
  const zh = text.slice(0, cut).trim();
  const en = text.slice(cut).trim();
  if (!en || !/[A-Za-z]/.test(en)) return null;
  if (strict && !SENTENCE_END.test(last[0])) return null;
  if (!strict && !/^\s/.test(text.slice(cut)) && !SENTENCE_END.test(last[0])) return null;
  return { zh, en };
}

/** Lines of one bullet or paragraph: Chinese lines first, then English ones. */
function splitLines(lines: string[], strict: boolean): Bilingual {
  const trimmed = lines.map((l) => l.trim()).filter(Boolean);
  if (trimmed.length > 1) {
    const zhFirst = isChinese(trimmed[0]);
    // The run of lines in the first language, then the rest.
    let n = 1;
    while (n < trimmed.length && isChinese(trimmed[n]) === zhFirst) n++;
    // Lines in the other language come after; any later lines join them.
    if (n < trimmed.length) {
      const first = joinLines(trimmed.slice(0, n));
      const rest = joinLines(trimmed.slice(n));
      return zhFirst
        ? { zh: inline(first), en: inline(rest) }
        : { en: inline(first), zh: inline(rest) };
    }
  }
  const text = joinLines(trimmed);
  const split = splitText(text, strict);
  return split ? { zh: inline(split.zh), en: inline(split.en) } : both(inline(text));
}

export function parseChangelog(markdown: string): Release[] {
  const lines = markdown.replace(/\r\n?/g, "\n").split("\n");
  const releases: Release[] = [];
  let release: Release | null = null;
  let group: Group | null = null;
  // The block being collected: a bullet or a paragraph.
  let block: { kind: "item" | "para"; lines: string[] } | null = null;
  let inFence = false;

  const flush = () => {
    if (!block || !release) {
      block = null;
      return;
    }
    if (block.kind === "item") {
      if (!group) {
        group = { heading: null, items: [], paragraphs: [] };
        release.groups.push(group);
      }
      group.items.push(splitLines(block.lines, true));
    } else {
      const text = splitLines(block.lines, true);
      if (group) group.paragraphs.push(text);
      else release.intro.push(text);
    }
    block = null;
  };

  for (const raw of lines) {
    const line = raw.replace(/\s+$/, "");

    if (/^\s*(```|~~~)/.test(line)) {
      inFence = !inFence;
      continue; // Code blocks aren't expected in release notes; their content is kept as text.
    }

    if (!inFence && /^##\s/.test(line) && !/^###/.test(line)) {
      flush();
      const m = VERSION.exec(line);
      release = m ? { version: m[1], date: m[2] ?? null, intro: [], groups: [] } : null;
      group = null;
      if (release) releases.push(release);
      continue;
    }
    if (!release) continue; // The preamble before the first version, or an unknown ## heading.

    if (!inFence && /^###\s/.test(line)) {
      flush();
      const text = line.replace(/^###+\s+/, "").replace(/\s+#+$/, "");
      const split = splitText(text, false);
      group = {
        heading: split ? { zh: inline(split.zh), en: inline(split.en) } : both(inline(text)),
        items: [],
        paragraphs: [],
      };
      release.groups.push(group);
      continue;
    }

    if (!line.trim()) {
      flush();
      continue;
    }
    if (LINK_DEF.test(line)) {
      flush();
      continue;
    }
    if (/^#\s/.test(line) || /^(-{3,}|\*{3,})$/.test(line.trim())) {
      flush();
      continue;
    }

    // A bullet starts a new item; indented (or lazy) lines continue the current one.
    const bullet = BULLET.exec(line);
    if (bullet) {
      flush();
      block = { kind: "item", lines: [bullet[1]] };
      continue;
    }
    if (block) {
      block.lines.push(line.trim());
    } else {
      block = { kind: "para", lines: [line.trim()] };
    }
  }
  flush();

  // An empty Unreleased section says nothing; drop it (and any other empty version).
  return releases.filter(
    (r) => r.intro.length > 0 || r.groups.some((g) => g.items.length > 0 || g.paragraphs.length > 0),
  );
}

const MONTHS = [
  "January", "February", "March", "April", "May", "June",
  "July", "August", "September", "October", "November", "December",
];

/** "November 12, 2026" / "2026 年 11 月 12 日", like the privacy page's date. */
export function formatDate(iso: string, lang: Lang): string {
  const [y, m, d] = iso.split("-").map(Number);
  if (!y || !m || !d || m > 12) return iso;
  return lang === "zh" ? `${y} 年 ${m} 月 ${d} 日` : `${MONTHS[m - 1]} ${d}, ${y}`;
}
