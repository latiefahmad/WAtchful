# Changelog

All notable changes to WAtchful are documented in this file.

The **Release** workflow (`.github/workflows/build.yml`) publishes the section
for the tagged version to GitHub **verbatim** as the release notes. Conventions:

- One section per release, headed `## [vX.Y.Z] - YYYY-MM-DD` (newest first).
- Everything under the heading until the next `## [` heading is published as-is.
- Write for end users: what is new, what is fixed, and which file to download.

## [v2.0.10] - 2026-09-24

### 💬 Direct Chat

- **One-click entry on the tab strip** — a `+ person` button sits beside the version tag (system icon font, DPI-scaled) with a native "Direct Chat" tooltip; hover highlight included. Settings card, tray menu and `Ctrl/Cmd+Shift+C` keep working.
- **Profile avatars on the strip** now use the same person icon inside the accent dot, matching the button's visual language.

### ✏️ Profiles

- **The active profile can be renamed** — rename only edits the registry, so the in-use row now also offers the pencil button; the window title, tab strip and in-process profile follow the new name immediately. Switch/Reset/Delete stay hidden for the running profile (the backend refuses those for safety).

### 🔄 Updating

Updates arrive automatically in-app: WAtchful checks for new releases shortly after startup and every 4 hours, then downloads, installs, and restarts itself — you only approve. Accept the banner when it appears, or download the file for your platform from this release manually.

**Full Changelog**: https://github.com/latiefahmad/WAtchful/compare/v2.0.9...v2.0.10

## [v2.0.9] - 2026-09-24

### 💬 Direct Chat

- **Chat a new number without saving it** — new modal with target number (country code included) and a Send-From profile dropdown, opening the conversation via WhatsApp's official click-to-chat deep link.
- **Three labeled entries** — a "Direct Chat" card in Settings, a tray menu item, and `Ctrl/Cmd+Shift+C` (listed in Help). An icon-only header button was tried and removed: too easy to miss.
- **Validated twice** — client-side plus native (8–15 digits, E.164); anything else is refused with a toast and never navigates.

### 🔄 Updating

Updates arrive automatically in-app: WAtchful checks for new releases shortly after startup and every 4 hours, then downloads, installs, and restarts itself — you only approve. Accept the banner when it appears, or download the file for your platform from this release manually.

**Full Changelog**: https://github.com/latiefahmad/WAtchful/compare/v2.0.8...v2.0.9

## [v2.0.8] - 2026-09-22

### 🔒 Security

- **Self-update locked to this repository's releases** — the updater refuses any download URL outside this repo's HTTPS release artifacts, so a page script can never point it at an arbitrary executable.
- **Open-file bridge jailed** — files opened from page JavaScript must live inside the download folder (including monthly subfolders) or the internal preview directory; system files and `..` escapes are refused.
- **Preview markup escaped** — filenames, paths and release titles in the document preview and update banner are HTML-escaped, closing an XSS path from crafted chat filenames into the privileged page.
- **Size caps before allocation** — attachments over 1 GB are rejected from their encoded length before decoding, and update downloads over 512 MB are refused up front and mid-stream with no partial file left behind.
- **Download folder validated** — system and autostart locations (including Windows Startup, symlink and dangling-link escapes) are rejected before anything is created, and a hostile settings file falls back to the default folder.
- **Spreadsheet preview sanitized** — SheetJS table markup is scrubbed in an inert template (dangerous elements removed, only structural attributes kept), blocking crafted-cell markup injection.
- **Linux: no elevated mode bits from update archives, DBus error format fixed.**

### 🛠 Fixed

- **`wa_crash.log` rotates** past 1 MB into a single backup, so a crash loop can no longer grow it without bound.
- **Drag & drop recovery** — the highlight always clears (global drop/dragend/blur net), and file injection probes over several rounds, firing only into an input WhatsApp left empty.

### 🔄 Updating

Updates arrive automatically in-app: WAtchful checks for new releases shortly after startup and every 4 hours, then downloads, installs, and restarts itself — you only approve. Accept the banner when it appears, or download the file for your platform from this release manually.

**Full Changelog**: https://github.com/latiefahmad/WAtchful/compare/v2.0.7...v2.0.8

## [v2.0.7] - 2026-09-22

### 🔔 Notifications

- **Desktop notification on/off switch** — a new Settings card enables or disables OS-level alerts for new chat messages (including service-worker and update notifications), persisted across restarts. Ports upstream's notification controls.
- **Download notifications switch** — activates the long-stored `notify_on_download` flag, which previously had no reader: turn off download popups while keeping error alerts, which always show.

### 📥 Downloads

- **Explicit Download no longer opens a preview** — clicking Download itself (viewer toolbar button, context-menu item, download link) now saves only; the in-app document preview is reserved for clicking the document. Ports upstream's explicit-download behavior, including the guard against save+preview double-handling.
- **"Already saved" is back and accurate** — the saver bridge now reports whether the exact bytes already existed (SHA-256 content match) instead of guessing by file size, so repeat downloads truthfully toast "Already saved" with no `name (1).ext` copies.
- **Removed the redundant size-based dedup index** — duplicate refusal lives natively in the SHA-256 saver; the page no longer keeps per-size download state.

### 🔄 Updating

Updates arrive automatically in-app: WAtchful checks for new releases shortly after startup and every 4 hours, then downloads, installs, and restarts itself — you only approve. Accept the banner when it appears, or download the file for your platform from this release manually.

**Full Changelog**: https://github.com/latiefahmad/WAtchful/compare/v2.0.6...v2.0.7

## [v2.0.6] - 2026-09-19

### 🛠 Fixed

- **App no longer freezes ("Not responding") when switching tabs** - Win32 re-enters our window procedure on the same thread in the middle of COM calls (caught live in a stack dump: `MoveFocus` synchronously redelivers `WM_ACTIVATE`, whose handler took the already-held tab mutex). With a plain mutex that deadlocked the pump thread forever at 0% CPU. The tab mutex is now re-entrant (owner-tracked), second launches use `SendMessageTimeout` so they can never pile up behind a busy window, and a pump watchdog writes goroutine stacks to Temp on any 25-second stall. Verified with an 8-minute stability run, a hibernated-tab wake, and an IPC tab-switch - all responsive, watchdog silent.

### 🔄 Updating

Updates arrive automatically in-app: WAtchful checks for new releases shortly after startup and every 4 hours, then downloads, installs, and restarts itself — you only approve. Accept the banner when it appears, or download the file for your platform from this release manually.

**Full Changelog**: https://github.com/latiefahmad/WAtchful/compare/v2.0.5...v2.0.6

## [v2.0.5] - 2026-09-19

### 🛠 Fixed

- **Privacy mode now redacts links and URLs** — link text in WhatsApp renders as a text node directly inside `<a>` with its own explicit color, so it never inherited the redacted span color and stayed readable while everything else blurred (including after the 60-second idle auto-lock). The redaction and hover-restore rules now cover anchors everywhere spans are covered: message bubbles and chat-list previews. Verified in headless Edge against the shipped stylesheet, including a hostile `!important` link color.

### 🔄 Updating

Updates arrive automatically in-app: WAtchful checks for new releases shortly after startup and every 4 hours, then downloads, installs, and restarts itself — you only approve. Accept the banner when it appears, or download the file for your platform from this release manually.

**Full Changelog**: https://github.com/latiefahmad/WAtchful/compare/v2.0.4...v2.0.5

## [v2.0.4] - 2026-09-18

### 🛠 Fixed

- **Renderer trim throttled to once per 10 minutes** — v2.0.3 trimmed the WebView2 child working sets on every hide longer than 5 seconds, so each alt-tab dropped ~800 MB of mapped cache pages and faulted them all back on return (a Manager CPU + disk-read spike per switch, verified head-to-head against upstream). The trim now runs at most once per 10 minutes, seeded at startup: rapid window switching stays spike-free while genuinely-away periods still get the memory back.

### 🔄 Updating

Updates arrive automatically in-app: WAtchful checks for new releases shortly after startup and every 4 hours, then downloads, installs, and restarts itself — you only approve. Accept the banner when it appears, or download the file for your platform from this release manually.

**Full Changelog**: https://github.com/latiefahmad/WAtchful/compare/v2.0.3...v2.0.4

## [v2.0.3] - 2026-09-18

### ⚡ Memory

- **Working-set trim now reaches the WebView2 child processes** — `releaseMemoryNative` previously trimmed only the ~2 MB Go process while the gigabytes sat in the renderer/GPU/utility children. When the window stays hidden for 5 seconds, the app now also drops every one of its own `msedgewebview2.exe` children's working sets to minimum (pages freed by the page-side `window.gc()` leave RAM immediately instead of waiting for OS paging). Non-destructive: trimmed pages fault back from standby on next use, and unrelated Edge instances are never touched.

### ✨ Interface

- **Running build shown on the tab strip** — the strip's right edge now carries a muted `v2.0.3` tag, so every screenshot or bug report identifies the exact build. Tab layout and click hit-testing are untouched: the tag only paints into leftover empty space.

### 🔄 Updating

Updates arrive automatically in-app: WAtchful checks for new releases shortly after startup and every 4 hours, then downloads, installs, and restarts itself — you only approve. Accept the banner when it appears, or download the file for your platform from this release manually.

**Full Changelog**: https://github.com/latiefahmad/WAtchful/compare/v2.0.2...v2.0.3

## [v2.0.2] - 2026-09-18

### ⚡ Memory

- **Active-tab recycler now runs every 90 minutes (was 6 hours)** — follow-up to v2.0.1: on a live single-account session the WhatsApp Web renderer still climbs to ~1 GB after about an hour of use, and single-tab setups never hibernate, so a 6-hour cycle left the high baseline in place all day. The rebuild is unchanged (same controller close/reopen path, login kept on disk, tab position kept, ~100–150 MB after refresh) and still waits when a download is in flight or a document preview is open.
- **Leaner Chromium engine flags (Windows)** — the WebView2 engine no longer loads unused media-key/media-session services or built-in component-extension background pages, and `window.gc()` is now actually exposed to the page so hiding the window triggers a real JS heap collection instead of a no-op.

### 🔄 Updating

Updates arrive automatically in-app: WAtchful checks for new releases shortly after startup and every 4 hours, then downloads, installs, and restarts itself — you only approve. Accept the banner when it appears, or download the file for your platform from this release manually.

**Full Changelog**: https://github.com/latiefahmad/WAtchful/compare/v2.0.1...v2.0.2

## [v2.0.1] - 2026-09-18

### ⚡ Memory

- **Renderer recycler for the active tab (Windows)** — measured on a live session: WhatsApp Web's renderer process climbs to ~0.7–0.9 GB after an hour of use (chat media, decoded images, DOM) and never gives that memory back, no matter how long the app idles. The tab shell now rebuilds the active tab's engine in place after 6 hours of page age, using the same controller close/reopen path as tab hibernation: the login lives on disk, the tab keeps its position, and the fresh renderer resumes at ~100–150 MB. The rebuild waits when a download is in flight or an in-app document preview is open (the page reports this to the host), and retries on the next sweep if the rebuild was skipped.
- Background tabs are unchanged: they already suspend after 30 s and hibernate (engine closed, ~400 MB returned) after 2 minutes.

### 🔄 Updating

Updates arrive automatically in-app: WAtchful checks for new releases shortly after startup and every 4 hours, then downloads, installs, and restarts itself — you only approve. Accept the banner when it appears, or download the file for your platform from this release manually.

**Full Changelog**: https://github.com/latiefahmad/WAtchful/compare/v2.0.0...v2.0.1

## [v2.0.0] - 2026-09-18

First release published from this repository. WAtchful is a small desktop wrapper for the official WhatsApp Web that uses the web engine already provided by each operating system — WebKit on macOS, WebView2 on Windows, WebKitGTK on Linux.

### ✨ Highlights

- **Settings shortcut works again** — `Ctrl + ,` (Windows/Linux) and `Cmd + ,` (macOS) open the Settings modal reliably on every platform. The Profiles UI script is now injected into the page init script in the correct order, so the modal opens from the keyboard, the toolbar button, and the tray menu alike.
- **Profiles card wired into Settings** — the multi-profile card is registered inside the Settings modal: create, switch, and remove profiles without leaving the page. Each profile keeps a fully isolated browser session, so accounts never log each other out.
- **Flat memory on long sessions** — the download deduplication index is now capped (100 entries, oldest evicted first). Previously every downloaded file left a permanent entry behind, so RAM crept up over days-long sessions. Disk cache keeps its 512 MB budget with startup and minimize purges, and idle GC trims the working set when the window is hidden.

### 🛠 Fixed

- `Ctrl + ,` / `Cmd + ,` no longer swallowed by the native web engine; the in-page shortcut handler registers without runtime errors.
- Cross-platform build restored for the vendored webview profiles patch (cgo symbol resolution on Linux/macOS, Objective-C class ordering and duplicate-symbol issues on macOS).
- Windows release packaging now works on CI (PowerShell `Compress-Archive` fallback when `zip` is unavailable).

### 📦 Downloads

| Platform | File |
| --- | --- |
| Windows 10/11 (x64) | `WAtchful.exe` or `WAtchful-Windows-x64.zip` |
| macOS (Apple Silicon + Intel) | `WAtchful-macOS-Universal.dmg` or `.zip` |
| Debian / Ubuntu (x64) | `WAtchful-Linux-amd64.deb` |
| Linux (x64, portable) | `WAtchful-Linux-x64.tar.gz` |
| Debian / Ubuntu (arm64) | `WAtchful-Linux-arm64.deb` |
| Linux (arm64, portable) | `WAtchful-Linux-arm64.tar.gz` |

### 🔄 Updating

The built-in update checker watches this repository's releases. WAtchful 1.5.x points at the upstream project, so it will not offer this release — please download 2.0.0 manually once. From 2.0.0 onward, updates arrive automatically in-app.

### ✅ Quality gates

- `go build` and the full `go test` suite pass on all platforms.
- A permanent CDP smoke test sends a **real** `Ctrl + ,` keyboard event into a real Chromium engine and asserts the Settings modal opens, the Profiles card is present, and the shortcut toggles the modal closed. It runs in CI on every pull request that touches the init script or platform integration files.

WAtchful is an independent application and is not affiliated with Meta. Licensed under the MIT License.

**Full Changelog**: https://github.com/latiefahmad/WAtchful/compare/v1.5.9.2...v2.0.0
