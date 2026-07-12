# LegacyStore Agent Instructions

## Project

LegacyStore is a software catalog and downloader for old Intel Mac OS X / macOS versions 10.5 through 10.15.

## Hard Requirements

- Do not use Node.js for backend.
- Backend must be written in Go.
- Database must be PostgreSQL.
- Native legacy client must be written in Objective-C and AppKit.
- Do not use Swift in the legacy client.
- Do not use SwiftUI.
- Do not use Storyboards.
- Do not use ARC in the legacy client.
- Do not use Auto Layout as a hard dependency.
- Target native client: Mac OS X 10.5 through macOS 10.15 Intel.
- Target architectures: i386 and x86_64.
- PPC is out of scope.
- Do not silently drop i386.
- Do not silently increase deployment target.
- Keep API responses compact.
- Use pagination everywhere.
- Do not implement software uploads in the legacy client.
- Uploads are allowed only through the HTTPS web interface.
- Legacy client must never send the primary account password.
- Legacy client may use only app-specific passwords.
- No authentication is allowed without working HTTPS/TLS.
- If TLS validation fails, authentication must be blocked.
- Hash mismatch must not delete the downloaded file automatically.
- Downloaded files must never auto-open after download.

## UI

Use the provided UI concept images as reference.

The native client should visually resemble the classic Mac App Store / OS X Mavericks-era AppKit interface.

The web UI should visually resemble the native app, but without a fake native window frame.

If UI concept images are missing, ask for them instead of inventing the interface.

## Development Style

- Keep code simple.
- Avoid large dependencies without justification.
- Do not make arbitrary product decisions.
- Do not change the architecture without explicit approval.
- Add clear TODO comments where old macOS toolchain-specific work is required.