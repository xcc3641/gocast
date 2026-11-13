// Fails the build when en.json and zh.json don't have the same shape:
// the same keys at every level and arrays of the same length.
import { readFileSync } from "node:fs";

const load = (name) =>
  JSON.parse(readFileSync(new URL(`../src/i18n/${name}.json`, import.meta.url), "utf8"));

const kind = (v) => (Array.isArray(v) ? "array" : v === null ? "null" : typeof v);
const problems = [];

function compare(a, b, path) {
  const ka = kind(a);
  const kb = kind(b);
  if (ka !== kb) {
    problems.push(`${path || "(root)"}: ${ka} in en.json, ${kb} in zh.json`);
    return;
  }
  if (ka === "array") {
    if (a.length !== b.length) {
      problems.push(`${path}: ${a.length} items in en.json, ${b.length} in zh.json`);
    }
    for (let i = 0; i < Math.min(a.length, b.length); i++) compare(a[i], b[i], `${path}[${i}]`);
  } else if (ka === "object") {
    for (const key of Object.keys(a)) {
      const p = path ? `${path}.${key}` : key;
      if (!(key in b)) problems.push(`${p}: missing in zh.json`);
      else compare(a[key], b[key], p);
    }
    for (const key of Object.keys(b)) {
      if (!(key in a)) problems.push(`${path ? `${path}.${key}` : key}: missing in en.json`);
    }
  }
}

compare(load("en"), load("zh"), "");

if (problems.length) {
  console.error("check-i18n: en.json and zh.json differ:\n  " + problems.join("\n  "));
  process.exit(1);
}
console.log("check-i18n: en.json and zh.json have the same keys");
