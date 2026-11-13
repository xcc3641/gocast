// The configuration of `mygo build` (MyGo docs/configuration.md). The
// signing identity is the Developer ID of the Mac that builds a release,
// from MACOS_SIGNING_IDENTITY (RELEASING.md); without it the app is signed
// ad hoc, which runs only on the Mac that built it.
export default {
  name: "Gocast",
  identifier: "com.hugo.gocast",
  version: "0.1.6",
  icon: "icon/icon.png",
  out: "build",
  resources: ["THIRD_PARTY_NOTICES.md", "LICENSE", "NOTICE"],
  macos: {
    minimumSystemVersion: "13.0",
    signingIdentity: process.env.MACOS_SIGNING_IDENTITY ?? "-",
    icon: "icon/AppIcon.icon",
    infoPlist: {
      LSUIElement: true,
    },
    entitlements: "entitlements.plist",
  },
  i18n: {
    dir: "locales",
    source: "en",
  },
  updates: {
    publicKey: "7xqUsSnSTZgGQ69hBuh+pm1jexm7Z3Ngl0lQ17EKPRQ=",
    github: "xcc3641/gocast",
  },
};
