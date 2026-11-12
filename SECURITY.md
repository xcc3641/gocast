# Security Policy

## Supported versions

Only the latest release of Gocast receives security fixes. Gocast updates
itself (Settings → General → Check for updates automatically), so please make
sure you are on the newest version before reporting.

## Reporting a vulnerability

Please **do not** open a public issue for security problems.

Report them privately through GitHub: open the repository's **Security** tab
and choose **Report a vulnerability**. Include the Gocast version, your macOS
version and chip (Apple Silicon or Intel), what an attacker could do, and the
steps to reproduce.

You will get a reply within 7 days. Once a fix ships, the advisory is published
and you are credited unless you ask not to be.

Things that are especially in scope:

- the update mechanism (update feed, update signatures, the downloaded archive);
- anything that lets another app or a web page run commands through Gocast's
  terminal or presets;
- presets or settings files being read or written somewhere they should not be.
