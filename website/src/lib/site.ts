// Links and strings shared by both languages.
export const REPO = "https://github.com/xcc3641/gocast";
export const DMG = `${REPO}/releases/latest`;
export const INSTALL =
  "curl -fsSL https://github.com/xcc3641/gocast/releases/latest/download/install.sh | sh";
export const LICENSE = `${REPO}/blob/main/LICENSE`;

// The favicon, drawn inline so it costs no request.
export const ICON_SVG =
  "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 64 64'%3E" +
  "%3Crect width='64' height='64' rx='14' fill='%231C1C1E'/%3E" +
  "%3Crect x='25' y='17' width='14' height='30' rx='3' fill='white'/%3E%3C/svg%3E";

// The stylesheet's address carries a hash of its contents: /assets/* is
// cached for a day, so a browser that saw the old file would otherwise pair
// it with new pages until the day runs out.
import { createHash } from "node:crypto";
import siteCss from "../../public/assets/site.css?raw";
export const SITE_CSS = `/assets/site.css?v=${createHash("sha256").update(siteCss).digest("hex").slice(0, 10)}`;
