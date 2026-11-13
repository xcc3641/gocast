# Gocast website

The site at gocast.yichenlab.com: a landing page, Support and Privacy, in English (`/`) and Simplified Chinese (`/zh/`).

- `dist/` is what gets served. Plain HTML and one stylesheet, no framework, nothing loaded from other sites.
- `build.py` writes the six pages into `dist/` from the copy at its top. Change the copy there, run `python3 website/build.py`, and commit `dist/` with it.
- `wrangler.jsonc` tells Cloudflare to serve `dist/` as static files (Workers static assets, no Worker code). `dist/_headers` sets security and cache headers; `dist/404.html` answers unknown paths.

Cloudflare builds this folder on every push to `main` (root directory `website`, no build command, deploy command `npx wrangler deploy`). The app's build and tests don't read anything in here.
