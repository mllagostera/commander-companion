# Screenshots: landing page and Play Store

Regenerates every marketing screenshot from the real apps against seeded demo
data:

| Output | Script |
|---|---|
| `web/public/landing/dashboard.webp`, `life-tracker.webp` | `capture-web.mjs` |
| `android/store/screenshots/phone/*.png` (Play Store, phone) | `capture-android.mjs` |
| `web/public/landing/android.webp` (landing "coming to Android" section) | made from `02-statistics.png`, see step 5 |
| `web/public/og-image.png` and the app icons | `og-image.py` |

`web/public/landing/tournament.webp` isn't regenerated yet: it shows no app
name and was still accurate when this tooling was written.

## Sequence

All commands run from the repo root.

1. **A separate, empty stack.** Use its own Compose project so your
   development database is untouched:

   ```sh
   docker compose stop                       # frees ports 8080/5432 if the dev stack is up
   docker compose -p tc-capture down -v      # start from an empty database
   docker compose -p tc-capture up -d db api
   ```

2. **Seed the demo data.** Five players with real commander art (from
   Scryfall), two playgroups, 20 finished games, friendships and a tournament
   in round 2, all through the public API:

   ```sh
   cd scripts/screenshots && npm install && cd ../..
   node scripts/screenshots/seed.mjs
   ```

   The demo login is `chandra@demo.tapeandocartones.es` / `demo-password-123`
   (see `demo.mjs`). The seed expects an empty database; re-run step 1 first
   to start over.

3. **Web captures.** Build and start the web client against the local API,
   then capture:

   ```sh
   cd web && npx nuxt build
   PORT=3000 NUXT_API_BASE=http://localhost:8080/api/v1 NUXT_PUBLIC_API_BASE=http://localhost:8080/api/v1 node .output/server/index.mjs &
   cd .. && node scripts/screenshots/capture-web.mjs
   ```

4. **Android captures.** Play Store phone screenshots can be at most 2:1, so
   use an AVD with a 1080x2160 screen (in Android Studio's Device Manager:
   a phone hardware profile at 1080x2160, density 420). Boot it, install a
   debug build (its default `API_BASE_URL` is `10.0.2.2:8080`, the host's
   local API) and capture:

   ```sh
   cd android && ./gradlew :app:installDebug && cd ..
   node scripts/screenshots/capture-android.mjs            # every screen
   node scripts/screenshots/capture-android.mjs tracker    # or just one step
   ```

   The script logs in, switches the status bar to demo mode (10:00, full
   battery, no notifications) and locates every element through
   `uiautomator dump`. The tracker step finishes a real group game, which the
   backend records, so capture the statistics before it (the default order
   does) or reseed. On Windows with Git Bash, set `MSYS_NO_PATHCONV=1` so
   `adb` paths like `/sdcard` aren't rewritten. `ADB` overrides the `adb`
   location if it isn't at `$LOCALAPPDATA/Android/Sdk/platform-tools/adb`.

5. **The landing's phone image** is the statistics capture, scaled down:

   ```sh
   node -e "require('./scripts/screenshots/node_modules/sharp')('android/store/screenshots/phone/02-statistics.png').resize(540,1080).webp({quality:82}).toFile('web/public/landing/android.webp')"
   ```

6. **Share image.** `python scripts/screenshots/og-image.py` (Pillow, and the
   Segoe UI fonts from Windows), then bump the `?v=` on `ogImage` in
   `web/app/components/LandingPage.vue` so link previews refetch it.
