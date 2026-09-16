# Current Task

Finish the core backend required by the native client and begin the native Objective-C/AppKit client.

## Backend scope

Complete the catalog/backend features required for native development:

1. Keep the existing public catalog endpoints working.
2. Finish admin CRUD for:
   - application versions;
   - artifacts;
   - artifact mirrors;
   - icons;
   - screenshots.
3. Add input validation for the new admin endpoints.
4. Add moderation queue entries for artifacts submitted by trusted users.
5. Keep authenticated endpoints HTTPS-only.
6. Do not trust proxy HTTPS headers unless proxy-header trust is explicitly enabled in configuration.
7. Keep JSON responses compact and paginated.
8. Extend API tests for the completed admin/catalog behavior where practical.

OAuth provider integration and real file upload transport are not part of this task. Their API surfaces may remain integration points, but no insecure placeholder behavior may be introduced.

## Native client scope

Create the first functional Objective-C/AppKit client implementation.

Target systems:

- Mac OS X 10.4 Tiger Intel: i386;
- Mac OS X 10.5 Leopard and newer: i386/x86_64 where supported;
- macOS 10.15 Catalina: x86_64;
- PPC is out of scope.

Requirements:

1. One source tree.
2. No Swift, SwiftUI, Storyboards, ARC, blocks, Objective-C literals or fast enumeration.
3. Programmatic AppKit UI is preferred for the initial implementation.
4. Main window must contain:
   - classic toolbar;
   - sidebar;
   - search field;
   - Featured/Catalog content area.
5. Implement system detection:
   - macOS version;
   - current process architecture;
   - 64-bit CPU capability where available;
   - 32-bit application support.
6. Implement a lightweight JSON parser that does not require NSJSONSerialization.
7. Implement the first public API client for bootstrap, categories, app list, search and app detail.
8. No primary-account password support in the legacy client.
9. Do not add legacy authentication until the bundled TLS backend is in place.
10. Add build tooling for Xcode 3.2-era SDKs.

## Build strategy

The preferred final package is one LegacyStore.app with a fat executable:

- i386 slice: Mac OS X 10.4 deployment target;
- x86_64 slice: Mac OS X 10.5 deployment target.

The two slices may be built separately and combined with `lipo` because 64-bit Cocoa application support begins with Mac OS X 10.5.

## Do not implement yet

- PPC support;
- Mac OS Classic/System 7-9 support;
- real P2P downloader;
- production OAuth provider exchange;
- software upload from the legacy client;
- payment system;
- Apple Silicon native client;
- full visual polish of every App Store screen.
