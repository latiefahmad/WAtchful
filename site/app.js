(function () {
  "use strict";

  var ASSETS = {
    mac: {
      label: { id: "Unduh untuk macOS", en: "Download for macOS" },
      hint: "macOS",
      url: "https://github.com/latiefahmad/WAtchful/releases/latest/download/WAtchful-macOS-Universal.dmg"
    },
    windows: {
      label: { id: "Unduh untuk Windows", en: "Download for Windows" },
      hint: "Windows",
      url: "https://github.com/latiefahmad/WAtchful/releases/latest/download/WAtchful.exe"
    },
    linuxArm: {
      label: { id: "Unduh untuk Linux ARM64", en: "Download for Linux ARM64" },
      hint: "Linux ARM64",
      url: "https://github.com/latiefahmad/WAtchful/releases/latest/download/WAtchful-Linux-arm64.deb"
    },
    linux: {
      label: { id: "Unduh untuk Linux", en: "Download for Linux" },
      hint: "Linux",
      url: "https://github.com/latiefahmad/WAtchful/releases/latest/download/WAtchful-Linux-x64.tar.gz"
    }
  };

  var STATIC_VERSION = "v2.0.14";
  var STATIC_FILES = 8;
  var REPO_API = "https://api.github.com/repos/latiefahmad/WAtchful/releases/latest";

  var PRESETS = {
    day: { start: 540, end: 1020 },
    night: { start: 1260, end: 420 },
    all: { start: 0, end: 0 }
  };

  var I18N = {
    id: {
      "meta.title": "WAtchful — WhatsApp di meja kerja Anda",
      "meta.description": "WAtchful: WhatsApp Web sebagai aplikasi desktop. Balas otomatis AFK berbatas, jadwal berulang, Quick Replies, pratinjau dokumen, multi-profil, pembaruan sendiri.",
      "nav.features": "Fitur",
      "nav.demo": "Coba AFK",
      "nav.download": "Unduh",
      "hero.eyebrow": "Aplikasi desktop mandiri untuk WhatsApp",
      "hero.title": "WhatsApp,<br><em>di meja kerja Anda.</em>",
      "hero.lead": "WhatsApp Web sebagai aplikasi desktop: multi-profil, balas otomatis AFK berbatas, jadwal berulang, pratinjau dokumen, dan pembaruan sendiri.",
      "hero.version": STATIC_VERSION,
      "hero.download": "Unduh rilis terbaru",
      "hero.allBuilds": "Lihat semua build",
      "hero.detecting": "Mendeteksi platform Anda…",
      "hero.detected": "Terdeteksi: {name}. Anda bisa memilih build lain di bawah.",
      "hero.choose": "Pilih platform di bawah untuk installer yang benar.",
      "stats.version": "Versi",
      "stats.downloads": "Unduhan",
      "stats.assets": "Berkas rilis",
      "features.eyebrow": "Fitur",
      "features.title": "Semua yang penting, tanpa ribet.",
      "f1t": "Balas otomatis AFK",
      "f1b": "Hanya membalas nama/nomor yang Anda daftarkan, hanya di jam yang Anda tentukan (termasuk lintas malam). Status aplikasi selalu menjelaskan alasan ia diam.",
      "f2t": "Jadwal berulang",
      "f2b": "Sekali, harian, mingguan (pilih hari), bulanan, atau tahunan — masing-masing dengan profil pengirim sendiri.",
      "f3t": "Quick Replies",
      "f3b": "Ketik /trigger + Spasi untuk mengisi templat; {name}, {date}, {time} terisi otomatis.",
      "f4t": "Dokumen & unduhan",
      "f4b": "Pratinjau PDF dan Office di dalam jendela; berkas sama tidak menumpuk berkat pencocokan isi (SHA-256).",
      "f5t": "Multi-profil hemat daya",
      "f5b": "Satu tab per akun dengan data terisolasi; tab tersembunyi ditangguhkan agar CPU dan RAM lega.",
      "f6t": "Privat",
      "f6b": "Mode privasi menyamarkan nama dan isi chat; semua data di mesin Anda, tanpa analitik.",
      "demo.eyebrow": "Coba langsung",
      "demo.title": "Aturan AFK, tanpa menunggu pesan masuk.",
      "demo.lead": "Logika yang sama dipakai aplikasi: ubah daftar atau jamnya, lihat siapa yang bakal dibalas.",
      "demo.allowTitle": "Daftar izin (satu per baris)",
      "demo.nameTitle": "Nama chat yang masuk",
      "demo.hoursTitle": "Jam balas",
      "demo.presetDay": "Siang 09–17",
      "demo.presetNight": "Malam 21–07",
      "demo.presetAll": "24 jam",
      "demo.hoursAll": "sepanjang hari — jam berapa pun",
      "demo.hoursActive": "aktif sekarang — {range}",
      "demo.hoursIdle": "di luar jam — {range}",
      "demo.allowYes": "Bakal dibalas — cocok dengan {entry}",
      "demo.allowNo": "Tidak dibalas — tidak ada entri yang cocok",
      "demo.allowEmpty": "Daftar izin kosong — tidak ada yang dibalas",
      "demo.note": "Nama cocok sebagian tanpa peduli huruf besar-kecil; angka dicocokkan per digit (min. 8, awalan +62/0 diabaikan). Daftar kosong berarti tidak ada yang dibalas.",
      "dl.eyebrow": "Unduhan",
      "dl.title": "Pilih build yang sesuai",
      "dl.mac": "Apple Silicon dan Intel, dipasang lewat DMG.",
      "dl.win": "Windows 10/11, butuh WebView2 (umumnya sudah terpasang).",
      "dl.lin": "Debian/Ubuntu, Fedora/RHEL, atau arsip portabel.",
      "dl.arm": "Paket DEB atau arsip portabel untuk mesin ARM64.",
      "dl.dmg": "DMG ↗",
      "dl.zip": "ZIP ↗",
      "dl.exe": "EXE ↗",
      "dl.deb": "DEB ↗",
      "dl.tar": "tar.gz ↗",
      "faq.eyebrow": "FAQ",
      "faq.title": "Sebelum Anda mengunduh",
      "q1t": "Apakah chat saya dikirim ke server?",
      "q1b": "Tidak. WAtchful memakai mesin web bawaan sistem dan tidak punya server analitik atau relay pesan.",
      "q3t": "Bagaimana cara pembaruannya?",
      "q3b": "WAtchful memeriksa rilis baru segera setelah mulai dan setiap 4 jam, lalu mengunduh, memasang, dan memulai ulang sendiri — Anda hanya menyetujui banner.",
      "q4t": "Bisakah beberapa akun jalan bersamaan?",
      "q4b": "Bisa. Setiap profil punya tab sendiri dengan data terisolasi; tab tersembunyi otomatis ditangguhkan.",
      "q5t": "Apa arti “hanya balas daftar”?",
      "q5b": "Fitur AFK punya daftar izin: hanya nama atau nomor di daftar yang dibalas otomatis. Sisanya dilewati tanpa dibuka.",
      "support.title": "Suka WAtchful?",
      "support.cta": "Donate ↗",
      "foot.left": "WAtchful adalah aplikasi independen dan tidak berafiliasi dengan Meta."
    },
    en: {
      "meta.title": "WAtchful — WhatsApp at your desk",
      "meta.description": "WAtchful turns WhatsApp Web into a desktop app: bounded AFK auto-reply, repeating schedules, Quick Replies, document previews, multiple profiles, self-updates.",
      "nav.features": "Features",
      "nav.demo": "Try AFK",
      "nav.download": "Download",
      "hero.eyebrow": "Independent desktop companion for WhatsApp",
      "hero.title": "WhatsApp,<br><em>at your desk.</em>",
      "hero.lead": "WhatsApp Web as a desktop app: multi-profile, bounded AFK auto-reply, repeating schedules, document previews, and self-updates.",
      "hero.version": STATIC_VERSION,
      "hero.download": "Download latest release",
      "hero.allBuilds": "See all builds",
      "hero.detecting": "Detecting your platform…",
      "hero.detected": "Detected: {name}. You can pick another build below.",
      "hero.choose": "Choose your platform below for the correct installer.",
      "stats.version": "Version",
      "stats.downloads": "Downloads",
      "stats.assets": "Release files",
      "features.eyebrow": "Features",
      "features.title": "Everything essential, no clutter.",
      "f1t": "AFK auto-reply",
      "f1b": "Replies only to the names/numbers you list, only during the hours you set (overnight included). A live status line always explains why it stays quiet.",
      "f2t": "Repeating schedules",
      "f2b": "One-time, daily, weekly (pick the weekday), monthly, or yearly — each with its own sending profile.",
      "f3t": "Quick Replies",
      "f3b": "Type /trigger + Space to expand a template; {name}, {date}, and {time} fill in automatically.",
      "f4t": "Documents & downloads",
      "f4b": "Preview PDFs and Office files in-window; matching content (SHA-256) keeps the same file from piling up.",
      "f5t": "Power-conscious profiles",
      "f5b": "One tab per account with isolated data; hidden tabs suspend so CPU and RAM stay free.",
      "f6t": "Private",
      "f6b": "Privacy mode masks names and chat content; everything stays on your machine, no analytics.",
      "demo.eyebrow": "Try it live",
      "demo.title": "AFK rules, without waiting for a message.",
      "demo.lead": "The same logic the app uses: change the list or the hours, see who would be replied to.",
      "demo.allowTitle": "Allowlist (one per line)",
      "demo.nameTitle": "Incoming chat name",
      "demo.hoursTitle": "Reply hours",
      "demo.presetDay": "Day 09–17",
      "demo.presetNight": "Night 21–07",
      "demo.presetAll": "24 hours",
      "demo.hoursAll": "all day — any time is fine",
      "demo.hoursActive": "active now — {range}",
      "demo.hoursIdle": "outside hours — {range}",
      "demo.allowYes": "Would be replied — matches {entry}",
      "demo.allowNo": "Not replied — nothing in the list matches",
      "demo.allowEmpty": "Allowlist is empty — nobody is replied to",
      "demo.note": "Names match partially, case-insensitively; numbers match by digits (min. 8, one leading +62/0 ignored). An empty list means nobody gets replied to.",
      "dl.eyebrow": "Download",
      "dl.title": "Pick the right build",
      "dl.mac": "Apple Silicon and Intel, installed from a DMG.",
      "dl.win": "Windows 10/11, needs WebView2 (usually already installed).",
      "dl.lin": "Debian/Ubuntu, Fedora/RHEL, or the portable archive.",
      "dl.arm": "DEB package or portable archive for ARM64 machines.",
      "dl.dmg": "DMG ↗",
      "dl.zip": "ZIP ↗",
      "dl.exe": "EXE ↗",
      "dl.deb": "DEB ↗",
      "dl.tar": "tar.gz ↗",
      "faq.eyebrow": "FAQ",
      "faq.title": "Before you download",
      "q1t": "Is my chat data sent to a server?",
      "q1b": "No. WAtchful uses the system's own web engine and has no analytics server or message relay.",
      "q3t": "How do updates work?",
      "q3b": "WAtchful checks for a new release shortly after start and every 4 hours, then downloads, installs, and restarts itself — you only approve the banner.",
      "q4t": "Can several accounts run at once?",
      "q4b": "Yes. Each profile gets its own tab with isolated data, and hidden tabs are suspended automatically.",
      "q5t": "What does “reply only to the list” mean?",
      "q5b": "The AFK feature has an allowlist: only listed names or numbers get auto-replied. Anything else is skipped without opening.",
      "support.title": "Like WAtchful?",
      "support.cta": "Donate ↗",
      "foot.left": "WAtchful is an independent application and is not affiliated with Meta."
    }
  };

  var state = {
    lang: "id",
    theme: "dark",
    release: null,
    platform: null,
    range: "night"
  };

  var $ = function (sel) { return document.querySelector(sel); };
  var $$ = function (sel) { return Array.prototype.slice.call(document.querySelectorAll(sel)); };

  function store(key, value) {
    try { localStorage.setItem(key, value); } catch (e) {}
  }

  function load(key) {
    try { return localStorage.getItem(key); } catch (e) { return null; }
  }

  function t(key, vars) {
    var dict = I18N[state.lang] || I18N.id;
    var out = dict[key] !== undefined ? dict[key] : (I18N.id[key] !== undefined ? I18N.id[key] : key);
    if (vars) {
      Object.keys(vars).forEach(function (name) {
        out = out.split("{" + name + "}").join(vars[name]);
      });
    }
    return out;
  }

  function applyI18n() {
    document.documentElement.lang = state.lang;
    var meta = $('meta[name="description"]');
    if (meta) meta.setAttribute("content", t("meta.description"));
    document.title = t("meta.title");
    $$("[data-i18n]").forEach(function (node) {
      node.textContent = t(node.getAttribute("data-i18n"));
    });
    $$("[data-i18n-html]").forEach(function (node) {
      node.innerHTML = t(node.getAttribute("data-i18n-html"));
    });
    var id = $("#lang-id"), en = $("#lang-en");
    if (id) id.setAttribute("aria-pressed", state.lang === "id" ? "true" : "false");
    if (en) en.setAttribute("aria-pressed", state.lang === "en" ? "true" : "false");
    renderDownload();
    renderRelease();
    renderDemo();
  }

  function detectPlatform() {
    var ua = navigator.userAgent || "";
    var uaData = (navigator.userAgentData && navigator.userAgentData.platform) || navigator.platform || "";
    var probe = (uaData + " " + ua).toLowerCase();
    if (/iphone|ipad|ipod/.test(probe)) return "mac";
    if (/mac/.test(probe)) return "mac";
    if (/win/.test(probe)) return "windows";
    if (/arm|aarch64/.test(probe)) return "linuxArm";
    if (/linux|x11/.test(probe)) return "linux";
    return null;
  }

  function renderDownload() {
    var btn = $("#smart-download");
    var label = $("#smart-download-label");
    var hint = $("#platform-hint");
    if (!btn || !label || !hint) return;
    if (!state.platform) {
      label.textContent = t("hero.download");
      hint.textContent = t("hero.choose");
      return;
    }
    var target = ASSETS[state.platform];
    btn.href = target.url;
    label.textContent = target.label[state.lang] || target.label.id;
    var name = target.hint;
    if (state.release && state.release.tag_name) name += " · " + state.release.tag_name;
    hint.textContent = t("hero.detected", { name: name });
  }

  function formatCount(n) {
    if (n >= 1000000) return (n / 1000000).toFixed(1).replace(/\.0$/, "") + "M";
    if (n >= 1000) return (n / 1000).toFixed(1).replace(/\.0$/, "") + "k";
    return String(n);
  }

  function renderRelease() {
    var rel = state.release;
    var version = rel && rel.tag_name ? rel.tag_name : STATIC_VERSION;
    var badge = $("#version-badge");
    var stat = $("#stat-version");
    var foot = $("#foot-version");
    var shot = $("#shot-main");
    var count = $("#stat-downloads");
    var files = $("#stat-files");
    if (badge) badge.textContent = version;
    if (stat) stat.textContent = version;
    if (foot) foot.textContent = version;
    if (shot) shot.setAttribute("data-version", version);
    var assets = rel && Array.isArray(rel.assets) ? rel.assets : null;
    var total = 0;
    if (assets) {
      assets.forEach(function (a) { total += Number(a.download_count) || 0; });
    }
    if (files) files.textContent = assets ? String(assets.length) : String(STATIC_FILES);
    if (count) count.textContent = assets ? formatCount(total) : "—";
  }

  function loadRelease() {
    if (typeof fetch !== "function") return;
    fetch(REPO_API, { headers: { Accept: "application/vnd.github+json" } })
      .then(function (res) { return res.ok ? res.json() : null; })
      .then(function (data) {
        if (!data || !data.tag_name) return;
        state.release = data;
        renderRelease();
        renderDownload();
      })
      .catch(function () {});
  }

  function setTheme(theme) {
    state.theme = theme;
    document.documentElement.setAttribute("data-theme", theme);
    store("wa_theme", theme);
    var meta = $('meta[name="theme-color"]');
    if (!meta) {
      meta = document.createElement("meta");
      meta.setAttribute("name", "theme-color");
      document.head.appendChild(meta);
    }
    meta.setAttribute("content", theme === "light" ? "#f2f6f7" : "#0b141a");
  }

  function initTheme() {
    var saved = load("wa_theme");
    if (saved === "light" || saved === "dark") {
      setTheme(saved);
      return;
    }
    var prefersLight = typeof matchMedia === "function" && matchMedia("(prefers-color-scheme: light)").matches;
    setTheme(prefersLight ? "light" : "dark");
  }

  function initLang() {
    var saved = load("wa_lang");
    var browser = ((navigator.language || "id").toLowerCase().indexOf("id") === 0 ||
      (navigator.languages && navigator.languages[0] && navigator.languages[0].toLowerCase().indexOf("id") === 0)) ? "id" : null;
    state.lang = saved === "en" || saved === "id" ? saved : (browser || "id");
  }

  function inWindow(start, end, now) {
    if (start === null || end === null) return true;
    if (start === end) return true;
    if (start < end) return now >= start && now < end;
    return now >= start || now < end;
  }

  function rangeLabel(start, end) {
    var fmt = function (mins) {
      var h = Math.floor(mins / 60), m = mins % 60;
      return (h < 10 ? "0" : "") + h + ":" + (m < 10 ? "0" : "") + m;
    };
    return fmt(start) + "–" + fmt(end) + (start > end ? " (overnight)" : "");
  }

  function findAllowEntry(list, key) {
    var k = String(key || "").toLowerCase().trim();
    if (!k) return null;
    var kd = k.replace(/\D/g, "");
    for (var i = 0; i < list.length; i++) {
      var e = String(list[i] || "").toLowerCase().trim();
      if (!e) continue;
      if (/[a-z]/i.test(e) && (k.indexOf(e) !== -1 || e.indexOf(k) !== -1)) return list[i];
      var ed = e.replace(/\D/g, "");
      if (ed.length >= 8 && kd.length >= 8) {
        var kn = kd.replace(/^(62|0)/, "");
        var en = ed.replace(/^(62|0)/, "");
        if (kn && en && (kn.indexOf(en) !== -1 || en.indexOf(kn) !== -1)) return list[i];
      }
    }
    return null;
  }

  function renderHoursBar(start, end) {
    var bar = $("#demo-bar");
    if (!bar) return;
    Array.prototype.slice.call(bar.querySelectorAll(".hours-seg")).forEach(function (n) { n.remove(); });
    var pct = function (mins) { return (mins / 1440) * 100; };
    var seg = function (from, to) {
      var s = document.createElement("span");
      s.className = "hours-seg";
      s.style.left = pct(from) + "%";
      s.style.width = pct(Math.max(to - from, 0)) + "%";
      bar.appendChild(s);
    };
    if (start === end) {
      seg(0, 1440);
    } else if (start < end) {
      seg(start, end);
    } else {
      seg(start, 1440);
      seg(0, end);
    }
    var now = new Date();
    var nowPct = ((now.getHours() * 60 + now.getMinutes()) / 1440) * 100;
    var marker = $("#demo-now");
    if (marker) marker.style.left = nowPct + "%";
  }

  function renderDemo() {
    var allowEl = $("#demo-allow");
    var nameEl = $("#demo-name");
    var verdict = $("#demo-verdict");
    var verdictText = $("#demo-verdict-text");
    if (!allowEl || !nameEl || !verdict || !verdictText) return;

    var list = allowEl.value.split("\n").map(function (s) { return s.trim(); })
      .filter(function (s) { return s.length > 0; }).slice(0, 50);
    var name = nameEl.value;
    var entry = findAllowEntry(list, name);

    verdict.classList.toggle("is-yes", !!entry);
    verdict.classList.toggle("is-no", !entry);
    if (!list.length) verdictText.textContent = t("demo.allowEmpty");
    else if (entry) verdictText.textContent = t("demo.allowYes", { entry: entry });
    else verdictText.textContent = t("demo.allowNo");

    var preset = PRESETS[state.range] || PRESETS.night;
    var now = new Date();
    var nowMin = now.getHours() * 60 + now.getMinutes();
    var active = inWindow(preset.start, preset.end, nowMin);
    var hv = $("#demo-hours-verdict");
    var ht = $("#demo-hours-text");
    if (hv && ht) {
      var label = preset.start === preset.end ? t("demo.hoursAll")
        : t(active ? "demo.hoursActive" : "demo.hoursIdle", { range: rangeLabel(preset.start, preset.end) });
      hv.classList.toggle("is-yes", active);
      hv.classList.toggle("is-no", !active);
      ht.textContent = label;
    }
    renderHoursBar(preset.start, preset.end);

    ["day", "night", "all"].forEach(function (key) {
      var btn = $("#preset-" + key);
      if (btn) btn.classList.toggle("is-on", state.range === key);
    });
  }

  function initPresets() {
    ["day", "night", "all"].forEach(function (key) {
      var btn = $("#preset-" + key);
      if (!btn) return;
      btn.addEventListener("click", function () {
        state.range = key;
        renderDemo();
      });
    });
  }

  function initLightbox() {
    var box = $("#lightbox");
    var img = $("#lightbox-img");
    var close = $("#lightbox-close");
    if (!box || !img) return;
    function hide() {
      box.classList.remove("is-on");
      document.body.style.overflow = "";
    }
    $$(".shot").forEach(function (shot) {
      shot.addEventListener("click", function () {
        img.src = shot.getAttribute("data-full") || "assets/app-dark.png";
        img.alt = shot.getAttribute("data-caption") || "";
        box.classList.add("is-on");
        document.body.style.overflow = "hidden";
      });
    });
    if (close) close.addEventListener("click", hide);
    box.addEventListener("click", function (ev) { if (ev.target === box) hide(); });
    document.addEventListener("keydown", function (ev) {
      if (ev.key === "Escape") hide();
    });
  }

  function initReveal() {
    var targets = $$(".section-head, .card, .platform, details");
    if (!("IntersectionObserver" in window) ||
      (typeof matchMedia === "function" && matchMedia("(prefers-reduced-motion: reduce)").matches)) {
      targets.forEach(function (n) { n.classList.add("reveal", "is-in"); });
      return;
    }
    targets.forEach(function (n) { n.classList.add("reveal"); });
    var io = new IntersectionObserver(function (entries) {
      entries.forEach(function (entry) {
        if (!entry.isIntersecting) return;
        entry.target.classList.add("is-in");
        io.unobserve(entry.target);
      });
    }, { rootMargin: "0px 0px -8% 0px", threshold: 0.06 });
    targets.forEach(function (n) { io.observe(n); });
  }

  function initActiveNav() {
    var links = $$(".nav-links a");
    var sections = links.map(function (a) {
      var id = a.getAttribute("href");
      return id && id.charAt(0) === "#" ? document.querySelector(id) : null;
    });
    if (!("IntersectionObserver" in window)) return;
    var io = new IntersectionObserver(function (entries) {
      entries.forEach(function (entry) {
        if (!entry.isIntersecting) return;
        var idx = sections.indexOf(entry.target);
        links.forEach(function (a, i) { a.classList.toggle("active", i === idx); });
      });
    }, { rootMargin: "-45% 0px -50% 0px" });
    sections.forEach(function (s) { if (s) io.observe(s); });
  }

  function initToTop() {
    var btn = $("#to-top");
    if (!btn) return;
    function sync() { btn.classList.toggle("is-on", window.scrollY > 640); }
    window.addEventListener("scroll", sync, { passive: true });
    btn.addEventListener("click", function () { window.scrollTo({ top: 0, behavior: "smooth" }); });
    sync();
  }

  function initLangToggle() {
    var id = $("#lang-id"), en = $("#lang-en");
    function pick(lang) {
      state.lang = lang;
      store("wa_lang", lang);
      applyI18n();
    }
    if (id) id.addEventListener("click", function () { pick("id"); });
    if (en) en.addEventListener("click", function () { pick("en"); });
  }

  function init() {
    initTheme();
    initLang();
    initLangToggle();
    var toggle = $("#theme-toggle");
    if (toggle) toggle.addEventListener("click", function () {
      setTheme(state.theme === "dark" ? "light" : "dark");
    });
    state.platform = detectPlatform();
    initPresets();
    initLightbox();
    initToTop();
    initReveal();
    initActiveNav();
    ["#demo-allow", "#demo-name"].forEach(function (sel) {
      var el = $(sel);
      if (!el) return;
      el.addEventListener("input", renderDemo);
      el.addEventListener("change", renderDemo);
    });
    applyI18n();
    renderRelease();
    loadRelease();
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }

  window.__waSite = {
    state: state,
    setLang: function (lang) {
      state.lang = lang;
      applyI18n();
    },
    inWindow: inWindow,
    findAllowEntry: findAllowEntry,
    detectPlatform: detectPlatform
  };
})();