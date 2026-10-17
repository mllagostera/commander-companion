# ADR-0027: Sideloaded Android release builds signed with the debug key

**Status:** Accepted (2026-10-16)

## Context

The APK testers install by hand is the one Android CI archives. Until now that
was `assembleDebug` built against the deployed backend, so every phone running
the app carried a debug build:

- `android:debuggable="true"`: with USB debugging on, `adb shell run-as` reads
  the session DataStore, a 30-day refresh token included, without root.
- `usesCleartextTraffic="true"` (the debug manifest, for the local HTTP backend).
- `HttpLoggingInterceptor.Level.BODY`: every request and response body went to
  logcat, so the login and password-change bodies (plain-text passwords) and
  every token pair end up in any `adb logcat` or bug report.

None of the three comes from the signing key; all three come from the build
type. A release build has none of them. The question was only which key signs
that release build: the app is not on Google Play yet, testers already have the
debug-signed APK installed, and the Google Cloud Android OAuth client is
registered with the debug key's SHA-1.

## Decision

The `release` build type signs with the existing debug signing config
(`signingConfig = signingConfigs.getByName("debug")` in `app/build.gradle.kts`).
CI restores the shared `debug.keystore` from `ANDROID_DEBUG_KEYSTORE_BASE64`,
builds `assembleRelease` against the deployed backend, and archives that as
`app-release-apk`. `versionCode` is the CI run number (`-PVERSION_CODE`), and
the job refuses an `API_BASE_URL` that isn't `https://`.

Alternative weighed: a dedicated release (upload) key now. It is where this
ends up, but today it would force every tester to uninstall the app first
(Android won't install over a different signature), which loses the local game
history — that only exists in Room, nowhere else — and it would need a second
OAuth client SHA-1, all for a protection (who can sign an update) that the
sideloaded distribution doesn't otherwise have either.

## Consequences

- The release APK installs over the debug APKs CI archived before, keeping
  their data, and Google Sign-In keeps the SHA-1 it is registered with.
- Whoever holds the keystore file can sign an update that installs over the app
  and inherits its data, session included. The password is the public
  `android`, so the file itself is the secret: it lives in the repo owner's
  `~/.android/` and in a GitHub Actions secret, which every workflow run with
  secrets can read. Acceptable for a handful of testers sideloading the app;
  not acceptable once it is distributed more widely.
- **Google Play rejects debug-signed uploads.** Moving there means generating a
  real upload key (with Play App Signing), registering its SHA-1 in a new OAuth
  Android client, and one forced reinstall for existing testers. That revisits
  this ADR.
- `isMinifyEnabled` stays `false`: turning R8 on needs its keep rules checked on
  a device and is a separate change.
