<!--
Commander Companion PR template. The rules behind it are in AGENTS.md §7
(definition of done) and §8 (git workflow) — this is the short form, not a
replacement. Delete any section that does not apply.
-->

## What changed

<!-- One or two sentences: what this does and why. English, imperative. -->

## Screenshot

<!--
Required when the PR changes what the web or the Android app looks like —
layout, styles, components, icons, or UI copy. One image per screen touched,
before/after when it modifies something that already existed, a clip or a GIF
when it is an animation or a flow. A reviewer cannot judge a visual change from
a diff.

Exception (Android only): if the change was made in an environment with no
Android SDK and no device/emulator, drop the image and say so instead — "no
Android SDK/emulator available in this environment, screenshot pending" — plus a
line describing what changed on screen. Capability, not convenience: with an SDK
and an emulator to hand the screenshot is still required, and the web has no
such excuse.

Not a visual change? Delete this section.
-->

## Checklist

- [ ] `sh .github/scripts/check-architecture.sh` passes
- [ ] The area's gate passes locally: `make lint && make test` / `npm run lint && npm run typecheck && npm run build` / `./gradlew lintDebug testDebugUnitTest`
- [ ] It functionally works — not just compiles, no stub returning dummy data
- [ ] Contract first: `openapi.yaml` / `schema.dbml` edited before the code they govern
- [ ] New user-facing strings added to **every** locale (`es`, `en`, `ca`)
- [ ] Screenshot attached above, or the Android-SDK exception stated
- [ ] `TASKS.md` updated in this same change; `DECISIONS-LOG.md` entry if there is narrative worth keeping
- [ ] ADR added for a non-trivial technical decision (`docs/decisions/`)

## Notes for the reviewer

<!-- Optional: trade-offs, what you deliberately left out, follow-ups. -->
