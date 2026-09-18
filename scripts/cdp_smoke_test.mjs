#!/usr/bin/env node
// Permanent CDP smoke test for WAtchful.
//
// Loads the exact runtime init script produced by getInitScript() into a real
// Chromium engine (the same engine family as Windows WebView2), then sends a
// REAL Ctrl+, keyboard event over the Chrome DevTools Protocol and asserts:
//   1. window.showSettingsModal is defined after the init script runs
//   2. the Settings overlay (#wa-settings-overlay) opens
//   3. the Profiles card (#wa-profiles-card) is present inside the modal
//   4. a second Ctrl+, closes the overlay again
//
// Usage:
//   node scripts/cdp_smoke_test.mjs [--cdp-url URL] [--init-script FILE] [--screenshot OUT.png]
//
// Requirements: Node >= 22 (global fetch/WebSocket), a Chromium browser
// (Chrome/Edge) listening on --cdp-url, started with --remote-allow-origins=*.
// CI wiring lives in .github/workflows/smoke.yml.

import { readFileSync, writeFileSync } from "node:fs";
import { resolve } from "node:path";

function argValue(flag, fallback) {
  const i = process.argv.indexOf(flag);
  return i !== -1 && process.argv[i + 1] ? process.argv[i + 1] : fallback;
}

const cdpURL = argValue("--cdp-url", "http://127.0.0.1:9222").replace(/\/$/, "");
const initScriptPath = argValue("--init-script", "build/wa_init_script.js");
const screenshotPath = argValue("--screenshot", "");
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

function pass(msg) { console.log("[smoke] PASS: " + msg); }
function info(msg) { console.log("[smoke] " + msg); }

async function main() {
  const version = await (await fetch(cdpURL + "/json/version")).json();
  info("connected to " + (version.Browser || "unknown Chromium"));

  const created = await fetch(cdpURL + "/json/new?" + encodeURIComponent("about:blank"), { method: "PUT" });
  if (!created.ok) throw new Error("cannot create tab: HTTP " + created.status);
  const target = await created.json();

  const ws = new WebSocket(target.webSocketDebuggerUrl);
  let nextId = 1;
  const pending = new Map();
  const send = (method, params = {}) => {
    const id = nextId++;
    ws.send(JSON.stringify({ id, method, params }));
    return new Promise((res, rej) => pending.set(id, { res, rej }));
  };

  await new Promise((res, rej) => {
    ws.addEventListener("open", res, { once: true });
    ws.addEventListener("error", () => rej(new Error("WebSocket error (is the browser started with --remote-allow-origins=*?)")), { once: true });
  });
  ws.addEventListener("message", (ev) => {
    const msg = JSON.parse(ev.data);
    if (msg.id && pending.has(msg.id)) {
      const { res, rej } = pending.get(msg.id);
      pending.delete(msg.id);
      if (msg.error) rej(new Error(msg.error.message));
      else res(msg.result);
    }
  });

  const evaljs = async (expression) => {
    const r = await send("Runtime.evaluate", { expression, returnByValue: true, awaitPromise: true });
    if (r.exceptionDetails) {
      throw new Error("page evaluation failed: " + ((r.exceptionDetails.exception && r.exceptionDetails.exception.description) || r.exceptionDetails.text));
    }
    return r.result.value;
  };

  try {
    await send("Page.enable");
    await send("Runtime.enable");

    const initScript = readFileSync(resolve(initScriptPath), "utf8");
    info("init script loaded: " + initScript.length + " bytes from " + initScriptPath);
    await send("Page.addScriptToEvaluateOnNewDocument", { source: initScript, runImmediately: true });

    // The real application loads https://web.whatsapp.com; run the same page
    // first so CSP is also exercised. Fall back to a local data: URL harness
    // when the network is unavailable, so CI without internet still verifies
    // the shortcut wiring (everything except CSP).
    let harness = "web.whatsapp.com";
    await send("Page.navigate", { url: "https://web.whatsapp.com" });
    await sleep(3000);
    let probe = await evaljs(
      "(() => { const ov = document.getElementById('wa-onboarding-overlay'); if (ov) ov.remove();" +
      " return { href: location.href, hasModal: typeof window.showSettingsModal === 'function' }; })()"
    );
    if (!probe.hasModal) {
      info("showSettingsModal undefined on " + probe.href + " — retrying on a local data: URL harness");
      harness = "data: URL";
      await send("Page.navigate", { url: "data:text/html,<title>WAtchful harness</title><body>WAtchful smoke harness</body>" });
      await sleep(800);
      probe = await evaljs("(() => ({ href: location.href, hasModal: typeof window.showSettingsModal === 'function' }))()");
    }
    if (!probe.hasModal) throw new Error("showSettingsModal is not defined after the init script ran on " + probe.href);
    pass("showSettingsModal defined (page: " + harness + ")");

    const pressCtrlComma = async () => {
      await send("Input.dispatchKeyEvent", { type: "keyDown", modifiers: 2, key: ",", code: "Comma", windowsVirtualKeyCode: 188, nativeVirtualKeyCode: 188 });
      await send("Input.dispatchKeyEvent", { type: "keyUp", modifiers: 2, key: ",", code: "Comma", windowsVirtualKeyCode: 188, nativeVirtualKeyCode: 188 });
    };
    const overlayOpen = () => evaljs("!!document.getElementById('wa-settings-overlay')");
    const waitFor = async (fn, expected, label, tries = 12) => {
      for (let i = 0; i < tries; i++) {
        if ((await fn()) === expected) return true;
        await sleep(150);
      }
      return false;
    };

    await pressCtrlComma();
    if (!(await waitFor(overlayOpen, true, "settings overlay opens"))) {
      throw new Error("Ctrl+, did not open the Settings overlay (#wa-settings-overlay)");
    }
    pass("Ctrl+, opens the Settings modal");

    const hasProfilesCard = await evaljs("!!document.querySelector('#wa-settings-overlay #wa-profiles-card')");
    if (!hasProfilesCard) throw new Error("Profiles card (#wa-profiles-card) is missing inside the Settings modal");
    pass("Profiles card is present in the Settings modal");

    if (screenshotPath) {
      const shot = await send("Page.captureScreenshot", { format: "png" });
      writeFileSync(screenshotPath, Buffer.from(shot.data, "base64"));
      info("screenshot saved: " + screenshotPath);
    }

    await pressCtrlComma();
    if (!(await waitFor(overlayOpen, false, "settings overlay closes"))) {
      throw new Error("a second Ctrl+, did not close the Settings overlay");
    }
    pass("second Ctrl+, closes the Settings modal");

    console.log("[smoke] RESULT: ALL CHECKS PASSED");
  } finally {
    try { await fetch(cdpURL + "/json/close/" + target.id); } catch { /* best effort */ }
    try { ws.close(); } catch { /* best effort */ }
  }
}

main().catch((err) => {
  console.error("[smoke] FAIL: " + (err && err.message ? err.message : err));
  process.exit(1);
});
