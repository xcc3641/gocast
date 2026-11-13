# Releasing

For maintainers. Releases are published as GitHub releases of
[xcc3641/gocast](https://github.com/xcc3641/gocast), which installed apps
check for updates: `updates` in [mygo.json](mygo.json) holds the public key
and that repository (see the Updates section of
[docs/dev-notes.md](docs/dev-notes.md)). Releases are built on a
maintainer's Mac, not in CI.

You need:

- The GitHub CLI (`gh`), signed in with access to the repository.
- The Developer ID Application certificate named in `macos.signingIdentity`
  in mygo.json, in a keychain codesign searches.
- The update signing key (`MYGO_UPDATER_PRIVATE_KEY`), from the
  maintainers' secret store. `mygo keygen` made it with its public half,
  which is `updates.publicKey` in mygo.json.
- Credentials for `xcrun notarytool` (an App Store Connect API key or an
  Apple ID with an app-specific password).

Never commit the update key, the certificate, or any password, paste them
anywhere, or keep them in the repository.

## Cut a release

1. Bump `version` in [mygo.json](mygo.json).
2. In [CHANGELOG.md](CHANGELOG.md), turn `## [Unreleased]` into
   `## [x.y.z] - YYYY-MM-DD` for that version. The section is required:
   `mygo build` stops without it, and it is the release notes the update
   window shows, so write it for users, in Chinese and English.
3. Make sure the checks in [CONTRIBUTING.md](CONTRIBUTING.md) pass and
   THIRD_PARTY_NOTICES.md is current. Commit, tag `vx.y.z` and push both.
4. Build and upload with the update signing key:

   ```sh
   MYGO_UPDATER_PRIVATE_KEY="$(cat path/to/mygo-update.key)" \
     mygo build -upload
   ```

   The build signs with the Developer ID in `macos.signingIdentity` under
   the hardened runtime. It builds the app, the disk image, the update
   archive and manifest (`gocast-x.y.z-darwin-arm64.tar.gz`,
   `update-darwin-arm64.json`), the deltas from earlier versions,
   `install.sh` and `SHA256SUMS`, and uploads them to a **draft** release
   for the version on xcc3641/gocast. Without the key, `mygo build` still
   builds the app and the disk image but skips the update files ("Not
   signing an update"), which is fine for local builds, and `-upload`
   refuses to run.
5. Copy the install script the build wrote over the one in the repository
   and commit it if it changed: the README points people at it.

   ```sh
   cp build/darwin-arm64/install.sh install.sh
   ```

6. Notarize the disk image and staple the ticket to it:

   ```sh
   xcrun notarytool submit build/darwin-arm64/gocast-x.y.z-arm64.dmg --wait <credentials>
   xcrun stapler staple build/darwin-arm64/gocast-x.y.z-arm64.dmg
   spctl -a -vv -t open --context context:primary-signature build/darwin-arm64/gocast-x.y.z-arm64.dmg
   ```

   `spctl` should say "source=Notarized Developer ID". `macos.notarize` in
   mygo.json is not used, as it needs a notarytool keychain profile.
7. Stapling changes the disk image, so regenerate its line of `SHA256SUMS`
   and put both back on the draft:

   ```sh
   cd build/darwin-arm64
   grep -v ' gocast-x.y.z-arm64.dmg$' ../SHA256SUMS > ../SHA256SUMS.new
   shasum -a 256 gocast-x.y.z-arm64.dmg >> ../SHA256SUMS.new
   mv ../SHA256SUMS.new ../SHA256SUMS
   shasum -a 256 -c ../SHA256SUMS   # every line should say OK
   cd ../..
   gh release upload vx.y.z build/darwin-arm64/gocast-x.y.z-arm64.dmg build/SHA256SUMS --clobber
   ```

8. Check the draft on GitHub (its notes and assets, and try the disk
   image), then publish it:

   ```sh
   gh release edit vx.y.z --draft=false --latest
   ```

   Publishing makes it the latest release: installed apps find it on their
   next automatic check, or when the user chooses Check for Updates.
9. Try the install script once it is published:

   ```sh
   curl -fsSL https://github.com/xcc3641/gocast/releases/latest/download/install.sh | sh
   ```

   GitHub's `releases/latest` can lag about 30 seconds after publishing,
   so a check right away may still get the previous release.

Installed apps update only from builds that had `updates` in mygo.json:
builds made before it never update themselves.

## Signing

Releases are signed with a Developer ID certificate and notarized by
Apple, so they open with a double-click. Builds from source
(`mygo build -sign -`) work the same, but macOS asks for permissions again
after each build. Updates and the install script check the Ed25519
signature against the public key in mygo.json.
