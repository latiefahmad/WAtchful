# Changelog

All notable changes to WAtchful are documented in this file.

The **Release** workflow (`.github/workflows/build.yml`) publishes the section
for the tagged version to GitHub **verbatim** as the release notes. Conventions:

- One section per release, headed `## [vX.Y.Z] - YYYY-MM-DD` (newest first).
- Everything under the heading until the next `## [` heading is published as-is.
- Write for end users: what is new, what is fixed, and which file to download.

## [Unreleased]

### ⚡ Memory

- **Renderer recycler for the active tab (Windows)** — measured on a live session: WhatsApp Web's renderer process climbs to ~0.7–0.9 GB after an hour of use (chat media, decoded images, DOM) and never gives that memory back, no matter how long the app idles. The tab shell now rebuilds the active tab's engine in place after 6 hours of page age, using the same controller close/reopen path as tab hibernation: the login lives on disk, the tab keeps its position, and the fresh renderer resumes at ~100–150 MB. The rebuild waits when a download is in flight or an in-app document preview is open (the page reports this to the host), and retries on the next sweep if the rebuild was skipped.
- Background tabs are unchanged: they already suspend after 30 s and hibernate (engine closed, ~400 MB returned) after 2 minutes.

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
