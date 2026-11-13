// @ts-check
import { fileURLToPath } from "node:url";
import { defineConfig } from "astro/config";

// The changelog lives at the repository root, next to the app.
// Resolved from this file, so it doesn't matter where the build runs from.
const changelogPath = fileURLToPath(new URL("../CHANGELOG.md", import.meta.url));

export default defineConfig({
  site: "https://gocast.yichenlab.com",
  output: "static",
  trailingSlash: "always",
  build: {
    format: "directory",
    inlineStylesheets: "never",
  },
  i18n: {
    defaultLocale: "en",
    locales: ["en", "zh"],
    routing: {
      prefixDefaultLocale: false,
    },
  },
  devToolbar: { enabled: false },
  vite: {
    define: {
      __CHANGELOG_PATH__: JSON.stringify(changelogPath),
    },
  },
});
