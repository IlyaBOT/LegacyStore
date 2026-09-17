# Snow Leopard / Xcode 3.2 native client validation

This document defines the smallest practical validation run for the LegacyStore native client on the Acer Snow Leopard development system.

The goal is to verify three things with the minimum number of builds:

1. the Xcode 3.2 project can parse and build the Tiger-compatible configuration;
2. the command-line build can produce both `i386` and `x86_64` slices and combine them into one application bundle;
3. both slices can start on Snow Leopard before further UI or network work continues.

## 1. Record the toolchain

From the repository root:

```sh
sw_vers
xcodebuild -version
which gcc-4.2 gcc-4.0 llvm-gcc-4.2 gcc 2>/dev/null
ls -ld /Developer/SDKs/MacOSX10.4u.sdk /Developer/SDKs/MacOSX10.5.sdk /Developer/SDKs/MacOSX10.6.sdk 2>/dev/null
```

Record:

- exact Snow Leopard version;
- Xcode version;
- available compiler;
- whether `MacOSX10.4u.sdk` exists;
- whether `MacOSX10.5.sdk` exists;
- whether `MacOSX10.6.sdk` exists.

The 10.4-compatible build must not silently fall back to a newer SDK.

## 2. Validate the Xcode 3.2 project

Run only the Tiger configuration first:

```sh
xcodebuild \
  -project legacy-client/LegacyStore.xcodeproj \
  -target LegacyStore \
  -configuration "Tiger i386" \
  build 2>&1 | tee /tmp/legacystore-xcode-tiger.log
```

Expected confirmation:

```text
** BUILD SUCCEEDED **
```

If it fails, record:

- the first compiler error;
- the source file and line;
- the complete error text;
- the first 20 lines around that error if there are follow-on errors.

Do not attempt to fix dozens of secondary errors before the first error is understood.

## 3. Build the release universal bundle

Run:

```sh
./scripts/build_client.sh 2>&1 | tee /tmp/legacystore-client-build.log
```

Expected result:

```text
build/legacy-client/i386/LegacyStore
build/legacy-client/x86_64/LegacyStore
build/LegacyStore.app/Contents/MacOS/LegacyStore
```

The final output from `lipo -info` must include both:

```text
i386
x86_64
```

The script must fail rather than silently dropping `i386` or raising the i386 deployment target above 10.4.

## 4. Validate the produced application bundle

Run:

```sh
chmod +x scripts/validate_client_build.sh
./scripts/validate_client_build.sh 2>&1 | tee /tmp/legacystore-client-validate.log
```

Required confirmations:

- `Info.plist` passes `plutil`;
- universal executable contains `i386`;
- universal executable contains `x86_64`;
- both slices can be extracted with `lipo`;
- linked frameworks are system frameworks only;
- no architecture is silently missing.

`LC_VERSION_MIN_MACOSX` output is informational because old Apple toolchains do not report it consistently. Deployment targets must also be confirmed from the compiler/build output.

## 5. Runtime smoke test on Snow Leopard

No installation is required. Run the executable directly.

First force the 32-bit slice:

```sh
arch -i386 build/LegacyStore.app/Contents/MacOS/LegacyStore
```

Confirm:

- application starts;
- main window appears;
- toolbar appears;
- category sidebar appears;
- search field appears;
- no immediate crash occurs.

Close LegacyStore before the next command.

Then force the 64-bit slice:

```sh
arch -x86_64 build/LegacyStore.app/Contents/MacOS/LegacyStore
```

Confirm the same points.

If `arch -x86_64` fails before LegacyStore starts, record the exact terminal error and run:

```sh
sysctl hw.cpu64bit_capable
uname -m
```

## 6. Optional public API smoke test

The native client currently uses only the public read-only API. Authentication is intentionally not part of this validation.

If a LegacyStore backend is reachable from the Acer, configure it with:

```sh
defaults write io.github.ilyabot.LegacyStore LegacyStoreServerURL "http://BACKEND_HOST:8080"
```

Then launch the application again and confirm:

- categories are loaded from `/api/v1/categories`;
- the app list is loaded from `/api/v1/apps`;
- selecting a category reloads the list;
- search returns results;
- double-clicking an application requests its app detail endpoint;
- the client does not crash on server errors or invalid/unreachable server address.

Do not test account login over this public HTTP path. Legacy authentication remains disabled until the bundled validated TLS stack exists.

## 7. What to report after the test

If everything succeeds, record exactly:

```text
Snow Leopard: <version>
Xcode: <version>
10.4u SDK: present
10.5 SDK: present/missing
10.6 SDK: present/missing
Xcode Tiger i386 build: PASS
Universal command-line build: PASS
Universal architectures: i386 x86_64
Forced i386 launch: PASS
Forced x86_64 launch: PASS
Public catalog API: PASS / NOT TESTED
```

If something fails, report:

```text
Stage: <Xcode Tiger build / command-line build / validation / i386 runtime / x86_64 runtime / API>
Command: <exact command>
First error: <exact first error>
Expected: <expected result>
Got: <actual result>
Log: </tmp/... log file>
```

The full logs to preserve are:

```text
/tmp/legacystore-xcode-tiger.log
/tmp/legacystore-client-build.log
/tmp/legacystore-client-validate.log
```

These three logs are sufficient for the next compatibility fix in most cases.
