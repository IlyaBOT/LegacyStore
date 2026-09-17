# LegacyStore Agent Instructions

## Project

LegacyStore is a software catalog and downloader for old Intel Mac OS X / macOS versions 10.4 through 10.15.

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
- Target native client: Mac OS X 10.4 through macOS 10.15 Intel.
- Target architectures: i386 and x86_64.
- Mac OS X 10.4 is i386-only for the Cocoa client; 64-bit Cocoa starts with Mac OS X 10.5.
- Keep one source tree. It is acceptable to build the i386/Tiger and x86_64/Leopard+ slices separately and combine the executable into one universal application bundle.
- PPC is out of scope.
- Do not silently drop i386.
- Do not silently increase the i386 deployment target above 10.4.
- Do not set an x86_64 deployment target lower than 10.5.
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

## Legacy Client Compatibility Rules

The Tiger build must remain compatible with the Mac OS X 10.4u SDK and Objective-C 1.x style code where practical.

Avoid language/runtime features that make the 10.4 build impossible, including:

- ARC;
- blocks;
- Objective-C literals;
- fast enumeration;
- mandatory Objective-C 2.0 properties;
- APIs introduced after 10.4 without runtime guards or an alternative path.

Prefer explicit ivars, manual memory management, NSEnumerator loops, AppKit controls available in 10.4, and Foundation APIs available in 10.4.

## UI

Use the provided UI concept images as reference.

The native client should visually resemble the classic Mac App Store / OS X Mavericks-era AppKit interface while remaining implementable with older AppKit APIs.

The web UI should visually resemble the native app, but without a fake native window frame.

If UI concept images are missing, ask for them instead of inventing the interface.

## Development Style

- Keep code simple.
- Avoid large dependencies without justification.
- Do not make arbitrary product decisions.
- Do not change the architecture without explicit approval.
- Add clear TODO comments where old macOS toolchain-specific work is required.
