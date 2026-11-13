# Gocast website

The site at gocast.yichenlab.com: a landing page, Support, Privacy and Changelog, in English (`/`) and Simplified Chinese (`/zh/`). Built with [Astro](https://astro.build) as plain static HTML and one stylesheet: no client JavaScript, nothing loaded from other sites.

```sh
npm ci && npm run build   # writes dist/
npm run dev               # local server with live reload
```

- Copy lives in `src/i18n/en.json` and `src/i18n/zh.json`. Both files must have the same keys; `npm run build` checks that first (`scripts/check-i18n.mjs`).
- Pages are in `src/pages/` (the `zh/` ones only pass the locale); the templates are in `src/components/` and `src/layouts/`.
- The changelog page is generated from the repository's `../CHANGELOG.md` at build time (`src/lib/changelog.ts`): English lines on `/changelog/`, Chinese lines on `/zh/changelog/`.
- `public/` is copied as is: `assets/` (stylesheet and icons) and `_headers` (security and cache headers).
- `wrangler.jsonc` tells Cloudflare to serve `dist/` as static files (Workers static assets, no Worker code); unknown paths get `404.html`.

Cloudflare Workers Builds deploys on every push to `main`:

| Setting        | Value                         |
| -------------- | ----------------------------- |
| Root directory | `website`                     |
| Build command  | `npm ci && npm run build`     |
| Deploy command | `npx wrangler deploy`         |
| Watch paths    | `website/*`, `CHANGELOG.md`   |

The app's build and tests don't read anything in here.
