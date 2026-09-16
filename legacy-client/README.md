# LegacyStore Legacy Client

Native Objective-C/AppKit client for Intel Mac OS X.

## Target systems

The client uses one source tree and two executable slices:

- `i386`: Mac OS X 10.4 Tiger and newer systems which can run 32-bit applications;
- `x86_64`: Mac OS X 10.5 Leopard and newer.

The release build combines both slices with `lipo` into one `LegacyStore.app` bundle.

PPC is not supported.

## Current implementation

The first native implementation contains:

- programmatic AppKit main window;
- classic toolbar with Featured, Top Charts, Categories, Downloads and Updates entries;
- sidebar populated from the server categories endpoint;
- search field;
- catalog table;
- compatibility status and recommended version display;
- system/macOS/architecture detection;
- lightweight Foundation-only JSON parser;
- asynchronous public catalog API client.

Authentication and downloading are intentionally not enabled yet. Legacy authentication must not be added until the bundled TLS backend is available.

## Xcode 3.2 project

Open:

```text
legacy-client/LegacyStore.xcodeproj
```

The project contains two configurations:

```text
Tiger i386
Leopard x86_64
```

The Tiger configuration expects the `MacOSX10.4u.sdk`. The x86_64 configuration expects the Mac OS X 10.5 SDK.

## Universal command-line build

On a Leopard/Snow Leopard development host with an Xcode 3.x toolchain:

```sh
./scripts/build_client.sh
```

The script builds:

```text
build/legacy-client/i386/LegacyStore
build/legacy-client/x86_64/LegacyStore
```

and combines them into:

```text
build/LegacyStore.app
```

If the SDKs are installed at non-standard paths:

```sh
LEGACYSTORE_TIGER_SDK=/path/to/MacOSX10.4u.sdk \
LEGACYSTORE_LEOPARD_SDK=/path/to/MacOSX10.5.sdk \
./scripts/build_client.sh
```

## Development server

The first client build uses the public, read-only API. The default server URL is:

```text
http://localhost:8080
```

It can be changed for development with the `LegacyStoreServerURL` user default:

```sh
defaults write io.github.ilyabot.LegacyStore LegacyStoreServerURL "http://server.example:8080"
```

Do not use this public HTTP path for authentication. Legacy account login will only be implemented over a bundled validated TLS stack.

## Source compatibility rules

Do not introduce features which break the Tiger build:

- ARC;
- blocks;
- Objective-C literals;
- fast enumeration;
- mandatory Objective-C 2.0 properties;
- APIs newer than 10.4 without runtime guards or an alternative implementation.

Use manual memory management and older Foundation/AppKit APIs.
