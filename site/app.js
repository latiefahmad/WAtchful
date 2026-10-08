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

  var I18N = {
    id: {
      "meta.title": "WAtchful — WhatsApp di meja kerja Anda",
      "meta.description": "WAtchful: WhatsApp Web sebagai aplikasi desktop. Balas otomatis AFK dengan jam dan daftar kontak, jadwal berulang, Quick Replies, pratinjau dokumen, multi-profil, dan pembaruan otomatis.",
      "nav.features": "Fitur",
      "nav.afk": "AFK",
      "nav.schedule": "Jadwal",
      "nav.privacy": "Privasi",
      "nav.download": "Unduh",
      "hero.eyebrow": "Aplikasi desktop mandiri untuk WhatsApp",
      "hero.title": "WhatsApp,<br><em>di meja kerja Anda.</em>",
      "hero.lead": "Semua kekuatan WhatsApp Web dalam satu jendela desktop yang rapi: multi-profil, balas otomatis AFK yang bisa Anda batasi, jadwal berulang, pratinjau dokumen, dan pembaruan sendiri.",
      "hero.version": STATIC_VERSION,
      "hero.download": "Unduh rilis terbaru",
      "hero.allBuilds": "Lihat semua build",
      "hero.detecting": "Mendeteksi platform Anda…",
      "hero.detected": "Terdeteksi: {name}. Anda bisa memilih build lain di bawah.",
      "hero.choose": "Pilih platform di bawah untuk installer yang benar.",
      "stats.version": "Versi",
      "stats.downloads": "Unduhan",
      "stats.assets": "Berkas rilis",
      "stats.license": "Lisensi",
      "trust.1t": "Mesin asli",
      "trust.1b": "WebView2, WebKit, atau WebKitGTK — bukan peramban tersembunyi.",
      "trust.2t": "Privat sejak awal",
      "trust.2b": "Tanpa server analitik atau relay pesan.",
      "trust.3t": "Sekali unduh",
      "trust.3b": "Berkas disimpan sekali, lalu dipakai ulang secara lokal.",
      "trust.4t": "Menjaga diri sendiri",
      "trust.4b": "Pembaruan otomatis di dalam aplikasi.",
      "features.eyebrow": "Yang berubah bulan ini",
      "features.title": "Kontrol desktop yang benar-benar dipakai.",
      "features.lead": "Enam bidang yang paling sering dipilih: apa yang berubah, dan mana yang hanya berjalan di belakang layar.",
      "tabs.afk": "Balas otomatis AFK",
      "tabs.schedule": "Jadwal berulang",
      "tabs.quick": "Quick Replies",
      "tabs.preview": "Dokumen & unduhan",
      "tabs.profiles": "Multi-profil",
      "tabs.privacy": "Privasi",
      "afk.title": "Balas otomatis yang tahu harus membalas siapa",
      "afk.f1t": "DOM baru WhatsApp didukung penuh",
      "afk.f1b": "Baris chat dikenali lewat struktur terbaru, dibuka dengan klik terverifikasi, lalu header dicek sebelum mengetik. Ketidakcocokan dicatat dan dilewati, bukan ditebak.",
      "afk.f2t": "Jam balas",
      "afk.f2b": "Batasi balasan ke rentang jam tertentu, termasuk rentang lintas malam seperti 21:00–07:00.",
      "afk.f3t": "Daftar izin",
      "afk.f3b": "Opsional: hanya balas nama atau nomor yang Anda daftarkan. Cocok sebagian, angka dicocokkan per digit dengan atau tanpa +62 / nol di depan.",
      "afk.f4t": "Status & diagnostik",
      "afk.f4b": "Baris status langsung menjelaskan alasan AFK diam, dan tombol Diagnose scan melaporkan yang dilihat pemindai.",
      "demo.allowTitle": "Daftar izin (satu per baris)",
      "demo.nameTitle": "Nama chat yang masuk",
      "demo.hoursTitle": "Jam balas",
      "demo.hoursAll": "sepanjang hari — jam berapa pun",
      "demo.hoursActive": "aktif sekarang — {range}",
      "demo.hoursIdle": "di luar jam — {range}",
      "demo.allowYes": "Bakal dibalas — cocok dengan {entry}",
      "demo.allowNo": "Tidak dibalas — tidak ada entri yang cocok",
      "demo.allowEmpty": "Daftar izin kosong — tidak ada yang dibalas",
      "demo.statusActive": "armed — {name} ada di daftar izin",
      "demo.statusBlocked": "idle — {name} di luar daftar izin",
      "demo.statusEmpty": "idle — daftar izin kosong, tidak ada yang dibalas",
      "sched.title": "Kirim sendiri, sekali atau terus",
      "sched.f1t": "Enam pola",
      "sched.f1b": "Sekali, harian, mingguan (pilih hari), bulanan (tanggal yang sama, otomatis menyesuaikan bulan pendek), dan tahunan.",
      "sched.f2t": "Tidak saling berebut",
      "sched.f2b": "AFK dan jadwal berbagi satu komposer: yang sedang mengirim diprioritaskan, yang lain menunggu giliran.",
      "sched.f3t": "Pengiriman dari profil",
      "sched.f3b": "Setiap jadwal punya asal pengirim, jadi pesan personal tetap terpisah dari nomor kantor.",
      "sched.cap": "Panel pengaturan: AFK, jadwal, dan jatah kirim berada di Automation.",
      "quick.title": "Balasan yang berulang, tanpa mengetik ulang",
      "quick.f1t": "Pintasan slash",
      "quick.f1b": "Ketik /trigger lalu Spasi di chat mana pun untuk memperluas templat, dikelola per profil.",
      "quick.f2t": "Variabel",
      "quick.f2b": "{name}, {date}, dan {time} terisi otomatis sesuai konteks.",
      "quick.f3t": "Aman dari ketikan hilang",
      "quick.f3b": "Ekspansi memakai jalur input editor sendiri dengan cadangan beberapa langkah; kegagalan melaporkan penyebabnya, bukan menelan teks.",
      "quick.sample": "Contoh pesan",
      "quick.preview": "Hasilnya: {out}",
      "prev.title": "Dokumen dibuka tanpa mengorbankan chat",
      "prev.f1t": "Pratinjau PDF dan Office",
      "prev.f1b": "Buka di dalam jendela dulu; simpan hanya setelah Anda memutuskan.",
      "prev.f2t": "Gulir mulus",
      "prev.f2b": "Overlay datar dan lapisan yang dipromosikan membuat pratinjau seluler seperti penampil bawaan.",
      "prev.f3t": "Tanpa duplikat",
      "prev.f3b": "Unduhan dicocokkan lewat isi berkas (SHA-256), jadi berkas sama tidak menumpuk di folder.",
      "prev.noteT": "Catatan rilis terbaru",
      "prev.noteB": "Semua perbaikan ini sudah ada di v2.0.14 — bukan VaporWare.",
      "prof.title": "Banyak akun, satu jendela",
      "prof.f1t": "Tab per profil",
      "prof.f1b": "Setiap profil punya folder data sendiri sehingga satu akun tidak keluar dari akun lain.",
      "prof.f2t": "Hemat daya",
      "prof.f2b": "Tab tersembunyi ditangguhkan lalu dihibernasi; renderer aktif didaur ulang berkala.",
      "prof.f3t": "Ganti nama langsung",
      "prof.f3b": "Profil aktif bisa diganti namanya tanpa menutup apa pun; tab dan daftar ikut.",
      "prof.note": "Direct Chat punya tombol sendiri di strip judul, juga tersedia lewat Ctrl/Cmd+Shift+C, sehingga nomor baru bisa dikunci tanpa disimpan ke kontak.",
      "priv.title": "Data tetap di mesin Anda",
      "priv.f1t": "Mode privasi",
      "priv.f1b": "Menyamar nama kontak, isi baris chat, dan tautan di dalam gelembung pesan.",
      "priv.f2t": "Lokal",
      "priv.f2b": "Pengaturan, daftar profil, dan log tinggal di folder data aplikasi; tidak ada yang diunggah otomatis.",
      "priv.f3t": "Laporan manual",
      "priv.f3b": "Tombol Report hanya membuka isu GitHub yang sudah terisi — Anda yang memutuskan mengirim.",
      "priv.cap": "Semua sakelar keamanan ada di tab Privacy dan Notifications.",
      "afksec.eyebrow": "AFK, tapi tetap terkendali",
      "afksec.title": "Aturan balasan Anda sendiri, bukan tebakan.",
      "afksec.lead": "Dua hal yang menjalankan semuanya: jam boleh membalas, dan siapa yang boleh dibalas. Keduanya murni, transparan, dan bisa diuji tanpa menunggu pesan masuk.",
      "afksec.rulesT": "Cara pencocokan",
      "afksec.statusT": "Contoh baris status",
      "rule1": "Entri berhuruf cocok sebagian, tanpa memandang huruf besar-kecil.",
      "rule2": "Entri angka dicocokkan per digit, minimal 8 digit, dengan satu awalan +62 atau 0 diabaikan.",
      "rule3": "Daftar kosong berarti tidak ada yang dibalas — bukan berarti semua.",
      "rule4": "Chat di luar daftar dilewati sebelum diklik, jadi tidak pernah terbuka tanpa alasan.",
      "st1": "armed — akan membalas chat belum dibaca berikutnya",
      "st2": "idle — chat belum dibaca di luar daftar izin (3 terdaftar)",
      "st3": "idle — di luar jam balas (21:00–07:00)",
      "st4": "jeda — panel pengaturan terbuka (tutup untuk lanjut)",
      "shortcut.eyebrow": "Pintasan",
      "shortcut.title": "Yang sering dipakai, satu ketukan",
      "sc.settings": "Buka pengaturan",
      "sc.direct": "Direct Chat ke nomor baru",
      "sc.quick": "Expand Quick Reply",
      "sc.typeKey": "ketik",
      "sc.typeHint": "Balasan manual mematikan AFK sementara itu",
      "faq.eyebrow": "Pertanyaan umum",
      "faq.title": "Sebelum Anda mengunduh",
      "q1t": "Apakah chat saya dikirim ke server?",
      "q1b": "Tidak. WAtchful memakai mesin web bawaan sistem dan tidak punya server analitik atau relay pesan. Pemeriksaan pembaruan hanya menghubungi GitHub, tanpa isi chat.",
      "q2t": "Apakah harus punya akun WhatsApp Business?",
      "q2b": "Tidak. Login biasa seperti di WhatsApp Web, per profil. WAtchful hanya membungkusnya jadi jendela desktop.",
      "q3t": "Bagaimana cara pembaruannya?",
      "q3b": "WAtchful memeriksa rilis baru segera setelah mulai dan setiap 4 jam, lalu mengunduh, memasang, dan memulai ulang dirinya sendiri — Anda hanya menyetujui banner. Kalau tidak suka, unduh manual dari halaman ini.",
      "q4t": "Bisakah beberapa akun jalan bersamaan?",
      "q4b": "Bisa. Setiap profil punya tab sendiri dengan folder data terisolasi; tab tersembunyi otomatis ditangguhkan agar hemat CPU dan RAM.",
      "q5t": "Apa arti “hanya balas daftar”?",
      "q5b": "Fitur AFK punya daftar izin: isi nama atau nomor chat yang boleh dibalas otomatis. Yang tidak ada di daftar dilewati tanpa dibuka, dan tercatat di log AFK sebagai skipped.",
      "dl.eyebrow": "Unduhan",
      "dl.title": "Pilih build yang sesuai",
      "dl.lead": "Deteksi platform di atas sudah memilih yang tepat. Semua build tersedia manual di bawah ini.",
      "dl.mac": "Apple Silicon dan Intel, dipasang lewat DMG.",
      "dl.win": "Windows 10/11, butuh WebView2 (umumnya sudah terpasang).",
      "dl.lin": "Debian/Ubuntu, Fedora/RHEL, atau arsip portabel.",
      "dl.arm": "Paket DEB atau arsip portabel untuk mesin ARM64.",
      "dl.dmg": "DMG ↗",
      "dl.zip": "ZIP ↗",
      "dl.exe": "EXE ↗",
      "dl.deb": "DEB ↗",
      "dl.tar": "tar.gz ↗",
      "support.title": "Temukan aplikasi ini berguna?",
      "support.body": "WAtchful gratis dan sumber terbuka. Kalau Anda membantu, tombol Donate di strip judul — atau klik di sini — membuka Saweria.",
      "support.cta": "Donate ↗",
      "foot.left": "WAtchful adalah aplikasi independen dan tidak berafiliasi dengan Meta."
    },
    en: {
      "meta.title": "WAtchful — WhatsApp at your desk",
      "meta.description": "WAtchful turns WhatsApp Web into a desktop app: AFK auto-reply with hours and an allowlist, repeating schedules, Quick Replies, document previews, multiple profiles, and self-updates.",
      "nav.features": "Features",
      "nav.afk": "AFK",
      "nav.schedule": "Schedules",
      "nav.privacy": "Privacy",
      "nav.download": "Download",
      "hero.eyebrow": "Independent desktop companion for WhatsApp",
      "hero.title": "WhatsApp,<br><em>at your desk.</em>",
      "hero.lead": "All of WhatsApp Web in one tidy desktop window: multi-profile, AFK auto-reply you can put boundaries around, repeating schedules, document previews, and updates that handle themselves.",
      "hero.version": STATIC_VERSION,
      "hero.download": "Download latest release",
      "hero.allBuilds": "See all builds",
      "hero.detecting": "Detecting your platform…",
      "hero.detected": "Detected: {name}. You can pick another build below.",
      "hero.choose": "Choose your platform below for the correct installer.",
      "stats.version": "Version",
      "stats.downloads": "Downloads",
      "stats.assets": "Release files",
      "stats.license": "License",
      "trust.1t": "Native engine",
      "trust.1b": "WebView2, WebKit, or WebKitGTK — not a hidden browser.",
      "trust.2t": "Private by design",
      "trust.2b": "No analytics server or message relay.",
      "trust.3t": "One download",
      "trust.3b": "Files saved once, then reused locally.",
      "trust.4t": "Maintains itself",
      "trust.4b": "Automatic in-app updates.",
      "features.eyebrow": "What changed this month",
      "features.title": "Desktop controls you actually use.",
      "features.lead": "The six areas that matter most: what changed, and what just runs quietly in the background.",
      "tabs.afk": "AFK auto-reply",
      "tabs.schedule": "Repeating schedules",
      "tabs.quick": "Quick Replies",
      "tabs.preview": "Documents & downloads",
      "tabs.profiles": "Multiple profiles",
      "tabs.privacy": "Privacy",
      "afk.title": "Auto-reply that knows who it may answer",
      "afk.f1t": "Built for WhatsApp's new chat list",
      "afk.f1b": "Rows are recognised through the newest structure, opened with a verified click, and the header is checked before typing. A mismatch is logged and skipped, never guessed.",
      "afk.f2t": "Reply hours",
      "afk.f2b": "Limit replies to a chosen range, including overnight spans like 21:00–07:00.",
      "afk.f3t": "Allowlist",
      "afk.f3b": "Optional: reply only to the names or numbers you list. Names match partially; numbers match by digits, with or without +62 / a leading 0.",
      "afk.f4t": "Status & diagnostics",
      "afk.f4b": "A live status line explains exactly why AFK is idle, and a Diagnose scan button reports what the scanner sees.",
      "demo.allowTitle": "Allowlist (one per line)",
      "demo.nameTitle": "Incoming chat name",
      "demo.hoursTitle": "Reply hours",
      "demo.hoursAll": "all day — any time is fine",
      "demo.hoursActive": "active now — {range}",
      "demo.hoursIdle": "outside hours — {range}",
      "demo.allowYes": "Would be replied — matches {entry}",
      "demo.allowNo": "Not replied — nothing in the list matches",
      "demo.allowEmpty": "Allowlist is empty — nobody is replied to",
      "demo.statusActive": "armed — {name} is on the allowlist",
      "demo.statusBlocked": "idle — {name} is not on the allowlist",
      "demo.statusEmpty": "idle — empty allowlist, nobody is replied to",
      "sched.title": "Send on its own, once or on repeat",
      "sched.f1t": "Six patterns",
      "sched.f1b": "One-time, daily, weekly (pick the weekday), monthly (same date, clamped in short months), and yearly.",
      "sched.f2t": "No fighting over the composer",
      "sched.f2b": "AFK and schedules share one composer: whoever is sending goes first, the rest wait their turn.",
      "sched.f3t": "Send-from per schedule",
      "sched.f3b": "Every schedule carries its own sender, so personal replies stay apart from the work number.",
      "sched.cap": "Settings: AFK, schedules, and send caps live under Automation.",
      "quick.title": "Repeat answers without retyping",
      "quick.f1t": "Slash triggers",
      "quick.f1b": "Type /trigger then Space in any chat to expand a saved template, managed per profile.",
      "quick.f2t": "Variables",
      "quick.f2b": "{name}, {date}, and {time} fill in from context.",
      "quick.f3t": "No swallowed typing",
      "quick.f3b": "Expansion goes through the editor's own input path with layered fallbacks; failures say why instead of eating text.",
      "quick.sample": "Sample message",
      "quick.preview": "Result: {out}",
      "prev.title": "Documents open without costing you the chat",
      "prev.f1t": "PDF and Office previews",
      "prev.f1b": "Open inside the window first; save only after you decide.",
      "prev.f2t": "Smooth scrolling",
      "prev.f2b": "A flat overlay and a promoted layer make the preview scroll like the built-in viewer.",
      "prev.f3t": "No duplicates",
      "prev.f3b": "Downloads are matched by content (SHA-256), so the same file never piles up twice.",
      "prev.noteT": "Latest release note",
      "prev.noteB": "All of these fixes ship in v2.0.14 — not vaporware.",
      "prof.title": "Many accounts, one window",
      "prof.f1t": "A tab per profile",
      "prof.f1b": "Each profile owns its own data folder, so one account never logs another out.",
      "prof.f2t": "Power-conscious",
      "prof.f2b": "Hidden tabs are suspended, then hibernated; the active renderer recycles on a schedule.",
      "prof.f3t": "Rename in place",
      "prof.f3b": "The active profile can be renamed without closing anything; tab and list follow.",
      "prof.note": "Direct Chat has its own title-strip button and Ctrl/Cmd+Shift+C, so a new number can be messaged without saving a contact.",
      "priv.title": "Your data stays on your machine",
      "priv.f1t": "Privacy mode",
      "priv.f1b": "Masks contact names, chat-row content, and links inside message bubbles.",
      "priv.f2t": "Local",
      "priv.f2b": "Settings, the profile registry, and logs live in the app data folder; nothing is uploaded automatically.",
      "priv.f3t": "Manual reporting",
      "priv.f3b": "The Report button only opens a pre-filled GitHub issue — you decide whether to send it.",
      "priv.cap": "Every privacy and notification switch lives under Privacy and Notifications.",
      "afksec.eyebrow": "AFK, but still bounded",
      "afksec.title": "Your own reply rules, not a guess.",
      "afksec.lead": "Two things drive everything: the hours you allow replies, and who may be replied to. Both are pure, inspectable, and testable without waiting for a message.",
      "afksec.rulesT": "How matching works",
      "afksec.statusT": "Example status lines",
      "rule1": "Entries containing letters match partially, case-insensitively.",
      "rule2": "Numeric entries match by digits, at least 8 of them, ignoring one leading +62 or 0.",
      "rule3": "An empty list means nobody gets replied to — not everybody.",
      "rule4": "Chats outside the list are skipped before any click, so nothing opens for no reason.",
      "st1": "armed — will reply to the next unread chat",
      "st2": "idle — unread chats are not in the reply list (3 listed)",
      "st3": "idle — outside reply hours (21:00–07:00)",
      "st4": "paused — settings panel is open (close it to resume)",
      "shortcut.eyebrow": "Shortcuts",
      "shortcut.title": "Frequent things, one keystroke",
      "sc.settings": "Open settings",
      "sc.direct": "Direct Chat to a new number",
      "sc.quick": "Expand a Quick Reply",
      "sc.typeKey": "typing",
      "sc.typeHint": "Sending by hand switches AFK off for that moment",
      "faq.eyebrow": "FAQ",
      "faq.title": "Before you download",
      "q1t": "Is my chat data sent to a server?",
      "q1b": "No. WAtchful uses the system's own web engine and has no analytics server or message relay. Update checks only contact GitHub, never chat content.",
      "q2t": "Do I need a WhatsApp Business account?",
      "q2b": "No. A normal login like WhatsApp Web, per profile. WAtchful only wraps it into a desktop window.",
      "q3t": "How do updates work?",
      "q3b": "WAtchful checks for a new release shortly after start and every 4 hours, then downloads, installs, and restarts itself — you only approve the banner. Prefer manual? Download from this page.",
      "q4t": "Can several accounts run at once?",
      "q4b": "Yes. Each profile gets its own tab with an isolated data folder, and hidden tabs are suspended to save CPU and RAM.",
      "q5t": "What does “reply only to the list” mean?",
      "q5b": "The AFK feature has an allowlist: fill in the chat names or numbers that may be auto-replied. Anything else is skipped without opening, and appears in the AFK log as skipped.",
      "dl.eyebrow": "Download",
      "dl.title": "Pick the right build",
      "dl.lead": "The detection above already picked one. Every build is available manually below.",
      "dl.mac": "Apple Silicon and Intel, installed from a DMG.",
      "dl.win": "Windows 10/11, needs WebView2 (usually already installed).",
      "dl.lin": "Debian/Ubuntu, Fedora/RHEL, or the portable archive.",
      "dl.arm": "DEB package or portable archive for ARM64 machines.",
      "dl.dmg": "DMG ↗",
      "dl.zip": "ZIP ↗",
      "dl.exe": "EXE ↗",
      "dl.deb": "DEB ↗",
      "dl.tar": "tar.gz ↗",
      "support.title": "Found this useful?",
      "support.body": "WAtchful is free and open source. If it helps, the Donate button in the title strip — or this link — opens Saweria.",
      "support.cta": "Donate ↗",
      "foot.left": "WAtchful is an independent application and is not affiliated with Meta."
    }
  };

  var state = {
    lang: "id",
    theme: "dark",
    release: null,
    platform: null
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
    var isMac = /Mac/i.test(navigator.platform || "") || /Mac/i.test(navigator.userAgent || "");
    var mod = isMac ? "Cmd" : "Ctrl";
    var k1 = $("#kbd-mod"), k2 = $("#kbd-mod2");
    if (k1) k1.textContent = mod;
    if (k2) k2.textContent = mod;
    var id = $("#lang-id"), en = $("#lang-en");
    if (id) id.setAttribute("aria-pressed", state.lang === "id" ? "true" : "false");
    if (en) en.setAttribute("aria-pressed", state.lang === "en" ? "true" : "false");
    renderDownload();
    renderRelease();
    renderDemo();
    renderQuickPreview();
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

  function isApple() {
    var probe = ((navigator.userAgentData && navigator.userAgentData.platform) || navigator.platform || "").toLowerCase();
    return /mac/.test(probe) && !/iphone|ipad/.test(probe);
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

  function initTabs() {
    var tabs = $$('[role="tab"]');
    function select(id) {
      tabs.forEach(function (btn) {
        var on = btn.id === id;
        btn.setAttribute("aria-selected", on ? "true" : "false");
        var panel = document.getElementById(btn.getAttribute("aria-controls"));
        if (!panel) return;
        panel.classList.toggle("is-on", on);
        panel.hidden = !on;
      });
    }
    tabs.forEach(function (btn, i) {
      btn.addEventListener("click", function () { select(btn.id); });
      btn.addEventListener("keydown", function (ev) {
        var dir = ev.key === "ArrowRight" ? 1 : ev.key === "ArrowLeft" ? -1 : 0;
        if (!dir) return;
        ev.preventDefault();
        var next = tabs[(i + dir + tabs.length) % tabs.length];
        next.focus();
        select(next.id);
      });
    });
    if (tabs.length) select(tabs[0].id);
  }

  function parseHM(value) {
    var m = /^(\d{1,2}):(\d{1,2})$/.exec(String(value || "").trim());
    if (!m) return null;
    var h = parseInt(m[1], 10), min = parseInt(m[2], 10);
    if (h < 0 || h > 23 || min < 0 || min > 59) return null;
    return h * 60 + min;
  }

  function hmToMins(value) {
    var m = /^(\d{1,2}):(\d{1,2})$/.exec(String(value || ""));
    return m ? parseInt(m[1], 10) * 60 + parseInt(m[2], 10) : null;
  }

  function inWindow(start, end, now) {
    if (start === null || end === null) return true;
    if (start === end) return true;
    if (start < end) return now >= start && now < end;
    return now >= start || now < end;
  }

  function rangeLabel(start, end) {
    var fmt = function (mins) {
      if (mins === null) return "?";
      var h = Math.floor(mins / 60), m = mins % 60;
      return (h < 10 ? "0" : "") + h + ":" + (m < 10 ? "0" : "") + m;
    };
    return fmt(start) + "–" + fmt(end) + (start !== null && end !== null && start > end ? " (overnight)" : "");
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

  function renderHoursBar() {
    var bar = $("#demo-bar");
    if (!bar) return;
    Array.prototype.slice.call(bar.querySelectorAll(".hours-seg")).forEach(function (n) { n.remove(); });
    var start = parseHM($("#demo-start") ? $("#demo-start").value : "");
    var end = parseHM($("#demo-end") ? $("#demo-end").value : "");
    var pct = function (mins) { return (mins / 1440) * 100; };
    var seg = function (from, to) {
      var s = document.createElement("span");
      s.className = "hours-seg";
      s.style.left = pct(from) + "%";
      s.style.width = pct(Math.max(to - from, 0)) + "%";
      bar.appendChild(s);
    };
    if (start === null || end === null) {
      seg(0, 1440);
    } else if (start === end) {
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
    var statusText = $("#demo-status-text");
    var status = $("#demo-status");
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

    if (status) status.classList.toggle("is-idle", !entry);
    if (statusText) {
      if (!list.length) statusText.textContent = t("demo.statusEmpty");
      else if (entry) statusText.textContent = t("demo.statusActive", { name: name || "—" });
      else statusText.textContent = t("demo.statusBlocked", { name: name || "—" });
    }

    var startRaw = $("#demo-start") ? $("#demo-start").value : "";
    var endRaw = $("#demo-end") ? $("#demo-end").value : "";
    var start = hmToMins(startRaw), end = hmToMins(endRaw);
    var now = new Date();
    var nowMin = now.getHours() * 60 + now.getMinutes();
    var active = start === null || end === null ? true : inWindow(start, end, nowMin);
    var hv = $("#demo-hours-verdict");
    var ht = $("#demo-hours-text");
    if (hv && ht) {
      var label = start === null || end === null || start === end ? t("demo.hoursAll")
        : t(active ? "demo.hoursActive" : "demo.hoursIdle", { range: rangeLabel(start, end) });
      hv.classList.toggle("is-yes", active);
      hv.classList.toggle("is-no", !active);
      ht.textContent = label;
    }
    renderHoursBar();
  }

  function renderQuickPreview() {
    var input = $("#quick-demo");
    var out = $("#quick-preview");
    if (!input || !out) return;
    var now = new Date();
    var days = ["Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"];
    var months = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];
    var hh = now.getHours(), mm = now.getMinutes();
    var date = days[now.getDay()] + ", " + now.getDate() + " " + months[now.getMonth()] + " " + now.getFullYear();
    var time = (hh < 10 ? "0" : "") + hh + ":" + (mm < 10 ? "0" : "") + mm;
    var replaced = input.value
      .split("{name}").join(state.lang === "id" ? "Budi" : "Budi")
      .split("{date}").join(date)
      .split("{time}").join(time);
    out.textContent = t("quick.preview", { out: replaced });
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
    var targets = $$(".section-head, .panel, .card, .shortcut, .platform, details, .strip div");
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
    initTabs();
    initLightbox();
    initToTop();
    initReveal();
    initActiveNav();
    ["#demo-allow", "#demo-name", "#demo-start", "#demo-end"].forEach(function (sel) {
      var el = $(sel);
      if (!el) return;
      el.addEventListener("input", renderDemo);
      el.addEventListener("change", renderDemo);
    });
    var quick = $("#quick-demo");
    if (quick) quick.addEventListener("input", renderQuickPreview);
    applyI18n();
    renderRelease();
    loadRelease();
    if (isApple()) {
      Array.prototype.slice.call(document.querySelectorAll('a[href$=".deb"], a[href$=".tar.gz"]'))
        .forEach(function (a) { a.classList.add("off-platform"); });
    }
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