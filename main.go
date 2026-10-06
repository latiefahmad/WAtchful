package main

import (
	"os"
	"runtime"
	"strings"
)

const (
	windowWidth  = 1100
	windowHeight = 750
)

func getInitScript(ua string) string {
	clientPlatform := "macOS"
	clientPlatformVersion := "15.0.0"
	clientArch := "arm"
	if runtime.GOOS == "windows" {
		clientPlatform = "Windows"
		clientPlatformVersion = "10.0.0"
		clientArch = "x86"
	} else if runtime.GOOS == "linux" {
		clientPlatform = "Linux"
		clientPlatformVersion = "6.8.0"
		clientArch = "x86"
	}

	script := `
		// UserAgent and platform override to Google Chrome
		Object.defineProperty(navigator, 'userAgent', {
			get: () => '` + ua + `'
		});
		Object.defineProperty(navigator, 'appVersion', {
			get: () => '` + ua + `'
		});
		Object.defineProperty(navigator, 'vendor', {
			get: () => 'Google Inc.'
		});

		// Emulate window.chrome
		if (!window.chrome) {
			window.chrome = {
				app: { isInstalled: false },
				runtime: {}
			};
		}

		// Remove Safari-specific markers
		try {
			delete window.safari;
		} catch (e) {}

		// NOTE (v1.5.9): a <meta> Content-Security-Policy allowlist was tried in
		// v1.5.8 and REVERTED — WhatsApp Web loads its boot bundles from Meta
		// CDN hosts outside any maintainable allowlist, so the policy blocked
		// boot and left the app stuck on the splash screen. Do not re-add a
		// meta CSP without a report-only phase first.

		// WhatsApp's virtualized lists can emit hundreds of DOM mutations while the
		// user scrolls. Our enhancements are non-critical during that gesture, so
		// defer them briefly instead of competing with WebKit's renderer. This is
		// deliberately a shared gate: observers keep their correctness but never
		// create a second rendering workload during fast scrolling.
		var waBackgroundWorkBusyUntil = 0;
		function markBackgroundWorkBusy() {
			waBackgroundWorkBusyUntil = Date.now() + 350;
		}
		window.addEventListener('scroll', markBackgroundWorkBusy, { passive: true, capture: true });
		window.addEventListener('wheel', markBackgroundWorkBusy, { passive: true, capture: true });
		window.addEventListener('touchmove', markBackgroundWorkBusy, { passive: true, capture: true });
		function shouldPauseBackgroundWork() {
			return document.hidden === true || Date.now() < waBackgroundWorkBusyUntil;
		}

		// Emulate navigator.userAgentData (User-Agent Client Hints)
		if (!navigator.userAgentData) {
			Object.defineProperty(navigator, 'userAgentData', {
				get: () => ({
					brands: [
						{ brand: 'Not(A:Brand', version: '99' },
						{ brand: 'Google Chrome', version: '133' },
						{ brand: 'Chromium', version: '133' }
					],
					mobile: false,
					platform: '` + clientPlatform + `',
					getHighEntropyValues: function() {
						return Promise.resolve({
							architecture: '` + clientArch + `',
							bitness: '64',
							brands: [
								{ brand: 'Not(A:Brand', version: '99' },
								{ brand: 'Google Chrome', version: '133' },
								{ brand: 'Chromium', version: '133' }
							],
							fullVersionList: [
								{ brand: 'Not(A:Brand', version: '99.0.0.0' },
								{ brand: 'Google Chrome', version: '133.0.0.0' },
								{ brand: 'Chromium', version: '133.0.0.0' }
							],
							mobile: false,
							model: '',
							platform: '` + clientPlatform + `',
							platformVersion: '` + clientPlatformVersion + `',
							uaFullVersion: '133.0.0.0'
						});
					}
				})
			});
		}

		// Keep WKWebView's real PDF capability untouched. Advertising Chrome's
		// PDF plugin makes WhatsApp open a viewer that WKWebView cannot render.

		// Native Notification Polyfill & ServiceWorker Notification Interceptor
		(function() {
			// Desktop notification master switch (Settings card, persisted
			// natively). Gated at dispatch so every path — page Notification,
			// service-worker showNotification, update banner — obeys one flag.
			var notificationsEnabled = true;
			var notificationsStateReady = false;
			window.isNotificationsEnabled = function() {
				return notificationsStateReady && notificationsEnabled;
			};
			window.setNotificationsEnabled = function(enabled) {
				enabled = !!enabled;
				if (!window.setNotificationsEnabledNative) {
					notificationsEnabled = enabled;
					notificationsStateReady = true;
					return Promise.resolve(notificationsEnabled);
				}
				return Promise.resolve(window.setNotificationsEnabledNative(enabled)).then(function(saved) {
					notificationsEnabled = !!saved;
					notificationsStateReady = true;
					return notificationsEnabled;
				});
			};
			window.refreshNotificationsEnabled = function() {
				if (!window.getNotificationsEnabledNative) {
					notificationsStateReady = true;
					return Promise.resolve(notificationsEnabled);
				}
				return Promise.resolve(window.getNotificationsEnabledNative()).then(function(saved) {
					notificationsEnabled = !!saved;
					notificationsStateReady = true;
					return notificationsEnabled;
				});
			};
			window.refreshNotificationsEnabled();

			function dispatchNativeNotification(title, options) {
				options = options || {};
				var body = options.body || '';
				if (notificationsStateReady && notificationsEnabled && window.sendNativeNotification) {
					window.sendNativeNotification(title, body);
				}
			}

			window.Notification = function(title, options) {
				options = options || {};
				dispatchNativeNotification(title, options);
				this.title = title;
				this.body = options.body || '';
				this.onclick = null;
				this.onclose = null;
				this.onerror = null;
				this.onshow = null;
			};
			window.Notification.permission = 'granted';
			window.Notification.maxActions = 2;
			window.Notification.requestPermission = function(callback) {
				var p = Promise.resolve('granted');
				if (typeof callback === 'function') {
					callback('granted');
				}
				return p;
			};

			try {
				if (typeof ServiceWorkerRegistration !== 'undefined' && ServiceWorkerRegistration.prototype) {
					ServiceWorkerRegistration.prototype.showNotification = function(title, options) {
						dispatchNativeNotification(title, options);
						return Promise.resolve();
					};
				}
			} catch (e) {}
		})();

		// Robust HTML5 Media Autoplay & Inline Playback Support for Status/Stories and Videos
		(function() {
			if (!window.HTMLMediaElement) return;

			function prepareMedia(el) {
				if (!el || el.__wa_media_ready) return;
				el.__wa_media_ready = true;
				if (el.tagName === 'VIDEO') {
					el.setAttribute('playsinline', '');
					el.setAttribute('webkit-playsinline', '');
					el.setAttribute('x5-playsinline', '');
				}
				if (!el.getAttribute('preload')) {
					el.setAttribute('preload', 'metadata');
				}
			}

			var origPlay = HTMLMediaElement.prototype.play;
			HTMLMediaElement.prototype.play = function() {
				var self = this;
				prepareMedia(self);
				var res = origPlay.apply(this, arguments);
				if (res && typeof res.catch === 'function') {
					return res.catch(function(err) {
						// When WebKit blocks unmuted autoplay, mute the media and retry playback
						if (err && (err.name === 'NotAllowedError' || err.name === 'AbortError')) {
							self.muted = true;
							return origPlay.apply(self);
						}
						return Promise.reject(err);
					});
				}
				return res;
			};

			// Automatically prepare video/audio elements injected into DOM. WhatsApp's
			// virtualized chat list mutates frequently, so queue only newly-added
			// subtrees and process them once per animation frame. Rescanning the entire
			// document on every busy frame makes scrolling unnecessarily expensive.
			if (window.MutationObserver) {
				var mediaScanScheduled = false;
				var pendingMediaRoots = [];
				function queueMediaRoot(node) {
					if (!node || node.nodeType !== 1 || pendingMediaRoots.length >= 24) return;
					try {
						if ((node.matches && node.matches('video, audio')) ||
							(node.querySelector && node.querySelector('video, audio'))) {
							pendingMediaRoots.push(node);
						}
					} catch (e) {}
				}
				function scanForUnpreparedMedia() {
					mediaScanScheduled = false;
					var roots = pendingMediaRoots.splice(0, pendingMediaRoots.length);
					if (shouldPauseBackgroundWork()) return;
					for (var r = 0; r < roots.length; r++) {
						var root = roots[r];
						if (root.matches && root.matches('video, audio')) prepareMedia(root);
						if (!root.querySelectorAll) continue;
						var list = root.querySelectorAll('video, audio');
						for (var l = 0; l < list.length; l++) prepareMedia(list[l]);
					}
				}
				function scheduleMediaScan() {
					if (mediaScanScheduled || pendingMediaRoots.length === 0) return;
					mediaScanScheduled = true;
					requestAnimationFrame(scanForUnpreparedMedia);
				}
				var mediaObserver = new MutationObserver(function(mutations) {
					if (shouldPauseBackgroundWork()) return;
					for (var m = 0; m < mutations.length; m++) {
						var added = mutations[m].addedNodes;
						for (var n = 0; n < added.length; n++) queueMediaRoot(added[n]);
					}
					scheduleMediaScan();
				});
				var targetNode = document.documentElement || document.body;
				if (targetNode) {
					mediaObserver.observe(targetNode, { childList: true, subtree: true });
					queueMediaRoot(targetNode);
					scheduleMediaScan();
				} else {
					document.addEventListener('DOMContentLoaded', function() {
						mediaObserver.observe(document.body, { childList: true, subtree: true });
						queueMediaRoot(document.body);
						scheduleMediaScan();
					});
				}
			}
		})();

		function isDocumentFileName(name) {
			if (!name) return false;
			var ext = name.toLowerCase();
			return ext.endsWith('.pdf') || ext.endsWith('.doc') || ext.endsWith('.docx') ||
				   ext.endsWith('.xls') || ext.endsWith('.xlsx') || ext.endsWith('.ppt') ||
				   ext.endsWith('.pptx') || ext.endsWith('.txt') || ext.endsWith('.csv') ||
				   ext.endsWith('.rtf');
		}

		// Dismiss WhatsApp Web's internal stuck viewer overlay
		function dismissStuckViewer() {
			var attempts = 0;
			// Bounded and gentle on purpose. This used to hammer WhatsApp's
			// viewer for 2.4s (30 rounds of 80ms), clicking close buttons and
			// dispatching synthetic Escape events - which fought the page and
			// helped turn a re-opened document into a preview loop. Eight
			// rounds is plenty for the overlay to mount, and nothing is sent
			// when there is no viewer to dismiss.
			var maxAttempts = 8;
			var dismissTimer = setInterval(function() {
				attempts++;
				if (attempts > maxAttempts) {
					clearInterval(dismissTimer);
					return;
				}
				var viewer = document.querySelector('[data-testid="media-viewer"], [data-animate-media-viewer="true"]');
				if (!viewer) {
					// No viewer: stop early instead of polling the void.
					if (attempts > 3) clearInterval(dismissTimer);
					return;
				}
				var closeSelectors = [
					'button[data-testid="x-viewer"]',
					'[data-testid="x-viewer"]',
					'[data-icon="x-viewer"]',
					'[data-icon="x"]',
					'[data-icon="back"]',
					'button[aria-label*="Close" i]',
					'button[aria-label*="Tutup" i]',
					'[role="button"][aria-label*="Close" i]',
					'[role="button"][aria-label*="Tutup" i]',
					'button[title*="Close" i]',
					'button[title*="Tutup" i]',
					'[data-testid="btn-close"]',
					'[data-testid="media-viewer-close"]'
				];
				var closed = false;
				for (var i = 0; i < closeSelectors.length; i++) {
					try {
						var el = viewer.querySelector(closeSelectors[i]) || document.querySelector(closeSelectors[i]);
						if (el) {
							var btn = (el.closest && el.closest('button, [role="button"]')) || el;
							btn.click();
							closed = true;
							break;
						}
					} catch (e) {}
				}
				// Dispatch synthetic Escape tagged so our preview modal ignores it
				var escEvt = new KeyboardEvent('keydown', { key: 'Escape', code: 'Escape', keyCode: 27, which: 27, bubbles: true, cancelable: true });
				escEvt._waViewerDismiss = true;
				try {
					viewer.dispatchEvent(escEvt);
					var app = document.getElementById('app');
					if (app) app.dispatchEvent(escEvt);
				} catch (e) {}

				if (closed || attempts > 6) {
					viewer.style.display = 'none';
					clearInterval(dismissTimer);
				}
			}, 80);
		}
		window.dismissStuckViewer = dismissStuckViewer;

		// Track clicked document filenames with robust regex matching
		var lastClickedDocName = '';
		var lastDocumentIntentAt = 0;
		// Explicit-download marker (upstream #18): clicking Download itself
		// (viewer button, context-menu item, download anchor) means save
		// only — the in-app preview is reserved for clicking the document.
		// The blob hook below stands down while this is recent so one user
		// action never saves+previews twice.
		var lastExplicitDownloadAt = 0;
		function isRecentExplicitDownload() {
			return (Date.now() - lastExplicitDownloadAt) < 6000;
		}
		// Preview loop guard (upstream v1.6.1): dismissing WhatsApp's own
		// viewer can make it re-create the attachment blob, which re-enters
		// the createObjectURL interceptor and re-opens our preview window in
		// an infinite loop. The same document is only auto-previewed once
		// per window (cooldown below); a fresh user click resets the guard
		// and is honoured again.
		var lastDocPreviewName = '';
		var lastDocPreviewAt = 0;
		var docPreviewCooldownMs = 8000;
		function extractDocumentName(el) {
			if (!el || typeof el.closest !== 'function') return '';
			// NEVER extract document names from inside the media viewer, modal dialogs, or top toolbars
			if (el.closest('[data-testid="media-viewer"]') ||
			    el.closest('#wa-doc-modal-overlay') ||
			    el.closest('[role="toolbar"]') ||
			    el.closest('header')) {
				return '';
			}

			// Only search within a chat message container / row / bubble
			var msgContainer = el.closest('[data-testid*="msg-container"], [role="row"], div[data-id], .message-in, .message-out');
			if (!msgContainer) return '';

			var node = el;
			while (node && node !== msgContainer.parentElement && node !== document.body) {
				var title = node.getAttribute && (node.getAttribute('title') || node.getAttribute('aria-label') || '');
				var titleMatch = title && title.match(/([^\n\r<>]{1,180}\.(pdf|docx?|xlsx?|pptx?|txt|csv|rtf))\b/i);
				if (titleMatch && titleMatch[1]) return titleMatch[1].trim();

				// Check text only on leaf-ish nodes to prevent matching unrelated long container text
				if (!node.children || node.children.length < 5) {
					var text = (node.innerText || '').trim();
					if (text.length > 0 && text.length < 250) {
						var textMatch = text.match(/([^\n\r<>]{1,180}\.(pdf|docx?|xlsx?|pptx?|txt|csv|rtf))\b/i);
						if (textMatch && textMatch[1]) return textMatch[1].trim();
					}
				}
				if (node === msgContainer) break;
				node = node.parentElement;
			}
			return '';
		}
		function isRecentPDFIntent() {
			return !!lastClickedDocName && isDocumentFileName(lastClickedDocName) &&
				(Date.now() - lastDocumentIntentAt) < 20000;
		}
		document.addEventListener('click', function(e) {
			var name = extractDocumentName(e.target);
			if (name) {
				lastClickedDocName = name;
				lastDocumentIntentAt = Date.now();
				// An explicit click is fresh intent: clear the loop guard so
				// the same document can be previewed again on purpose.
				lastDocPreviewName = '';
				lastDocPreviewAt = 0;
			}
		}, true);

		window.closeDocumentViewerAfterNativePreview = function() {
			var selectors = [
				'button[data-testid="x-viewer"]', '[data-testid="x-viewer"]',
				'[data-icon="x-viewer"]', '[data-icon="x"]', '[data-icon="back"]',
				'button[aria-label*="Close" i]', 'button[aria-label*="Tutup" i]',
				'[role="button"][aria-label*="Close" i]', '[role="button"][aria-label*="Tutup" i]',
				'button[title*="Close" i]', 'button[title*="Tutup" i]'
			].join(',');
			var candidates = document.querySelectorAll(selectors);
			var best = null;
			var bestScore = -1;
			for (var i = 0; i < candidates.length; i++) {
				var raw = candidates[i];
				if (raw.closest && raw.closest('#wa-doc-modal-overlay')) continue;
				var control = (raw.closest && raw.closest('button, [role="button"]')) || raw;
				var rect = control.getBoundingClientRect();
				if (rect.width < 8 || rect.height < 8 || rect.bottom <= 0 || rect.right <= 0 ||
					rect.top >= window.innerHeight || rect.left >= window.innerWidth) continue;
				var style = window.getComputedStyle(control);
				if (style.display === 'none' || style.visibility === 'hidden' || Number(style.opacity) === 0) continue;
				var score = 0;
				if (rect.top < window.innerHeight * 0.3) score += 4;
				if (rect.left > window.innerWidth * 0.7) score += 4;
				if (control.closest && control.closest('[role="dialog"], [data-testid*="viewer"], header, [role="toolbar"]')) score += 5;
				if (score > bestScore) { best = control; bestScore = score; }
			}

			if (best && bestScore >= 8) {
				best.click();
			} else {
				var esc = new KeyboardEvent('keydown', {
					key: 'Escape', code: 'Escape', keyCode: 27, which: 27,
					bubbles: true, cancelable: true
				});
				document.dispatchEvent(esc);
				window.dispatchEvent(esc);
			}
			lastDocumentIntentAt = 0;
			lastClickedDocName = '';
		};

		// Handle explicit user clicks on WhatsApp Web's Media Viewer ✕ close button
		// Guarantees immediate exit to chat view even if internal viewer state is desynced
		document.addEventListener('click', function(e) {
			var target = e.target;
			if (!target || typeof target.closest !== 'function') return;
			var viewer = target.closest('[data-testid="media-viewer"]');
			if (!viewer) return;

			var isCloseBtn = target.closest([
				'button[data-testid="x-viewer"]',
				'[data-testid="x-viewer"]',
				'[data-icon="x-viewer"]',
				'[data-icon="x"]',
				'[data-icon="back"]',
				'button[aria-label*="Close" i]',
				'button[aria-label*="Tutup" i]',
				'button[title*="Close" i]',
				'button[title*="Tutup" i]',
				'[data-testid="btn-close"]'
			].join(','));

			if (isCloseBtn) {
				lastDocumentIntentAt = 0;
				lastClickedDocName = '';
				setTimeout(function() {
					var activeViewer = document.querySelector('[data-testid="media-viewer"]');
					if (activeViewer) {
						var escEvt = new KeyboardEvent('keydown', {
							key: 'Escape',
							code: 'Escape',
							keyCode: 27,
							which: 27,
							bubbles: true,
							cancelable: true
						});
						document.dispatchEvent(escEvt);
						window.dispatchEvent(escEvt);
					}
				}, 60);
			}
		}, false);

		// Intercept external link clicks to open in default browser
		document.addEventListener('click', function(e) {
			var target = e.target;
			while (target && target !== document.body && target.tagName !== 'A') {
				target = target.parentElement;
			}
			if (target && target.tagName === 'A' && target.href) {
				try {
					var url = new URL(target.href);
					if (!url.hostname.endsWith('whatsapp.com') && !url.hostname.endsWith('whatsapp.net') && (url.protocol === 'http:' || url.protocol === 'https:')) {
						e.preventDefault();
						e.stopPropagation();
						if (window.openExternalLink) {
							window.openExternalLink(target.href);
						}
					}
				} catch(err) {}
			}
		}, true);

		// Drag & Drop file upload to chat
		(function() {
			var dropZone = null;
			var dragCounter = 0;

			function getDropZone() {
				// WhatsApp Web's main chat area where files can be dropped
				return document.querySelector('#main') || document.querySelector('[data-testid="conversation-panel"]') || document.body;
			}

			function handleDragEnter(e) {
				dragCounter++;
				e.preventDefault();
				e.stopPropagation();
				var dz = getDropZone();
				if (dz) dz.classList.add('wa-drag-over');
			}

			function handleDragLeave(e) {
				dragCounter--;
				if (dragCounter <= 0) {
					dragCounter = 0;
					var dz = getDropZone();
					if (dz) dz.classList.remove('wa-drag-over');
				}
			}

			function handleDragOver(e) {
				e.preventDefault();
				e.stopPropagation();
				e.dataTransfer.dropEffect = 'copy';
			}

			// A drop landing anywhere (dialog, settings modal, cloud
			// placeholder with an empty file list) must clear the highlight:
			// the class disables pointer events app-wide, so a stuck one
			// silently eats every click until restart.
			function resetDragState() {
				dragCounter = 0;
				var dz = getDropZone();
				if (dz) dz.classList.remove('wa-drag-over');
			}

			function findFileInput() {
				return document.querySelector('input[type="file"][accept*="*"], input[type="file"][accept*="image"], input[type="file"][accept*="video"], input[type="file"][accept*="document"], input[type="file"][accept*="audio"]');
			}

			async function handleDrop(e) {
				e.preventDefault();
				e.stopPropagation();
				resetDragState();

				var files = e.dataTransfer.files;
				if (!files || files.length === 0) return;

				// Find the file input for the attach menu
				var attachBtn = document.querySelector('[data-testid="clip"], [data-icon="clip"], [aria-label*="Attach"], [aria-label*="Lampirkan"]');
				if (attachBtn) {
					attachBtn.click();
					// Probe for the file input over several rounds instead of
					// a single fixed delay: when WhatsApp's editor mounts
					// slower than one check, a single early injection lands a
					// second copy over the batch WhatsApp already accepted.
					// Inject only while the input is still empty.
					var rounds = 0;
					var probeTimer = setInterval(function() {
						rounds++;
						var fileInput = findFileInput();
						if (fileInput && fileInput.files.length === 0) {
							// Create a DataTransfer to set files on the input
							var dt = new DataTransfer();
							for (var i = 0; i < files.length; i++) {
								dt.items.add(files[i]);
							}
							fileInput.files = dt.files;
							// Trigger change event
							var event = new Event('change', { bubbles: true });
							fileInput.dispatchEvent(event);
							clearInterval(probeTimer);
						} else if ((fileInput && fileInput.files.length > 0) || rounds >= 8) {
							clearInterval(probeTimer);
						}
					}, 200);
				}
			}

			function initDragDrop() {
				var dz = getDropZone();
				if (dz) {
					dz.addEventListener('dragenter', handleDragEnter, true);
					dz.addEventListener('dragleave', handleDragLeave, true);
					dz.addEventListener('dragover', handleDragOver, true);
					dz.addEventListener('drop', handleDrop, true);
				}
				// Global safety net, registered once: any drop, drag
				// cancellation, or window blur clears the highlight, even
				// when the gesture ends outside the chat drop zone.
				if (!initDragDrop.guarded) {
					initDragDrop.guarded = true;
					document.addEventListener('drop', resetDragState, true);
					document.addEventListener('dragend', resetDragState, true);
					window.addEventListener('blur', resetDragState);
				}
			}

			// Initialize when DOM is ready
			if (document.readyState === 'loading') {
				document.addEventListener('DOMContentLoaded', initDragDrop);
			} else {
				initDragDrop();
			}

			// Re-initialize on navigation (WhatsApp Web is SPA)
			var lastUrl = location.href;
			setInterval(function() {
				if (location.href !== lastUrl) {
					lastUrl = location.href;
					setTimeout(initDragDrop, 500);
				}
			}, 1000);
		})();

		// Helper: Decode base64 dataURI to Uint8Array
		function base64ToUint8Array(dataUri) {
			try {
				var base64 = dataUri.indexOf(';base64,') !== -1 ? dataUri.split(';base64,')[1] : dataUri;
				var binary = atob(base64);
				var bytes = new Uint8Array(binary.length);
				for (var i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
				return bytes;
			} catch (e) {
				return null;
			}
		}

		async function decompressDeflateRaw(compressedData) {
			if (typeof DecompressionStream === 'undefined') return null;
			try {
				var ds = new DecompressionStream('deflate-raw');
				var stream = new Response(compressedData).body.pipeThrough(ds);
				return await new Response(stream).text();
			} catch (e) {
				try {
					var ds2 = new DecompressionStream('deflate');
					var stream2 = new Response(compressedData).body.pipeThrough(ds2);
					return await new Response(stream2).text();
				} catch (e2) {
					return null;
				}
			}
		}

		// Helper: Read a specific file from ZIP payload (e.g. word/document.xml, xl/worksheets/sheet1.xml)
		async function readZipEntryText(uint8Array, targetPath) {
			if (!uint8Array || uint8Array.length < 30) return null;
			try {
				var view = new DataView(uint8Array.buffer, uint8Array.byteOffset, uint8Array.byteLength);
				var offset = 0;
				while (offset < uint8Array.length - 30) {
					if (view.getUint32(offset, true) === 0x04034b50) {
						var compMethod = view.getUint16(offset + 8, true);
						var compSize = view.getUint32(offset + 18, true);
						var nameLen = view.getUint16(offset + 26, true);
						var extraLen = view.getUint16(offset + 28, true);
						var nameBytes = uint8Array.subarray(offset + 30, offset + 30 + nameLen);
						var name = new TextDecoder().decode(nameBytes);
						var dataStart = offset + 30 + nameLen + extraLen;
						var dataEnd = dataStart + compSize;

						if (name.toLowerCase() === targetPath.toLowerCase()) {
							var compressedData = uint8Array.subarray(dataStart, dataEnd);
							if (compMethod === 0) {
								return new TextDecoder().decode(compressedData);
							} else if (compMethod === 8) {
								return await decompressDeflateRaw(compressedData);
							}
						}
						offset = dataEnd > offset ? dataEnd : (offset + 1);
					} else {
						offset++;
					}
				}
			} catch (e) {
				console.warn('Zip read error:', e);
			}
			return null;
		}

		function parsePptxToHtml(slideXmls) {
			if (!slideXmls || !slideXmls.length) return '';
			var html = ['<div style="width:100%;height:100%;overflow-y:auto;padding:24px 16px;box-sizing:border-box;display:flex;flex-direction:column;align-items:center;background:#0c1317;contain:strict;">'];
			for (var i = 0; i < slideXmls.length; i++) {
				var xml = slideXmls[i];
				if (!xml) continue;
				var tMatches = xml.match(/<a:t\b[^>]*>([\s\S]*?)<\/a:t>/g) || [];
				var lines = [];
				for (var t = 0; t < tMatches.length; t++) {
					var rawT = tMatches[t].replace(/<a:t\b[^>]*>|<\/a:t>/g, '');
					rawT = rawT.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').trim();
					if (rawT) lines.push(rawT);
				}
				if (lines.length) {
					html.push('<div style="width:100%;max-width:760px;background:#ffffff;border-radius:8px;box-shadow:0 4px 16px rgba(0,0,0,0.4);padding:28px 32px;box-sizing:border-box;margin-bottom:18px;">');
					html.push('<div style="font-size:11px;font-weight:700;color:#00a884;text-transform:uppercase;margin-bottom:10px;letter-spacing:0.5px;">Slide ' + (i + 1) + '</div>');
					html.push('<h3 style="font-size:17px;font-weight:700;margin:0 0 10px;color:#111b21;">' + lines[0] + '</h3>');
					for (var l = 1; l < lines.length; l++) {
						html.push('<p style="font-size:13px;color:#3b4a54;margin:5px 0;line-height:1.5;">• ' + lines[l] + '</p>');
					}
					html.push('</div>');
				}
			}
			html.push('</div>');
			return html.length > 2 ? html.join('') : '';
		}

		function parseDocxToHtml(xmlStr) {
			if (!xmlStr) return '';
			var pMatches = xmlStr.match(/<w:p\b[\s\S]*?<\/w:p>/g) || [];
			var html = [];
			for (var i = 0; i < pMatches.length; i++) {
				var pStr = pMatches[i];
				var isH1 = /<w:pStyle\b[^>]*w:val="Heading1"/i.test(pStr);
				var isH2 = /<w:pStyle\b[^>]*w:val="Heading2"/i.test(pStr);
				var isH3 = /<w:pStyle\b[^>]*w:val="Heading[3-6]"/i.test(pStr);
				var rMatches = pStr.match(/<w:r\b[\s\S]*?<\/w:r>/g) || [];
				var pText = '';
				for (var j = 0; j < rMatches.length; j++) {
					var rStr = rMatches[j];
					var isBold = /<w:b\b/.test(rStr);
					var isItalic = /<w:i\b/.test(rStr);
					var tMatches = rStr.match(/<w:t\b[^>]*>([\s\S]*?)<\/w:t>/g) || [];
					for (var k = 0; k < tMatches.length; k++) {
						var tVal = tMatches[k].replace(/<w:t\b[^>]*>|<\/w:t>/g, '');
						tVal = tVal.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
						if (isBold) tVal = '<strong>' + tVal + '</strong>';
						if (isItalic) tVal = '<em>' + tVal + '</em>';
						pText += tVal;
					}
				}
				if (pText.trim()) {
					if (isH1) html.push('<h2 style="color:#111b21;margin:18px 0 8px;font-size:18px;font-weight:700;">' + pText + '</h2>');
					else if (isH2) html.push('<h3 style="color:#111b21;margin:14px 0 6px;font-size:16px;font-weight:600;">' + pText + '</h3>');
					else if (isH3) html.push('<h4 style="color:#111b21;margin:12px 0 4px;font-size:14px;font-weight:600;">' + pText + '</h4>');
					else html.push('<p style="color:#222e35;margin:8px 0;line-height:1.65;font-size:13.5px;">' + pText + '</p>');
				}
			}
			return html.join('');
		}

		// Spreadsheet preview (.xlsx, .xls, .csv) is rendered via the bundled SheetJS
		// library (see renderSpreadsheetPreview / showInAppDocModal below), which can
		// read both modern OOXML and legacy binary Excel formats directly from bytes,
		// so a hand-rolled XML/CSV parser is no longer needed here.

		// In-App Document Preview Modal Overlay (PDF, Excel, Word, Text)
		// Filenames and paths here come from chat content or release
		// metadata, so they must never be concatenated raw: a name like
		// '"><img src=x onerror=...>x.pdf' would otherwise execute in the
		// privileged page context that can reach every native bridge.
		// Valid names render identically.
		function escapeHtml(s) {
			return String(s == null ? '' : s).replace(/[&<>"']/g, function(c) {
				return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c];
			});
		}
		function showInAppDocModal(filename, blobUrl, savedPath, dataUri, ownedBlobUrl) {
			var existing = document.getElementById('wa-doc-modal-overlay');
			if (existing && existing.parentNode) existing.parentNode.removeChild(existing);

			// Immediately dismiss WhatsApp Web's stuck background viewer
			if (window.dismissStuckViewer) window.dismissStuckViewer();

			var ext = (filename && filename.indexOf('.') !== -1 ? filename.split('.').pop() : '').toLowerCase();
			var isPdf = ext === 'pdf';
			var isExcel = ext === 'xlsx' || ext === 'xls' || ext === 'csv';
			var isWord = ext === 'docx' || ext === 'doc' || ext === 'rtf' || ext === 'txt';
			var isPpt = ext === 'pptx' || ext === 'ppt';

			// WKWebView has no reliable built-in renderer for PDF blob URLs.
			// On macOS, render the already-saved file with PDFKit instead.
			if (isPdf && savedPath && window.showPDFPreviewNative) {
				if (window.dismissStuckViewer) window.dismissStuckViewer();
				window.showPDFPreviewNative(savedPath);
				if (ownedBlobUrl) {
					try { URL.revokeObjectURL(ownedBlobUrl); } catch (e) {}
				}
				return;
			}

			var docIcon = '📄';
			var openBtnText = '📂 Open in System App';
			var docTypeLabel = 'Document';
			if (isPdf) {
				docIcon = '📄';
				openBtnText = '📂 Open in System App';
				docTypeLabel = 'PDF Document';
			} else if (isExcel) {
				docIcon = '📊';
				openBtnText = '📊 Open in Excel / Numbers';
				docTypeLabel = 'Excel Spreadsheet';
			} else if (isWord) {
				docIcon = '📝';
				openBtnText = '📝 Open in Word / Pages';
				docTypeLabel = 'Word Document';
			} else if (isPpt) {
				docIcon = '📽️';
				openBtnText = '📽️ Open in PowerPoint / Keynote';
				docTypeLabel = 'PowerPoint Presentation';
			}

			var overlay = document.createElement('div');
			overlay.id = 'wa-doc-modal-overlay';
			// Flat dim, no backdrop blur: blurring the backdrop forces the
			// compositor to re-render the page behind the modal on every
			// scrolled frame, which made long previews scroll heavily.
			overlay.style.cssText = 'position:fixed;top:0;left:0;width:100%;height:100%;background:rgba(0,0,0,0.85);z-index:99999999;display:flex;flex-direction:column;align-items:center;justify-content:center;padding:16px;box-sizing:border-box;animation:waFadeIn 0.2s ease;';

			var modal = document.createElement('div');
			// Layer-promoted card: its own compositor layer, so scrolling the
			// page behind (or inside) never repaints the whole overlay.
			modal.style.cssText = 'width:94%;max-width:1020px;height:92%;background:#111b21;border:1px solid rgba(255,255,255,0.14);border-radius:12px;display:flex;flex-direction:column;overflow:hidden;box-shadow:0 24px 60px rgba(0,0,0,0.85);transform:translateZ(0);';

			// Header
			var header = document.createElement('div');
			header.style.cssText = 'display:flex;align-items:center;justify-content:space-between;padding:10px 16px;border-bottom:1px solid rgba(255,255,255,0.08);background:#202c33;flex-shrink:0;';
			header.innerHTML = '' +
				'<div style="display:flex;align-items:center;gap:10px;min-width:0;">' +
				'  <span style="font-size:22px;">' + docIcon + '</span>' +
				'  <div style="min-width:0;">' +
				'    <strong style="font-size:13.5px;color:#e9edef;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;display:block;max-width:420px;" title="' + escapeHtml(filename) + '">' + escapeHtml(filename) + '</strong>' +
				'    <span style="font-size:11px;color:#8696a0;">' + docTypeLabel + ' · Direct Preview</span>' +
				'  </div>' +
				'</div>' +
				'<div style="display:flex;align-items:center;gap:8px;">' +
				'  <button id="wa-btn-open-preview" style="background:#00a884;color:#111b21;border:none;padding:6px 14px;border-radius:6px;font-size:12px;font-weight:600;cursor:pointer;display:flex;align-items:center;gap:4px;box-shadow:0 2px 6px rgba(0,168,132,0.3);">' +
				'    ' + openBtnText +
				'  </button>' +
				'  <button id="wa-btn-folder-doc" style="background:#2a3942;color:#e9edef;border:1px solid rgba(255,255,255,0.1);padding:6px 12px;border-radius:6px;font-size:12px;font-weight:500;cursor:pointer;">' +
				'    📂 Show in Folder' +
				'  </button>' +
				'  <button id="wa-btn-save-doc" style="background:#2a3942;color:#e9edef;border:1px solid rgba(255,255,255,0.1);padding:6px 12px;border-radius:6px;font-size:12px;font-weight:500;cursor:pointer;">' +
				'    💾 Download' +
				'  </button>' +
				'  <button id="wa-btn-close-doc" style="background:transparent;border:none;color:#8696a0;cursor:pointer;font-size:20px;padding:4px 8px;border-radius:6px;line-height:1;">✕</button>' +
				'</div>';
			modal.appendChild(header);

			// Body Container
			var body = document.createElement('div');
			body.style.cssText = 'flex:1;width:100%;height:100%;position:relative;background:#0c1317;overflow:hidden;display:flex;flex-direction:column;align-items:center;justify-content:center;';
			modal.appendChild(body);

			function triggerOpenSystem() {
				if (savedPath && window.showPDFPreviewNative && isPdf) {
					window.showPDFPreviewNative(savedPath);
				} else if (savedPath && window.openFileNative) {
					window.openFileNative(savedPath);
				} else if (window.previewDocumentNative) {
					window.previewDocumentNative(filename, dataUri || blobUrl);
				}
			}

			function renderCardFallback(hint) {
				var displayPath = savedPath || 'Downloads folder';
				body.innerHTML = '' +
					'<div style="display:flex;flex-direction:column;align-items:center;justify-content:center;padding:40px;text-align:center;">' +
					'  <div style="font-size:64px;margin-bottom:16px;">' + docIcon + '</div>' +
					'  <h2 style="color:#e9edef;font-size:18px;font-weight:600;margin:0 0 8px;max-width:540px;word-break:break-all;">' + escapeHtml(filename) + '</h2>' +
					'  <div style="color:#00a884;font-size:12px;font-weight:600;text-transform:uppercase;letter-spacing:0.5px;margin-bottom:12px;">' + docTypeLabel + ' · Saved</div>' +
					'  <p style="color:#8696a0;font-size:13px;max-width:460px;line-height:1.5;margin:0 0 16px;">' +
					(hint || ('The ' + docTypeLabel + ' is saved on your computer. Click below to open it in your default application.')) +
					'  </p>' +
					'  <div style="font-family:monospace;font-size:11px;color:#8696a0;background:rgba(255,255,255,0.06);padding:6px 14px;border-radius:6px;max-width:520px;overflow:hidden;text-overflow:ellipsis;margin-bottom:24px;border:1px solid rgba(255,255,255,0.08);">' + escapeHtml(displayPath) + '</div>' +
					'  <div style="display:flex;gap:12px;align-items:center;">' +
					'    <button id="wa-btn-card-launch" style="background:#00a884;color:#111b21;border:none;padding:10px 24px;border-radius:8px;font-size:13.5px;font-weight:600;cursor:pointer;display:flex;align-items:center;gap:6px;box-shadow:0 4px 12px rgba(0,168,132,0.3);">' +
					openBtnText +
					'    </button>' +
					'    <button id="wa-btn-card-folder" style="background:#2a3942;color:#e9edef;border:1px solid rgba(255,255,255,0.1);padding:10px 20px;border-radius:8px;font-size:13px;font-weight:500;cursor:pointer;">' +
					'📂 Show in Folder' +
					'    </button>' +
					'  </div>' +
					'</div>';
				var cardBtn = document.getElementById('wa-btn-card-launch');
				if (cardBtn) cardBtn.onclick = triggerOpenSystem;
				var folderBtn = document.getElementById('wa-btn-card-folder');
				if (folderBtn) folderBtn.onclick = function() {
					if (window.openDownloadDirNative) window.openDownloadDirNative();
				};
			}

			// Lazy-load SheetJS (xlsx.core.min.js) only when spreadsheet preview is first needed.
			var xlsxLoadPromise = null;
			function ensureXLSXLoaded() {
				// Only a library that exposes XLSX.utils is usable; an empty stub would
				// make every later XLSX.utils call throw, so treat that as "not loaded".
				if (window.XLSX && window.XLSX.utils) return Promise.resolve();
				if (xlsxLoadPromise) return xlsxLoadPromise;
				xlsxLoadPromise = new Promise(function(resolve, reject) {
					// Fetch the bundled SheetJS from the native side
					if (window.loadXLSXLibraryNative) {
						window.loadXLSXLibraryNative().then(function(jsCode) {
							try {
								  eval(jsCode);
								  // If the host page happens to expose CommonJS exports/module,
								  // the SheetJS core build initialises that object instead of a global
								  // and leaves window.XLSX as an empty stub. The direct eval above also
								  // created an eval-scoped XLSX binding, so prefer it in that case.
								  if (typeof XLSX !== 'undefined' && (!window.XLSX || !window.XLSX.utils)) {
								      window.XLSX = XLSX;
								  }
								  if (!window.XLSX || !window.XLSX.utils) {
								      throw new Error('spreadsheet library failed to initialise');
								  }
								  resolve();
							} catch (e) {
								  reject(e);
							}
						}).catch(reject);
					} else {
						reject(new Error('loadXLSXLibraryNative not available'));
					}
				});
				return xlsxLoadPromise;
			}

			// Render a parsed spreadsheet workbook (from the bundled SheetJS library) as an
			// HTML table, with a sheet-switcher tab bar when the workbook has multiple sheets.
			// XLSX.utils.sheet_to_html escapes cell text but writes the raw value
			// into a data-v attribute, so a cell whose value is '"><img src=x
			// onerror=...>' closes the attribute early and injects live markup --
			// reachable from any spreadsheet sent in a chat. Parse the generated
			// markup inside an inert <template> (its content is a separate document
			// fragment, so images do not load and handlers never fire) and drop
			// every attribute we do not control, leaving cell content as text only.
			function sanitizeSheetHtml(html) {
				var tpl = document.createElement('template');
				tpl.innerHTML = String(html == null ? '' : html);
				// Elements a spreadsheet cell must never be able to introduce. The
				// attribute pass below already strips on* handlers and src/href, but
				// leaving an inert <img>/<iframe> behind would still be a rendering
				// artifact, so remove them outright.
				var banned = tpl.content.querySelectorAll('script,style,img,svg,iframe,frame,object,embed,link,meta,base,form,input,button,textarea,select,audio,video,source,track,math,template');
				for (var b = banned.length - 1; b >= 0; b--) {
					if (banned[b].parentNode) banned[b].parentNode.removeChild(banned[b]);
				}
				var nodes = tpl.content.querySelectorAll('*');
				for (var i = 0; i < nodes.length; i++) {
					var attrs = nodes[i].attributes;
					for (var a = attrs.length - 1; a >= 0; a--) {
						var attrName = attrs[a].name.toLowerCase();
						// id carries our wa-xlsx-table styling hook; colspan and
						// rowspan are structural. Everything else goes.
						if (attrName === 'id' || attrName === 'colspan' || attrName === 'rowspan') continue;
						nodes[i].removeAttribute(attrs[a].name);
					}
				}
				return tpl.innerHTML;
			}
			function renderSpreadsheetPreview(workbook, activeSheetName) {
				var sheetNames = (workbook && workbook.SheetNames) || [];
				if (!sheetNames.length) {
					renderCardFallback('This spreadsheet has no readable sheets.');
					return;
				}
				var activeName = (activeSheetName && sheetNames.indexOf(activeSheetName) !== -1) ? activeSheetName : sheetNames[0];
				var worksheet = workbook.Sheets[activeName];
				var tableHtml = sanitizeSheetHtml(XLSX.utils.sheet_to_html(worksheet, { id: 'wa-xlsx-table' }));

				var tabsHtml = '';
				if (sheetNames.length > 1) {
					tabsHtml = '<div id="wa-xlsx-tabs" style="display:flex;gap:4px;padding:8px 12px;background:#202c33;border-bottom:1px solid #2a3942;overflow-x:auto;flex-shrink:0;">';
					for (var si = 0; si < sheetNames.length; si++) {
						var name = sheetNames[si];
						var active = name === activeName;
						var safeName = name.replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
						tabsHtml += '<button data-sheet="' + safeName + '" style="padding:5px 12px;border-radius:6px;font-size:11.5px;font-weight:500;cursor:pointer;white-space:nowrap;border:1px solid ' + (active ? '#00a884' : '#2a3942') + ';background:' + (active ? '#00a884' : 'transparent') + ';color:' + (active ? '#111b21' : '#8696a0') + ';">' + safeName + '</button>';
					}
					tabsHtml += '</div>';
				}

				var tableStyle = '<style>#wa-xlsx-table{border-collapse:collapse;width:100%;font-family:system-ui,-apple-system,sans-serif;font-size:12px;color:#e9edef;}#wa-xlsx-table td,#wa-xlsx-table th{border:1px solid #2a3942;padding:6px 10px;white-space:nowrap;}#wa-xlsx-table tr:nth-child(even){background:#182229;}#wa-xlsx-table tr:nth-child(odd){background:#111b21;}</style>';

				body.innerHTML = '<div style="width:100%;height:100%;display:flex;flex-direction:column;">' + tabsHtml +
					'<div style="flex:1;overflow:auto;background:#111b21;contain:strict;">' + tableStyle + tableHtml + '</div></div>';

				var tabsEl = document.getElementById('wa-xlsx-tabs');
				if (tabsEl) {
					var tabBtns = tabsEl.querySelectorAll('button');
					for (var bi2 = 0; bi2 < tabBtns.length; bi2++) {
						tabBtns[bi2].onclick = function() {
							renderSpreadsheetPreview(workbook, this.getAttribute('data-sheet'));
						};
					}
				}
			}

			// Render content according to file type
			if (isPdf) {
				var pdfSrc = ownedBlobUrl || blobUrl || '';
				if ((!pdfSrc || pdfSrc.indexOf('blob:') !== 0) && dataUri && dataUri.indexOf(';base64,') !== -1) {
					pdfSrc = 'data:application/pdf;base64,' + dataUri.split(';base64,')[1];
				}
				if (pdfSrc) {
					body.innerHTML = '<iframe src="' + pdfSrc + '" style="width:100%;height:100%;border:none;background:#525659;" title="' + escapeHtml(filename) + '"></iframe>';
				} else {
					renderCardFallback();
				}
			} else if (ext === 'csv' || ext === 'xlsx' || ext === 'xls') {
				body.innerHTML = '<div style="color:#8696a0;font-size:13px;display:flex;align-items:center;gap:8px;">⏳ Loading spreadsheet preview...</div>';
				var rawXlsxB64 = (dataUri || '').indexOf(';base64,') !== -1 ? dataUri.split(';base64,')[1] : (dataUri || '');
				if (rawXlsxB64) {
					ensureXLSXLoaded().then(function() {
						try {
							// SheetJS auto-detects the real format from the bytes (OOXML zip for
							// .xlsx, binary OLE2/BIFF for legacy .xls, or plain text for .csv), so
							// one code path correctly previews all three, including .xls which the
							// previous hand-rolled parser never actually supported.
							var workbook = XLSX.read(rawXlsxB64, { type: 'base64', cellDates: true });
							renderSpreadsheetPreview(workbook);
						} catch (e) {
							renderCardFallback('Unable to render an in-app preview for this spreadsheet. Click below to open it in your default application.');
						}
					}).catch(function() {
						renderCardFallback('Unable to load spreadsheet library.');
					});
				} else {
					renderCardFallback();
				}
			} else if (ext === 'docx') {
				body.innerHTML = '<div style="color:#8696a0;font-size:13px;display:flex;align-items:center;gap:8px;">⏳ Loading Word preview...</div>';
				var uint8Doc = base64ToUint8Array(dataUri || '');
				if (uint8Doc) {
					var parsePromiseDoc = readZipEntryText(uint8Doc, 'word/document.xml');
					var timeoutPromiseDoc = new Promise(function(resolve) { setTimeout(function() { resolve(null); }, 1500); });
					Promise.race([parsePromiseDoc, timeoutPromiseDoc]).then(function(docXml) {
						if (docXml) {
							var docHtml = parseDocxToHtml(docXml);
						body.innerHTML = '' +
							'<div style="width:100%;height:100%;overflow-y:auto;padding:24px 16px;box-sizing:border-box;display:flex;justify-content:center;background:#0c1317;contain:strict;">' +
								'  <div style="width:100%;max-width:760px;background:#ffffff;border-radius:6px;box-shadow:0 4px 20px rgba(0,0,0,0.5);padding:40px 48px;box-sizing:border-box;min-height:90%;">' +
								docHtml +
								'  </div>' +
								'</div>';
						} else {
							renderCardFallback();
						}
					}).catch(function() {
						renderCardFallback();
					});
				} else {
					renderCardFallback();
				}
			} else if (ext === 'doc') {
				renderCardFallback('Word 97-2003 Document (.doc). Click below to open in Microsoft Word or default application.');
			} else if (ext === 'pptx') {
				body.innerHTML = '<div style="color:#8696a0;font-size:13px;display:flex;align-items:center;gap:8px;">⏳ Loading PowerPoint preview...</div>';
				var uint8Ppt = base64ToUint8Array(dataUri || '');
				if (uint8Ppt) {
					var parsePromisePpt = Promise.all([
						readZipEntryText(uint8Ppt, 'ppt/slides/slide1.xml'),
						readZipEntryText(uint8Ppt, 'ppt/slides/slide2.xml'),
						readZipEntryText(uint8Ppt, 'ppt/slides/slide3.xml'),
						readZipEntryText(uint8Ppt, 'ppt/slides/slide4.xml'),
						readZipEntryText(uint8Ppt, 'ppt/slides/slide5.xml')
					]);
					var timeoutPromisePpt = new Promise(function(resolve) { setTimeout(function() { resolve(null); }, 1500); });
					Promise.race([parsePromisePpt, timeoutPromisePpt]).then(function(slides) {
						var validSlides = slides ? slides.filter(Boolean) : [];
						if (validSlides.length) {
							body.innerHTML = parsePptxToHtml(validSlides);
						} else {
							renderCardFallback();
						}
					}).catch(function() {
						renderCardFallback();
					});
				} else {
					renderCardFallback();
				}
			} else if (ext === 'ppt') {
				renderCardFallback('PowerPoint 97-2003 Presentation (.ppt). Click below to open in PowerPoint or default application.');
			} else if (ext === 'txt' || ext === 'rtf' || ext === 'log') {
				try {
					var rawTxtB64 = (dataUri || '').indexOf(';base64,') !== -1 ? (dataUri || '').split(';base64,')[1] : (dataUri || '');
					var binTxt = atob(rawTxtB64);
					var bytesTxt = new Uint8Array(binTxt.length);
					for (var ti = 0; ti < binTxt.length; ti++) bytesTxt[ti] = binTxt.charCodeAt(ti);
					var textContent = new TextDecoder('utf-8').decode(bytesTxt);
					body.innerHTML = '<div style="width:100%;height:100%;overflow:auto;padding:24px;box-sizing:border-box;background:#111b21;color:#e9edef;font-family:monospace;font-size:13px;line-height:1.6;white-space:pre-wrap;contain:strict;">' +
						textContent.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;') +
						'</div>';
				} catch (e) {
					renderCardFallback();
				}
			} else {
				renderCardFallback();
			}

			overlay.appendChild(modal);
			document.body.appendChild(overlay);

			// Tell the host a document preview is open: the renderer recycler
		// must not rebuild underneath the modal.
			if (window.setBusyStateNative) window.setBusyStateNative('docmodal', true);

			function closeDocModal() {
				window.removeEventListener('keydown', onEsc);
				if (overlay.parentNode) overlay.parentNode.removeChild(overlay);
				if (ownedBlobUrl) {
					try { URL.revokeObjectURL(ownedBlobUrl); } catch (e) {}
					ownedBlobUrl = '';
				}
				dataUri = '';
				if (window.setBusyStateNative) window.setBusyStateNative('docmodal', false);
				if (window.dismissStuckViewer) window.dismissStuckViewer();
			}

			document.getElementById('wa-btn-close-doc').onclick = closeDocModal;
			overlay.onclick = function(e) {
				if (e.target === overlay) closeDocModal();
			};

			document.getElementById('wa-btn-open-preview').onclick = triggerOpenSystem;

			var btnFolder = document.getElementById('wa-btn-folder-doc');
			if (btnFolder) {
				btnFolder.onclick = function() {
					if (window.openDownloadDirNative) window.openDownloadDirNative();
				};
			}

			document.getElementById('wa-btn-save-doc').onclick = function() {
				if (dataUri && window.saveDownloadedFileNative) {
					window.saveDownloadedFileNative(filename, dataUri).then(function(res) {
						var p = res && res.path;
						if (p) showDownloadToast(((res && res.alreadyExisted) ? '💾 Already saved: ' : '💾 Saved: ') + filename);
					});
				} else if (savedPath) {
					showDownloadToast('💾 File is saved at: ' + savedPath);
				}
			};

			var onEsc = function(e) {
				if (e.key === 'Escape' && e.isTrusted && !e._waViewerDismiss) {
					closeDocModal();
				}
			};
			window.addEventListener('keydown', onEsc);
		}
		window.showInAppDocModal = showInAppDocModal;

		// Renderer recycler handshakes (Windows tab shell): report activity
		// that must block an in-place renderer rebuild, and answer the
		// host's staleness probe. Best-effort no-ops elsewhere.
		(function() {
			function waReportBusy(kind, on) {
				if (window.setBusyStateNative) {
					try { Promise.resolve(window.setBusyStateNative(kind, !!on)).catch(function() {}); } catch (e) {}
				}
			}
			window.__waBusyProbe = function(kind) {
				if (kind === 'docmodal') {
					waReportBusy('query-docmodal', !!document.getElementById('wa-doc-modal-overlay'));
				}
			};
			window.__waReportBusy = waReportBusy;
		})();

		// Intercept URL.createObjectURL to catch decrypted PDF/document blobs directly
		var origCreateObjectURL = URL.createObjectURL;
		URL.createObjectURL = function(blob) {
			var url = origCreateObjectURL.apply(this, arguments);
			try {
				var bType = (blob && blob.type) ? blob.type.toLowerCase() : '';
				var isDocBlob = bType.indexOf('pdf') >= 0 || bType.indexOf('officedocument') >= 0 ||
					bType.indexOf('msword') >= 0 || bType.indexOf('ms-excel') >= 0 ||
					bType.indexOf('spreadsheet') >= 0 || bType.indexOf('wordprocessing') >= 0 ||
					bType === 'text/csv' || bType === 'text/plain' ||
					(blob && (blob.type === 'application/octet-stream' || bType === '') && isRecentPDFIntent());

				if (blob && isDocBlob && !isRecentExplicitDownload()) {
					var name = lastClickedDocName || 'document';
					if (!name.includes('.')) {
						if (bType.indexOf('pdf') >= 0) name += '.pdf';
						else if (bType.indexOf('sheet') >= 0 || bType.indexOf('excel') >= 0) name += '.xlsx';
						else if (bType.indexOf('word') >= 0) name += '.docx';
						else name += '.pdf';
					}
					// Loop guard: WhatsApp re-creating the same attachment
					// blob (its own viewer being dismissed) must not re-open
					// the preview. Skip silently; a real user click clears
					// the guard in the click handler above.
					if (name === lastDocPreviewName && (Date.now() - lastDocPreviewAt) < docPreviewCooldownMs) {
						return url;
					}
					lastDocPreviewName = name;
					lastDocPreviewAt = Date.now();
					var isPdf = name.toLowerCase().endsWith('.pdf');
					var previewBlob = isPdf ? blob.slice(0, blob.size, 'application/pdf') : blob;
					var ownedBlobUrl = isPdf ? origCreateObjectURL(previewBlob) : '';
					var reader = new FileReader();
					reader.onloadend = function() {
						var base64data = reader.result;
						if (window.saveDownloadedFileNative) {
							window.saveDownloadedFileNative(name, base64data).then(function(res) {
								var savedPath = (res && res.path) || '';
								showInAppDocModal(name, ownedBlobUrl, savedPath, base64data, ownedBlobUrl);
								dismissStuckViewer();
								showDownloadToast(((res && res.alreadyExisted) ? '📄 Already saved: ' : '📄 Document preview: ') + name);
							});
						} else {
							showInAppDocModal(name, ownedBlobUrl, '', base64data, ownedBlobUrl);
							dismissStuckViewer();
						}
					};
					reader.readAsDataURL(blob);
				}
			} catch (e) {}
			return url;
		};

		function handleBlobDocumentPreview(blobUrl) {
			var name = lastClickedDocName || 'document.pdf';
			fetch(blobUrl)
				.then(function(res) { return res.blob(); })
				.then(function(blob) {
					var isPdf = name.toLowerCase().endsWith('.pdf');
					var previewBlob = isPdf ? blob.slice(0, blob.size, 'application/pdf') : blob;
					var ownedBlobUrl = isPdf ? origCreateObjectURL(previewBlob) : '';
					var reader = new FileReader();
					reader.onloadend = function() {
						var base64data = reader.result;
						if (window.saveDownloadedFileNative) {
							window.saveDownloadedFileNative(name, base64data).then(function(res) {
								var savedPath = (res && res.path) || '';
								showInAppDocModal(name, ownedBlobUrl, savedPath, base64data, ownedBlobUrl);
								dismissStuckViewer();
								showDownloadToast(((res && res.alreadyExisted) ? '📄 Already saved: ' : '📄 Document preview: ') + name);
							});
						} else {
							showInAppDocModal(name, ownedBlobUrl, '', base64data, ownedBlobUrl);
							dismissStuckViewer();
						}
					};
					reader.readAsDataURL(blob);
				})
				.catch(function(err) {
					console.error('Error handling blob preview:', err);
				});
		}

		// Intercept window.open for Blob URLs (PDF/Document previews) and external URLs
		var origWindowOpen = window.open;
		window.open = function(url, target, features) {
			if (url && typeof url === 'string') {
				if (url.indexOf('blob:') === 0) {
					handleBlobDocumentPreview(url);
					return null;
				}
				try {
					var parsed = new URL(url, window.location.href);
					if (!parsed.hostname.endsWith('whatsapp.com') && !parsed.hostname.endsWith('whatsapp.net') && (parsed.protocol === 'http:' || parsed.protocol === 'https:')) {
						if (window.openExternalLink) {
							window.openExternalLink(parsed.href);
							return null;
						}
					}
				} catch(err) {}
			}
			return origWindowOpen.apply(this, arguments);
		};

		// Display zoom (Cmd/Ctrl + = / - / 0 and Ctrl/Cmd + scroll wheel),
		// persisted across restarts and restored on every load. Steps follow
		// the same preset ladder a browser uses, so levels like 67%, 75% and
		// 125% are reachable. On Windows the level is applied natively at the
		// WebView2 controller, which reflows the page exactly like a browser;
		// the CSS fallback only covers platforms without that bridge.
		(function() {
			var currentZoom = 1.0;
			var zoomSaveTimer = null;
			var ZOOM_LEVELS = [0.5, 0.67, 0.75, 0.8, 0.9, 1.0, 1.1, 1.25, 1.5, 1.75, 2.0];
			function nearestZoomIndex(z) {
				var best = 0, bestDelta = Infinity;
				for (var i = 0; i < ZOOM_LEVELS.length; i++) {
					var d = Math.abs(ZOOM_LEVELS[i] - z);
					if (d < bestDelta) { bestDelta = d; best = i; }
				}
				return best;
			}
			function snapZoom(z) {
				if (!isFinite(z)) return 1.0;
				if (z < ZOOM_LEVELS[0]) return ZOOM_LEVELS[0];
				if (z > ZOOM_LEVELS[ZOOM_LEVELS.length - 1]) return ZOOM_LEVELS[ZOOM_LEVELS.length - 1];
				return ZOOM_LEVELS[nearestZoomIndex(z)];
			}
			function stepZoom(dir) {
				var next = nearestZoomIndex(currentZoom) + (dir < 0 ? -1 : 1);
				if (next < 0) next = 0;
				if (next >= ZOOM_LEVELS.length) next = ZOOM_LEVELS.length - 1;
				return ZOOM_LEVELS[next];
			}
			function applyZoom(z, save) {
				currentZoom = snapZoom(z);
				if (typeof window.applyZoomNative === 'function') {
					try { window.applyZoomNative(currentZoom); } catch (e) {}
				} else if (document.body) {
					document.body.style.zoom = currentZoom;
				}
				if (window.syncZoomLabel) window.syncZoomLabel(currentZoom);
				if (save) {
					// Coalesce key-repeat/wheel bursts into one settings write.
					clearTimeout(zoomSaveTimer);
					zoomSaveTimer = setTimeout(function() {
						if (window.setZoomNative) {
							Promise.resolve(window.setZoomNative(currentZoom)).catch(function() {});
						}
					}, 400);
				}
			}
			// Shared with the Settings modal stepper below.
			window.setPageZoom = function(z, save) { applyZoom(z, save !== false); };
			window.stepPageZoom = function(dir, save) { applyZoom(stepZoom(dir), save !== false); };
			window.getPageZoom = function() { return currentZoom; };
			if (window.getZoomNative) {
				window.getZoomNative().then(function(saved) {
					if (typeof saved === 'number' && isFinite(saved)) applyZoom(saved, false);
				}).catch(function() {});
			}
			window.addEventListener('keydown', function(e) {
				if (!(e.metaKey || e.ctrlKey)) return;
				if (e.key === '=' || e.key === '+') {
					e.preventDefault();
					applyZoom(stepZoom(1), true);
				} else if (e.key === '-' || e.key === '_') {
					e.preventDefault();
					applyZoom(stepZoom(-1), true);
				} else if (e.key === '0') {
					e.preventDefault();
					applyZoom(1.0, true);
				}
			});
			// Ctrl/Cmd + scroll-wheel zooms the page like a browser: scroll up
			// = zoom in, scroll down = zoom out. passive:false so the engine's
			// own zoom gesture stays out of the way; capture so WhatsApp's
			// virtualized lists cannot swallow the event first.
			window.addEventListener('wheel', function(e) {
				if (!(e.ctrlKey || e.metaKey)) return;
				if (!e.deltaY) return;
				e.preventDefault();
				e.stopImmediatePropagation();
				applyZoom(stepZoom(e.deltaY < 0 ? 1 : -1), true);
			}, { passive: false, capture: true });
		})();

		// Dock Badge Unread Count Synchronizer
		(function() {
			var lastBadge = null;
			function syncBadge() {
				var title = document.title || '';
				var match = title.match(/\(([^)]+)\)/);
				var badge = match ? match[1] : '';
				if (badge !== lastBadge) {
					lastBadge = badge;
					if (window.updateDockBadge) {
						window.updateDockBadge(badge);
					}
				}
			}
			var titleEl = document.querySelector('title');
			if (titleEl) {
				new MutationObserver(syncBadge).observe(titleEl, { childList: true, characterData: true, subtree: true });
			} else {
				setInterval(syncBadge, 3000);
			}
		})();

		// Memory Optimization: Idle Garbage Collection
		(function() {
			var releaseTimer = null;
			document.addEventListener('visibilitychange', function() {
				clearTimeout(releaseTimer);
				if (!document.hidden) return;
				lastClickedDocName = '';
				lastDocumentIntentAt = 0;
				// A hidden window resets the preview loop guard too, so the
				// document can be re-previewed after coming back.
				lastDocPreviewName = '';
				lastDocPreviewAt = 0;
				// Wait a bit longer than a quick alt-tab before trimming memory, so briefly
				// switching windows doesn't repeatedly trigger native working-set trims.
				releaseTimer = setTimeout(function() {
					if (typeof window.gc === 'function') window.gc();
					if (window.releaseMemoryNative) window.releaseMemoryNative();
				}, 5000);
			});
		})();

		// Debounced window resize persistence
		(function() {
			var resizeTimer = null;
			window.addEventListener('resize', function() {
				clearTimeout(resizeTimer);
				resizeTimer = setTimeout(function() {
					if (window.saveWindowStateNative) {
						var w = window.outerWidth || window.innerWidth;
						var h = window.outerHeight || window.innerHeight;
						if (w && h) {
							window.saveWindowStateNative(Math.round(w), Math.round(h));
						}
					}
				}, 500);
			});
		})();

		// Floating HUD Toast for User Feedback. Optional action renders a
		// clickable button inside the toast (e.g. "Open folder" after a
		// download); the toast then stays interactive for a few seconds longer.
		function showFloatingToast(msg, action) {
			var toast = document.getElementById('wa-hud-toast');
			if (!toast) {
				toast = document.createElement('div');
				toast.id = 'wa-hud-toast';
				toast.style.cssText = 'position:fixed;top:16px;left:50%;transform:translateX(-50%);background:rgba(32,44,51,0.94);backdrop-filter:blur(10px);color:#00a884;border:1px solid rgba(0,168,132,0.4);border-radius:20px;padding:8px 20px;font-size:12.5px;font-weight:600;z-index:9999999;box-shadow:0 8px 24px rgba(0,0,0,0.6);transition:all 0.22s cubic-bezier(0.16,1,0.3,1);opacity:0;display:flex;align-items:center;gap:12px;max-width:90vw;';
				var parent = document.body || document.documentElement;
				if (parent) parent.appendChild(toast);
			}
			if (!toast) return;
			toast.textContent = '';
			toast.style.pointerEvents = 'none';
			var label = document.createElement('span');
			label.textContent = msg;
			label.style.cssText = 'white-space:nowrap;overflow:hidden;text-overflow:ellipsis;';
			toast.appendChild(label);
			if (action && action.label && typeof action.onClick === 'function') {
				toast.style.pointerEvents = 'auto';
				var btn = document.createElement('button');
				btn.textContent = action.label;
				btn.style.cssText = 'background:#00a884;color:#111b21;border:none;padding:3px 10px;border-radius:12px;font-size:11px;font-weight:700;cursor:pointer;flex-shrink:0;';
				btn.onclick = function(e) {
					e.stopPropagation();
					action.onClick();
					toast.style.opacity = '0';
				};
				toast.appendChild(btn);
			}
			toast.style.opacity = '1';
			toast.style.transform = 'translateX(-50%) translateY(4px)';
			clearTimeout(toast._timer);
			toast._timer = setTimeout(function() {
				toast.style.opacity = '0';
				toast.style.transform = 'translateX(-50%) translateY(0)';
				toast.style.pointerEvents = 'none';
			}, action ? 6000 : 2500);
		}

		// Download feedback switch (Settings card, persisted natively as the
		// long-stored notify_on_download flag, which previously had no reader).
		// Completion/progress popups go through showDownloadToast; errors
		// always use showFloatingToast directly so failures are never silent.
		var notifyOnDownload = true;
		var notifyOnDownloadReady = false;
		window.isNotifyOnDownload = function() {
			return notifyOnDownloadReady && notifyOnDownload;
		};
		window.setNotifyOnDownload = function(on) {
			on = !!on;
			if (!window.setNotifyOnDownloadNative) {
				notifyOnDownload = on;
				notifyOnDownloadReady = true;
				return Promise.resolve(notifyOnDownload);
			}
			return Promise.resolve(window.setNotifyOnDownloadNative(on)).then(function(saved) {
				notifyOnDownload = !!saved;
				notifyOnDownloadReady = true;
				return notifyOnDownload;
			});
		};
		window.refreshNotifyOnDownload = function() {
			if (!window.getNotifyOnDownloadNative) {
				notifyOnDownloadReady = true;
				return Promise.resolve(notifyOnDownload);
			}
			return Promise.resolve(window.getNotifyOnDownloadNative()).then(function(saved) {
				notifyOnDownload = !!saved;
				notifyOnDownloadReady = true;
				return notifyOnDownload;
			});
		};
		window.refreshNotifyOnDownload();
		function showDownloadToast(msg, action) {
			if (window.isNotifyOnDownload && !window.isNotifyOnDownload()) return;
			showFloatingToast(msg, action);
		}

		// Issue reporter: page errors are buffered locally (never uploaded),
		// and the Control Center offers a one-click pre-filled GitHub issue.
		// Nothing leaves the machine until the user presses Report — the
		// browser then shows the composed issue for review before submitting.
		(function() {
			window.__waMeta = { ver: '__WA_APP_VERSION__', platform: '` + runtime.GOOS + `' };

			var waErrBuf = [];
			function waPushErr(kind, msg) {
				msg = String(msg || 'unknown error').slice(0, 200);
				var last = waErrBuf[waErrBuf.length - 1];
				if (last && last.m === msg) { last.n++; return; }
				waErrBuf.push({ k: kind, m: msg, n: 1 });
				if (waErrBuf.length > 25) waErrBuf.shift();
			}
			window.addEventListener('error', function(e) {
				var src = '';
				try { src = String(e.filename || '').split('/').pop(); } catch (x) {}
				waPushErr('error', (e.message || 'unknown') + ' @ ' + (src || '?') + ':' + (e.lineno || '?'));
			}, true);
			window.addEventListener('unhandledrejection', function(e) {
				var r = e.reason;
				waPushErr('unhandled', String((r && (r.stack || r.message)) || r).slice(0, 200));
			});

			function resolveMaybe(v) {
				if (v && typeof v.then === 'function') return v;
				return Promise.resolve(v);
			}
			window.openIssueReporter = function(crashTail) {
				var meta = window.__waMeta || { ver: '?', platform: '?' };
				var lines = ['WAtchful v' + meta.ver + ' (' + meta.platform + ')', ''];
				if (waErrBuf.length) {
					lines.push('Recent page errors:');
					waErrBuf.slice(-8).forEach(function(e) {
						lines.push('- [' + e.k + '] ' + e.m + (e.n > 1 ? ' (x' + e.n + ')' : ''));
					});
					lines.push('');
				} else {
					lines.push('No page errors captured.');
					lines.push('');
				}
				if (crashTail) {
					var fence = String.fromCharCode(96, 96, 96);
					lines.push('Crash log tail:');
					lines.push(fence);
					lines.push(String(crashTail).slice(0, 1200));
					lines.push(fence);
				}
				lines.push('_Submitted from the in-app reporter — please add steps to reproduce._');
				var url = 'https://github.com/latiefahmad/WAtchful/issues/new' +
					'?title=' + encodeURIComponent('Report v' + meta.ver + ' (' + meta.platform + '): ') +
					'&body=' + encodeURIComponent(lines.join('\n').slice(0, 2500)) +
					'&labels=' + encodeURIComponent('bug');
				if (window.openExternalLink) window.openExternalLink(url);
				if (window.markCrashNotifiedNative) {
					try { resolveMaybe(window.markCrashNotifiedNative()); } catch (e) {}
				}
			};
			window.reportIssueNow = function() {
				if (window.getPendingCrashNative) {
					try {
						resolveMaybe(window.getPendingCrashNative()).then(function(t) {
							window.openIssueReporter(t || '');
						});
						return;
					} catch (e) {}
				}
				window.openIssueReporter('');
			};

			// Startup nudge, once per crash: offer reporting instead of nagging.
			setTimeout(function() {
				if (!window.getPendingCrashNative || typeof showFloatingToast !== 'function') return;
				try {
					resolveMaybe(window.getPendingCrashNative()).then(function(tail) {
						if (!tail) return;
						showFloatingToast('⚠️ Previous session crashed — tap to report', {
							label: 'Report',
							onClick: function() { window.reportIssueNow(); }
						});
					});
				} catch (e) {}
			}, 10000);
		})();

		// Privacy Mode Toggle (Cmd + Shift + P)
		(function() {
			var isPrivacyActive = false;
			var styleEl = document.createElement('style');
			styleEl.id = 'whatsapp-privacy-style';
			// PRIVACY STRATEGY (perf-critical, see Fedora report): text is hidden
			// with solid redaction blocks, NOT filter:blur(). Filters
			// force a compositing layer per element (1.8 GB spikes on Wayland)
			// and a blurred PARENT can never be un-blurred by a hovered child,
			// which rules out container blur entirely. Solid redaction hides only
			// the glyphs — layout, timestamps and the reply box stay
			// intact — and :hover restores the inherited color with a single
			// static switch (never transitioned/animated).
			// Primary target is WhatsApp's long-stable span.selectable-text
			// (message bodies, chat names, previews); structural fallbacks
			// cover rows whose spans lack that class. Timestamps/meta spans
			// don't carry selectable-text, so they stay readable by design.
			styleEl.textContent = [
				// The Archived navigation control is UI guidance, not private
				// chat data: it stays readable (and keeps its readable clock)
				// while the archived rows themselves are still redacted.
				// Tagged by the tagger below; the exempt selectors win over
				// the row rules because they come later in the sheet.
				'.privacy-mode [data-wa-archived-nav] span,',
				'.privacy-mode [data-wa-archived-nav] a',
				'{ color: inherit !important; text-shadow: none !important; background: transparent !important; }',
				// Layer 1: names + previews in the chat list, hover row to peek.
				// Spans tagged data-wa-time by the timestamp tagger below are
				// always spared, so clock times stay readable.
				'.privacy-mode #pane-side [role="row"] span.selectable-text:not([data-wa-time]),',
				'.privacy-mode [data-testid="chat-list"] [role="row"] span.selectable-text:not([data-wa-time]),',
				'.privacy-mode #pane-side [role="row"] span[title]:not([data-wa-time]),',
				'.privacy-mode [data-testid="chat-list"] [role="row"] span[title]:not([data-wa-time]),',
				// Links carry their own explicit color, so a URL text node sitting
				// directly inside <a> never inherits the redacted span color —
				// redact anchors too (chat-list previews can contain URLs).
				'.privacy-mode #pane-side [role="row"] a:not([data-wa-time]),',
				'.privacy-mode [data-testid="chat-list"] [role="row"] a:not([data-wa-time])',
				'{ color: transparent !important; text-shadow: none !important; background: rgba(134,150,160,.42) !important; border-radius: 3px; }',
				// Hovering a row restores every span beneath it, so restore can
				// never disagree with blur even if WhatsApp rotates classes.
				'.privacy-mode #pane-side [role="row"]:hover span,',
				'.privacy-mode [data-testid="chat-list"] [role="row"]:hover span,',
				'.privacy-mode #pane-side [role="row"]:hover a,',
				'.privacy-mode [data-testid="chat-list"] [role="row"]:hover a',
				'{ color: inherit !important; text-shadow: none !important; background: transparent !important; }',
				// Layer 2: everything textual inside a message bubble, keyed ONLY
				// on the long-stable [data-testid="msg-container"] hook — never
				// on hashed cosmetic classes (those rotate; .message-in and
				// span.selectable-text no longer exist, which is exactly why
				// hover-to-peek silently died). Hovering the bubble restores
				// the whole subtree, so blur and restore can never disagree.
				// The reply box lives outside msg-container and stays usable.
				// Anchors are covered too: link/URL text sits directly inside
				// <a> with its own explicit color, so it never inherits the
				// redacted span color (this was the un-blurred-URL leak).
				'.privacy-mode #main [data-testid="msg-container"] span:not([data-wa-time]),',
				'.privacy-mode #main [data-testid="msg-container"] a:not([data-wa-time])',
				'{ color: transparent !important; text-shadow: none !important; background: rgba(134,150,160,.42) !important; border-radius: 3px; }',
				'.privacy-mode #main [data-testid="msg-container"]:hover span,',
				'.privacy-mode #main [data-testid="msg-container"]:hover a',
				'{ color: inherit !important; text-shadow: none !important; background: transparent !important; }',
				// In-chat photos/videos hide the same way (filter is the only
				// tool for replaced elements); hover restores symmetrically.
				'.privacy-mode #main [data-testid="msg-container"] img,',
				'.privacy-mode #main [data-testid="msg-container"] video',
				'{ filter: blur(12px) !important; }',
				'.privacy-mode #main [data-testid="msg-container"]:hover img,',
				'.privacy-mode #main [data-testid="msg-container"]:hover video',
				'{ filter: none !important; }',
				// Layer 3: conversation header name redacted as a solid bar (like
				// a marker pen), hover the header to reveal. Timestamps spared.
				'.privacy-mode #main header span:not([data-wa-time])',
				'{ color: transparent !important; text-shadow: none !important; background: #000 !important; border-radius: 4px; }',
				'.privacy-mode #main header:hover span:not([data-wa-time])',
				'{ color: inherit !important; text-shadow: none !important; background: transparent !important; }',
				// Layer 4: fullscreen media viewer stays fully hidden while
				// privacy is on (one layer, no hover needed there).
				'.privacy-mode [data-testid="media-viewer"]',
				'{ filter: blur(12px) !important; }',
				// Layer 5: profile photos, only when the "blur avatars" setting
				// is on (html.blur-avatars). Hovering the row/message reveals.
				'.privacy-mode.blur-avatars #pane-side [role="row"] img,',
				'.privacy-mode.blur-avatars #side header img,',
				'.privacy-mode.blur-avatars #main header img',
				'{ filter: blur(8px) !important; }',
				'.privacy-mode.blur-avatars #pane-side [role="row"] img:hover,',
				'.privacy-mode.blur-avatars #pane-side [role="row"]:hover img,',
				'.privacy-mode.blur-avatars #side header img:hover,',
				'.privacy-mode.blur-avatars #main header img:hover,',
				'.privacy-mode.blur-avatars #main .message-in:hover img,',
				'.privacy-mode.blur-avatars #main .message-out:hover img',
				'{ filter: none !important; }',
				// Drag & drop visual feedback
				'.wa-drag-over { outline: 3px solid #00a884; outline-offset: -3px; }',
				'.wa-drag-over * { pointer-events: none; }'
			].join('\n');

			function applyPrivacyMode(active, silent) {
				isPrivacyActive = !!active;
				// State lives on <html>, never on WhatsApp's mutable <body>.
				// All privacy selectors are descendant selectors, so they
				// match identically from the <html> ancestor.
				var rootEl = document.documentElement;
				if (isPrivacyActive) {
					if (!document.getElementById('whatsapp-privacy-style')) {
						document.head.appendChild(styleEl);
					}
					rootEl.classList.add('privacy-mode');
					if (!silent) showFloatingToast('🔒 Privacy Mode: Enabled');
				} else {
					rootEl.classList.remove('privacy-mode');
					if (!silent) showFloatingToast('🔓 Privacy Mode: Disabled');
				}
				return isPrivacyActive;
			}

			window.togglePrivacyMode = function() {
				// A manual toggle also cancels any pending auto-lock timer.
				return applyPrivacyMode(!isPrivacyActive, false);
			};
			window.isPrivacyModeActive = function() {
				return isPrivacyActive;
			};

			// "Blur profile photos" setting: gates the .blur-avatars layer.
			// Applied on <html> next to .privacy-mode; persisted natively.
			window.isBlurAvatars = function() {
				return document.documentElement.classList.contains('blur-avatars');
			};

			// Timestamp sparing: tag short clock/day strings so the CSS above
			// can exclude them via :not([data-wa-time]). textContent never
			// forces layout; each span is visited once (__waTimeSeen); the
			// :not() selector keeps repeat runs cheap. Ticks at most every 3s,
			// only while privacy is on and the page is visible, so the steady
			// state cost is ~zero. Attribute writes don't trip the childList
			// observers, so this can't feed an observer loop.
			var WA_TIME_RE = /^(\d{1,2}:\d{2}(\s?(AM|PM))?|Today|Yesterday|Monday|Tuesday|Wednesday|Thursday|Friday|Saturday|Sunday|Hari ini|Kemarin|Senin|Selasa|Rabu|Kamis|Jumat|Sabtu|Minggu)$/i;
			// Archived navigation labels ("Archived", "Diarsipkan", ...) in any
			// supported locale. Matched against whitespace-collapsed text.
			var WA_ARCHIVED_RE = /^(Archived|Diarsipkan|Archiviert|Archivio|Archiviati|Archivados?|Archivada|Archive)$/i;
			// Archived-view explanation copy ("These chats stay archived...")
			// is UI guidance, never private content.
			var WA_ARCHIVED_INFO_RE = /these chats stay archived|obrolan ini tetap diarsipkan/i;
			function rowTextIsArchivedNav(text) {
				return WA_ARCHIVED_RE.test(String(text || '').replace(/\s+/g, ' ').trim());
			}
			function tagArchivedNavIn(root) {
				if (!root || !root.querySelectorAll) return;
				var spans = root.querySelectorAll('span, [role="button"]');
				for (var i = 0; i < spans.length; i++) {
					var s = spans[i];
					if (s.__waArchSeen) continue;
					s.__waArchSeen = true;
					try {
						var t = (s.textContent || '').trim();
						if (!rowTextIsArchivedNav(t)) continue;
						// Only tag the navigation control itself, never a chat
						// row that merely contains the word: the row holds far
						// more text (name + preview + time), the control does not.
						if (t.length > 24) continue;
						var node = s;
						for (var depth = 0; node && node !== root && depth < 6; depth++, node = node.parentElement) {
							var nt = (node.textContent || '').trim();
							if (nt.length > 24) break;
							node.setAttribute('data-wa-archived-nav', '1');
						}
					} catch (e) {}
				}
			}
			function tagTimesIn(root) {
				if (!root || !root.querySelectorAll) return;
				var spans = root.querySelectorAll('span:not([data-wa-time])');
				var n = 0;
				for (var i = 0; i < spans.length && n < 250; i++) {
					var s = spans[i];
					if (s.__waTimeSeen) continue;
					s.__waTimeSeen = true;
					n++;
					try {
						var t = (s.textContent || '').trim();
						if (WA_TIME_RE.test(t)) s.setAttribute('data-wa-time', '1');
					} catch (e) {}
				}
			}
			// Archive explanation copy is spared the same way timestamps are.
			function tagArchiveInfoIn(root) {
				if (!root || !root.querySelectorAll) return;
				var spans = root.querySelectorAll('span, p, div');
				var n = 0;
				for (var i = 0; i < spans.length && n < 60; i++) {
					var s = spans[i];
					if (s.__waArchSeen) continue;
					s.__waArchSeen = true;
					n++;
					try {
						var t = (s.textContent || '').trim();
						if (t.length > 12 && t.length < 220 && WA_ARCHIVED_INFO_RE.test(t)) {
							s.setAttribute('data-wa-time', '1');
						}
					} catch (e) {}
				}
			}
			setInterval(function() {
				if (!isPrivacyActive || shouldPauseBackgroundWork()) return;
				tagTimesIn(document.getElementById('main'));
				tagTimesIn(document.getElementById('pane-side'));
				tagArchivedNavIn(document.getElementById('pane-side'));
				tagArchiveInfoIn(document.getElementById('pane-side'));
			}, 3000);
			window.setBlurAvatars = function(on) {
				on = !!on;
				if (on) document.documentElement.classList.add('blur-avatars');
				else document.documentElement.classList.remove('blur-avatars');
				if (window.setBlurAvatarsNative) {
					Promise.resolve(window.setBlurAvatarsNative(on)).catch(function() {});
				}
				return on;
			};
			if (window.getBlurAvatarsNative) {
				window.getBlurAvatarsNative().then(function(on) {
					if (on) document.documentElement.classList.add('blur-avatars');
				}).catch(function() {});
			}

			// Auto-lock on idle: the Control Center copy promises "blur chats and
			// media when cursor is idle", so honor it. When enabled, the app
			// blurs after a period of no mouse/keyboard activity, or immediately
			// when the window loses focus, and unblurs on the next interaction.
			// Persisted in localStorage so it survives reloads.
			var AUTO_LOCK_KEY = 'wa_desk_privacy_autolock';
			var autoLockEnabled = localStorage.getItem(AUTO_LOCK_KEY) === '1';
			var IDLE_MS = 60000;
			var idleTimer = null;
			var autoLocked = false;

			function isAutoLockEnabled() { return autoLockEnabled; }
			function setAutoLockEnabled(on) {
				autoLockEnabled = !!on;
				localStorage.setItem(AUTO_LOCK_KEY, on ? '1' : '0');
				if (!on && autoLocked) { autoLocked = false; applyPrivacyMode(false, true); }
				if (on) resetIdleTimer();
				return autoLockEnabled;
			}
			window.isPrivacyAutoLock = isAutoLockEnabled;
			window.setPrivacyAutoLock = setAutoLockEnabled;

			function lockForIdle() {
				if (!autoLockEnabled || autoLocked) return;
				autoLocked = true;
				applyPrivacyMode(true, true);
			}
			function unlockFromIdle() {
				if (!autoLocked) return;
				autoLocked = false;
				applyPrivacyMode(false, true);
			}
			function resetIdleTimer() {
				clearTimeout(idleTimer);
				if (!autoLockEnabled) return;
				// If an idle-lock is active, any activity lifts it immediately.
				unlockFromIdle();
				idleTimer = setTimeout(lockForIdle, IDLE_MS);
			}

			var activityEvents = ['mousemove', 'mousedown', 'keydown', 'scroll', 'touchstart', 'wheel'];
			activityEvents.forEach(function(ev) {
				window.addEventListener(ev, resetIdleTimer, { passive: true, capture: true });
			});
			// Losing window focus is the strongest "stepping away" signal.
			window.addEventListener('blur', function() { if (autoLockEnabled) lockForIdle(); });
			window.addEventListener('focus', function() { resetIdleTimer(); });
			document.addEventListener('visibilitychange', function() {
				if (document.hidden) { if (autoLockEnabled) lockForIdle(); }
				else resetIdleTimer();
			});
			resetIdleTimer();

			window.addEventListener('keydown', function(e) {
				if ((e.metaKey || e.ctrlKey) && e.shiftKey && (e.key === 'p' || e.key === 'P')) {
					e.preventDefault();
					window.togglePrivacyMode();
				}
			});
		})();

		// Always on Top Toggle (Cmd/Ctrl + Shift + T)
		(function() {
			var isPinnedState = false;
			window.toggleAlwaysOnTop = function() {
				if (window.toggleAlwaysOnTopNative) {
					return window.toggleAlwaysOnTopNative().then(function(isPinned) {
						isPinnedState = isPinned;
						showFloatingToast(isPinned ? '📌 Always on Top: Enabled' : '📌 Always on Top: Disabled');
						return isPinned;
					});
				}
				return Promise.resolve(false);
			};
			window.isAlwaysOnTopActive = function() {
				return isPinnedState;
			};

			window.addEventListener('keydown', function(e) {
				if ((e.metaKey || e.ctrlKey) && e.shiftKey && (e.key === 't' || e.key === 'T')) {
					e.preventDefault();
					window.toggleAlwaysOnTop();
				}
			});
		})();

		// Reload and Refresh Functions (Cmd/Ctrl + R, Cmd/Ctrl + Shift + R, F5)
		window.reloadWhatsApp = function() {
			showFloatingToast('🔄 Reloading conversation...');
			setTimeout(function() { window.location.reload(); }, 200);
		};
		window.hardRefreshWhatsApp = function() {
			showFloatingToast('⚡ Hard refresh (clearing cache)...');
			try {
				if (window.caches && caches.keys) {
					caches.keys().then(function(names) {
						names.forEach(function(name) { caches.delete(name); });
					});
				}
			} catch (e) {}
			setTimeout(function() {
				window.location.href = window.location.origin + window.location.pathname + '?_t=' + Date.now();
			}, 200);
		};

		window.addEventListener('keydown', function(e) {
			if (e.key === 'F5' || ((e.metaKey || e.ctrlKey) && (e.key === 'r' || e.key === 'R') && !e.shiftKey && !e.altKey)) {
				e.preventDefault();
				window.reloadWhatsApp();
			} else if ((e.metaKey || e.ctrlKey) && e.shiftKey && (e.key === 'r' || e.key === 'R')) {
				e.preventDefault();
				window.hardRefreshWhatsApp();
			}
		});

		// Audio Mute Toggle (Cmd/Ctrl + Shift + M)
		(function() {
			var isMuted = false;
			window.toggleMuteAudio = function() {
				isMuted = !isMuted;
				document.querySelectorAll('audio, video').forEach(function(el) {
					el.muted = isMuted;
				});
				showFloatingToast(isMuted ? '🔇 Notification Audio: Muted' : '🔊 Notification Audio: Unmuted');
				return isMuted;
			};
			window.isAudioMuted = function() {
				return isMuted;
			};

			window.addEventListener('keydown', function(e) {
				if ((e.metaKey || e.ctrlKey) && e.shiftKey && (e.key === 'm' || e.key === 'M')) {
					e.preventDefault();
					window.toggleMuteAudio();
				}
			});
			document.addEventListener('play', function(e) {
				if (isMuted && e.target && (e.target.tagName === 'AUDIO' || e.target.tagName === 'VIDEO')) {
					e.target.muted = true;
				}
			}, true);
		})();

		// Auto-Start at Login Toggle (Cmd/Ctrl + Shift + S)
		(function() {
			var isAutoStartState = false;
			window.toggleAutoStart = function() {
				if (window.toggleAutoStartNative) {
					return window.toggleAutoStartNative().then(function(isEnabled) {
						isAutoStartState = isEnabled;
						showFloatingToast(isEnabled ? '🚀 Launch on Boot: Enabled' : '🚀 Launch on Boot: Disabled');
						return isEnabled;
					});
				}
				return Promise.resolve(false);
			};
			window.isAutoStartActive = function() {
				return isAutoStartState;
			};

			window.addEventListener('keydown', function(e) {
				if ((e.metaKey || e.ctrlKey) && e.shiftKey && (e.key === 's' || e.key === 'S')) {
					e.preventDefault();
					window.toggleAutoStart();
				}
			});
		})();

		// Quick Replies: type "/trigger" + Space in the chat composer to
		// expand a saved template. Stored in localStorage (per profile, since
		// each profile owns its WebView data dir). Variables: {date},
		// {time}, {name} (active chat title).
		(function() {
			var QR_KEY = 'wa_desk_quick_replies';
			var QR_MAX = 50;

			function normalizeQrTrigger(raw) {
				var t = String(raw || '').trim().toLowerCase().replace(/^\//, '');
				if (!/^[a-z0-9_-]{1,32}$/.test(t)) return '';
				return t;
			}
			function loadQuickReplies() {
				try {
					var list = JSON.parse(localStorage.getItem(QR_KEY));
					if (!Array.isArray(list)) return [];
					var out = [];
					for (var i = 0; i < list.length && out.length < QR_MAX; i++) {
						var item = list[i] || {};
						var trigger = normalizeQrTrigger(item.trigger);
						var text = String(item.text || '').slice(0, 2000);
						if (trigger && text) out.push({ trigger: trigger, text: text });
					}
					return out;
				} catch (e) {
					return [];
				}
			}
			function saveQuickReplies(list) {
				try {
					localStorage.setItem(QR_KEY, JSON.stringify(list.slice(0, QR_MAX)));
				} catch (e) {}
			}
			window.getQuickReplies = loadQuickReplies;
			window.addQuickReply = function(trigger, text) {
				trigger = normalizeQrTrigger(trigger);
				text = String(text || '').trim().slice(0, 2000);
				if (!trigger || !text) return null;
				var list = loadQuickReplies().filter(function(q) { return q.trigger !== trigger; });
				list.push({ trigger: trigger, text: text });
				saveQuickReplies(list);
				return trigger;
			};
			window.deleteQuickReply = function(trigger) {
				saveQuickReplies(loadQuickReplies().filter(function(q) { return q.trigger !== trigger; }));
			};

			function activeChatName() {
				try {
					var titled = document.querySelector('#main header [title]');
					if (titled) {
						var name = (titled.getAttribute('title') || '').trim();
						if (name) return name.slice(0, 60);
					}
					var header = document.querySelector('#main header');
					if (header) {
						var first = ((header.innerText || '').split('\n')[0] || '').trim();
						if (first) return first.slice(0, 60);
					}
				} catch (e) {}
				return '';
			}
			function expandQrVariables(text) {
				var now = new Date();
				var dateStr = now.toLocaleDateString();
				var timeStr = now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
				return String(text || '')
					.split('{date}').join(dateStr)
					.split('{time}').join(timeStr)
					.split('{name}').join(activeChatName());
			}

			function findComposer() {
				return document.querySelector('div[contenteditable="true"][role="textbox"]') ||
					document.querySelector('div[contenteditable="true"]') ||
					document.querySelector('textarea');
			}
			function caretAtEnd(el, range) {
				try {
					var end = document.createRange();
					end.selectNodeContents(el);
					end.collapse(false);
					if (range.compareBoundaryPoints(Range.END_TO_END, end) === 0) return true;
					// Tolerant: editors often keep trailing break nodes (or
					// empty inline placeholders) after the caret. Only
					// whitespace text and content-free elements may sit
					// between the caret and the true end.
					var tail = range.cloneRange();
					tail.setEnd(end.endContainer, end.endOffset);
					if (!/^[\s\u200b\ufeff]*$/.test(tail.toString())) return false;
					var probe = tail.cloneContents();
					var els = probe.querySelectorAll ? probe.querySelectorAll('*') : [];
					for (var i = 0; i < els.length; i++) {
						var tag = els[i].tagName || '';
						if (/^BR$/i.test(tag)) continue;
						if (((els[i].textContent || '').trim()) === '') continue;
						return false;
					}
					return true;
				} catch (e) {
					return false;
				}
			}
			// Resolve the token before the caret even when the caret sits
			// between nodes (nodeType 1): use the previous text leaf.
			function tokenBeforeCaret(range) {
				var node = range.startContainer;
				var soff = range.startOffset;
				if (node && node.nodeType !== 3) {
					var prev = (node.childNodes && soff > 0) ? node.childNodes[soff - 1] : null;
					while (prev && prev.nodeType !== 3 && prev.lastChild) prev = prev.lastChild;
					if (prev && prev.nodeType === 3) {
						node = prev;
						soff = (prev.textContent || '').length;
					} else {
						return null;
					}
				}
				if (!node) return null;
				var before = (node.textContent || '').slice(0, soff);
				var m = before.match(/\/([a-z0-9_-]{1,32})$/i);
				if (!m) return null;
				return { node: node, offset: soff, text: m[0], trigger: m[1] };
			}
			// Space expands a trailing /trigger; Enter expands only a known
			// trigger (otherwise the message must send as usual). Never fires
			// inside our own overlays (Settings inputs, Direct Chat modal).
			document.addEventListener('keydown', function(e) {
				if (e.key !== ' ' && e.key !== 'Enter') return;
				if (e.metaKey || e.ctrlKey || e.altKey) return;
				var el = document.activeElement;
				if (!el) return;
				if (el.closest && el.closest('#wa-settings-overlay, #wa-directchat-overlay, #wa-doc-modal-overlay')) return;
				var composer = findComposer();
				if (!composer || (el !== composer && !composer.contains(el))) return;
				var sel = window.getSelection();
				if (!sel || !sel.isCollapsed || !sel.rangeCount) return;
				var range = sel.getRangeAt(0);
				if (!caretAtEnd(composer, range)) return;
				var tok = null;
				try {
					tok = tokenBeforeCaret(range);
				} catch (err) {
					return;
				}
				if (!tok) return;
				var trigger = normalizeQrTrigger(tok.trigger);
				var found = null;
				var list = loadQuickReplies();
				for (var i = 0; i < list.length; i++) {
					if (list[i].trigger === trigger) { found = list[i]; break; }
				}
				if (!found) {
					// Diagnostic + UX: Space on an unknown /word tells the
					// user instead of staying silent; Enter stays quiet so a
					// normal message always sends.
					if (e.key === ' ') {
						showFloatingToast('⚡ No quick reply /' + trigger + ' — manage them in Settings');
					}
					return;
				}
				var expanded = expandQrVariables(found.text) + (e.key === ' ' ? ' ' : '');
				if (!expanded.trim()) {
					// Never destroy typed text when there is nothing to put
					// in its place (empty template): leave the key alone and
					// say why, for both Space and Enter.
					showFloatingToast('⚡ Quick reply /' + trigger + ' is empty — edit it in Settings');
					return;
				}
				e.preventDefault();
				e.stopPropagation();
				try {
					var tokenLen = tok.text.length;
					// Select exactly the "/trigger" token, then ask the
					// editor to replace the live selection like real typing:
					// frameworks (Lexical) honor synthetic beforeinput and
					// apply it through their own model, while raw DOM surgery
					// risks being reverted on reconcile.
					var tokenRange = range.cloneRange();
					tokenRange.setStart(tok.node, Math.max(0, tok.offset - tokenLen));
					sel.removeAllRanges();
					sel.addRange(tokenRange);
					var handled = false;
					try {
						var bie = new InputEvent('beforeinput', { bubbles: true, cancelable: true, inputType: 'insertText', data: expanded });
						handled = !composer.dispatchEvent(bie);
					} catch (bieErr) {}
					if (!handled) {
						// No editor took it (plain contenteditable/textarea):
						// do the replacement ourselves.
						if (composer.tagName === 'TEXTAREA') {
							var tv = composer.value || '';
							var te = (typeof composer.selectionEnd === 'number') ? composer.selectionEnd : tv.length;
							var nv = tv.slice(0, Math.max(0, te - tokenLen)) + expanded + tv.slice(te);
							composer.value = nv;
							var nc = Math.max(0, te - tokenLen) + expanded.length;
							try { composer.selectionStart = composer.selectionEnd = nc; } catch (se) {}
							composer.dispatchEvent(new Event('input', { bubbles: true }));
						} else {
							tokenRange.deleteContents();
							var textNode = document.createTextNode(expanded);
							tokenRange.insertNode(textNode);
							var after = document.createRange();
							after.setStartAfter(textNode);
							after.collapse(true);
							sel.removeAllRanges();
							sel.addRange(after);
							try {
								composer.dispatchEvent(new InputEvent('input', { bubbles: true, cancelable: false, data: expanded, inputType: 'insertText' }));
							} catch (evtErr) {}
						}
					}
					// Self-verify instead of swallowing the key silently. Deferred:
					// editor frameworks may commit the replacement async after
					// the event dispatch returns, so a synchronous check would
					// cry wolf while the text is still on its way in. Compare
					// whitespace/zero-width normalized: editors re-encode
					// spacing (NBSP, trailing trims) without changing looks.
					var verifyCore = expanded.replace(/ $/, '');
					var verifyToken = tok.text;
					setTimeout(function() {
						try {
							var live = (composer.tagName === 'TEXTAREA') ? (composer.value || '') : (composer.textContent || '');
							var norm = function(s) {
								return String(s || '').replace(/[\u200b-\u200d\ufeff]/g, '').replace(/\s+/g, ' ').trim();
							};
							var gone = norm(live).indexOf(norm(verifyToken)) === -1;
							var present = norm(live).indexOf(norm(verifyCore)) !== -1;
							if (!gone || !present) {
								showFloatingToast('⚠️ Gagal menyisipkan template — coba lagi');
							}
						} catch (ve) {}
					}, 400);
				} catch (err) {
					try { document.execCommand('insertText', false, (e.key === ' ' ? ' ' : '')); } catch (e2) {}
				}
			}, true);
		})();

		// Scheduler: one-off / daily / weekly / monthly messages to phone
		// numbers. Per-profile localStorage store (survives reloads). At fire
		// time the tab persists a pending-send payload and navigates to the
		// official /send?phone= deep link; the boot handler below completes
		// it (wait composer -> insert -> send -> verify). Requires the app
		// running with this profile's tab alive: tabs holding enabled
		// schedules report busy('scheduled') so the shell neither suspends
		// nor hibernates them (stated in Settings).
		(function() {
			// Per-evaluation id: every log line carries [boot/attempt] so a
			// runaway's screenshot alone tells same-routine-looping apart
			// from many-routines-firing.
			var bootId = (function() {
				try {
					var a = new Uint32Array(1);
					(window.crypto || window.msCrypto).getRandomValues(a);
					return (a[0] % 1296).toString(36) + ((a[0] >> 6) % 1296).toString(36);
				} catch (e) {
					return Math.floor(Math.random() * 46656).toString(36);
				}
			})();
			var SCHED_KEY = 'wa_desk_schedules';
			var SCHED_LOG = 'wa_desk_schedule_log';
			var PENDING_KEY = 'wa_desk_pending_send';
			// Last successfully sent payload. A boot that finds a pending
			// payload identical to the last completed send drops it silently:
			// whatever replays boots (reload loops, renderer recovery) can
			// never turn one schedule into duplicate sends.
			var LAST_SENT_KEY = 'wa_desk_last_sent';
			var SCHED_MAX = 100;
			var LOG_MAX = 50;
			var PENDING_TTL = 30 * 60 * 1000;
			var TICK_MS = 20000;
			var SEND_TIMEOUT = 20000;
			// Burst breaker: no legitimate use completes many sends in
			// minutes; a runaway (reload loops, wedged state) must stop
			// cold instead of spamming the recipient. Sliding window,
			// persisted so it survives the very reloads it guards against;
			// it self-heals as the window slides, so paused schedules resume
			// on their own instead of being lost.
			var BURST_KEY = 'wa_desk_send_times';
			var BURST_CAP = 10;
			var BURST_WINDOW = 10 * 60 * 1000;
			var breakerToasted = false;
			// Per-payload single processing per page lifetime. Whatever
			// invokes the send routine twice for one payload in one page
			// (re-entrant timers, double boot handling), the second pass
			// finds its key already here and stands down. Legitimate
			// retries always carry a fresh payload (new queuedAt), so they
			// are never blocked by this.
			var seenPayloads = {};
			// Emergency kill switch (Settings checkbox): when set, the tick
			// never fires and boots never process pendings. Payloads stay
			// stored, so re-enabling resumes normally.
			var KILLED_KEY = 'wa_desk_sched_killed';
			function schedKilled() {
				try {
					return localStorage.getItem(KILLED_KEY) === '1';
				} catch (e) {
					return false;
				}
			}
			window.setSchedulerKilled = function(on) {
				try {
					localStorage.setItem(KILLED_KEY, on ? '1' : '0');
				} catch (e) {}
				// Killing withdraws any queued payload too, so "emergency
				// stop" is visible immediately (log line + gone pending),
				// not only honored at the next boot. Row data is kept; the
				// schedule itself can fire again once un-killed.
				if (on) {
					try {
						var pend = JSON.parse(localStorage.getItem(PENDING_KEY));
						if (pend && pend.phone) {
							localStorage.removeItem(PENDING_KEY);
							pushSchedLog(pend.phone, pend.text || '', 'cancelled (emergency stop)');
						}
					} catch (e) {}
				}
				recountScheduledBusy();
				return !schedKilled();
			};
			// True while a scheduled send attempt owns the composer; the
			// AFK listener uses this to not mistake our own click for the
			// user sending a message (which would turn AFK off silently).
			window.__schedInFlight = function() { return sending; };
			// Send choke-point: scheduler AND AFK route every intended
			// composer send through window.__waGuardSend before typing or
			// clicking. Gates, in order: the owner's kill switch (no
			// violation counted), a minimum gap between sends persisted in
			// localStorage (survives reloads — sub-second repetition is
			// never legitimate), and a per-page budget. A first pace
			// violation just blocks that send; a second one, or an
			// exhausted budget, trips the EMERGENCY: both features killed
			// with a loud log. Whatever path sends — known or future — it
			// is bounded to a handful of attempts instead of hundreds.
			var SEND_GAP_MS = 1000;
			var SEND_BUDGET = 40;
			var LAST_CLICK_KEY = 'wa_last_send_click';
			var BUDGET_KEY = 'wa_send_budget';
			var VIOL_KEY = 'wa_send_violations';
			function emergencySendTrip(why) {
				try { localStorage.setItem(KILLED_KEY, '1'); } catch (e) {}
				try { localStorage.setItem('wa_desk_afk_killed', '1'); } catch (e) {}
				pushSchedLog('', '', 'emergency stop: send ' + why + ' tripped [' + bootId + ']');
				showFloatingToast('🛑 Send guard tripped (' + why + ') — scheduler and AFK halted');
			}
			window.__waGuardSend = function(owner) {
				try {
					var killKey = owner === 'afk' ? 'wa_desk_afk_killed' : KILLED_KEY;
					if (localStorage.getItem(killKey) === '1') return { ok: false, why: 'kill' };
				} catch (e) {
					return { ok: false, why: 'kill' };
				}
				var now = Date.now();
				var last = 0;
				try { last = Number(localStorage.getItem(LAST_CLICK_KEY)) || 0; } catch (e) {}
				if (last && now - last < SEND_GAP_MS) {
					var viol = 0;
					try { viol = parseInt(sessionStorage.getItem(VIOL_KEY), 10) || 0; } catch (e) {}
					viol++;
					try { sessionStorage.setItem(VIOL_KEY, String(viol)); } catch (e) {}
					if (viol >= 2) {
						emergencySendTrip('pace');
						return { ok: false, why: 'emergency' };
					}
					return { ok: false, why: 'pace' };
				}
				var budget = SEND_BUDGET;
				try {
					var b = sessionStorage.getItem(BUDGET_KEY);
					if (b !== null) {
						budget = parseInt(b, 10) || 0;
					} else {
						sessionStorage.setItem(BUDGET_KEY, String(SEND_BUDGET));
					}
				} catch (e) {}
				if (budget <= 0) {
					emergencySendTrip('cap');
					return { ok: false, why: 'emergency' };
				}
				try {
					sessionStorage.setItem(BUDGET_KEY, String(budget - 1));
					localStorage.setItem(LAST_CLICK_KEY, String(now));
				} catch (e) {}
				return { ok: true };
			};
			// ── Composer send pipeline (shared by scheduler and AFK) ───
			// Insert ladder: native execCommand first (real editing
			// pipeline, framework editors update their model), then
			// synthetic beforeinput (Lexical intercepts it), then direct
			// replace + an INPUT event. The input event is the crucial
			// last rung: without it the app still believes the composer
			// is empty — MIC never swaps to SEND, Enter no-ops, and the
			// text sits there as an unsent draft.
			window.__waComposerText = function(el) {
				try {
					return ((el.tagName === 'TEXTAREA' ? el.value : el.textContent) || '').trim();
				} catch (e) { return ''; }
			};
			// Returns which rung landed the text: 'exec' | 'beforeinput'
			// | 'dom' — '' means nothing took it. The caller logs the path
			// so a field report shows exactly where delivery breaks.
			window.__waInsertText = function(composer, text) {
				text = String(text || '');
				if (!composer || !text) return '';
				try { composer.focus(); } catch (e) {}
				function selectAll() {
					try {
						var sel = window.getSelection();
						var r = document.createRange();
						r.selectNodeContents(composer);
						sel.removeAllRanges();
						sel.addRange(r);
					} catch (e) {}
				}
				try {
					selectAll();
					if (document.execCommand && document.execCommand('insertText', false, text) &&
						window.__waComposerText(composer)) {
						return 'exec';
					}
				} catch (e) {}
				// PASTE: WhatsApp's paste handler writes through the editor
				// MODEL (state stays synced → MIC becomes SEND). Direct
				// typed-looking edits are trust-gated: the field log showed
				// 'insert path=none len=0' because WhatsApp reverts them
				// synchronously; paste does not go through that gate.
				try {
					selectAll();
					var dt = null;
					try {
						if (typeof DataTransfer !== 'undefined') {
							dt = new DataTransfer();
							dt.setData('text/plain', text);
						}
					} catch (e) { dt = null; }
					if (dt) {
						try {
							composer.dispatchEvent(new ClipboardEvent('paste', { bubbles: true, cancelable: true, clipboardData: dt }));
						} catch (e) {}
						if (window.__waComposerText(composer)) return 'paste';
					}
					try {
						var bieP = new InputEvent('beforeinput', { bubbles: true, cancelable: true, inputType: 'insertFromPaste', dataTransfer: dt || undefined });
						composer.dispatchEvent(bieP);
						if (window.__waComposerText(composer)) return 'paste';
					} catch (e) {}
					try {
						var pe = new Event('paste', { bubbles: true, cancelable: true });
						var shim = { getData: function(f) { return f === 'text/plain' ? text : ''; }, types: ['text/plain'] };
						try { Object.defineProperty(pe, 'clipboardData', { value: shim, configurable: true }); }
						catch (e2) { try { pe.clipboardData = shim; } catch (e3) {} }
						composer.dispatchEvent(pe);
						if (window.__waComposerText(composer)) return 'paste';
					} catch (e) {}
				} catch (e) {}
				try {
					selectAll();
					var bie = new InputEvent('beforeinput', { bubbles: true, cancelable: true, inputType: 'insertText', data: text });
					composer.dispatchEvent(bie);
					if (window.__waComposerText(composer)) return 'beforeinput';
				} catch (e) {}
				try {
					if (composer.tagName === 'TEXTAREA') {
						composer.value = text;
					} else {
						composer.textContent = text;
					}
					// Try the data-carrying input first (state sync); if
					// WhatsApp reverts it synchronously (trust gate), fall
					// back to the plain input event older builds used —
					// that one kept the text visible at least.
					try {
						composer.dispatchEvent(new InputEvent('input', { bubbles: true, cancelable: false, inputType: 'insertText', data: text }));
					} catch (ie) {}
					if (window.__waComposerText(composer)) return 'dom';
					if (composer.tagName === 'TEXTAREA') {
						composer.value = text;
					} else {
						composer.textContent = text;
					}
					composer.dispatchEvent(new Event('input', { bubbles: true }));
					return window.__waComposerText(composer) ? 'dom-plain' : '';
				} catch (e) { return ''; }
			};
			// ── Trusted-input bridge (WebView2 CDP) ────────────────────
			// WhatsApp reverts every synthetic edit: the field log showed
			// 'insert path=none len=0' for exec, paste, beforeinput and
			// plain input alike (Lexical trust-gates typed-looking
			// events). The only channel that produces isTrusted=true
			// input at the renderer is the Chrome DevTools Protocol,
			// exposed to the page by the waCdpNative host binding
			// (Windows shell only — absent elsewhere, so callers fall
			// back to the rung ladder above).
			window.__waCdpCb = {};
			window.__waCdpResult = function(payload) {
				try {
					var o = JSON.parse(payload);
					var cb = window.__waCdpCb[o.id];
					if (cb) {
						delete window.__waCdpCb[o.id];
						cb(o.ok ? o.result : null);
					}
				} catch (e) {}
			};
			window.__waCdp = function(method, params, cb) {
				try {
					if (typeof window.waCdpNative !== 'function') { cb(null); return; }
					var id = 'c' + Date.now().toString(36) + Math.floor(Math.random() * 1e9).toString(36);
					var fired = false;
					function done(v) {
						if (fired) return;
						fired = true;
						if (window.__waCdpCb[id]) delete window.__waCdpCb[id];
						cb(v);
					}
					window.__waCdpCb[id] = done;
					setTimeout(function() { done(null); }, 4000);
					var p = window.waCdpNative(id, method, params);
					function settle(v) { if (String(v).indexOf('ok') !== 0) done(null); }
					if (p && typeof p.then === 'function') { p.then(settle, function() { done(null); }); }
					else settle(p);
				} catch (e) { cb(null); }
			};
			// Trusted insert: focus + select first (CDP routes input to
			// the active element), then Input.insertText through the
			// bridge. The completion callback runs after the text has
			// landed in the composer.
			window.__waTrustedInsert = function(composer, text, cb) {
				try { composer.focus(); } catch (e) {}
				try {
					var sel = window.getSelection();
					var r = document.createRange();
					r.selectNodeContents(composer);
					sel.removeAllRanges();
					sel.addRange(r);
				} catch (e) {}
				window.__waCdp('Input.insertText', JSON.stringify({ text: String(text || '') }), function(res) {
					if (res === null) { cb(''); return; }
					cb(window.__waComposerText(composer).length ? 'cdp' : '');
				});
			};
			// Insert orchestration: on the Windows shell the trusted
			// channel goes FIRST (it is the only rung WhatsApp accepted
			// in the field); its failures fall back to the rung ladder.
			window.__waInsertAsync = function(composer, text, cb) {
				if (typeof window.waCdpNative === 'function') {
					window.__waTrustedInsert(composer, text, function(path) {
						cb(path || (window.__waInsertText ? window.__waInsertText(composer, text) : ''));
					});
					return;
				}
				cb(window.__waInsertText ? window.__waInsertText(composer, text) : '');
			};
			// One send-button attempt: chat footer first, then broader
			// scopes; accepts button or role="button" hosts, data-icon
			// and data-testid variants, then LOCALIZED aria-labels
			// (Indonesian UI says "Kirim", not "Send").
			window.__waClickSend = function() {
				var sels = [
					'#main footer [data-icon="send"]',
					'#main [data-icon="send"]',
					'footer [data-icon="send"]',
					'#main [data-testid="send"]',
					'footer [data-testid="send"]',
					'#main button[aria-label="Send"]',
					'#main [role="button"][aria-label="Send"]',
					'button[aria-label="Send"]',
					'[data-icon="send"]'
				];
				for (var i = 0; i < sels.length; i++) {
					var icon = null;
					try { icon = document.querySelector(sels[i]); } catch (e) {}
					if (!icon || !icon.closest) continue;
					var btn = icon.closest('button, [role="button"]');
					if (btn) {
						btn.click();
						return true;
					}
				}
				// Localized label scan over the chat footer. Only
				// send-ish labels qualify — never mic/record controls
				// (clicking those would start a voice note).
				try {
					var f = document.querySelector('#main footer') || document.querySelector('footer');
					if (f) {
						var ctrls = f.querySelectorAll('button, [role="button"]');
						var re = /^(send|kirim|enviar|envoyer|senden|invia|отправить|gönder|invio|送信|发送)\b/i;
						for (var ci = 0; ci < ctrls.length; ci++) {
							var lbl = String(ctrls[ci].getAttribute('aria-label') || '').trim();
							if (re.test(lbl)) {
								ctrls[ci].click();
								return true;
							}
						}
					}
				} catch (e) {}
				return false;
			};
			// What the chat footer currently offers — logged at send-wait
			// start so a field report shows whether SEND was ever there.
			window.__waFooterSnapshot = function() {
				try {
					var f = document.querySelector('#main footer') || document.querySelector('footer');
					if (!f) return 'no-footer';
					var ctrls = f.querySelectorAll('button, [role="button"]');
					var out = [];
					for (var i = 0; i < ctrls.length; i++) {
						var ic = ctrls[i].querySelector('[data-icon]');
						out.push((ctrls[i].getAttribute('aria-label') || '-') + ':' + (ic ? ic.getAttribute('data-icon') : '-'));
					}
					return out.length ? out.join('|') : 'no-ctrls';
				} catch (e) { return 'err'; }
			};
			window.__waEnterSend = function(composer) {
				try {
					var ev = new KeyboardEvent('keydown', { key: 'Enter', code: 'Enter', keyCode: 13, which: 13, bubbles: true, cancelable: true });
					try {
						// Belt: the constructor may drop keyCode/which in
						// some engines; handlers that check 13 must not
						// see a zeroed event.
						Object.defineProperty(ev, 'keyCode', { get: function() { return 13; } });
						Object.defineProperty(ev, 'which', { get: function() { return 13; } });
					} catch (e) {}
					composer.dispatchEvent(ev);
					return true;
				} catch (e) { return false; }
			};
			// Send stage: click the moment the button exists, keep
			// polling while WhatsApp re-renders MIC→SEND after our
			// insert, fire Enter at 1.5s as a fallback, and — new — KEEP
			// polling after Enter so a late-rendering button still gets
			// clicked (5s cap). Every outcome is logged to the scheduler
			// log with the owner tag so a field report shows exactly
			// where delivery stopped.
			window.__waSendWithWait = function(composer, owner) {
				owner = owner || 'sched';
				var t0 = Date.now();
				var done = false;
				var enterFired = false;
				function diag(msg) {
					try { pushSchedLog('', '', 'diag(' + owner + '): ' + msg); } catch (e) {}
				}
				function attempt() {
					if (done) return;
					if (window.__waClickSend()) {
						done = true;
						diag('send clicked after ' + (Date.now() - t0) + 'ms' + (enterFired ? ' (after enter)' : ''));
						return;
					}
					var el = Date.now() - t0;
					if (!enterFired && el >= 1500) {
						enterFired = true;
						window.__waEnterSend(composer);
						diag('enter fired at ' + el + 'ms, send button not found; footer=' + window.__waFooterSnapshot());
					}
					if (el >= 5000) {
						done = true;
						diag('no send trigger by ' + el + 'ms; footer=' + window.__waFooterSnapshot() +
							' composerLen=' + window.__waComposerText(composer).length);
						clearInterval(iv);
					}
				}
				diag('send-wait start; footer=' + window.__waFooterSnapshot());
				attempt();
				if (done) return;
				var iv = setInterval(function() {
					attempt();
					if (done) clearInterval(iv);
				}, 100);
			};
			// Reboot-loop backstop: many boots in a few minutes means
			// something is reloading the page underneath us, and every boot
			// would otherwise attempt the pending send again. Freeze all
			// scheduler activity while boots are that frequent; thaws on its
			// own once boots calm down. Nothing is deleted while frozen.
			var BOOT_KEY = 'wa_desk_sched_boots';
			var FROZEN_KEY = 'wa_desk_sched_frozen_until';
			var BOOT_WINDOW = 5 * 60 * 1000;
			var BOOT_CAP = 8;
			var FREEZE_MS = 30 * 60 * 1000;
			var sending = false;
			var scheduledBusyOn = false;

			// Natural time parser for the scheduler ("2h", "tomorrow 08:00",
			// "fri 18:00", "besok 08:00", "21:00"). English + Indonesian,
			// pure function of (text, now) so it stays unit-testable.
			// Returns epoch ms, or 0 when not understood.
			var SCHED_UNITS = { h: 3600000, hour: 3600000, hours: 3600000, jam: 3600000,
				m: 60000, min: 60000, mins: 60000, minute: 60000, minutes: 60000, menit: 60000,
				d: 86400000, day: 86400000, days: 86400000, hari: 86400000,
				w: 604800000, week: 604800000, weeks: 604800000, minggu: 604800000 };
			var SCHED_DAYS = { sunday: 0, monday: 1, tuesday: 2, wednesday: 3, thursday: 4, friday: 5, saturday: 6,
				sun: 0, mon: 1, tue: 2, wed: 3, thu: 4, fri: 5, sat: 6,
				ahad: 0, senin: 1, selasa: 2, rabu: 3, kamis: 4, jumat: 5, sabtu: 6,
				min: 0, sen: 1, sel: 2, rab: 3, kam: 4, jum: 5, sab: 6 };
			function parseSchedWhen(raw, nowMs) {
				var now = new Date(typeof nowMs === 'number' ? nowMs : Date.now());
				var s = String(raw || '').trim().toLowerCase().replace(/\s+/g, ' ');
				if (!s) return 0;
				var atTime = function(base, hh, mm) {
					var d = new Date(base.getTime());
					d.setHours(hh, mm, 0, 0);
					return d.getTime();
				};
				// Relative: "2h", "30m", "1d", "1h30m", "2 jam", "besok"? no -
				// combos of number+unit only.
				var relBody = s.match(/^((?:\d+\s*(?:hours|hour|minutes|minute|mins|menit|minggu|min|weeks|week|days|day|hari|jam|h|m|d|w)\s*)+)$/);
				if (relBody) {
					var total = 0, ok = false;
					var re = /(\d+)\s*(hours|hour|minutes|minute|mins|menit|minggu|min|weeks|week|days|day|hari|jam|h|m|d|w)/g;
					var m;
					while ((m = re.exec(relBody[1])) !== null) {
						ok = true;
						total += parseInt(m[1], 10) * (SCHED_UNITS[m[2]] || 0);
					}
					if (ok && total > 0 && total < 366 * 86400000) return now.getTime() + total;
					return 0;
				}
				// Clock time today (else tomorrow): "21:00", "21.00".
				var hm = s.match(/^(\d{1,2})[:.](\d{2})$/);
				if (hm) {
					var hh = parseInt(hm[1], 10), mm = parseInt(hm[2], 10);
					if (hh > 23 || mm > 59) return 0;
					var t = atTime(now, hh, mm);
					return t > now.getTime() ? t : t + 86400000;
				}
				// today|hari ini [HH:MM], tomorrow|besok [HH:MM].
				var day = s.match(/^(today|hari ini|tomorrow|besok)(?:\s+(\d{1,2})[:.](\d{2}))?$/);
				if (day) {
					var base = new Date(now.getTime());
					if (day[1] === 'tomorrow' || day[1] === 'besok') base.setDate(base.getDate() + 1);
					if (day[2] !== undefined) {
						var hh2 = parseInt(day[2], 10), mm2 = parseInt(day[3], 10);
						if (hh2 > 23 || mm2 > 59) return 0;
						return atTime(base, hh2, mm2);
					}
					if (day[1] === 'today' || day[1] === 'hari ini') return now.getTime() + 3600000;
					return atTime(base, 9, 0);
				}
				// Weekday [HH:MM], EN + ID: "fri 18:00", "jumat 18:00", "fri".
				var wd = s.match(/^([a-z]+)(?:\s+(\d{1,2})[:.](\d{2}))?$/);
				if (wd && Object.prototype.hasOwnProperty.call(SCHED_DAYS, wd[1])) {
					var delta = (SCHED_DAYS[wd[1]] - now.getDay() + 7) % 7;
					var base2 = new Date(now.getTime());
					if (wd[2] !== undefined) {
						var hh3 = parseInt(wd[2], 10), mm3 = parseInt(wd[3], 10);
						if (hh3 > 23 || mm3 > 59) return 0;
						var t3 = atTime(base2, hh3, mm3);
						if (delta === 0 && t3 > now.getTime()) return t3;
						base2.setDate(base2.getDate() + (delta === 0 ? 7 : delta));
						return atTime(base2, hh3, mm3);
					}
					base2.setDate(base2.getDate() + (delta === 0 ? 7 : delta));
					return atTime(base2, 9, 0);
				}
				return 0;
			}
			window.parseSchedWhen = parseSchedWhen;
			function uid() {
				return 's' + Date.now().toString(36) + Math.floor(Math.random() * 1e6).toString(36);
			}
			function validSchedPhone(raw) {
				var d = String(raw || '').trim().replace(/^\+/, '').replace(/[\s\-().]/g, '');
				if (!/^[0-9]{8,15}$/.test(d)) return '';
				return d;
			}
			function loadSchedules() {
				try {
					var list = JSON.parse(localStorage.getItem(SCHED_KEY));
					if (!Array.isArray(list)) return [];
					var out = [];
					for (var i = 0; i < list.length && out.length < SCHED_MAX; i++) {
						var s = list[i] || {};
						var digits = validSchedPhone(s.phone);
						var text = String(s.text || '').slice(0, 2000);
					var nextFire = Number(s.nextFire) || 0;
					var repeat = (s.repeat === 'daily' || s.repeat === 'weekly' || s.repeat === 'monthly' ||
						s.repeat === 'yearly') ? s.repeat : 'once';
					if (!digits || !text || !nextFire) continue;
					var rd = Number(s.repeatDay);
					var item = { id: String(s.id || uid()), phone: digits, text: text, nextFire: nextFire, repeat: repeat, enabled: s.enabled !== false, lastFiredAt: Number(s.lastFiredAt) || 0 };
					if (repeat === 'weekly' && rd >= 0 && rd <= 6 && rd === Math.floor(rd)) item.repeatDay = rd;
					var dmn = Number(s.dom);
					if (dmn >= 1 && dmn <= 31 && dmn === Math.floor(dmn)) item.dom = dmn;
					var ydn = Number(s.yday);
					if (ydn >= 1 && ydn <= 31 && ydn === Math.floor(ydn)) item.yday = ydn;
					out.push(item);
					}
					return out;
				} catch (e) {
					return [];
				}
			}
			function saveSchedules(list) {
				try {
					localStorage.setItem(SCHED_KEY, JSON.stringify((list || []).slice(0, SCHED_MAX)));
				} catch (e) {}
				recountScheduledBusy();
			}
			function pushSchedLog(phone, text, status) {
				try {
					var log = [];
					try {
						var raw = JSON.parse(localStorage.getItem(SCHED_LOG));
						if (Array.isArray(raw)) log = raw;
					} catch (e) {}
					log.unshift({ at: Date.now(), phone: String(phone || ''), text: String(text || '').slice(0, 80), status: status });
					localStorage.setItem(SCHED_LOG, JSON.stringify(log.slice(0, LOG_MAX)));
				} catch (e) {}
			}
			window.getSchedules = loadSchedules;
			window.getScheduleLog = function() {
				try {
					var raw = JSON.parse(localStorage.getItem(SCHED_LOG));
					return Array.isArray(raw) ? raw.slice(0, LOG_MAX) : [];
				} catch (e) {
					return [];
				}
			};
			window.addSchedule = function(phone, text, whenMs, repeat, repeatDay) {
				var digits = validSchedPhone(phone);
				text = String(text || '').trim().slice(0, 2000);
				whenMs = Number(whenMs) || 0;
				if (!digits || !text || !(whenMs > Date.now())) return null;
				if (repeat !== 'daily' && repeat !== 'weekly' && repeat !== 'monthly' &&
					repeat !== 'yearly') repeat = 'once';
				var item = { id: uid(), phone: digits, text: text, nextFire: whenMs, repeat: repeat, enabled: true };
				if (repeat === 'weekly') {
					// Chosen weekday (0=Sun..6=Sat) for every recurrence
					// after the first fire; default = the weekday the
					// first fire itself lands on.
					var rd = Number(repeatDay);
					item.repeatDay = (rd >= 0 && rd <= 6 && rd === Math.floor(rd)) ?
						rd : new Date(whenMs).getDay();
				} else if (repeat === 'monthly') {
					// Original day-of-month so Jan 31 keeps coming back
					// as 31 (clamped only in February).
					item.dom = new Date(whenMs).getDate();
				} else if (repeat === 'yearly') {
					item.yday = new Date(whenMs).getDate();
				}
				var list = loadSchedules();
				list.push(item);
				saveSchedules(list);
				return item.id;
			};
			window.deleteSchedule = function(id) {
				saveSchedules(loadSchedules().filter(function(s) { return s.id !== id; }));
			};
			window.toggleSchedule = function(id, on) {
				var list = loadSchedules();
				for (var i = 0; i < list.length; i++) {
					if (list[i].id === id) list[i].enabled = !!on;
				}
				saveSchedules(list);
				// Switching a schedule off withdraws ITS queued payload:
				// "off" must mean off, even when the payload was already
				// fired and is waiting for the next boot to deliver it.
				if (!on) {
					try {
						var pend = JSON.parse(localStorage.getItem(PENDING_KEY));
						if (pend && pend.schedId === id) {
							localStorage.removeItem(PENDING_KEY);
							pushSchedLog(pend.phone, pend.text || '', 'cancelled (switched off)');
							showFloatingToast('🚫 Scheduled send cancelled — schedule switched off');
						}
					} catch (e) {}
				}
			};
			// Reports whether this tab currently holds work the shell must
			// not tear down. Transitions only, like the other busy kinds.
			// A killed engine holds nothing alive.
			function recountScheduledBusy() {
				var now = Date.now();
				var n = 0;
				if (!schedKilled()) {
					var list = loadSchedules();
					for (var i = 0; i < list.length; i++) {
						if (list[i].enabled && list[i].nextFire > now) n++;
					}
					if (sending) n++;
				}
				var on = n > 0;
				if (on !== scheduledBusyOn) {
					scheduledBusyOn = on;
					if (window.__waReportBusy) window.__waReportBusy('scheduled', on);
				}
			}
			// Advance a fired schedule to its next occurrence. Pure
			// function of (schedule, now) so the repeat matrix is
			// unit-testable: once disables; daily/weekly step in fixed
			// increments (weekly honoring a chosen weekday, repeatDay
			// 0=Sun..6=Sat, always keeping the time of day); monthly
			// clamps to the shortest month using the ORIGINAL
			// day-of-month (dom, stored at add time — Jan 31 → Feb 28
			// → Mar 31, not 3); yearly keeps month+day (yday) the same
			// way, so Feb 29 skips to Feb 28 in common years and
			// returns to Feb 29 in leap years.
			function advanceSchedule(s, now) {
				if (s.repeat === 'once') {
					s.enabled = false;
					return;
				}
				var DAY = 24 * 3600 * 1000;
				if (s.repeat === 'daily') {
					while (s.nextFire <= now) s.nextFire += DAY;
					return;
				}
				if (s.repeat === 'weekly') {
					var wd = (typeof s.repeatDay === 'number' && s.repeatDay >= 0 && s.repeatDay <= 6) ?
						Math.floor(s.repeatDay) : -1;
					if (wd < 0) {
						// Legacy rows without a chosen weekday: same
						// weekday as the original fire.
						while (s.nextFire <= now) s.nextFire += 7 * DAY;
						return;
					}
					var dw = new Date(s.nextFire);
					do { dw.setDate(dw.getDate() + 1); } while (dw.getDay() !== wd);
					s.nextFire = dw.getTime();
					while (s.nextFire <= now) {
						do { dw.setDate(dw.getDate() + 1); } while (dw.getDay() !== wd);
						s.nextFire = dw.getTime();
					}
					return;
				}
				if (s.repeat === 'monthly') {
					var day = Number(s.dom) || new Date(s.nextFire).getDate();
					if (!(day >= 1 && day <= 31)) day = new Date(s.nextFire).getDate();
					s.dom = day;
					var dm = new Date(s.nextFire);
					for (;;) {
						dm.setDate(1);
						dm.setMonth(dm.getMonth() + 1);
						var dim = new Date(dm.getFullYear(), dm.getMonth() + 1, 0).getDate();
						dm.setDate(Math.min(day, dim));
						if (dm.getTime() > now) break;
					}
					s.nextFire = dm.getTime();
					return;
				}
				if (s.repeat === 'yearly') {
					var yd = Number(s.yday) || new Date(s.nextFire).getDate();
					if (!(yd >= 1 && yd <= 31)) yd = new Date(s.nextFire).getDate();
					s.yday = yd;
					var dy = new Date(s.nextFire);
					var mo = dy.getMonth();
					for (;;) {
						dy.setDate(1);
						dy.setFullYear(dy.getFullYear() + 1);
						dy.setMonth(mo);
						var diy = new Date(dy.getFullYear(), mo + 1, 0).getDate();
						dy.setDate(Math.min(yd, diy));
						if (dy.getTime() > now) break;
					}
					s.nextFire = dy.getTime();
					return;
				}
				// Unknown repeat value: behave like once.
				s.enabled = false;
			}
			window.__waAdvanceSchedule = advanceSchedule;
			function findSchedComposer() {
				// The real composer lives in #main > footer. Newer
				// layouts ALSO have a contenteditable search-in-
				// conversation box in the header that matches first in
				// document order — typing there produced text that never
				// sends. Prefer footer; keep #main as the boundary
				// (chat-list search lives outside #main).
				var main = document.getElementById('main');
				if (!main) return null;
				var footer = main.querySelector('footer');
				if (footer) {
					var inFooter = footer.querySelector('div[contenteditable="true"][role="textbox"]') ||
						footer.querySelector('div[contenteditable="true"]') ||
						footer.querySelector('textarea');
					if (inFooter) return inFooter;
				}
				return main.querySelector('div[contenteditable="true"][role="textbox"]') ||
					main.querySelector('div[contenteditable="true"]') ||
					main.querySelector('textarea');
			}
			// Burst breaker state: completion timestamps (persisted). True
			// while completions inside the window reach the cap.
			function burstTripped() {
				var now = Date.now();
				var stamps = [];
				try {
					var raw = JSON.parse(localStorage.getItem(BURST_KEY));
					if (Array.isArray(raw)) stamps = raw;
				} catch (e) {}
				stamps = stamps.filter(function(t) { return (now - Number(t)) < BURST_WINDOW; });
				try {
					localStorage.setItem(BURST_KEY, JSON.stringify(stamps.slice(-BURST_CAP * 2)));
				} catch (e) {}
				return stamps.length >= BURST_CAP;
			}
			function recordSendCompletion() {
				var now = Date.now();
				var stamps = [];
				try {
					var raw = JSON.parse(localStorage.getItem(BURST_KEY));
					if (Array.isArray(raw)) stamps = raw;
				} catch (e) {}
				stamps.push(now);
				stamps = stamps.filter(function(t) { return (now - Number(t)) < BURST_WINDOW; });
				try {
					localStorage.setItem(BURST_KEY, JSON.stringify(stamps.slice(-BURST_CAP * 2)));
				} catch (e) {}
			}
			function noteBreakerTripped() {
				if (breakerToasted) return;
				breakerToasted = true;
				pushSchedLog('', '', 'paused: burst limit, resumes automatically');
				showFloatingToast('⏰ Scheduler paused: too many sends — resumes automatically, check Settings');
			}
			function schedFrozen() {
				try {
					return Date.now() < (Number(localStorage.getItem(FROZEN_KEY)) || 0);
				} catch (e) {
					return false;
				}
			}
			// Returns true when THIS boot trips the freeze (toast + log once
			// here; later boots stay silent until it thaws).
			function recordBootWatched() {
				var now = Date.now();
				var wasFrozen = schedFrozen();
				var arr = [];
				try {
					var raw = JSON.parse(localStorage.getItem(BOOT_KEY));
					if (Array.isArray(raw)) arr = raw;
				} catch (e) {}
				arr.push(now);
				arr = arr.filter(function(t) { return (now - Number(t)) < BOOT_WINDOW; });
				try {
					localStorage.setItem(BOOT_KEY, JSON.stringify(arr.slice(-BOOT_CAP * 2)));
				} catch (e) {}
				if (arr.length >= BOOT_CAP && !wasFrozen) {
					try {
						localStorage.setItem(FROZEN_KEY, String(now + FREEZE_MS));
					} catch (e) {}
					pushSchedLog('', '', 'frozen: reboot loop detected, thaws automatically');
					showFloatingToast('⏰ Scheduler frozen: app restarting repeatedly — resumes automatically');
					return true;
				}
				return wasFrozen;
			}
			// Claim window: a send attempt owns its payload for this long.
			// Another boot seeing a fresh claim skips instead of resending;
			// a stale claim means the attempt died mid-flight (logged failed).
			// This bounds every payload to one send attempt no matter how
			// often the page reloads underneath it.
			var CLAIM_TTL = 10 * 60 * 1000;
			function fireSchedule(s) {
				var now = Date.now();
				// Never overwrite a payload another attempt owns: one
				// in-flight send per boot is plenty.
				try {
					if (localStorage.getItem(PENDING_KEY)) return;
				} catch (e) {}
				// Persist first: a reload mid-flight must find correct state.
				try {
					localStorage.setItem(PENDING_KEY, JSON.stringify({ phone: s.phone, text: s.text, queuedAt: now, schedId: s.id }));
				} catch (e) {}
				var list = loadSchedules();
				for (var i = 0; i < list.length; i++) {
					if (list[i].id === s.id) {
						advanceSchedule(list[i], now);
						// Stamp the fire moment on the row: a later boot only
						// honors a pending whose queuedAt matches this stamp,
						// so deleting the schedule (or a stale leftover from
						// an older build) cancels it instead of haunting.
						list[i].lastFiredAt = now;
					}
				}
				saveSchedules(list);
				showFloatingToast('⏰ Sending scheduled message to +' + s.phone + '...');
				window.location.href = 'https://web.whatsapp.com/send?phone=' + s.phone;
			}
			function completePendingSend() {
				var pending = null;
				try {
					pending = JSON.parse(localStorage.getItem(PENDING_KEY));
				} catch (e) {}
				if (!pending || !pending.phone || !pending.text) return;
				if (Date.now() - (Number(pending.queuedAt) || 0) > PENDING_TTL) {
					try { localStorage.removeItem(PENDING_KEY); } catch (e) {}
					pushSchedLog(pending.phone, pending.text, 'skipped (expired)');
					return;
				}
				// The payload is only honored for a schedule row that still
				// exists and fired it: deleting the schedule cancels the
				// in-flight send instead of letting it haunt later boots.
				// (A row the user switched off after firing still matches via
				// lastFiredAt, so the in-flight send completes exactly once.)
				var ownerRow = null;
				var allRows = loadSchedules();
				for (var ri = 0; ri < allRows.length; ri++) {
					if (allRows[ri].id === pending.schedId) { ownerRow = allRows[ri]; break; }
				}
				if (!ownerRow || (!ownerRow.enabled && Number(ownerRow.lastFiredAt) !== Number(pending.queuedAt))) {
					try { localStorage.removeItem(PENDING_KEY); } catch (e) {}
					pushSchedLog(pending.phone, pending.text, 'cancelled (schedule removed)');
					showFloatingToast('🚫 Scheduled send cancelled — schedule no longer exists');
					return;
				}
				// Atomic claim: stamp ownership BEFORE touching the chat. A
				// later boot seeing a fresh claim knows an attempt is alive
				// (or died seconds ago) and must not start a second one; a
				// stale claim means the attempt died mid-flight: log it
				// failed and drop it instead of retry-spamming.
				var claimedAt = Number(pending.claimedAt) || 0;
				if (claimedAt && Date.now() - claimedAt < CLAIM_TTL) {
					return;
				}
				if (claimedAt) {
					try { localStorage.removeItem(PENDING_KEY); } catch (e) {}
					pushSchedLog(pending.phone, pending.text, 'failed (interrupted)');
					showFloatingToast('❌ Scheduled message to +' + pending.phone + ' was interrupted');
					return;
				}
				pending.claimedAt = Date.now();
				try {
					localStorage.setItem(PENDING_KEY, JSON.stringify(pending));
				} catch (e) {
					return;
				}
				// Idempotency: this exact payload already went out (a replayed
				// boot replays the send otherwise). Drop without a sound.
				try {
					var last = JSON.parse(localStorage.getItem(LAST_SENT_KEY));
					if (last && last.schedId === pending.schedId &&
						Number(last.queuedAt) === Number(pending.queuedAt)) {
						localStorage.removeItem(PENDING_KEY);
						return;
					}
				} catch (e) {}
				// Burst breaker BEFORE touching the chat: a tripped breaker
				// keeps the payload for later instead of dropping it.
				if (burstTripped()) {
					noteBreakerTripped();
					return;
				}
				// One payload, one processing per page lifetime. A repeat
				// sighting means a duplicate invocation, never a new send.
				var payloadKey = String(pending.schedId) + '|' + String(pending.queuedAt);
				if (seenPayloads[payloadKey]) {
					return;
				}
				seenPayloads[payloadKey] = true;
				sending = true;
				recountScheduledBusy();
				// Per-invocation id stamped into the log line: repeated sends
				// with one id = one routine looping; many ids = many boots.
				var attemptId = uid().slice(-4);
				// Latches: clearInterval is belt, these are suspenders. If a
				// timer object ever outlives its clear call in this runtime,
				// the bodies below still execute at most once per routine.
				var pollDone = false;
				var sendDone = false;
				var verifyDone = false;
				var waited = 0;
				var timer = setInterval(function() {
					if (pollDone) return;
					waited += 500;
					var composer = findSchedComposer();
					if (!composer && waited < SEND_TIMEOUT) return;
					// Latch the polling stage itself: this interval may
					// outlive clearInterval in a hostile runtime (the very
					// failure class the field spam rode on). Only the
					// completion decision arms it — the wait loop above
					// must keep polling.
					pollDone = true;
					clearInterval(timer);
					if (!composer) {
						finishSend(pending, false);
						return;
					}
					// Re-checks before typing: kill or a Settings action
					// may have withdrawn the payload AFTER claim. Never
					// type text we won't send.
					if (schedKilled()) {
						finishSend(pending, false, 'cancelled (emergency stop)');
						return;
					}
					try {
						if (!localStorage.getItem(PENDING_KEY)) {
							finishSend(pending, false, 'cancelled (pending withdrawn)');
							return;
						}
					} catch (e) {}
					var guard = window.__waGuardSend ? window.__waGuardSend('sched') : { ok: true };
					if (!guard.ok) {
						finishSend(pending, false, (guard.why === 'pace' || guard.why === 'cap') ?
							('blocked (send ' + guard.why + ')') : 'cancelled (emergency stop)');
						return;
					}
					try { composer.focus(); } catch (e) {}
					// Insert may travel the trusted CDP channel, which is
					// asynchronous: everything after it runs from
					// beginSend inside the insert callback.
					function beginSend(insertPath) {
						pushSchedLog(pending.phone, pending.text, 'diag: insert path=' + (insertPath || 'none') +
							' composer=' + (composer.closest && composer.closest('footer') ? 'footer' : 'main') +
							' len=' + window.__waComposerText(composer).length);
						if (!insertPath) {
							finishSend(pending, false);
							return;
						}
						// Latched: one send per routine no matter what.
						if (sendDone) {
							return;
						}
						sendDone = true;
						// Click the send button once it has rendered: our
						// insert updates WhatsApp's state asynchronously
						// (MIC→SEND swap), so a synchronous lookup raced the
						// render and dropped to an Enter that many builds
						// ignore — text stayed as an unsent draft. Enter is
						// the first fallback; a late button still gets
						// clicked afterwards.
						window.__waSendWithWait(composer, 'sched');
						var checks = 0;
						var verify = setInterval(function() {
							if (verifyDone) return;
							checks++;
							var txt = '';
							try {
								txt = ((composer.tagName === 'TEXTAREA' ? composer.value : composer.textContent) || '').trim();
							} catch (e) {}
						if (txt === '' || checks >= 16) {
							verifyDone = true;
							clearInterval(verify);
							finishSend(pending, txt === '');
							}
						}, 500);
					}
					window.__waInsertAsync(composer, pending.text, function(insertPath) {
						// The async gap lets kill/withdraw land mid-flight:
						// re-check both before the send stage starts.
						if (schedKilled()) {
							finishSend(pending, false, 'cancelled (emergency stop)');
							return;
						}
						try {
							if (!localStorage.getItem(PENDING_KEY)) {
								finishSend(pending, false, 'cancelled (pending withdrawn)');
								return;
							}
						} catch (e) {}
						beginSend(insertPath);
					});
				}, 500);
				var finishedSend = false;
				function finishSend(p, ok, failStatus) {
					if (finishedSend) return;
					finishedSend = true;
					try { localStorage.removeItem(PENDING_KEY); } catch (e) {}
					if (ok) {
						recordSendCompletion();
						try {
							localStorage.setItem(LAST_SENT_KEY, JSON.stringify({ schedId: p.schedId, queuedAt: p.queuedAt }));
						} catch (e) {}
					}
					sending = false;
					recountScheduledBusy();
					var tag = ' [' + bootId + '/' + attemptId + ']';
					pushSchedLog(p.phone, p.text, (ok ? 'sent' : (failStatus || 'failed')) + tag);
					showFloatingToast(ok ? ('✅ Scheduled message sent to +' + p.phone) : ('❌ Scheduled message to +' + p.phone + ' failed'));
				}
			}
			setInterval(function() {
				// Headless tabs still fire: page timers run while the engine
				// is alive (the busy report keeps it that way).
				if (sending) return;
				if (schedKilled()) return;
				// An AFK auto-reply owns the composer right now: firing
				// would navigate mid-attempt and kill it. The next tick
				// (20s later) fires instead — schedules stay armed.
				if (window.__afkInFlight && window.__afkInFlight()) return;
				if (burstTripped()) {
					noteBreakerTripped();
					return;
				}
				var now = Date.now();
				var list = loadSchedules();
				for (var i = 0; i < list.length; i++) {
					if (list[i].enabled && list[i].nextFire <= now) {
						fireSchedule(list[i]);
						return;
					}
				}
			}, TICK_MS);
			var bootFroze = recordBootWatched();
			if (!bootFroze && !schedFrozen() && !schedKilled()) {
				recountScheduledBusy();
				completePendingSend();
			}
		})();

		// AFK auto-reply: while armed, every chat that shows new unread gets
		// ONE automatic reply per cooldown window; the moment YOU send a
		// message yourself the mode turns itself off. Safety mirrors the
		// scheduler: emergency kill, per-chat cooldown claimed BEFORE any
		// click, burst breaker, in-flight marker for boot recovery, timer
		// latches, and an identity check that refuses to type into any chat
		// other than the row that was clicked. Yields the composer entirely
		// while a scheduler payload is queued (never two senders at once).
		(function() {
			var CFG_KEY = 'wa_desk_afk_cfg';
			var LAST_KEY = 'wa_desk_afk_last';
			var BURST_KEY = 'wa_desk_afk_send_times';
			var LOG_KEY = 'wa_desk_afk_log';
			var KILLED_KEY = 'wa_desk_afk_killed';
			var INFLIGHT_KEY = 'wa_desk_afk_inflight';
			var SCHED_PENDING_KEY = 'wa_desk_pending_send';
			var TICK_MS = 5000;
			var BURST_CAP = 8;
			var BURST_WINDOW = 10 * 60 * 1000;
			var LOG_MAX = 20;
			var bootId = (function() {
				try {
					var a = new Uint32Array(1);
					(window.crypto || window.msCrypto).getRandomValues(a);
					// >>> not >>: Uint32 above 2^31 would sign-extend and
					// put a '-' into the id.
					return (a[0] % 1296).toString(36) + ((a[0] >>> 6) % 1296).toString(36);
				} catch (e) {
					return Math.floor(Math.random() * 46656).toString(36);
				}
			})();
			var sending = false;
			var busyOn = false;
			var breakerToasted = false;
			// Last genuine (trusted) user input anywhere on the page. Our
			// own synthetic row presses are untrusted and must not count,
			// or every attempt would silence the tick for 30s afterwards.
			var lastUserActivityAt = 0;
			try {
				['keydown', 'mousedown', 'wheel'].forEach(function(t) {
					document.addEventListener(t, function(e) {
						try { if (e && e.isTrusted) lastUserActivityAt = Date.now(); } catch (e2) {}
					}, true);
				});
			} catch (e) {}
			function clampDayMin(v, fallback) {
				var n = Math.floor(Number(v));
				if (!(n >= 0 && n < 1440)) return fallback;
				return n;
			}
			function loadAfkCfg() {
				try {
					var c = JSON.parse(localStorage.getItem(CFG_KEY)) || {};
					var cd = Number(c.cooldownMin);
					var allowed = [];
					try {
						if (Array.isArray(c.allowed)) {
							for (var ai = 0; ai < c.allowed.length && allowed.length < 50; ai++) {
								var as = String(c.allowed[ai] || '').trim().slice(0, 80);
								if (as) allowed.push(as);
							}
						}
					} catch (e2) {}
					return {
						enabled: c.enabled === true,
						text: String(c.text || '').slice(0, 500),
						cooldownMin: (cd === 5 || cd === 30 || cd === 60) ? cd : 10,
						groups: c.groups === true,
						// Daily reply window (minutes since midnight). Off by
						// default: legacy configs keep replying around the
						// clock until the user opts in.
						useWindow: c.useWindow === true,
						startMin: clampDayMin(c.startMin, 21 * 60),
						endMin: clampDayMin(c.endMin, 7 * 60),
						// Reply allowlist: when on, only listed chats get
						// replies. Entries are names (partial, case-
						// insensitive) or numbers (digit-compared).
						allowOnly: c.allowOnly === true,
						allowed: allowed
					};
				} catch (e) {
					return { enabled: false, text: '', cooldownMin: 10, groups: false, useWindow: false, startMin: 21 * 60, endMin: 7 * 60, allowOnly: false, allowed: [] };
				}
			}
			function saveAfkCfg(c) {
				try { localStorage.setItem(CFG_KEY, JSON.stringify(c)); } catch (e) {}
			}
			// Pure function of (config, now): true when the clock is inside
			// the daily reply window. Overnight spans (e.g. 21:00-07:00)
			// wrap past midnight; start === end means the whole day.
			function afkInWindow(cfg, nowMs) {
				cfg = cfg || {};
				if (cfg.useWindow !== true) return true;
				var s = Number(cfg.startMin), e = Number(cfg.endMin);
				if (!(s >= 0 && s < 1440) || !(e >= 0 && e < 1440)) return true;
				if (s === e) return true;
				var d = new Date(typeof nowMs === 'number' ? nowMs : Date.now());
				var mins = d.getHours() * 60 + d.getMinutes();
				if (s < e) return mins >= s && mins < e;
				return mins >= s || mins < e;
			}
			window.__waAfkInWindow = afkInWindow;
			// Pure: may this chat name receive replies under the allowlist?
			// Off (or empty list + off) allows everything; an enabled empty
			// list allows nothing. Entries WITH letters match partially
			// case-insensitively; purely numeric entries match by digits
			// only (8+ digits, one leading 62/0 tolerated), so 62812…,
			// 0812… and +62 812-… all find each other while a fragment
			// like "12" never matches a name by accident.
			function afkAllowed(cfg, key) {
				cfg = cfg || {};
				if (cfg.allowOnly !== true) return true;
				var list = Array.isArray(cfg.allowed) ? cfg.allowed : [];
				if (!list.length) return false;
				var k = String(key || '').toLowerCase().trim();
				if (!k) return false;
				var kd = k.replace(/\D/g, '');
				for (var i = 0; i < list.length; i++) {
					var e = String(list[i] || '').toLowerCase().trim();
					if (!e) continue;
					if (/[a-z]/i.test(e) && (k.indexOf(e) !== -1 || e.indexOf(k) !== -1)) return true;
					var ed = e.replace(/\D/g, '');
					if (ed.length >= 8 && kd.length >= 8) {
						var kn = kd.replace(/^(62|0)/, '');
						var en = ed.replace(/^(62|0)/, '');
						if (kn && en && (kn.indexOf(en) !== -1 || en.indexOf(kn) !== -1)) return true;
					}
				}
				return false;
			}
			window.__waAfkAllowed = afkAllowed;
			// "HH:MM" <-> minutes helpers shared with the Settings inputs.
			window.__waAfkMinsToHM = function(m) {
				m = clampDayMin(m, 0);
				var h = Math.floor(m / 60), mm = m % 60;
				return (h < 10 ? '0' : '') + h + ':' + (mm < 10 ? '0' : '') + mm;
			};
			window.__waAfkHMToMins = function(str) {
				var m = /^(\d{1,2}):(\d{1,2})$/.exec(String(str || '').trim());
				if (!m) return -1;
				var h = Number(m[1]), mm = Number(m[2]);
				if (!(h >= 0 && h < 24 && mm >= 0 && mm < 60)) return -1;
				return h * 60 + mm;
			};
			// Armed = enabled + not killed + non-empty text + inside the
			// reply window; without text there is nothing to send, so never
			// report busy or scan. Outside the window the tick stands down
			// (and the tab may hibernate) until the hours come back.
			function afkArmed() {
				var c = loadAfkCfg();
				if (!c.enabled || !c.text) return false;
				if (!afkInWindow(c, Date.now())) return false;
				try { return localStorage.getItem(KILLED_KEY) !== '1'; } catch (e) { return false; }
			}
			window.getAfkConfig = loadAfkCfg;
			window.setAfkConfig = function(patch) {
				var c = loadAfkCfg();
				patch = patch || {};
				if (typeof patch.enabled === 'boolean') c.enabled = patch.enabled;
				if (typeof patch.text === 'string') c.text = patch.text.slice(0, 500);
				if (patch.cooldownMin === 5 || patch.cooldownMin === 30 || patch.cooldownMin === 60) {
					c.cooldownMin = patch.cooldownMin;
				}
				if (typeof patch.groups === 'boolean') c.groups = patch.groups;
				if (typeof patch.useWindow === 'boolean') c.useWindow = patch.useWindow;
				if (typeof patch.allowOnly === 'boolean') c.allowOnly = patch.allowOnly;
				if (typeof patch.allowed === 'string' || Array.isArray(patch.allowed)) {
					var rawList = (typeof patch.allowed === 'string') ? patch.allowed.split(/\n/) : patch.allowed;
					var cleanList = [];
					for (var li = 0; li < rawList.length && cleanList.length < 50; li++) {
						var ls = String(rawList[li] || '').trim().slice(0, 80);
						if (ls) cleanList.push(ls);
					}
					c.allowed = cleanList;
				}
				var sm = (typeof patch.startMin === 'string') ?
					window.__waAfkHMToMins(patch.startMin) : Math.floor(Number(patch.startMin));
				if (sm >= 0 && sm < 1440) c.startMin = sm;
				var em = (typeof patch.endMin === 'string') ?
					window.__waAfkHMToMins(patch.endMin) : Math.floor(Number(patch.endMin));
				if (em >= 0 && em < 1440) c.endMin = em;
				// Arming without a message is meaningless: clamp instead of
				// scanning unread forever with nothing to say.
				if (c.enabled && !c.text) c.enabled = false;
				saveAfkCfg(c);
				recountAfkBusy();
				return loadAfkCfg();
			};
			window.setAFKKilled = function(on) {
				try { localStorage.setItem(KILLED_KEY, on ? '1' : '0'); } catch (e) {}
				recountAfkBusy();
				// Reports whether replying is allowed (not killed), like
				// setSchedulerKilled.
				try { return localStorage.getItem(KILLED_KEY) !== '1'; } catch (e) { return false; }
			};
			window.getAfkLog = function() {
				try {
					var raw = JSON.parse(localStorage.getItem(LOG_KEY));
					return Array.isArray(raw) ? raw.slice(0, LOG_MAX) : [];
				} catch (e) {
					return [];
				}
			};
			function pushAfkLog(chat, status) {
				try {
					var log = [];
					try {
						var raw = JSON.parse(localStorage.getItem(LOG_KEY));
						if (Array.isArray(raw)) log = raw;
					} catch (e) {}
					log.unshift({ at: Date.now(), chat: String(chat || '').slice(0, 80), status: String(status || '') });
					localStorage.setItem(LOG_KEY, JSON.stringify(log.slice(0, LOG_MAX)));
				} catch (e) {}
			}
			// Transition-only busy report (like the scheduler's): armed tabs
			// must not hibernate, or their timers die with the auto-reply.
			function recountAfkBusy() {
				var on = afkArmed();
				if (on !== busyOn) {
					busyOn = on;
					if (window.__waReportBusy) window.__waReportBusy('scheduled', on);
				}
			}
			function afkUid() {
				return 'a' + Date.now().toString(36) + Math.floor(Math.random() * 1e6).toString(36);
			}
			// Per-chat cooldown map, pruned to 24h and 100 entries so a
			// long-running account cannot grow it without bound.
			function loadCooldowns() {
				var map = {};
				try {
					var raw = JSON.parse(localStorage.getItem(LAST_KEY));
					if (raw && typeof raw === 'object') map = raw;
				} catch (e) {}
				var cutoff = Date.now() - 24 * 3600 * 1000;
				var out = {};
				var n = 0;
				for (var k in map) {
					if (!Object.prototype.hasOwnProperty.call(map, k)) continue;
					if (!(Number(map[k]) > cutoff)) continue;
					out[k] = Number(map[k]);
					if (++n >= 100) break;
				}
				return out;
			}
			function burstTripped() {
				var now = Date.now();
				var stamps = [];
				try {
					var raw = JSON.parse(localStorage.getItem(BURST_KEY));
					if (Array.isArray(raw)) stamps = raw;
				} catch (e) {}
				stamps = stamps.filter(function(t) { return (now - Number(t)) < BURST_WINDOW; });
				try { localStorage.setItem(BURST_KEY, JSON.stringify(stamps.slice(-BURST_CAP * 2))); } catch (e) {}
				return stamps.length >= BURST_CAP;
			}
			function recordAfkCompletion() {
				var now = Date.now();
				var stamps = [];
				try {
					var raw = JSON.parse(localStorage.getItem(BURST_KEY));
					if (Array.isArray(raw)) stamps = raw;
				} catch (e) {}
				stamps.push(now);
				stamps = stamps.filter(function(t) { return (now - Number(t)) < BURST_WINDOW; });
				try { localStorage.setItem(BURST_KEY, JSON.stringify(stamps.slice(-BURST_CAP * 2))); } catch (e) {}
			}
			function noteAfkBreaker() {
				if (breakerToasted) return;
				breakerToasted = true;
				pushAfkLog('', 'paused: burst limit, resumes automatically');
				showFloatingToast('🤖 AFK paused: too many auto-replies — resumes automatically');
			}
			function expandAfkVars(text, name) {
				var now = new Date();
				return String(text || '')
					.split('{date}').join(now.toLocaleDateString())
					.split('{time}').join(now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }))
					.split('{name}').join(String(name || ''));
			}
			// Name for a candidate row: the bare chat name without badge
			// copies. Prefers the titled name span (cell-frame-title >
			// span[title]), then badge-stripped title text, then a legacy
			// button title. Pure over (row, btn) so the harness can drive
			// it with detached fixtures.
			function afkRowName(row, btn) {
				var name = '';
				try {
					var titleEl = row.querySelector('[data-testid="cell-frame-title"]');
					if (titleEl) {
						var named = titleEl.querySelector('span[title]');
						if (named && named.getAttribute('title')) {
							name = String(named.getAttribute('title')).replace(/\s+/g, ' ').trim().slice(0, 80);
						}
					}
				} catch (e) {}
				if (!name) {
					try {
						var titleEl2 = row.querySelector('[data-testid="cell-frame-title"]');
						if (titleEl2) {
							// The unread badge lives INSIDE the title node in
							// the new layout — strip badge copies so the key
							// stays the bare chat name (cooldown + identity
							// depend on it).
							var clone = titleEl2.cloneNode(true);
							var badges = clone.querySelectorAll('[data-testid="icon-unread-count"], [data-testid="unread-pill"], [aria-label*="unread" i], [aria-label*="belum dibaca" i]');
							for (var b = 0; b < badges.length; b++) {
								try { if (badges[b].parentNode) badges[b].parentNode.removeChild(badges[b]); } catch (e2b) {}
							}
							name = String(clone.textContent || '').replace(/\s+/g, ' ').trim().slice(0, 80);
						}
					} catch (e2) {}
				}
				if (!name && btn) {
					try { name = (btn.getAttribute('title') || '').trim().slice(0, 80); } catch (e3) {}
				}
				return name;
			}
			window.__waAfkRowName = afkRowName;
			// Chat rows carrying an unread badge. Two list generations are
			// supported: the legacy rows (button[title] + unread-pill) and
			// the current cell-frame rows (gridcell[tabindex] to click,
			// icon-unread-count badge, titled name span for the name). The
			// badge alone is never enough — a click target AND a name are
			// both required, and the name is later verified against the
			// opened header before anything is typed.
			function findUnreadCandidates() {
				var out = [];
				try {
					var root = document.getElementById('pane-side') || document.body;
					if (!root) return out;
					var rows = root.querySelectorAll('div[role="row"]');
					for (var i = 0; i < rows.length && out.length < 5; i++) {
						var row = rows[i];
						var unread = null;
						try {
							unread = row.querySelector('[data-testid="icon-unread-count"], [data-testid="unread-pill"], [aria-label*="unread" i], [aria-label*="belum dibaca" i]');
						} catch (e) {}
						if (!unread) continue;
						var btn = row.querySelector('[role="button"][title]') ||
							row.querySelector('div[role="gridcell"]') ||
							row.querySelector('[role="button"], [tabindex="0"]');
						if (!btn) continue;
						var name = afkRowName(row, btn);
						if (!name) continue;
						out.push({ key: name, el: btn });
					}
				} catch (e) {}
				return out;
			}
			// Opened-chat name for the identity gate. The current header
			// carries a single [title] ("Profile details" avatar button)
			// and glues the bare name to icon ligatures + presence
			// ("kopikilo.idic-video…online"). Layer 1 reads a stripped
			// clone: only icon buttons, icon glyphs and titled blocks go
			// — role=button containers stay because the name itself may
			// live in one. Layer 2 (headerContainsKey below) covers a
			// stripped-away name via the raw text. An empty strip falls
			// back to the first titled node (legacy layout).
			function headerTitle() {
				try {
					var main = document.getElementById('main');
					if (!main) return '';
					var header = main.querySelector('header');
					if (!header) return '';
					try {
						var clone = header.cloneNode(true);
						var drop = clone.querySelectorAll('button, [data-icon], [title]');
						for (var d = 0; d < drop.length; d++) {
							try { if (drop[d].parentNode) drop[d].parentNode.removeChild(drop[d]); } catch (e2) {}
						}
						var txt = String(clone.textContent || '').replace(/\s+/g, ' ').trim().slice(0, 80);
						if (txt) return txt;
					} catch (e3) {}
					var t = header.querySelector('[title]');
					return t ? String(t.getAttribute('title') || '').trim().slice(0, 80) : '';
				} catch (e) {
					return '';
				}
			}
			// Layer 2: does the open conversation's raw header text
			// contain the candidate key verbatim? Short keys (<4) are
			// refused — a 2-letter key could hide inside icon ligatures.
			function headerContainsKey(key) {
				try {
					key = String(key || '').trim();
					if (key.length < 4) return false;
					var main = document.getElementById('main');
					var header = main && main.querySelector('header');
					if (!header) return false;
					var txt = String(header.textContent || '').replace(/\s+/g, ' ').trim();
					return txt.indexOf(key) !== -1;
				} catch (e) { return false; }
			}
			window.__waHeaderContainsKey = headerContainsKey;
			// Best-effort group detection. False positives only SKIP a
			// reply (safe); a miss still replies once per cooldown.
			function looksLikeGroup() {
				try {
					var main = document.getElementById('main');
					if (!main) return false;
					if (main.querySelector('header [data-testid="group-name"]')) return true;
					var header = main.querySelector('header');
					var txt = header ? String(header.innerText || header.textContent || '') : '';
					if (/\b(participants?|peserta)\b/i.test(txt)) return true;
					var lines = txt.split('\n');
					for (var i = 1; i < lines.length && i < 4; i++) {
						var ln = (lines[i] || '').trim();
						if (ln.indexOf(',') !== -1 && !/^\+?[\d(]/.test(ln)) return true;
					}
				} catch (e) {}
				return false;
			}
			// Row title vs opened header: exact match, or one contains the
			// other (WA occasionally decorates one side). Anything else is
			// a different chat — never type into it.
			function namesMatch(a, b) {
				a = String(a || '').replace(/\s+/g, ' ').trim();
				b = String(b || '').replace(/\s+/g, ' ').trim();
				if (!a || !b || a.length < 2 || b.length < 2) return false;
				if (a === b) return true;
				return a.indexOf(b) !== -1 || b.indexOf(a) !== -1;
			}
			function findAfkComposer() {
				// Footer-first, same as the scheduler: the header's
				// search-in-conversation box is contenteditable too and
				// would swallow the reply.
				var main = document.getElementById('main');
				if (!main) return null;
				var footer = main.querySelector('footer');
				if (footer) {
					var inFooter = footer.querySelector('div[contenteditable="true"][role="textbox"]') ||
						footer.querySelector('div[contenteditable="true"]') ||
						footer.querySelector('textarea');
					if (inFooter) return inFooter;
				}
				return main.querySelector('div[contenteditable="true"][role="textbox"]') ||
					main.querySelector('div[contenteditable="true"]') ||
					main.querySelector('textarea');
			}
			// Pure: why did the open-chat check find no reply box? When the
			// header already shows the target chat, the view itself offers
			// no composer (channel/broadcast/announcement) — skipping is
			// correct. Otherwise the list click never navigated anywhere.
			function afkNoComposerReason(opened, key) {
				if (namesMatch(opened, key)) return 'skipped (channel, no reply box)';
				return 'failed (no composer)';
			}
			window.__waAfkNoComposerReason = afkNoComposerReason;
			// Row activation for the current list DOM: a lone synthetic
			// click on the gridcell does not always navigate (verified in
			// the field — personal chats stayed closed), so send the full
			// press sequence plus keyboard activation, whatever the list
			// handlers listen for. Synthetic input only ever reaches the
			// row: our own send guards key off #main targets and real
			// send buttons, which this never touches.
			function activateAfkRow(el) {
				if (!el) return;
				try { if (el.focus) el.focus(); } catch (e) {}
				try {
					var opts = { bubbles: true, cancelable: true, view: window };
					if (window.MouseEvent) {
						el.dispatchEvent(new MouseEvent('pointerdown', opts));
						el.dispatchEvent(new MouseEvent('mousedown', opts));
						el.dispatchEvent(new MouseEvent('mouseup', opts));
					}
					el.click();
				} catch (e2) {
					try { el.click(); } catch (e3) {}
				}
				try {
					if (window.KeyboardEvent) {
						el.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', code: 'Enter', bubbles: true, cancelable: true }));
					}
				} catch (e4) {}
			}
			window.__waActivateAfkRow = activateAfkRow;
			// Trusted click through the CDP bridge: dispatches real
			// (isTrusted) mousePressed/mouseReleased at the row's center so
			// the list navigates where synthetic presses are ignored.
			// Fire-and-forget — the poll loop observes the result.
			// No-ops without the Windows bridge. Never runs while the
			// Settings panel is open (the tick defers first), so there is
			// nothing to hide from the hit-test. The expected chat key
			// travels along: at dispatch time the row's name is re-read
			// and the press is dropped when the list has re-sorted under
			// us (a neighbour row would otherwise open — the identity
			// gate would refuse it, but never opening is cleaner).
			function afkTrustedClick(el, key) {
				try {
					if (typeof window.__waCdp !== 'function') return false;
					if (typeof window.waCdpNative !== 'function') return false;
					if (!el || !el.getBoundingClientRect) return false;
					try { if (el.scrollIntoView) el.scrollIntoView({ block: 'nearest' }); } catch (e0) {}
					// The list may smooth-scroll after scrollIntoView (or
					// recycle the row node): re-measure at dispatch time and
					// re-validate badge AND name, so the press cannot land
					// on a recycled stranger row.
					setTimeout(function() {
						try {
							if (!el.isConnected) return;
							var rowNow = el.closest ? el.closest('div[role="row"]') : null;
							if (rowNow) {
								var still = null;
								try {
									still = rowNow.querySelector('[data-testid="icon-unread-count"], [data-testid="unread-pill"], [aria-label*="unread" i], [aria-label*="belum dibaca" i]');
								} catch (eb) {}
								if (!still) return;
								if (key && window.__waAfkRowName) {
									var stillKey = '';
									try { stillKey = window.__waAfkRowName(rowNow, el); } catch (en) {}
									if (!stillKey || stillKey !== key) return;
								}
							}
							var r = el.getBoundingClientRect();
							if (!r || !(r.width > 0) || !(r.height > 0)) return;
							var x = Math.round(r.left + r.width / 2);
							var y = Math.round(r.top + r.height / 2);
							if (!(x > 0 && y > 0)) return;
							var press = JSON.stringify({ type: 'mousePressed', x: x, y: y, button: 'left', clickCount: 1 });
							var release = JSON.stringify({ type: 'mouseReleased', x: x, y: y, button: 'left', clickCount: 1 });
							window.__waCdp('Input.dispatchMouseEvent', press, function() {
								window.__waCdp('Input.dispatchMouseEvent', release, function() {});
							});
						} catch (e) {}
					}, 350);
					return true;
				} catch (e) { return false; }
			}
			window.__waAfkTrustedClick = afkTrustedClick;
			// Manual send ends AFK mode (capture phase, guarded so the
			// routine's own send and a queued scheduler send never count;
			// an empty composer is a no-op keypress, not a sent message).
			document.addEventListener('keydown', function(e) {
				if (e.key !== 'Enter' || e.shiftKey || e.isComposing) return;
				if (sending || !afkArmed()) return;
				if (window.__schedInFlight && window.__schedInFlight()) return;
				var t = e.target;
				if (!t || !t.closest || !t.closest('#main')) return;
				var txt = '';
				try { txt = ((t.tagName === 'TEXTAREA' ? t.value : t.textContent) || '').trim(); } catch (e2) {}
				if (!txt) return;
				endAfkByUser();
			}, true);
			document.addEventListener('click', function(e) {
				if (sending || !afkArmed()) return;
				// The scheduler's OWN send click (other IIFE) must not be
				// mistaken for the user typing — that silently killed AFK
				// mode whenever a scheduled send fired while AFK was armed.
				if (window.__schedInFlight && window.__schedInFlight()) return;
				var t = e.target;
				if (!t || !t.closest) return;
				var btn = t.closest('button');
				if (!btn) return;
				if (btn.querySelector('[data-icon="send"]') || btn.getAttribute('data-tab') === 'send') {
					endAfkByUser();
				}
			}, true);
			function endAfkByUser() {
				var c = loadAfkCfg();
				if (!c.enabled) return;
				c.enabled = false;
				saveAfkCfg(c);
				recountAfkBusy();
				pushAfkLog('', 'off (you sent a message)');
				showFloatingToast('🤖 AFK auto-reply turned off — you sent a message');
			}
			function startAfkReply(cand, cfg) {
				var now = Date.now();
				// Claim BEFORE clicking: stamp the cooldown and an in-flight
				// marker so a reload or double tick can never start a second
				// attempt for the same chat. Failures do not retry within
				// the cooldown either — a broken DOM must not loop.
				var map = loadCooldowns();
				map[cand.key] = now;
				try { localStorage.setItem(LAST_KEY, JSON.stringify(map)); } catch (e) { return; }
				try { localStorage.setItem(INFLIGHT_KEY, JSON.stringify({ key: cand.key, at: now })); } catch (e) {}
				sending = true;
				recountAfkBusy();
				var attemptId = afkUid().slice(-4);
				var pollDone = false;
				var sendDone = false;
				var verifyDone = false;
				var finished = false;
				var waited = 0;
				var triedFallback = false;
				activateAfkRow(cand.el);
				afkTrustedClick(cand.el, cand.key);
				var timer = setInterval(function() {
					if (pollDone) return;
					waited += 400;
					var composer = findAfkComposer();
					if (!composer && waited < 9000) {
						// Second-chance activation halfway through: the row
						// itself, in case the cell swallowed the first press.
						if (!triedFallback && waited >= 2400) {
							triedFallback = true;
							try {
								var rowEl = (cand.el.closest) ? cand.el.closest('div[role="row"]') : null;
								if (rowEl && rowEl !== cand.el) {
									activateAfkRow(rowEl);
									afkTrustedClick(rowEl, cand.key);
								}
							} catch (e2) {}
						}
						return;
					}
					pollDone = true;
					clearInterval(timer);
					if (!composer) {
						// Name the failure: channel view vs dead click.
						finishAfkReply(afkNoComposerReason(headerTitle(), cand.key));
						return;
					}
					// Identity gate: only the chat we clicked may receive
					// the reply. Unverified or mismatched header aborts.
					// Two layers: parsed name match, else raw-text contains
					// (the stripped parse may drop a button-wrapped name).
					var opened = headerTitle();
					if (!opened) {
						finishAfkReply('failed (unverified)');
						return;
					}
					var nameOk = namesMatch(opened, cand.key);
					if (!nameOk && !headerContainsKey(cand.key)) {
						// Name the witness: the opened header text tells
						// apart a wrongly-opened chat from a misparsed one.
						finishAfkReply('failed (chat mismatch: saw "' + String(opened || '').slice(0, 40) + '")');
						return;
					}
					// {name} expands from the verified identity: the parsed
					// header when it matched, else the candidate key itself.
					var displayName = nameOk ? opened : cand.key;
					if (!cfg.groups && looksLikeGroup()) {
						finishAfkReply('skipped (group)');
						return;
					}
					// Same choke-point as the scheduler, right before we
					// type: owner kill (no leftover text in the composer),
					// send gap, per-page budget. Trips emergency on repeat.
					var guard = window.__waGuardSend ? window.__waGuardSend('afk') : { ok: true };
					if (!guard.ok) {
						finishAfkReply(guard.why === 'pace' || guard.why === 'cap' ?
							('blocked (send ' + guard.why + ')') : 'cancelled (emergency stop)');
						return;
					}
					var msg = expandAfkVars(cfg.text, displayName);
					// Insert may travel the trusted CDP channel
					// (asynchronous): the send stage continues from
					// beginAfkSend inside the insert callback.
					function beginAfkSend(insertPath) {
						if (!insertPath) {
							finishAfkReply('failed (insert)');
							return;
						}
						if (sendDone) return;
						sendDone = true;
						// Same send stage as the scheduler: wait out the
						// async MIC→SEND re-render, Enter as fallback, then
						// keep polling for a late button.
						window.__waSendWithWait(composer, 'afk');
						var checks = 0;
						var verify = setInterval(function() {
							if (verifyDone) return;
							checks++;
							var txt = '';
							try {
								txt = ((composer.tagName === 'TEXTAREA' ? composer.value : composer.textContent) || '').trim();
							} catch (e) {}
						if (txt === '' || checks >= 32) {
							verifyDone = true;
							clearInterval(verify);
							finishAfkReply(txt === '' ? 'sent' : 'failed (not delivered)');
							}
						}, 250);
					}
					window.__waInsertAsync(composer, msg, function(insertPath) {
						// The async gap lets the owner kill switch land
						// mid-flight: never send after a kill.
						var killed = false;
						try { killed = localStorage.getItem('wa_desk_sched_killed') === '1'; } catch (e) {}
						if (killed || finished) {
							finishAfkReply('cancelled (emergency stop)');
							return;
						}
						beginAfkSend(insertPath);
					});
				}, 400);
				function finishAfkReply(status) {
					if (finished) return;
					finished = true;
					try { localStorage.removeItem(INFLIGHT_KEY); } catch (e) {}
					if (status === 'sent') recordAfkCompletion();
					sending = false;
					recountAfkBusy();
					pushAfkLog(cand.key, status + ' [' + bootId + '/' + attemptId + ']');
					if (status === 'sent') showFloatingToast('🤖 AFK auto-replied to ' + cand.key);
				}
			}
			function afkTick() {
				if (sending || !afkArmed()) return;
				// Never yank the open conversation while the user is
				// configuring or actively using the app: defer the scan,
				// don't disarm. (The status line says exactly this.)
				try {
					if (document.getElementById('wa-settings-overlay')) return;
					if (Date.now() - lastUserActivityAt < 30000) return;
				} catch (e) {}
				// The scheduler owns the composer whenever a payload is
				// queued: never two senders at once.
				try { if (localStorage.getItem(SCHED_PENDING_KEY)) return; } catch (e) {}
				if (burstTripped()) {
					noteAfkBreaker();
					return;
				}
				var cands = findUnreadCandidates();
				if (!cands.length) return;
				var cfg = loadAfkCfg();
				var map = loadCooldowns();
				var now = Date.now();
				var cdMs = cfg.cooldownMin * 60000;
				for (var i = 0; i < cands.length; i++) {
					var lastAt = Number(map[cands[i].key]) || 0;
					if (now - lastAt < cdMs) continue;
					// Allowlist short-circuit (before any click): not ours,
					// claim the cooldown and note it once per window so the
					// log explains the silence instead of hiding it.
					if (!afkAllowed(cfg, cands[i].key)) {
						map[cands[i].key] = now;
						try { localStorage.setItem(LAST_KEY, JSON.stringify(map)); } catch (e2) {}
						pushAfkLog(cands[i].key, 'skipped (not in list)');
						continue;
					}
					startAfkReply(cands[i], cfg);
					return;
				}
			}
			// Deterministic entry point for the harness: one scan exactly
			// like the interval performs.
			window.__afkTick = afkTick;
			// Live diagnosis for the Settings status line (and the
			// harness): why the tick is idle right now. Silent gates —
			// cooldown, an already-read chat, a stuck scheduler payload —
			// otherwise leave zero trace and look "broken".
			window.__waAfkStatus = function() {
				var c = loadAfkCfg();
				var killed = false;
				try { killed = localStorage.getItem(KILLED_KEY) === '1'; } catch (e) {}
				var now = Date.now();
				var inWin = afkInWindow(c, now);
				var schedPending = false;
				try { schedPending = !!localStorage.getItem(SCHED_PENDING_KEY); } catch (e) {}
				var settingsOpen = false;
				var userActive = false;
				try {
					settingsOpen = !!document.getElementById('wa-settings-overlay');
					userActive = (now - lastUserActivityAt) < 30000;
				} catch (e2) {}
				var st = {
					armed: afkArmed(), enabled: !!c.enabled, killed: killed,
					hasText: !!c.text, useWindow: !!c.useWindow,
					startMin: c.startMin, endMin: c.endMin, inWindow: inWin,
					sending: sending, burst: burstTripped(),
					schedPending: schedPending,
					settingsOpen: settingsOpen, userActive: userActive,
					candidates: 0, cooldownSkips: 0, reason: '',
					allowOnly: !!c.allowOnly,
					allowedTotal: Array.isArray(c.allowed) ? c.allowed.length : 0,
					allowedHits: 0
				};
				if (st.armed) {
					var cands = findUnreadCandidates();
					st.candidates = cands.length;
					var map = loadCooldowns();
					var cdMs = (Number(c.cooldownMin) || 10) * 60000;
					for (var i = 0; i < cands.length; i++) {
						var lastAt = Number(map[cands[i].key]) || 0;
						if (now - lastAt < cdMs) st.cooldownSkips++;
						try { if (afkAllowed(c, cands[i].key)) st.allowedHits++; } catch (e3) {}
					}
				}
				var hm = window.__waAfkMinsToHM;
				var span = function(m) { try { return hm(m); } catch (e) { return String(m); } };
				if (!c.enabled) st.reason = 'off — enable AFK auto-reply';
				else if (!c.text) st.reason = 'off — reply message is empty';
				else if (killed) st.reason = 'halted — emergency stop is on';
				else if (!inWin) st.reason = 'idle — outside reply hours (' +
					span(c.startMin) + '–' + span(c.endMin) + ')';
				else if (sending) st.reason = 'working — a reply is being sent';
				else if (st.settingsOpen) st.reason = 'paused — settings panel is open (close it to resume)';
				else if (st.userActive) st.reason = 'paused — you were just active (resumes when idle)';
				else if (schedPending) st.reason = 'waiting — a scheduled send owns the composer';
				else if (st.burst) st.reason = 'paused — burst limit, resumes automatically';
				else if (st.allowOnly && st.candidates > 0 && st.allowedHits === 0) st.reason = 'idle — unread chats are not in the reply list (' +
					st.allowedTotal + ' listed)';
				else if (!st.candidates) st.reason = 'idle — no unread chats right now';
				else if (st.cooldownSkips >= st.candidates) st.reason = 'waiting — replied recently, cooldown per chat (' +
					(Number(c.cooldownMin) || 10) + ' min)';
				else st.reason = 'armed — will reply to the next unread chat';
				return st;
			};
			// True while an AFK reply attempt owns the composer; the
			// scheduler tick yields to this instead of navigating away
			// mid-attempt (and possibly killing it).
			window.__afkInFlight = function() { return sending; };
			// Chat-list probe for the "Diagnose scan" button: attribute
			// inventory only (titles/testids/aria-labels, truncated — no
			// message text), so a changed WhatsApp DOM shows up as counts
			// instead of a silent "no unread chats".
			window.__waAfkProbe = function() {
				var out = {
					probe: 'afk5',
					paneSide: false, rows: 0, pills: 0, ariaUnread: 0,
					ariaID: 0, titledButtons: 0, candidates: [],
					docTitle: '', samples: [], unreadEls: [], rowDetail: [],
					mainFound: false, mainFooter: false, mainEditables: 0,
					mainHeader: '', mainHeaderAll: [], mainHeaderText: '',
					mainHeaderHTML: '',
					appShell: [], appDeep: [], docFooters: 0, docEditables: 0,
					titleHTML: ''
				};
				// App skeleton: top-level layout children (structure only)
				// so a renamed conversation pane shows up. Document-wide
				// footer/editable counts tell whether ANY composer exists.
				try {
					var app = document.getElementById('app');
					if (app) {
						var kids = app.children;
						for (var k = 0; k < kids.length && k < 10; k++) {
							var kd = (kids[k].tagName || '?').toLowerCase();
							try {
								if (kids[k].id) kd += '#' + kids[k].id;
								if (kids[k].getAttribute('role')) kd += '[role=' + kids[k].getAttribute('role') + ']';
							} catch (ek) {}
							out.appShell.push(kd);
						}
					}
					out.docFooters = document.querySelectorAll('footer').length;
					out.docEditables = document.querySelectorAll('div[contenteditable="true"], textarea').length;
					// One level deeper than appShell: where the conversation
					// pane mounts when a chat opens.
					try {
						var kid0 = app && app.children && app.children[0];
						if (kid0 && kid0.children) {
							for (var g = 0; g < kid0.children.length && g < 10; g++) {
								var gd = (kid0.children[g].tagName || '?').toLowerCase();
								try {
									if (kid0.children[g].id) gd += '#' + kid0.children[g].id;
									if (kid0.children[g].getAttribute('role')) gd += '[role=' + kid0.children[g].getAttribute('role') + ']';
								} catch (eg) {}
								out.appDeep.push(gd);
							}
						}
					} catch (eg2) {}
				} catch (esk) {}
				try { out.docTitle = String(document.title || '').slice(0, 60); } catch (e) {}
				try {
					var root = document.getElementById('pane-side');
					out.paneSide = !!root;
					if (!root) root = document.body;
					if (!root) return out;
					var rows = root.querySelectorAll('div[role="row"]');
					out.rows = rows.length;
					out.pills = root.querySelectorAll('[data-testid="unread-pill"]').length;
					try { out.ariaUnread = root.querySelectorAll('[aria-label*="unread" i]').length; } catch (e) {}
					try { out.ariaID = root.querySelectorAll('[aria-label*="belum dibaca" i]').length; } catch (e) {}
					out.titledButtons = root.querySelectorAll('[role="button"][title]').length;
					var cands = findUnreadCandidates();
					for (var i = 0; i < cands.length; i++) out.candidates.push(cands[i].key);
					// Deep section: every aria-unread badge (capped) with its
					// tag/testid/label, its row index, and a short ancestor
					// path — tells us what the unread marker looks like now.
					try {
						var un = root.querySelectorAll('[aria-label*="unread" i]');
						var allRows = root.querySelectorAll('div[role="row"]');
						var rowIdxOf = function(el) {
							for (var r = 0; r < allRows.length; r++) {
								try { if (allRows[r].contains(el)) return r; } catch (e) {}
							}
							return -1;
						};
						for (var u = 0; u < un.length && u < 8; u++) {
							var el = un[u];
							var path = [];
							try {
								var p = el;
								for (var d = 0; d < 3 && p && p !== root; d++) {
									var bits = (p.tagName || '?').toLowerCase();
									try {
										if (p.getAttribute('role')) bits += '[role=' + p.getAttribute('role') + ']';
										if (p.getAttribute('data-testid')) bits += '[tid=' + String(p.getAttribute('data-testid')).slice(0, 32) + ']';
									} catch (e2) {}
									path.push(bits);
									p = p.parentElement;
								}
							} catch (e3) {}
							var desc = { tag: '', testid: '', aria: '', row: -1, path: path };
							try {
								desc.tag = (el.tagName || '').toLowerCase();
								desc.testid = String(el.getAttribute('data-testid') || '').slice(0, 40);
								desc.aria = String(el.getAttribute('aria-label') || '').slice(0, 60);
								desc.row = rowIdxOf(el);
							} catch (e4) {}
							out.unreadEls.push(desc);
						}
					} catch (e5) {}
					// Deep section: per-row clickable inventory + name
					// container presence (text length only, never content).
					try {
						var rowList = root.querySelectorAll('div[role="row"]');
						for (var j2 = 0; j2 < rowList.length && j2 < 3; j2++) {
							var row = rowList[j2];
							var rinfo = { rowAttrs: '', clickables: [], nameLen: -1, titleTidCount: 0 };
							try {
								var ra = [];
								if (row.getAttribute('tabindex') !== null) ra.push('tabindex=' + row.getAttribute('tabindex'));
								if (row.getAttribute('aria-selected') !== null) ra.push('aria-selected=' + row.getAttribute('aria-selected'));
								if (row.getAttribute('aria-label')) ra.push('aria-label=' + String(row.getAttribute('aria-label')).slice(0, 40));
								rinfo.rowAttrs = ra.join(' ') || '(none)';
							} catch (e6) {}
							try {
								var cl = row.querySelectorAll('button, a, [role="button"], [tabindex], [data-action]');
								for (var c2 = 0; c2 < cl.length && c2 < 6; c2++) {
									var cb = [];
									try {
										cb.push((cl[c2].tagName || '?').toLowerCase());
										if (cl[c2].getAttribute('role')) cb.push('role=' + cl[c2].getAttribute('role'));
										if (cl[c2].getAttribute('tabindex') !== null) cb.push('tabindex=' + cl[c2].getAttribute('tabindex'));
										if (cl[c2].getAttribute('title')) cb.push('title=' + String(cl[c2].getAttribute('title')).slice(0, 30));
										if (cl[c2].getAttribute('aria-label')) cb.push('aria=' + String(cl[c2].getAttribute('aria-label')).slice(0, 30));
										if (cl[c2].getAttribute('data-testid')) cb.push('tid=' + String(cl[c2].getAttribute('data-testid')).slice(0, 30));
									} catch (e7) {}
									rinfo.clickables.push(cb.join(' '));
								}
							} catch (e8) {}
							try {
								var nm = row.querySelector('[data-testid="cell-frame-title"]');
								if (nm) {
									rinfo.nameLen = String(nm.textContent || '').trim().length;
									rinfo.titleTidCount = row.querySelectorAll('[data-testid="cell-frame-title"]').length;
								}
							} catch (e9) {}
							out.rowDetail.push(rinfo);
						}
					} catch (e10) {}
				// Raw title markup of the first unread row (truncated):
				// shows exactly where the badge text hides so the name
				// extractor can strip it precisely.
				try {
					var urows = root.querySelectorAll('div[role="row"]');
					for (var h = 0; h < urows.length; h++) {
						try {
							if (!urows[h].querySelector('[data-testid="icon-unread-count"]')) continue;
							var th = urows[h].querySelector('[data-testid="cell-frame-title"]');
							if (th) out.titleHTML = String(th.innerHTML || '').slice(0, 400);
							break;
						} catch (eh) {}
					}
				} catch (e11b) {}
				// Main panel state: distinguishes "click opened a view with
				// no reply box" (channel/broadcast) from "click navigated
				// nowhere" — both surface as no-composer otherwise.
				try {
					var main = document.getElementById('main');
					out.mainFound = !!main;
					if (main) {
						out.mainFooter = !!main.querySelector('footer');
						out.mainEditables = main.querySelectorAll('div[contenteditable="true"], textarea').length;
						var mh = main.querySelector('header [title]');
						if (mh) out.mainHeader = String(mh.getAttribute('title') || '').slice(0, 40);
						// Every titled node in the conversation header (the
						// first [title] is "Profile details" now — the real
						// chat name lives elsewhere) plus the header's own
						// text sample.
						try {
							var mhh = main.querySelectorAll('header [title]');
							for (var hi = 0; hi < mhh.length && hi < 8; hi++) {
								try {
									out.mainHeaderAll.push((mhh[hi].tagName || '?').toLowerCase() +
										'[title="' + String(mhh[hi].getAttribute('title')).slice(0, 40) + '"]');
								} catch (ehh) {}
							}
						} catch (eh2) {}
						try {
							var mhdr = main.querySelector('header');
							if (mhdr) {
								out.mainHeaderText = String(mhdr.textContent || '').replace(/\s+/g, ' ').trim().slice(0, 120);
								out.mainHeaderHTML = String(mhdr.innerHTML || '').slice(0, 600);
							}
						} catch (eh3) {}
					}
				} catch (e11) {}
					for (var j = 0; j < rows.length && j < 3; j++) {
						var btns = rows[j].querySelectorAll('[role="button"]');
						var inv = [];
						for (var k = 0; k < btns.length && k < 4; k++) {
							var names = [];
							try {
								var at = btns[k].attributes;
								for (var a = 0; a < at.length; a++) {
									var n = at[a].name;
									if (n === 'title' || n === 'aria-label' || n.indexOf('data-testid') === 0) {
										names.push(n + '=' + String(at[a].value).slice(0, 40));
									} else {
										names.push(n);
									}
								}
							} catch (e2) {}
							inv.push('button[' + names.join(' ') + ']');
						}
						var tids = [];
						try {
							var marked = rows[j].querySelectorAll('[data-testid]');
							for (var t = 0; t < marked.length && t < 6; t++) {
								tids.push(String(marked[t].getAttribute('data-testid')).slice(0, 40));
							}
						} catch (e3) {}
						out.samples.push({ buttons: inv, testids: tids });
					}
				} catch (e) {
					try { out.error = String((e && e.message) || e).slice(0, 120); } catch (e2) {}
				}
				return out;
			};
			// Boot recovery: an in-flight marker found at startup means the
			// previous page died mid-reply. Log it as interrupted once (if
			// fresh) and clear it; the cooldown was already claimed.
			try {
				var infRaw = localStorage.getItem(INFLIGHT_KEY);
				if (infRaw) {
					var inf = JSON.parse(infRaw);
					if (Date.now() - (Number(inf && inf.at) || 0) < 120000) {
						pushAfkLog(inf && inf.key, 'failed (interrupted)');
					}
					localStorage.removeItem(INFLIGHT_KEY);
				}
			} catch (e) {}
			recountAfkBusy();
			setInterval(afkTick, TICK_MS);
		})();

		// In-App Auto Updater UI and Handlers
		(function() {
			window.showUpdateBanner = function(latestVersion, releaseTitle, downloadUrl) {
				if (document.getElementById('wa-update-banner')) return;
				if (sessionStorage.getItem('dismissed_update_' + latestVersion) === 'true') return;

				if (!document.getElementById('wa-update-anim')) {
					var animStyle = document.createElement('style');
					animStyle.id = 'wa-update-anim';
					animStyle.textContent = '@keyframes waSlideDown { from { transform: translateY(-100%); opacity: 0; } to { transform: translateY(0); opacity: 1; } }' +
						// The banner is fixed-overlay, so the app root yields
						// its height back while the banner is visible. Without
						// this the banner covers WhatsApp's own top header
						// (Archived row, back button) instead of pushing it down.
						'html.wa-update-visible #app { height: calc(100% - var(--wa-update-banner-height, 0px)) !important; margin-top: var(--wa-update-banner-height, 0px) !important; }' +
						'#wa-btn-update:hover { background: #029070 !important; transform: translateY(-1px); }' +
						'#wa-btn-dismiss:hover { color: #e9edef !important; }';
					document.head.appendChild(animStyle);
				}

				var banner = document.createElement('div');
				banner.id = 'wa-update-banner';
				banner.style.cssText = 'position:fixed;top:0;left:0;right:0;background:rgba(17,27,33,0.97);backdrop-filter:blur(14px);-webkit-backdrop-filter:blur(14px);border-bottom:1px solid rgba(0,168,132,0.35);padding:9px 18px;display:flex;align-items:center;justify-content:space-between;gap:12px;z-index:9999998;box-shadow:0 6px 24px rgba(0,0,0,0.6);font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,Helvetica,Arial,sans-serif;color:#e9edef;font-size:13px;animation:waSlideDown 0.25s cubic-bezier(0.16,1,0.3,1);';

				var leftWrap = document.createElement('div');
				leftWrap.style.cssText = 'display:flex;align-items:center;gap:10px;min-width:0;flex:1;';

				var badge = document.createElement('span');
				badge.style.cssText = 'background:rgba(0,168,132,0.15);color:#00a884;border:1px solid rgba(0,168,132,0.35);padding:2px 8px;border-radius:12px;font-size:11px;font-weight:600;letter-spacing:0.3px;flex-shrink:0;';
				badge.textContent = 'v' + latestVersion;

				var msg = document.createElement('span');
				msg.id = 'wa-update-text';
				msg.style.cssText = 'white-space:nowrap;overflow:hidden;text-overflow:ellipsis;font-size:12.5px;color:#d1d7db;';
				var titleText = releaseTitle ? releaseTitle : ('WAtchful v' + latestVersion);
				msg.innerHTML = 'Update available: <strong style="color:#e9edef;">' + escapeHtml(titleText) + '</strong>';

				leftWrap.appendChild(badge);
				leftWrap.appendChild(msg);

				var rightWrap = document.createElement('div');
				rightWrap.style.cssText = 'display:flex;align-items:center;gap:8px;flex-shrink:0;';

				var actionsDiv = document.createElement('div');
				actionsDiv.id = 'wa-update-actions';
				actionsDiv.style.cssText = 'display:flex;align-items:center;gap:8px;';

				var btnUpdate = document.createElement('button');
				btnUpdate.id = 'wa-btn-update';
				btnUpdate.textContent = 'Update Now';
				btnUpdate.style.cssText = 'background:#00a884;color:#111b21;border:none;padding:5px 14px;border-radius:14px;font-size:12px;font-weight:600;cursor:pointer;outline:none;transition:all 0.15s ease;box-shadow:0 2px 8px rgba(0,168,132,0.3);';

				var btnDismiss = document.createElement('button');
				btnDismiss.id = 'wa-btn-dismiss';
				btnDismiss.textContent = 'Later';
				btnDismiss.style.cssText = 'background:transparent;color:#8696a0;border:none;padding:5px 10px;border-radius:14px;font-size:12px;cursor:pointer;outline:none;transition:color 0.15s ease;';

				actionsDiv.appendChild(btnUpdate);
				actionsDiv.appendChild(btnDismiss);

				var progressWrap = document.createElement('div');
				progressWrap.id = 'wa-update-progress-wrap';
				progressWrap.style.cssText = 'display:none;align-items:center;gap:10px;';

				var barTrack = document.createElement('div');
				barTrack.style.cssText = 'width:130px;height:6px;background:rgba(255,255,255,0.12);border-radius:3px;overflow:hidden;';

				var barFill = document.createElement('div');
				barFill.id = 'wa-update-progress-bar';
				barFill.style.cssText = 'width:0%;height:100%;background:#00a884;border-radius:3px;transition:width 0.18s ease;';
				barTrack.appendChild(barFill);

				var pctLabel = document.createElement('span');
				pctLabel.id = 'wa-update-progress-pct';
				pctLabel.style.cssText = 'font-size:11.5px;color:#00a884;font-weight:600;min-width:32px;text-align:right;';
				pctLabel.textContent = '0%';

				progressWrap.appendChild(barTrack);
				progressWrap.appendChild(pctLabel);

				rightWrap.appendChild(actionsDiv);
				rightWrap.appendChild(progressWrap);

				banner.appendChild(leftWrap);
				banner.appendChild(rightWrap);
				var bannerResizeObserver = null;
				var bannerParent = document.body || document.documentElement;
				if (bannerParent) {
					bannerParent.appendChild(banner);
					// Track the live banner height (it grows when the
					// download progress row appears) and yield that space
					// from the app root. Disconnect on dismiss so no
					// observer outlives the banner.
					var layoutRoot = document.documentElement;
					var syncBannerLayout = function() {
						if (!banner.isConnected || !layoutRoot) return;
						layoutRoot.style.setProperty('--wa-update-banner-height', banner.offsetHeight + 'px');
						layoutRoot.classList.add('wa-update-visible');
					};
					syncBannerLayout();
					if (window.ResizeObserver) {
						bannerResizeObserver = new ResizeObserver(syncBannerLayout);
						bannerResizeObserver.observe(banner);
					}
				}

				try {
					if ((!window.isNotificationsEnabled || window.isNotificationsEnabled()) && window.sendNativeNotification) {
						var notifTitle = 'Update Available';
						var notifBody = 'WAtchful v' + latestVersion + ' is available. Click to update the application.';
						window.sendNativeNotification(notifTitle, notifBody);
					}
				} catch (e) {}

				btnUpdate.onclick = function() {
					if (!downloadUrl && window.triggerCheckForUpdate) {
						window.triggerCheckForUpdate();
						return;
					}
					actionsDiv.style.display = 'none';
					progressWrap.style.display = 'flex';
					msg.textContent = 'Downloading update package...';
					if (window.startUpdateNative) {
						window.startUpdateNative(downloadUrl);
					}
				};

				btnDismiss.onclick = function() {
					sessionStorage.setItem('dismissed_update_' + latestVersion, 'true');
					if (bannerResizeObserver) {
						bannerResizeObserver.disconnect();
						bannerResizeObserver = null;
					}
					if (banner.parentNode) {
						banner.parentNode.removeChild(banner);
					}
					document.documentElement.classList.remove('wa-update-visible');
					document.documentElement.style.removeProperty('--wa-update-banner-height');
				};
			};

			window.onUpdateProgress = function(pct) {
				var bar = document.getElementById('wa-update-progress-bar');
				var label = document.getElementById('wa-update-progress-pct');
				if (bar) bar.style.width = pct + '%';
				if (label) label.textContent = pct + '%';
			};

			window.onUpdateStatus = function(statusMsg) {
				var msg = document.getElementById('wa-update-text');
				if (msg) msg.textContent = statusMsg;
			};

			window.onUpdateError = function(errMsg) {
				var actions = document.getElementById('wa-update-actions');
				var prog = document.getElementById('wa-update-progress-wrap');
				var msg = document.getElementById('wa-update-text');
				if (actions) actions.style.display = 'flex';
				if (prog) prog.style.display = 'none';
				if (msg) msg.textContent = 'Update available';
				showFloatingToast('❌ Failed to update: ' + errMsg);
			};

			// Manual Check Function and Shortcut (Cmd/Ctrl + Shift + U)
			window.triggerCheckForUpdate = function() {
				showFloatingToast('🔍 Checking for updates...');
				if (window.checkForUpdateNative) {
					return window.checkForUpdateNative(true).then(function(res) {
						if (res && res.available) {
							window.showUpdateBanner(res.latest_version, res.release_title, res.download_url);
						} else if (res && res.check_error) {
							showFloatingToast('⚠️ Update check failed: ' + res.check_error);
						} else {
							var cur = (res && res.current_version) ? res.current_version : '__WA_APP_VERSION__';
							showFloatingToast('✅ WAtchful is up to date (v' + cur + ')');
						}
						return res;
					}).catch(function() {
						showFloatingToast('⚠️ Unable to check for updates at this time.');
					});
				}
				return Promise.resolve(null);
			};

			window.addEventListener('keydown', function(e) {
				if ((e.metaKey || e.ctrlKey) && e.shiftKey && (e.key === 'u' || e.key === 'U')) {
					e.preventDefault();
					window.triggerCheckForUpdate();
				}
			});

			// Direct Chat shortcut (Ctrl/Cmd+Shift+C): browser accelerators
			// stay disabled, so this never collides with Inspect Element.
			window.addEventListener('keydown', function(e) {
				if ((e.metaKey || e.ctrlKey) && e.shiftKey && (e.key === 'c' || e.key === 'C')) {
					e.preventDefault();
					if (window.openDirectChatModal) window.openDirectChatModal();
				}
			});
		})();

		// Dynamic Responsive Desktop Layout (enables seamless shrinking and expanding)
		(function() {
			var respStyle = document.createElement('style');
			respStyle.id = 'watchful-responsive';
			respStyle.textContent = '' +
				'html, body, #app { width: 100% !important; height: 100% !important; min-width: 0 !important; overflow: hidden !important; -webkit-font-smoothing: antialiased; }' +
				'#app > div, #app .two { width: 100% !important; height: 100% !important; min-width: 0 !important; max-width: 100% !important; top: 0 !important; margin: 0 !important; border-radius: 0 !important; }' +
				'[data-testid="status-v3"] { min-width: 0 !important; width: 100% !important; height: 100% !important; }' +
				'@media screen and (min-width: 641px) {' +
				'  #pane-side, div[data-testid="chat-list"] { min-width: 200px !important; -webkit-overflow-scrolling: touch !important; }' +
				'  #main { min-width: 240px !important; -webkit-overflow-scrolling: touch !important; }' +
				'}' +
				'@media screen and (max-width: 640px) {' +
				'  #pane-side, div[data-testid="chat-list"], #main { min-width: 0 !important; }' +
				'}';

			// Once <head> exists the style never needs re-injection, so poll only
			// via a cheap head observer instead of an endless 2.5s interval.
			function injectResponsive() {
				if (document.head && !document.getElementById('watchful-responsive')) {
					document.head.appendChild(respStyle);
					observer.disconnect();
				}
			}
			var observer = new MutationObserver(injectResponsive);
			function watchHead() {
				// Init scripts run before the parser creates <head>/<html>,
				// so never observe a null root: that throws and aborts every
				// enhancement defined after this block (Settings, Profiles,
				// shortcuts). Defer instead, like the other observers.
				if (document.head) {
					injectResponsive();
					return;
				}
				if (document.documentElement) {
					observer.observe(document.documentElement, { childList: true });
				} else {
					document.addEventListener('DOMContentLoaded', injectResponsive, { once: true });
				}
			}
			watchHead();
			document.addEventListener('DOMContentLoaded', function() {
				injectResponsive();
				observer.disconnect();
			}, { once: true });
		})();

		// Native Spell Check for textareas (macOS NSSpellChecker, Windows ISpellCheckProvider, Linux GTK)
		(function() {
			var spellCheckEnabled = true;
			var spellCheckLang = 'auto';

			function applySpellCheck(el) {
				if (!el || el.nodeType !== 1 || el.dataset.spellCheckInitialized) return;
				el.dataset.spellCheckInitialized = 'true';
				el.spellcheck = spellCheckEnabled;
				if (spellCheckLang !== 'auto') {
					el.lang = spellCheckLang;
				}
			}

			function enableSpellCheckOnTextareas(scope) {
				var selector = 'textarea[contenteditable="true"], div[contenteditable="true"][role="textbox"], textarea';
				var root = (scope && scope.querySelectorAll) ? scope : document;
				try {
					if (scope && scope.matches && scope.matches(selector)) applySpellCheck(scope);
					var textareas = root.querySelectorAll(selector);
					for (var i = 0; i < textareas.length; i++) applySpellCheck(textareas[i]);
				} catch (e) {}
			}

			function initSpellCheck() {
				// Initial enable
				enableSpellCheckOnTextareas();

				// Watch for new textareas (WhatsApp Web is SPA). The chat list is
				// virtualized, so scrolling continuously adds and removes rows. The old
				// version ran a document-wide querySelectorAll for every single added
				// node, which is what made long chat-list scrolls stutter. Coalesce to
				// one pass per animation frame and only for subtrees that can hold
				// editable text.
				var spellCheckScheduled = false;
				var pendingSpellRoots = [];
				function spellCheckRelevant(node) {
					if (!node || node.nodeType !== 1) return false;
					try {
						return !!(node.matches && node.matches('[contenteditable="true"], textarea')) ||
							!!(node.querySelector && node.querySelector('[contenteditable="true"], textarea'));
					} catch (e) {
						return false;
					}
				}
				function runSpellCheckScan() {
					spellCheckScheduled = false;
					var roots = pendingSpellRoots.splice(0, pendingSpellRoots.length);
					if (shouldPauseBackgroundWork()) return;
					for (var r = 0; r < roots.length; r++) enableSpellCheckOnTextareas(roots[r]);
				}
				function scheduleSpellCheck() {
					if (spellCheckScheduled || pendingSpellRoots.length === 0) return;
					spellCheckScheduled = true;
					requestAnimationFrame(runSpellCheckScan);
				}
				var observer = new MutationObserver(function(mutations) {
					if (shouldPauseBackgroundWork()) return;
					for (var i = 0; i < mutations.length && pendingSpellRoots.length < 12; i++) {
						var added = mutations[i].addedNodes;
						for (var j = 0; j < added.length && pendingSpellRoots.length < 12; j++) {
							if (spellCheckRelevant(added[j])) pendingSpellRoots.push(added[j]);
						}
					}
				scheduleSpellCheck();
			});
			function initSpellCheckObserver() {
				// Same document-start hazard as above: document.body is still
				// null when the init script runs, so defer rather than throw.
				var target = document.body || document.documentElement;
				if (target) {
					observer.observe(target, { childList: true, subtree: true });
				} else {
					document.addEventListener('DOMContentLoaded', initSpellCheckObserver, { once: true });
				}
			}
			initSpellCheckObserver();

				// Also re-check on navigation
				var lastUrl = location.href;
				setInterval(function() {
					if (location.href !== lastUrl) {
						lastUrl = location.href;
						setTimeout(enableSpellCheckOnTextareas, 300);
					}
				}, 1000);
			}

			// Expose toggle for settings
			window.toggleSpellCheck = function(enabled) {
				spellCheckEnabled = !!enabled;
				enableSpellCheckOnTextareas();
				if (window.setSpellCheckEnabledNative) {
					window.setSpellCheckEnabledNative(spellCheckEnabled);
				}
			};

			window.setSpellCheckLanguage = function(lang) {
				spellCheckLang = lang;
				enableSpellCheckOnTextareas();
			};

			if (document.readyState === 'loading') {
				document.addEventListener('DOMContentLoaded', initSpellCheck);
			} else {
				initSpellCheck();
			}
		})();

		// Context Menu: Search/Translate selected text
		(function() {
			var contextMenu = null;
			var lastSelection = '';
			var lastSelectionRect = null;

			function createContextMenu() {
				if (contextMenu) return;
				contextMenu = document.createElement('div');
				contextMenu.id = 'wa-context-menu';
				contextMenu.style.cssText = 'position:fixed;z-index:9999999;background:#202c33;border:1px solid #2a3942;border-radius:8px;padding:6px 0;box-shadow:0 8px 24px rgba(0,0,0,0.4);min-width:180px;font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",system-ui,sans-serif;font-size:13px;color:#e9edef;';
				contextMenu.innerHTML = '' +
					'<div class="wa-cm-item" data-action="search" style="padding:8px 16px;cursor:pointer;display:flex;align-items:center;gap:10px;">' +
					'  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="color:#00a884;"><circle cx="11" cy="11" r="8"></circle><line x1="21" y1="21" x2="16.65" y2="16.65"></line></svg>' +
					'  <span>Search on Google</span>' +
					'</div>' +
					'<div class="wa-cm-item" data-action="translate" style="padding:8px 16px;cursor:pointer;display:flex;align-items:center;gap:10px;">' +
					'  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="color:#00a884;"><path d="M21 11.5a8.38 8.38 0 0 1-.9 3.8 8.5 8.5 0 0 1-7.6 4.7 8.38 8.38 0 0 1-3.8-.9L3 21l1.9-5.7a8.38 8.38 0 0 1-.9-3.8 8.5 8.5 0 0 1 4.7-7.6 8.38 8.38 0 0 1 3.8.9h.5a8.48 8.48 0 0 1 8 8v.5z"></path><line x1="12" y1="12" x2="12" y2="12"></line></svg>' +
					'  <span>Translate</span>' +
					'</div>' +
					'<hr style="margin:6px 8px;border:none;border-top:1px solid #2a3942;">' +
					'<div class="wa-cm-item" data-action="copy" style="padding:8px 16px;cursor:pointer;display:flex;align-items:center;gap:10px;">' +
					'  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="color:#8696a0;"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path></svg>' +
					'  <span>Copy</span>' +
					'</div>';
				document.body.appendChild(contextMenu);

				contextMenu.querySelectorAll('.wa-cm-item').forEach(function(item) {
					item.addEventListener('mouseenter', function() {
						this.style.background = '#2a3942';
					});
					item.addEventListener('mouseleave', function() {
						this.style.background = 'transparent';
					});
					item.addEventListener('click', function() {
						var action = this.dataset.action;
						handleContextAction(action);
						hideContextMenu();
					});
				});

				document.addEventListener('click', hideContextMenu, true);
				document.addEventListener('scroll', hideContextMenu, true);
			}

			function showContextMenu(x, y, text) {
				createContextMenu();
				lastSelection = text;
				contextMenu.style.left = x + 'px';
				contextMenu.style.top = y + 'px';
				contextMenu.style.display = 'block';
			}

			function hideContextMenu() {
				if (contextMenu) {
					contextMenu.style.display = 'none';
				}
			}

			function handleContextAction(action) {
				if (!lastSelection) return;
				var encoded = encodeURIComponent(lastSelection);
				if (action === 'search') {
					window.openExternalLink && window.openExternalLink('https://www.google.com/search?q=' + encoded);
				} else if (action === 'translate') {
					window.openExternalLink && window.openExternalLink('https://translate.google.com/?sl=auto&tl=id&text=' + encoded + '&op=translate');
				} else if (action === 'copy') {
					navigator.clipboard.writeText(lastSelection).then(function() {
						if (window.showFloatingToast) window.showFloatingToast('📋 Copied to clipboard');
					});
				}
			}

			function getSelectedText() {
				var selection = window.getSelection();
				if (!selection || selection.rangeCount === 0) return '';
				var text = selection.toString().trim();
				return text.length > 0 && text.length < 500 ? text : '';
			}

			function onContextMenu(e) {
				var text = getSelectedText();
				if (text) {
					e.preventDefault();
					showContextMenu(e.clientX, e.clientY, text);
				}
			}

			document.addEventListener('contextmenu', onContextMenu, true);

			// Also show on long-press for touch devices
			var longPressTimer = null;
			document.addEventListener('touchstart', function(e) {
				var text = getSelectedText();
				if (text) {
					longPressTimer = setTimeout(function() {
						var touch = e.touches[0];
						showContextMenu(touch.clientX, touch.clientY, text);
					}, 500);
				}
			}, { passive: true });
			document.addEventListener('touchend', function() {
				if (longPressTimer) clearTimeout(longPressTimer);
			});
			document.addEventListener('touchmove', function() {
				if (longPressTimer) clearTimeout(longPressTimer);
			});
		})();

		// Automatic Download & Document Preview Interceptor for Chat Files & Media
		(function() {
			var activeDownloadKeys = Object.create(null);

			function downloadRequestKey(href, filename) {
				return String(filename || '') + '\n' + String(href || '');
			}

			function releaseDownloadRequest(requestKey, immediately) {
					delete activeDownloadKeys[requestKey];
					// Every release path is an exit of an in-flight download:
					// clear the recycler gate (clamped at zero on the host side).
					if (window.__waReportBusy) window.__waReportBusy('download', false);
				}

			// Toast action: opens the downloads folder in Finder/Explorer.
			function openFolderAction() {
				if (!window.openDownloadDirNative) return null;
				return {
					label: 'Open folder',
					onClick: function() { window.openDownloadDirNative(); }
				};
			}				function markDownloadComplete(requestKey, savedPath) {						var completedRequest = { status: 'complete', savedPath: savedPath };
						activeDownloadKeys[requestKey] = completedRequest;
						// Save finished: bytes are on disk. Clear the recycler gate;
						// the key lingers only for in-flight coalescing below.
						if (window.__waReportBusy) window.__waReportBusy('download', false);
				// Tell the badge layer this filename is now on disk so the next
				// scan badges it without a redundant native stat.
				var savedBase = (savedPath || '').split(/[\\/]/).pop();
				if (savedBase && window.__waMarkSaved) window.__waMarkSaved(savedBase);
				// Retain only the tiny path entry, never the Blob or base64 payload.
				setTimeout(function() {
					if (activeDownloadKeys[requestKey] === completedRequest) {
						delete activeDownloadKeys[requestKey];
					}
				}, 300000);
			}

			function isDocumentFileName(name) {
				if (!name) return false;
				var ext = name.toLowerCase();
				return ext.endsWith('.pdf') || ext.endsWith('.doc') || ext.endsWith('.docx') ||
					   ext.endsWith('.xls') || ext.endsWith('.xlsx') || ext.endsWith('.ppt') ||
					   ext.endsWith('.pptx') || ext.endsWith('.txt') || ext.endsWith('.csv') ||
					   ext.endsWith('.rtf');
			}

			function captureDownload(href, filename, shouldAutoOpen) {
				if (!filename) filename = 'whatsapp_file';
				var isDoc = isDocumentFileName(filename);
				if (shouldAutoOpen === undefined) {
					shouldAutoOpen = isDoc;
				}
				var requestKey = downloadRequestKey(href, filename);
				var existingRequest = activeDownloadKeys[requestKey];
				if (existingRequest && existingRequest.status === 'downloading') {
					return;
				}
				activeDownloadKeys[requestKey] = { status: 'downloading' };
				// The renderer recycler must not rebuild while bytes are in
				// flight: the fetch would die with the old renderer.
				if (window.__waReportBusy) window.__waReportBusy('download', true);
				showDownloadToast(isDoc ? ('📄 Opening preview: ' + filename + '...') : ('⏳ Downloading: ' + filename + '...'));

				fetch(href)
					.then(function(response) {
						return response.blob();
					})
					.then(function(blob) {
						// Duplicate clicks (second click, viewer button) simply
						// save again: the native saver refuses byte-identical
						// duplicates by SHA-256 content hash and returns the
						// existing path, so no "name (1).ext" copy is created.
						var isPdf = filename.toLowerCase().endsWith('.pdf');
						var previewBlob = isPdf ? blob.slice(0, blob.size, 'application/pdf') : blob;
						var ownedBlobUrl = isPdf ? origCreateObjectURL(previewBlob) : '';
						var reader = new FileReader();
						reader.onloadend = function() {
							var base64data = reader.result;
							if (window.saveDownloadedFileNative) {
								window.saveDownloadedFileNative(filename, base64data).then(function(res) {
									var savedPath = res && res.path;
									if (savedPath) {
										var alreadySaved = !!(res && res.alreadyExisted);
										markDownloadComplete(requestKey, savedPath);
										if (shouldAutoOpen) {
											showInAppDocModal(filename, ownedBlobUrl || href, savedPath, base64data, ownedBlobUrl);
											if (window.dismissStuckViewer) window.dismissStuckViewer();
											showDownloadToast((alreadySaved ? '📄 Already saved: ' : '📄 Preview opened: ') + filename, openFolderAction());
										} else {
											showDownloadToast((alreadySaved ? '💾 File already saved: ' : '💾 Saved successfully: ') + filename, openFolderAction());
										}
									} else {
										if (ownedBlobUrl) URL.revokeObjectURL(ownedBlobUrl);
										showFloatingToast('❌ Failed to save file.');
										releaseDownloadRequest(requestKey);
									}
								}).catch(function() {
									if (ownedBlobUrl) URL.revokeObjectURL(ownedBlobUrl);
									showFloatingToast('❌ Error saving file.');
									releaseDownloadRequest(requestKey);
								});
							} else {
								if (ownedBlobUrl) URL.revokeObjectURL(ownedBlobUrl);
								releaseDownloadRequest(requestKey);
							}
						};
						reader.onerror = function() {
							if (ownedBlobUrl) URL.revokeObjectURL(ownedBlobUrl);
							releaseDownloadRequest(requestKey);
						};
						reader.readAsDataURL(blob);
					})
					.catch(function(err) {
						console.error('Download intercept fetch error:', err);
						releaseDownloadRequest(requestKey);
					});
			}

			var forwardingDocumentDownload = false;
			var pendingViewerDownloadClick = false;
			var viewerDownloadSelector = [
				'button[data-testid*="download"]',
				'[role="button"][data-testid*="download"]',
				'button[aria-label*="Download" i]',
				'button[aria-label*="Unduh" i]',
				'[role="button"][aria-label*="Download" i]',
				'[role="button"][aria-label*="Unduh" i]',
				'button[title*="Download" i]',
				'button[title*="Unduh" i]',
				'[data-icon="download"]',
				'[data-icon="download-refreshed"]',
				'[data-icon*="download"]'
			].join(',');

			// Stamp explicit Download clicks (viewer toolbar button or context
			// menu item) so the blob hook stands down: the anchor path above
			// already saved the file, and a second preview must not open.
			function isExplicitDownloadMenuItem(target) {
				if (!target || !target.closest) return false;
				var item = target.closest('[role="menuitem"]');
				if (!item) return false;
				var label = ((item.innerText || '') + ' ' + (item.getAttribute('aria-label') || '')).toLowerCase();
				return label.indexOf('download') !== -1 || label.indexOf('unduh') !== -1;
			}
			document.addEventListener('click', function(e) {
				var target = e.target;
				if (target && target.closest &&
					(target.closest(viewerDownloadSelector) || isExplicitDownloadMenuItem(target))) {
					lastExplicitDownloadAt = Date.now();
				}
			}, true);

			function findVisibleViewerDownloadControl() {
				var candidates = document.querySelectorAll(viewerDownloadSelector);
				var best = null;
				var bestScore = -1;
				for (var i = 0; i < candidates.length; i++) {
					var raw = candidates[i];
					if (raw.closest && raw.closest('#wa-doc-modal-overlay')) continue;
					var control = (raw.closest && raw.closest('button, a, [role="button"]')) || raw;
					var rect = control.getBoundingClientRect();
					if (rect.width < 8 || rect.height < 8 || rect.bottom <= 0 || rect.right <= 0 ||
						rect.top >= window.innerHeight || rect.left >= window.innerWidth) continue;
					var style = window.getComputedStyle(control);
					if (style.display === 'none' || style.visibility === 'hidden' || Number(style.opacity) === 0) continue;

					var score = 0;
					if (rect.top < window.innerHeight * 0.3) score += 4;
					if (rect.left > window.innerWidth * 0.55) score += 3;
					if (control.closest && control.closest('[role="dialog"], [data-testid*="viewer"], header, [role="toolbar"]')) score += 5;
					if (score > bestScore) {
						best = control;
						bestScore = score;
					}
				}
				return bestScore >= 4 ? best : null;
			}

			function triggerVisibleViewerDownload() {
				if (pendingViewerDownloadClick || !isRecentPDFIntent()) return false;
				var control = findVisibleViewerDownloadControl();
				if (!control) return false;
				pendingViewerDownloadClick = true;
				control.click();
				setTimeout(function() { pendingViewerDownloadClick = false; }, 1500);
				return true;
			}

			function findDocumentDownloadControl(start) {
				var selector = 'a[download], button[data-testid*="download"], [role="button"][data-testid*="download"], button[aria-label*="Unduh"], button[aria-label*="Download"], [role="button"][aria-label*="Unduh"], [role="button"][aria-label*="Download"], [data-icon="download"], [data-icon="download-refreshed"]';
				var node = start;
				for (var depth = 0; node && node !== document.body && depth < 12; depth++, node = node.parentElement) {
					var found = node.querySelector && node.querySelector(selector);
					if (found) return found.closest('button, a, [role="button"]') || found;
				}
				return null;
			}

			// Hook 1: Override HTMLAnchorElement.prototype.click (programmatic downloads)
			var originalAnchorClick = HTMLAnchorElement.prototype.click;
			HTMLAnchorElement.prototype.click = function() {
				var downloadAttr = this.getAttribute('download');
				var href = this.href || this.getAttribute('href');
				if ((downloadAttr !== null || this.download) && href && (href.indexOf('blob:') === 0 || href.indexOf('data:') === 0)) {
					var name = downloadAttr || this.download || lastClickedDocName || 'whatsapp_media';
					// An explicit download anchor means save only. Opening a document
					// preview is reserved for clicking the document itself.
					lastExplicitDownloadAt = Date.now();
					captureDownload(href, name, false);
					return;
				}
				return originalAnchorClick.apply(this, arguments);
			};

			// Hook 2: User click event capturing (direct clicks on <a> with download)
			document.addEventListener('click', function(e) {
				var target = e.target;
				while (target && target !== document.body) {
					if (target.tagName === 'A') {
						var downloadAttr = target.getAttribute('download');
						var href = target.href || target.getAttribute('href');
						if ((downloadAttr !== null || target.download) && href && (href.indexOf('blob:') === 0 || href.indexOf('data:') === 0)) {
							e.preventDefault();
							e.stopPropagation();
							var name = downloadAttr || target.download || lastClickedDocName || 'whatsapp_media';
							// The user clicked Download directly: do not open a second preview.
							lastExplicitDownloadAt = Date.now();
							captureDownload(href, name, false);
							return;
						}
					}
					target = target.parentElement;
				}
			}, true);

			// Hook 3: Watch document bubble clicks in chat to handle viewer spinner
			document.addEventListener('click', function(e) {
				if (forwardingDocumentDownload) return;
				var el = e.target;
				if (!el) return;

				// Completely ignore clicks inside media-viewer or custom modal overlay
				if (typeof el.closest === 'function') {
					if (el.closest('[data-testid="media-viewer"]') || el.closest('#wa-doc-modal-overlay')) {
						return;
					}
				}

				var foundName = extractDocumentName(el);
				var clickedDoc = !!foundName;

				if (clickedDoc) {
					lastClickedDocName = foundName;
					lastDocumentIntentAt = Date.now();
					if (isDocumentFileName(foundName)) {
						var directDownload = findDocumentDownloadControl(el);
						if (directDownload && !directDownload.contains(el)) {
							e.preventDefault();
							e.stopImmediatePropagation();
							forwardingDocumentDownload = true;
							directDownload.click();
							forwardingDocumentDownload = false;
							return;
						}
					}

					var checkCount = 0;
					var checkTimer = setInterval(function() {
						if (shouldPauseBackgroundWork()) {
							clearInterval(checkTimer);
							return;
						}
						checkCount++;
						if (checkCount > 30) {
							clearInterval(checkTimer);
							return;
						}

						if (triggerVisibleViewerDownload()) {
							clearInterval(checkTimer);
						}
					}, 200);
				}
			}, true);

			// Hook 4: MutationObserver to auto-dismiss stuck media viewer and trigger download/preview.
			// This observes the whole document body (subtree), which also churns heavily while
			// the chat list is scrolled, so coalesce to at most one check per animation frame
			// instead of running on every individual mutation batch.
			var viewerCheckScheduled = false;
			var viewerObserver = new MutationObserver(function() {
				if (shouldPauseBackgroundWork() || !isRecentPDFIntent() || viewerCheckScheduled) return;
				viewerCheckScheduled = true;
				requestAnimationFrame(function() {
					viewerCheckScheduled = false;
					if (!isRecentPDFIntent()) return;
					if (!document.getElementById('wa-doc-modal-overlay')) triggerVisibleViewerDownload();
				});
			});

			function initViewerObserver() {
				var target = document.body || document.documentElement;
				if (target) {
					viewerObserver.observe(target, { childList: true, subtree: true });
				} else {
					document.addEventListener('DOMContentLoaded', initViewerObserver, { once: true });
				}
			}
			initViewerObserver();
		})();

		// "Saved to disk" badges on the Media/Docs panel. WhatsApp has no notion
		// of local downloads, so bridge it: for each document/media item shown in
		// the all-chats panel, check whether the same filename exists in the
		// configured downloads folder and tag it with a small green check.
		(function() {
			var savedScanQueued = false;
			var lastSavedScanAt = 0;
			var savedCache = {};
			var savedPending = {};
			var badgeStyle = 'display:inline-flex;align-items:center;gap:2px;margin-left:6px;padding:0 6px;border-radius:8px;' +
				'font-size:10px;font-weight:600;line-height:14px;vertical-align:middle;background:rgba(6,174,116,.16);color:#06ae74;';

			function fileExistsOnDisk(name) {
				if (!name || !window.checkFileExistsNative) return Promise.resolve(false);
				if (name in savedCache) return Promise.resolve(savedCache[name]);
				// Coalesce concurrent lookups for the same name: repeated scans
				// while a check is in flight must not spam the native binding.
				if (savedPending[name]) return savedPending[name];
				var p = window.checkFileExistsNative(name).then(function(exists) {
					delete savedPending[name];
					savedCache[name] = !!exists;
					return !!exists;
				}).catch(function() {
					delete savedPending[name];
					return false;
				});
				savedPending[name] = p;
				return p;
			}

			// Called by the download path so a just-saved file badges instantly.
			window.__waMarkSaved = function(name) {
				if (name) savedCache[name] = true;
			};

			function decorateItem(el, name) {
				if (el.__waSavedBadge) return;
				fileExistsOnDisk(name).then(function(exists) {
					if (!exists) return;
					el.__waSavedBadge = true;
					var badge = document.createElement('span');
					badge.className = 'wa-saved-badge';
					badge.setAttribute('aria-label', 'Already saved to downloads folder');
					badge.style.cssText = badgeStyle;
					badge.textContent = '✓ Saved';
					// Prefer overlaying media thumbnails; append for text rows.
					var host = el.querySelector('[data-testid="cell-frame-container"], .copyable-text') || el;
					host.style.position = host.style.position || 'relative';
					host.appendChild(badge);
				});
			}

			function itemFileName(el) {
				var t = el.getAttribute && (el.getAttribute('title') || '');
				if (!t) {
					var titleEl = el.querySelector && el.querySelector('span[title], div[title]');
					t = titleEl ? (titleEl.getAttribute('title') || '') : '';
				}
				if (!t) return '';
				var m = t.match(/([^\n\r<>]{1,180}\.(pdf|docx?|xlsx?|pptx?|txt|csv|rtf|zip|mp4|mkv|mov|mp3|wav|jpe?g|png|webp|heic))\b/i);
				return m ? m[1].trim() : '';
			}

			function scanPanel() {
				// Only scan where items can actually be seen: the open media/docs
				// panel (dialog/viewer) or the current chat pane. Scanning the
				// whole document on every chat-list mutation is exactly the
				// background churn this app is supposed to avoid.
				var scope = document.querySelector('[role="dialog"], [data-testid="media-viewer"]') ||
					document.getElementById('main');
				if (!scope) return;
				var rows = scope.querySelectorAll('[role="row"], [data-testid="cell-frame-outer"], .message-in, .message-out');
				for (var i = 0; i < rows.length; i++) {
					var row = rows[i];
					if (row.__waSavedBadge) continue;
					var name = itemFileName(row);
					if (name) decorateItem(row, name);
				}
			}

			function scheduleScan() {
				if (savedScanQueued || shouldPauseBackgroundWork()) return;
				// Hard throttle: the panel observer fires on every DOM mutation
				// while WhatsApp virtualizes lists; 2s between scans is plenty
				// for a "saved" badge that is purely informational.
				var now = Date.now();
				if (now - lastSavedScanAt < 2000) return;
				savedScanQueued = true;
				requestAnimationFrame(function() {
					savedScanQueued = false;
					lastSavedScanAt = Date.now();
					scanPanel();
				});
			}

			// Observe only where badges can appear (open dialog/viewer or the
			// chat pane). Chat-list churn in #pane-side never needs a rescan,
			// so ignore mutations outside the relevant scope entirely.
			function panelMutationRelevant(muts) {
				for (var i = 0; i < muts.length; i++) {
					var t = muts[i].target;
					if (t && t.closest) {
						try {
							if (t.closest('#main, [role="dialog"], [data-testid="media-viewer"], #wa-doc-modal-overlay')) return true;
						} catch (e) {}
					}
				}
				return false;
			}
			var panelObserver = new MutationObserver(function(muts) {
				if (panelMutationRelevant(muts)) scheduleScan();
			});
			function watchRoot() {
				var root = document.body;
				if (root) panelObserver.observe(root, { childList: true, subtree: true });
			}
			watchRoot();
			document.addEventListener('DOMContentLoaded', watchRoot, { once: true });
			document.addEventListener('click', function(e) {
				// Rescan when the user opens the media/docs panel from the toolbar.
				if (e.target && e.target.closest && e.target.closest('[data-testid="chat-menu"], [data-icon="default-image"], [data-icon="docs"], [data-icon="image"]')) {
					setTimeout(scheduleScan, 300);
				}
			}, true);
		})();

		// Theme Manager, In-Flow Header Toolbar Button & Control Center Modal
		(function() {
			var isMac = navigator.platform.toUpperCase().indexOf('MAC') >= 0;
			var currentTheme = 'dark';
			var themeChoiceVersion = 0;
			var themeLoadStarted = false;
			var themeReloadTimer = null;
			var themeReapplyTimers = [];
			var themeStyle = null;
			// Keep the engine's native MediaQueryList intact. Replacing matchMedia with
			// a partial object breaks framework listeners on some WebView2/WebKitGTK
			// versions and was the main cross-platform difference in theme switching.
			var origMatchMedia = window.matchMedia ? window.matchMedia.bind(window) : null;

			// --- Theme Management ---
			function getSystemIsDark() {
				if (origMatchMedia) {
					return origMatchMedia('(prefers-color-scheme: dark)').matches;
				}
				return true;
			}

			function applyThemeClasses(isDark) {
				var mode = isDark ? 'dark' : 'light';
				var opposite = isDark ? 'light' : 'dark';
				var root = document.documentElement;
				root.classList.add(mode);
				root.classList.remove(opposite);
				root.setAttribute('data-theme', mode);
				root.setAttribute('data-wa-desk-theme', mode);
				root.style.colorScheme = mode;
				if (document.body) {
					document.body.classList.add(mode);
					document.body.classList.remove(opposite);
					document.body.setAttribute('data-theme', mode);
					document.body.style.colorScheme = mode;
				}
			}

			function ensureThemeStyle() {
				if (!themeStyle) themeStyle = document.getElementById('wa-desk-theme-style');
				if (!themeStyle) {
					themeStyle = document.createElement('style');
					themeStyle.id = 'wa-desk-theme-style';
					themeStyle.textContent = [
						'html[data-wa-desk-theme="light"], html[data-wa-desk-theme="light"] body { color-scheme: light !important; background: #f7f9fa !important; }',
						'html[data-wa-desk-theme="light"] #app, html[data-wa-desk-theme="light"] #side, html[data-wa-desk-theme="light"] #pane-side, html[data-wa-desk-theme="light"] #main { color-scheme: light !important; }'
					].join('\\n');
					(document.head || document.documentElement).appendChild(themeStyle);
				}
			}

			function persistThemePreference(theme, isDark) {
				try {
					localStorage.setItem('system-theme-mode', theme === 'system' ? 'true' : 'false');
					localStorage.setItem('theme', JSON.stringify(theme === 'system' ? (isDark ? 'dark' : 'light') : theme));
					localStorage.setItem('wa-desk-theme', theme);
				} catch(e) {}
			}

			function scheduleThemeReapply() {
				while (themeReapplyTimers.length) clearTimeout(themeReapplyTimers.pop());
				[0, 350, 1200, 2600].forEach(function(delay) {
					themeReapplyTimers.push(setTimeout(function() {
						var isDark = currentTheme === 'system' ? getSystemIsDark() : currentTheme === 'dark';
						applyThemeClasses(isDark);
						persistThemePreference(currentTheme, isDark);
					if (window.syncToolbarBtnTheme) window.syncToolbarBtnTheme(isDark);
				}, delay));
				});
			}

			function applyThemeToDOM(theme) {
				currentTheme = theme;
				var isDark = (theme === 'system') ? getSystemIsDark() : (theme === 'dark');

				// 1. Update the document immediately for our controls and current page.
				applyThemeClasses(isDark);
				ensureThemeStyle();

				// 2. Synchronize WhatsApp Web's own localStorage keys before its tree settles.
				persistThemePreference(theme, isDark);

				// 3. Update modal and toolbar button if visible
				if (window.syncModalTheme) {
					window.syncModalTheme(isDark);
				}
				if (window.syncToolbarBtnTheme) {
					window.syncToolbarBtnTheme(isDark);
				}

				// WhatsApp may finish mounting after our script. Reapply a small, bounded
				// number of times instead of observing body classes forever: that old
				// observer could enter a feedback loop and raise CPU on Windows/macOS.
				scheduleThemeReapply();
			}

			window.getAppTheme = function() {
				return currentTheme;
			};

			window.setAppTheme = function(theme) {
				if (theme !== 'dark' && theme !== 'light' && theme !== 'system') {
					theme = 'dark';
				}
				themeChoiceVersion++;
				applyThemeToDOM(theme);
				if (window.setAppThemeNative) {
					Promise.resolve(window.setAppThemeNative(theme)).catch(function() {});
				}
				showFloatingToast(theme === 'dark' ? 'Theme: Dark' : (theme === 'light' ? 'Theme: Light' : 'Theme: System'));
				// WhatsApp keeps theme state inside its running application tree. Reload
				// once after persisting the choice so every engine starts from the same
				// localStorage state instead of leaving part of the UI in the old theme.
				clearTimeout(themeReloadTimer);
				themeReloadTimer = setTimeout(function() {
					window.location.reload();
				}, 300);
			};

			// Listen for system appearance changes
			if (origMatchMedia) {
				var sysMedia = origMatchMedia.call(window, '(prefers-color-scheme: dark)');
				var onSysChange = function() {
					if (currentTheme === 'system') {
						applyThemeToDOM('system');
						if (window.setAppThemeNative) Promise.resolve(window.setAppThemeNative('system')).catch(function() {});
					}
				};
				if (sysMedia.addEventListener) {
					sysMedia.addEventListener('change', onSysChange);
				} else if (sysMedia.addListener) {
					sysMedia.addListener(onSysChange);
				}
			}

			// Load saved theme from native settings and keep synced
			function initTheme() {
				if (themeLoadStarted || !window.getAppThemeNative) return;
				themeLoadStarted = true;
				var requestVersion = themeChoiceVersion;
				window.getAppThemeNative().then(function(savedTheme) {
					if (requestVersion !== themeChoiceVersion) return;
					if (savedTheme) applyThemeToDOM(savedTheme);
				}).catch(function() {
					themeLoadStarted = false;
				});
			}
			initTheme();
			document.addEventListener('DOMContentLoaded', function() {
				initTheme();
				applyThemeToDOM(currentTheme);
			}, { once: true });

			// --- In-Flow Header Toolbar Button (Non-Floating, Clean WhatsApp Style) ---
			function injectHeaderToolbarBtn() {
				if (document.getElementById('wa-toolbar-settings-btn')) return;

				// Target WhatsApp Web's left header above chats
				var header = document.querySelector('#side header') || document.querySelector('header');
				if (!header) return;

				// Header descendants change frequently. Use its direct trailing child, not
				// querySelector('div:last-child'), which can select an invisible nested node.
				var actionsWrap = header.lastElementChild || header;
				if (!actionsWrap) return;

				var btn = document.createElement('button');
				btn.id = 'wa-toolbar-settings-btn';
				btn.setAttribute('aria-label', 'Settings & Controls');
				btn.title = 'Settings & Controls (' + (isMac ? 'Cmd' : 'Ctrl') + ' + ,)';
				btn.style.cssText = 'width:40px;height:40px;border-radius:50%;display:inline-flex;align-items:center;justify-content:center;background:transparent;border:none;cursor:pointer;outline:none;transition:background-color 0.15s ease, color 0.15s ease;flex-shrink:0;margin:0 2px;';
				btn.innerHTML = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">' +
					'<circle cx="12" cy="12" r="3"></circle>' +
					'<path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"></path>' +
					'</svg>';

				// The header lives outside WhatsApp Web's own dark/light class toggling on
				// <body>, so this button previously always kept the dark-theme icon color
				// even when the app was switched to Light. Keep its resting color in sync
				// with the current app theme instead of a hardcoded dark-mode gray.
				function restingIconColor() {
					var isDarkNow = currentTheme === 'system' ?
						(window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches) :
						(currentTheme === 'dark');
					return isDarkNow ? '#aebac1' : '#54656f';
				}
				btn.style.color = restingIconColor();
				window.syncToolbarBtnTheme = function() {
					btn.style.color = restingIconColor();
				};

				btn.onmouseenter = function() {
					btn.style.backgroundColor = document.body.classList.contains('dark') ? 'rgba(255,255,255,0.08)' : 'rgba(0,0,0,0.06)';
					btn.style.color = document.body.classList.contains('dark') ? '#e9edef' : '#111b21';
				};
				btn.onmouseleave = function() {
					btn.style.backgroundColor = 'transparent';
					btn.style.color = restingIconColor();
				};
				btn.onclick = function(e) {
					e.stopPropagation();
					window.showSettingsModal();
				};

				actionsWrap.appendChild(btn);
			}

			// The floating fallback gear was removed: Ctrl+, (plus the header
			// button above and the tray menu) is the way into Settings, so no
			// overlay button may cover the chat surface anymore.
			injectHeaderToolbarBtn();
			document.addEventListener('DOMContentLoaded', function() {
				injectHeaderToolbarBtn();
				setTimeout(injectHeaderToolbarBtn, 600);
			});
			window.addEventListener('load', injectHeaderToolbarBtn);
			// WhatsApp rebuilds its header when switching chats, dropping our
			// button. Watch only #side/header region changes (rAF-coalesced)
			// instead of scanning the whole page every 2 seconds forever.
			var toolbarCheckQueued = false;
			var toolbarNarrowed = false;
			var toolbarObserver = new MutationObserver(function() {
				if (toolbarCheckQueued) return;
				toolbarCheckQueued = true;
				requestAnimationFrame(function() {
					toolbarCheckQueued = false;
					if (!document.getElementById('wa-toolbar-settings-btn')) {
						injectHeaderToolbarBtn();
					}
					// Narrow the observed root once the header exists.
					if (!toolbarNarrowed) {
						var hdr = document.querySelector('#side header');
						if (hdr) {
							toolbarNarrowed = true;
							toolbarObserver.disconnect();
							toolbarObserver.observe(hdr, { childList: true, subtree: true });
						}
					}
				});
			});
			function watchToolbarRoot() {
				// Prefer the header itself: the chat list churns constantly and
				// never affects our button. Fall back to #side, then body, and
				// narrow down to the header as soon as it exists.
				var root = document.querySelector('#side header') || document.querySelector('#side') || document.body;
				if (root) {
					toolbarNarrowed = !!document.querySelector('#side header');
					toolbarObserver.disconnect();
					toolbarObserver.observe(root, { childList: true, subtree: true });
				}
			}
			watchToolbarRoot();
			document.addEventListener('DOMContentLoaded', watchToolbarRoot, { once: true });

			// --- Direct Chat (click-to-chat without saving the contact) ---
			// Number input + Send-From profile dropdown (hidden on single
			// profile installs). Start validates client-side, then the native
			// bridge validates again, parks on the profile's tab and navigates
			// it to the official /send?phone= deep link.
			window.openDirectChatModal = function() {
				if (document.getElementById('wa-directchat-overlay')) return;

				var overlay = document.createElement('div');
				overlay.id = 'wa-directchat-overlay';
				overlay.style.cssText = 'position:fixed;inset:0;background:rgba(8,15,19,.68);z-index:9999999;display:flex;align-items:center;justify-content:center;padding:16px;box-sizing:border-box;font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",system-ui,sans-serif;';

				var modal = document.createElement('div');
				modal.style.cssText = 'width:420px;max-width:94vw;border-radius:12px;box-sizing:border-box;padding:26px 26px 22px;box-shadow:0 18px 48px rgba(0,0,0,.4);background:#ffffff;border:1px solid #d1d7db;';
				modal.innerHTML = '' +
					'<h2 style="margin:0 0 6px;font-size:22px;font-weight:700;color:#00a884;">Direct Chat</h2>' +
					'<p style="margin:0 0 18px;font-size:13px;color:#667781;line-height:1.45;">Chat with a new number without saving it to your contacts.</p>' +
					'<label style="display:block;font-size:13px;font-weight:600;color:#111b21;margin-bottom:6px;">Target Number (with country code):</label>' +
					'<input id="wa-directchat-number" type="tel" inputmode="tel" placeholder="e.g. 6281234567890" autocomplete="off" spellcheck="false" ' +
					'style="width:100%;box-sizing:border-box;padding:10px 12px;border:1px solid #d1d7db;border-radius:8px;font-size:14px;color:#111b21;background:#f0f2f5;outline:none;margin-bottom:16px;" />' +
					'<div id="wa-directchat-profile-row">' +
					'  <label style="display:block;font-size:13px;font-weight:600;color:#111b21;margin-bottom:6px;">Send From Profile:</label>' +
					'  <select id="wa-directchat-profile" style="width:100%;box-sizing:border-box;padding:10px 12px;border:1px solid #d1d7db;border-radius:8px;font-size:14px;color:#111b21;background:#f0f2f5;outline:none;margin-bottom:16px;"></select>' +
					'</div>' +
					'<div style="display:flex;gap:12px;">' +
					'  <button id="wa-directchat-cancel" style="flex:1;padding:11px;border:none;border-radius:8px;font-size:14px;font-weight:600;color:#111b21;background:#e9edef;cursor:pointer;">Cancel</button>' +
					'  <button id="wa-directchat-start" style="flex:1;padding:11px;border:none;border-radius:8px;font-size:14px;font-weight:700;color:#ffffff;background:#00a884;cursor:pointer;">Start Chat</button>' +
					'</div>';
				overlay.appendChild(modal);
				(document.body || document.documentElement).appendChild(overlay);

				var input = document.getElementById('wa-directchat-number');
				var profileRow = document.getElementById('wa-directchat-profile-row');
				var profileSel = document.getElementById('wa-directchat-profile');

				function closeDirectChatModal() {
					if (overlay.parentNode) overlay.parentNode.removeChild(overlay);
					window.removeEventListener('keydown', onDirectChatKey, true);
				}
				function onDirectChatKey(e) {
					if (e.key === 'Escape') closeDirectChatModal();
					else if (e.key === 'Enter' && document.getElementById('wa-directchat-overlay')) startDirectChat();
				}
				function startDirectChat() {
					var raw = (input && input.value) || '';
					var digits = raw.trim().replace(/^\+/, '').replace(/[\s\-().]/g, '');
					if (!/^[0-9]{8,15}$/.test(digits)) {
						showFloatingToast('⚠️ Enter a valid number with country code (8-15 digits)');
						if (input) input.focus();
						return;
					}
					var profileKey = (profileSel && profileSel.value) || '';
					if (!window.startDirectChatNative) {
						showFloatingToast('⚠️ Direct Chat is not available in this build');
						return;
					}
					window.startDirectChatNative(profileKey, raw).then(function(errText) {
						if (errText) {
							showFloatingToast('⚠️ ' + errText);
							return;
						}
						closeDirectChatModal();
						showFloatingToast('💬 Opening chat with +' + digits + '...');
					}).catch(function() {
						showFloatingToast('⚠️ Could not start the chat');
					});
				}

				// Profiles for the Send-From dropdown; single-profile setups
				// skip the row entirely.
				function fillProfiles() {
					if (!window.listProfilesNative) {
						if (profileRow) profileRow.style.display = 'none';
						return Promise.resolve();
					}
					return Promise.resolve(window.listProfilesNative()).then(function(profiles) {
						profiles = profiles || [];
						if (profiles.length <= 1) {
							if (profileRow) profileRow.style.display = 'none';
						}
						for (var i = 0; i < profiles.length; i++) {
							var opt = document.createElement('option');
							opt.value = profiles[i].id || profiles[i].name || '';
							opt.textContent = profiles[i].name || opt.value;
							if (profiles[i].active) opt.selected = true;
							profileSel.appendChild(opt);
						}
					}).catch(function() {
						if (profileRow) profileRow.style.display = 'none';
					});
				}

				document.getElementById('wa-directchat-cancel').onclick = closeDirectChatModal;
				document.getElementById('wa-directchat-start').onclick = startDirectChat;
				overlay.onclick = function(e) {
					if (e.target === overlay) closeDirectChatModal();
				};
				window.addEventListener('keydown', onDirectChatKey, true);
				fillProfiles();
				if (input) input.focus();
			};

			// --- Minimalist WhatsApp Control Center Modal ---
			window.showSettingsModal = function() {
				if (document.getElementById('wa-settings-overlay')) {
					var ex = document.getElementById('wa-settings-overlay');
					if (ex.parentNode) ex.parentNode.removeChild(ex);
					return;
				}

				var isDark = currentTheme === 'system' ?
					(window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches) :
					(currentTheme === 'dark');

				var overlay = document.createElement('div');
				overlay.id = 'wa-settings-overlay';
				overlay.style.cssText = 'position:fixed;inset:0;background:rgba(8,15,19,.68);z-index:9999999;display:flex;align-items:center;justify-content:center;padding:16px;box-sizing:border-box;font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",system-ui,sans-serif;';

				var modal = document.createElement('div');
				modal.id = 'wa-settings-container';
				// Opaque from birth: isDark is already known here, so never rely
				// solely on the later syncModalTheme pass — if badge wiring ever
				// throws first, the panel must still not leak the page behind it.
				modal.style.cssText = 'width:640px;max-width:96vw;max-height:90vh;border-radius:14px;box-sizing:border-box;display:flex;flex-direction:column;gap:0;overflow-y:auto;padding:0;box-shadow:0 18px 48px rgba(0,0,0,.32);' +
					'background:' + (isDark ? '#111b21' : '#ffffff') + ';border:1px solid ' + (isDark ? '#2a3942' : '#d1d7db') + ';';
				// Seed the palette immediately (syncModalTheme repaints it on
				// every theme change) so the styled sections never compute
				// against missing custom properties.
				modal.style.setProperty('--w-bg', isDark ? '#111b21' : '#ffffff');
				modal.style.setProperty('--w-surface', isDark ? '#18262e' : '#f7f8fa');
				modal.style.setProperty('--w-border', isDark ? '#2a3942' : '#d1d7db');
				modal.style.setProperty('--w-text', isDark ? '#e9edef' : '#111b21');
				modal.style.setProperty('--w-muted', isDark ? '#8696a0' : '#667781');
				modal.style.setProperty('--w-accent', isDark ? '#00a884' : '#008069');
				modal.style.setProperty('--w-field', isDark ? '#111b21' : '#ffffff');
				modal.style.setProperty('--w-hover', isDark ? '#1f2f39' : '#eef1f4');
				modal.style.setProperty('--w-accent-soft', isDark ? 'rgba(0,168,132,.55)' : 'rgba(0,128,105,.5)');
				modal.style.colorScheme = isDark ? 'dark' : 'light';

				// One stylesheet scoped to this overlay: rounded cards, sticky
				// search + category pills, switch-style toggles and the light
				// entrance motion. Every colour flows from the --w-* custom
				// properties that syncModalTheme repaints, so the page around
				// the modal is never touched.
				var panelCss = '' +
					'#wa-settings-overlay *{box-sizing:border-box}' +
					'#wa-settings-overlay ::selection{background:rgba(0,168,132,.35);color:#fff}' +
					'#wa-settings-overlay .wa-modal-card{background:var(--w-surface);border:1px solid var(--w-border);border-radius:12px;padding:14px 16px;display:flex;flex-direction:column;gap:8px;transition:border-color .15s ease}' +
					'#wa-settings-overlay .wa-modal-card:hover{border-color:var(--w-accent-soft)}' +
					'#wa-settings-overlay .wa-set-tools{position:sticky;top:0;z-index:2;background:var(--w-bg);padding:12px 22px 0}' +
					'#wa-settings-searchwrap{position:relative}' +
					'#wa-settings-searchicon{position:absolute;left:11px;top:50%;transform:translateY(-50%);font-size:12px;opacity:.6;pointer-events:none}' +
					'#wa-settings-search{width:100%;padding:9px 12px 9px 32px;border-radius:9px;font-size:12.5px}' +
					'#wa-settings-tabs{display:flex;gap:6px;padding:10px 0 2px;overflow-x:auto;scrollbar-width:none}' +
					'#wa-settings-tabs::-webkit-scrollbar{display:none}' +
					'.wa-set-tab{border:1px solid var(--w-border);background:var(--w-surface);color:var(--w-muted);padding:6px 12px;border-radius:999px;font-size:11.5px;font-weight:600;font-family:inherit;cursor:pointer;white-space:nowrap;transition:background .12s,color .12s,border-color .12s}' +
					'.wa-set-tab:hover{border-color:var(--w-accent-soft);color:var(--w-text)}' +
					'.wa-set-tab.wa-on{background:var(--w-accent);border-color:var(--w-accent);color:#fff}' +
					'.wa-set-tab.wa-on:hover{color:#fff}' +
					'#wa-settings-body{padding:12px 22px 4px;display:flex;flex-direction:column;gap:10px}' +
					'.wa-set-foot{padding:8px 22px 18px}' +
					'.wa-set-hidden{display:none!important}' +
					'#wa-profiles-card{margin:12px 22px 0}' +
					'#wa-settings-overlay .wa-card-btn{background:var(--w-surface);border:1px solid var(--w-border);color:var(--w-text);font-family:inherit;transition:background .12s,border-color .12s,color .12s}' +
					'#wa-settings-overlay .wa-card-btn:not(.wa-switch):hover{background:var(--w-hover);border-color:var(--w-accent-soft)}' +
					'#wa-settings-overlay .wa-switch{position:relative;width:40px;height:22px;min-width:40px;border-radius:11px;padding:0;cursor:pointer;background:rgba(134,150,160,.45);border:1px solid transparent}' +
					'#wa-settings-overlay .wa-switch::after{content:"";position:absolute;top:2px;left:2px;width:16px;height:16px;border-radius:50%;background:#fff;box-shadow:0 1px 3px rgba(0,0,0,.35);transition:left .16s ease}' +
					'#wa-settings-overlay .wa-switch.wa-on{background:var(--w-accent)}' +
					'#wa-settings-overlay .wa-switch.wa-on::after{left:21px}' +
					'#wa-settings-overlay .wa-seg{display:flex;gap:2px;background:var(--w-field);border:1px solid var(--w-border);border-radius:8px;padding:2px}' +
					'#wa-settings-overlay .wa-segbtn{border:none;background:transparent;color:var(--w-muted);font-size:11.5px;font-weight:600;font-family:inherit;padding:5px 11px;border-radius:6px;cursor:pointer;transition:background .12s,color .12s}' +
					'#wa-settings-overlay .wa-segbtn:hover{color:var(--w-text)}' +
					'#wa-settings-overlay .wa-segbtn.wa-on{background:var(--w-accent);color:#fff}' +
					'#wa-settings-overlay input:not([type=checkbox]):not([type=radio]),#wa-settings-overlay select,#wa-settings-overlay textarea{background:var(--w-field)!important;border:1px solid var(--w-border)!important;color:var(--w-text)!important;font-family:inherit}' +
					'#wa-settings-overlay input:focus,#wa-settings-overlay select:focus,#wa-settings-overlay textarea:focus{border-color:var(--w-accent)!important;outline:none}' +
					'#wa-settings-overlay input::placeholder,#wa-settings-overlay textarea::placeholder{color:var(--w-muted)!important;opacity:.85}' +
					'#wa-settings-overlay{animation:waSetFade .14s ease}' +
					'#wa-settings-container{animation:waSetRise .16s ease}' +
					'@keyframes waSetFade{from{opacity:0}}' +
					'@keyframes waSetRise{from{opacity:0;transform:translateY(6px)}}' +
					'@media (prefers-reduced-motion:reduce){#wa-settings-overlay,#wa-settings-container{animation:none}}';
				var panelStyle = document.createElement('style');
				panelStyle.textContent = panelCss;
				overlay.appendChild(panelStyle);

				// Header — sticky title bar with the close affordance.
				var header = document.createElement('div');
				header.id = 'wa-modal-header';
				header.style.cssText = 'position:sticky;top:0;z-index:3;display:flex;align-items:center;justify-content:space-between;padding:14px 22px;border-bottom-width:1px;border-bottom-style:solid;background:' +
					(isDark ? '#111b21' : '#ffffff') + ';border-bottom-color:' + (isDark ? '#2a3942' : '#d1d7db') + ';';
				header.innerHTML = '' +
					'<div style="display:flex;align-items:center;gap:10px;">' +
					'  <div id="wa-modal-icon-wrap" style="width:10px;height:10px;border-radius:50%;display:flex;align-items:center;justify-content:center;background:#00a884;">' +
					'  </div>' +
					'  <div>' +
					'    <h3 id="wa-modal-title" style="margin:0;font-size:15px;font-weight:600;">WAtchful</h3>' +
					'    <span id="wa-modal-sub" style="font-size:11px;">Application settings · version __WA_APP_VERSION__</span>' +
					'  </div>' +
					'</div>' +
					'<button id="wa-settings-close-x" aria-label="Close settings" style="background:transparent;border:none;cursor:pointer;font-size:18px;line-height:1;padding:4px 8px;border-radius:6px;">✕</button>';
				modal.appendChild(header);

				// Sticky tool rail: live search over every section, then the
				// category pills. The Profiles card lands between the header
				// and this block (insertBefore(header.nextSibling)), so the
				// active account still leads the panel.
				var toolsRow = document.createElement('div');
				toolsRow.id = 'wa-settings-tools';
				toolsRow.className = 'wa-set-tools';
				toolsRow.innerHTML = '' +
					'<div id="wa-settings-searchwrap">' +
					'  <span id="wa-settings-searchicon">🔍</span>' +
					'  <input id="wa-settings-search" type="text" placeholder="Search settings — try “zoom”, “scheduler”, “blur”…" autocomplete="off" spellcheck="false" />' +
					'</div>' +
					'<div id="wa-settings-tabs" role="tablist">' +
					'  <button class="wa-set-tab" type="button" data-cat="general" role="tab">⚙️ General</button>' +
					'  <button class="wa-set-tab" type="button" data-cat="notif" role="tab">🔔 Notifications</button>' +
					'  <button class="wa-set-tab" type="button" data-cat="privacy" role="tab">🛡️ Privacy</button>' +
					'  <button class="wa-set-tab" type="button" data-cat="automation" role="tab">🤖 Automation</button>' +
					'  <button class="wa-set-tab" type="button" data-cat="files" role="tab">📁 Downloads</button>' +
					'  <button class="wa-set-tab" type="button" data-cat="help" role="tab">❓ Help</button>' +
					'</div>';
				modal.appendChild(toolsRow);

				// Section container: every card below carries data-cat (its
				// pill) and data-name (search synonyms).
				var setBody = document.createElement('div');
				setBody.id = 'wa-settings-body';
				modal.appendChild(setBody);

				// Section 0: Theme Switcher Segmented Control
				var themeBox = document.createElement('div');
				themeBox.className = 'wa-modal-card';
				themeBox.setAttribute('data-cat', 'general');
				themeBox.setAttribute('data-name', 'appearance theme dark light system color');
				themeBox.style.cssText = 'display:flex;align-items:center;justify-content:space-between;gap:16px;';
				themeBox.innerHTML = '' +
					'<div>' +
					'  <strong class="wa-text-primary" style="font-size:12.5px;display:block;">Appearance</strong>' +
					'  <span class="wa-text-muted" style="font-size:11px;">Application interface theme</span>' +
					'</div>' +
					'<div class="wa-seg" role="group" aria-label="Theme">' +
					'  <button id="wa-theme-btn-dark" class="wa-segbtn" type="button">Dark</button>' +
					'  <button id="wa-theme-btn-light" class="wa-segbtn" type="button">Light</button>' +
					'  <button id="wa-theme-btn-system" class="wa-segbtn" type="button">System</button>' +
					'</div>';
				setBody.appendChild(themeBox);

				// Section 0b: Display Zoom stepper (persisted across restarts).
				var zoomBox = document.createElement('div');
				zoomBox.className = 'wa-modal-card';
				zoomBox.setAttribute('data-cat', 'general');
				zoomBox.setAttribute('data-name', 'display zoom scale size browser ctrl scroll');
				zoomBox.style.cssText = 'display:flex;align-items:center;justify-content:space-between;gap:16px;';
				zoomBox.innerHTML = '' +
					'<div>' +
					'  <strong class="wa-text-primary" style="font-size:12.5px;display:block;">Display zoom</strong>' +
					'  <span class="wa-text-muted" style="font-size:11px;">Page zoom, like a browser. Ctrl+scroll also works. Kept when the app restarts.</span>' +
					'</div>' +
					'<div style="display:flex;align-items:center;gap:4px;">' +
					'  <button id="wa-zoom-out" class="wa-card-btn" style="padding:5px 12px;border-radius:6px;font-size:13px;font-weight:600;cursor:pointer;border-width:1px;border-style:solid;" aria-label="Zoom out">−</button>' +
					'  <span id="wa-zoom-label" class="wa-text-muted" style="font-size:11.5px;min-width:44px;text-align:center;font-variant-numeric:tabular-nums;">100%</span>' +
					'  <button id="wa-zoom-in" class="wa-card-btn" style="padding:5px 12px;border-radius:6px;font-size:13px;font-weight:600;cursor:pointer;border-width:1px;border-style:solid;" aria-label="Zoom in">+</button>' +
					'  <button id="wa-zoom-reset" class="wa-card-btn" style="padding:5px 10px;border-radius:6px;font-size:11.5px;cursor:pointer;border-width:1px;border-style:solid;font-weight:500;">Reset</button>' +
					'</div>';
				setBody.appendChild(zoomBox);

				// Card 1: Privacy Mode
				var cardPrivacy = document.createElement('div');
				cardPrivacy.className = 'wa-modal-card';
				cardPrivacy.setAttribute('data-cat', 'privacy');
				cardPrivacy.setAttribute('data-name', 'privacy mode blur hide names auto-lock avatars peek confidential');
				cardPrivacy.innerHTML = '' +
					'<div style="display:flex;align-items:center;justify-content:space-between;gap:16px;">' +
					'  <div>' +
					'    <div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:2px;">' +
					'      <strong class="wa-text-primary" style="font-size:12.5px;">Privacy Mode</strong>' +
					'      <span id="wa-badge-priv" style="font-size:10px;padding:1px 5px;border-radius:4px;font-weight:600;">...</span>' +
					'    </div>' +
					'    <div class="wa-text-muted" style="font-size:11px;">Hide names, previews & message text until you turn this off. Hover to peek; timestamps stay visible; reply box stays usable.</div>' +
					'  </div>' +
					'  <div style="display:flex;align-items:center;justify-content:space-between;">' +
					'    <span class="wa-text-muted" style="font-size:10px;font-family:monospace;">' + (isMac ? 'Cmd' : 'Ctrl') + '+Shift+P</span>' +
					'    <button id="wa-action-toggle-priv" class="wa-card-btn wa-switch" type="button" role="switch" aria-label="Privacy Mode"></button>' +
					'  </div>' +
					'</div>' +
					'<label style="display:flex;align-items:center;gap:8px;cursor:pointer;user-select:none;">' +
					'  <input type="checkbox" id="wa-priv-autolock" style="width:14px;height:14px;accent-color:#00a884;cursor:pointer;margin:0;" />' +
					'  <span class="wa-text-muted" style="font-size:11px;">Auto-lock when idle or window loses focus (unblurs on activity)</span>' +
					'</label>' +
					'<label style="display:flex;align-items:center;gap:8px;cursor:pointer;user-select:none;">' +
					'  <input type="checkbox" id="wa-blur-avatars" style="width:14px;height:14px;accent-color:#00a884;cursor:pointer;margin:0;" />' +
					'  <span class="wa-text-muted" style="font-size:11px;">Also blur profile photos (hover to peek)</span>' +
					'</label>';
				setBody.appendChild(cardPrivacy);

				// Card 2: Always on Top
				var cardPin = document.createElement('div');
				cardPin.className = 'wa-modal-card';
				cardPin.setAttribute('data-cat', 'general');
				cardPin.setAttribute('data-name', 'always on top pin window float');
				cardPin.style.cssText = 'display:flex;align-items:center;justify-content:space-between;gap:16px;';
				cardPin.innerHTML = '' +
					'<div>' +
					'  <div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:2px;">' +
					'    <strong class="wa-text-primary" style="font-size:12.5px;">Always on Top</strong>' +
					'    <span id="wa-badge-pin" style="font-size:10px;padding:1px 5px;border-radius:4px;font-weight:600;">...</span>' +
					'  </div>' +
					'  <div class="wa-text-muted" style="font-size:11px;">Keep window floating above other applications.</div>' +
					'</div>' +
					'<div style="display:flex;align-items:center;justify-content:space-between;">' +
					'  <span class="wa-text-muted" style="font-size:10px;font-family:monospace;">' + (isMac ? 'Cmd' : 'Ctrl') + '+Shift+T</span>' +
					'  <button id="wa-action-toggle-pin" class="wa-card-btn wa-switch" type="button" role="switch" aria-label="Always on Top"></button>' +
					'</div>';
				setBody.appendChild(cardPin);

				// Card 3: Audio Mute
				var cardMute = document.createElement('div');
				cardMute.className = 'wa-modal-card';
				cardMute.setAttribute('data-cat', 'notif');
				cardMute.setAttribute('data-name', 'audio mute sound notification media');
				cardMute.style.cssText = 'display:flex;align-items:center;justify-content:space-between;gap:16px;';
				cardMute.innerHTML = '' +
					'<div>' +
					'  <div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:2px;">' +
					'    <strong class="wa-text-primary" style="font-size:12.5px;">Notification Audio</strong>' +
					'    <span id="wa-badge-mute" style="font-size:10px;padding:1px 5px;border-radius:4px;font-weight:600;">...</span>' +
					'  </div>' +
					'  <div class="wa-text-muted" style="font-size:11px;">Mute all notification sounds and media audio.</div>' +
					'</div>' +
					'<div style="display:flex;align-items:center;justify-content:space-between;">' +
					'  <span class="wa-text-muted" style="font-size:10px;font-family:monospace;">' + (isMac ? 'Cmd' : 'Ctrl') + '+Shift+M</span>' +
					'  <button id="wa-action-toggle-mute" class="wa-card-btn wa-switch" type="button" role="switch" aria-label="Notification Audio"></button>' +
					'</div>';
				setBody.appendChild(cardMute);

				// Card 3b: Desktop Notifications
				var cardNotif = document.createElement('div');
				cardNotif.className = 'wa-modal-card';
				cardNotif.setAttribute('data-cat', 'notif');
				cardNotif.setAttribute('data-name', 'desktop notifications alerts os popup');
				cardNotif.style.cssText = 'display:flex;align-items:center;justify-content:space-between;gap:16px;';
				cardNotif.innerHTML = '' +
					'<div>' +
					'  <div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:2px;">' +
					'    <strong class="wa-text-primary" style="font-size:12.5px;">Desktop Notifications</strong>' +
					'    <span id="wa-badge-notif" style="font-size:10px;padding:1px 5px;border-radius:4px;font-weight:600;">...</span>' +
					'  </div>' +
					'  <div class="wa-text-muted" style="font-size:11px;">Show OS-level alerts for new chat messages.</div>' +
					'</div>' +
					'<div style="display:flex;align-items:center;justify-content:space-between;">' +
					'  <button id="wa-action-toggle-notif" class="wa-card-btn wa-switch" type="button" role="switch" aria-label="Desktop Notifications"></button>' +
					'</div>';
				setBody.appendChild(cardNotif);

				// Card 3c: Download Notifications
				var cardDlNotif = document.createElement('div');
				cardDlNotif.className = 'wa-modal-card';
				cardDlNotif.setAttribute('data-cat', 'notif');
				cardDlNotif.setAttribute('data-name', 'download notifications popup finished saving');
				cardDlNotif.style.cssText = 'display:flex;align-items:center;justify-content:space-between;gap:16px;';
				cardDlNotif.innerHTML = '' +
					'<div>' +
					'  <div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:2px;">' +
					'    <strong class="wa-text-primary" style="font-size:12.5px;">Download Notifications</strong>' +
					'    <span id="wa-badge-dlnotif" style="font-size:10px;padding:1px 5px;border-radius:4px;font-weight:600;">...</span>' +
					'  </div>' +
					'  <div class="wa-text-muted" style="font-size:11px;">Show a popup when a chat file finishes downloading.</div>' +
					'</div>' +
					'<div style="display:flex;align-items:center;justify-content:space-between;">' +
					'  <button id="wa-action-toggle-dlnotif" class="wa-card-btn wa-switch" type="button" role="switch" aria-label="Download Notifications"></button>' +
					'</div>';
				setBody.appendChild(cardDlNotif);

				// Card 4: Auto-Start
				var cardAuto = document.createElement('div');
				cardAuto.className = 'wa-modal-card';
				cardAuto.setAttribute('data-cat', 'general');
				cardAuto.setAttribute('data-name', 'launch startup boot login autostart');
				cardAuto.style.cssText = 'display:flex;align-items:center;justify-content:space-between;gap:16px;';
				cardAuto.innerHTML = '' +
					'<div>' +
					'  <div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:2px;">' +
					'    <strong class="wa-text-primary" style="font-size:12.5px;">Launch at Startup</strong>' +
					'    <span id="wa-badge-auto" style="font-size:10px;padding:1px 5px;border-radius:4px;font-weight:600;">...</span>' +
					'  </div>' +
					'  <div class="wa-text-muted" style="font-size:11px;">Automatically start WAtchful on system login.</div>' +
					'</div>' +
					'<div style="display:flex;align-items:center;justify-content:space-between;">' +
					'  <span class="wa-text-muted" style="font-size:10px;font-family:monospace;">' + (isMac ? 'Cmd' : 'Ctrl') + '+Shift+S</span>' +
					'  <button id="wa-action-toggle-auto" class="wa-card-btn wa-switch" type="button" role="switch" aria-label="Launch at Startup"></button>' +
					'</div>';
				setBody.appendChild(cardAuto);

				// Card 5: Direct Chat (labeled entry: icon-only buttons are
				// hard to discover, so the modal also opens from here, the
				// tray menu and Ctrl/Cmd+Shift+C).
				var cardDirect = document.createElement('div');
				cardDirect.className = 'wa-modal-card';
				cardDirect.setAttribute('data-cat', 'general');
				cardDirect.setAttribute('data-name', 'direct chat new number message without contact');
				cardDirect.style.cssText = 'display:flex;align-items:center;justify-content:space-between;gap:16px;';
				cardDirect.innerHTML = '' +
					'<div>' +
					'  <div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:2px;">' +
					'    <strong class="wa-text-primary" style="font-size:12.5px;">💬 Direct Chat</strong>' +
					'  </div>' +
					'  <div class="wa-text-muted" style="font-size:11px;">Chat a new number without saving it to contacts.</div>' +
					'</div>' +
					'<div style="display:flex;align-items:center;justify-content:space-between;">' +
					'  <span class="wa-text-muted" style="font-size:10px;font-family:monospace;">' + (isMac ? 'Cmd' : 'Ctrl') + '+Shift+C</span>' +
					'  <button id="wa-action-open-directchat" class="wa-card-btn" style="padding:6px 14px;border-radius:8px;font-size:11.5px;font-weight:600;cursor:pointer;border-width:1px;border-style:solid;">Open</button>' +
					'</div>';
				setBody.appendChild(cardDirect);

				// Section 2: Download Folder Settings
				var folderSection = document.createElement('div');
				folderSection.className = 'wa-modal-card';
				folderSection.setAttribute('data-cat', 'files');
				folderSection.setAttribute('data-name', 'downloads folder path save location organize monthly subfolders');
				folderSection.innerHTML = '' +
					'<div style="display:flex;align-items:center;justify-content:space-between;">' +
					'  <strong class="wa-text-primary" style="font-size:12.5px;">Downloads folder</strong>' +
					'  <button id="wa-btn-reset-folder" style="background:transparent;border:none;color:#00a884;font-size:11px;cursor:pointer;padding:2px 4px;">Use default</button>' +
					'</div>' +
					'<div class="wa-text-muted" style="font-size:11px;">Files & media downloaded from chat are permanently saved here:</div>' +
					'<div id="wa-folder-box" style="display:flex;align-items:center;border-width:1px;border-style:solid;border-radius:6px;padding:6px 8px;min-width:0;">' +
					'  <span id="wa-folder-path" style="font-size:11px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;flex:1;font-family:monospace;">Loading...</span>' +
					'</div>' +
					'<div style="display:flex;align-items:center;gap:6px;margin-top:2px;">' +
					'  <button id="wa-btn-change-folder" class="wa-card-btn" style="flex:1;padding:6px 10px;border-radius:6px;font-size:11.5px;font-weight:500;cursor:pointer;border-width:1px;border-style:solid;">Change Folder Location...</button>' +
					'  <button id="wa-btn-open-folder" style="background:#00a884;color:#111b21;border:none;padding:6px 12px;border-radius:6px;font-size:11.5px;font-weight:600;cursor:pointer;">' + (isMac ? 'Open in Finder' : 'Open Folder') + '</button>' +
					'</div>' +
					'<label style="display:flex;align-items:center;gap:8px;cursor:pointer;user-select:none;margin-top:2px;">' +
					'  <input type="checkbox" id="wa-organize-month" style="width:14px;height:14px;accent-color:#00a884;cursor:pointer;margin:0;" />' +
					'  <span class="wa-text-muted" style="font-size:11px;">Organize into monthly subfolders (2026-09)</span>' +
					'</label>';
				setBody.appendChild(folderSection);

				// Section 2b: Quick Replies (slash templates for the composer).
				var qrSection = document.createElement('div');
				qrSection.className = 'wa-modal-card';
				qrSection.setAttribute('data-cat', 'automation');
				qrSection.setAttribute('data-name', 'quick replies template slash trigger variables composer');
				qrSection.innerHTML = '' +
					'<div style="display:flex;align-items:center;justify-content:space-between;">' +
					'  <strong class="wa-text-primary" style="font-size:12.5px;">⚡ Quick Replies</strong>' +
					'</div>' +
					'<div class="wa-text-muted" style="font-size:11px;">Type <span style="font-family:monospace;">/trigger</span> + Space in any chat to expand a template. Variables: <span style="font-family:monospace;">{name}</span> <span style="font-family:monospace;">{date}</span> <span style="font-family:monospace;">{time}</span>.</div>' +
					'<div id="wa-qr-list" style="display:flex;flex-direction:column;gap:6px;"></div>' +
					'<div style="display:flex;gap:6px;">' +
					'  <input id="wa-qr-trigger" placeholder="trigger e.g. followup" maxlength="32" style="width:130px;flex-shrink:0;padding:6px 8px;border-width:1px;border-style:solid;border-radius:6px;font-size:11.5px;font-family:monospace;background:transparent;color:inherit;outline:none;" />' +
					'  <input id="wa-qr-text" placeholder="Template text..." maxlength="2000" style="flex:1;min-width:0;padding:6px 8px;border-width:1px;border-style:solid;border-radius:6px;font-size:11.5px;background:transparent;color:inherit;outline:none;" />' +
					'  <button id="wa-qr-add" class="wa-card-btn" style="padding:6px 12px;border-radius:6px;font-size:11.5px;font-weight:600;cursor:pointer;border-width:1px;border-style:solid;flex-shrink:0;">Add</button>' +
					'</div>';
				setBody.appendChild(qrSection);

				// Section 2c: Scheduler (timed messages to phone numbers).
				var schedSection = document.createElement('div');
				schedSection.className = 'wa-modal-card';
				schedSection.setAttribute('data-cat', 'automation');
				schedSection.setAttribute('data-name', 'scheduler schedule timed message send repeat daily weekly monthly yearly emergency stop');
				schedSection.innerHTML = '' +
					'<div style="display:flex;align-items:center;justify-content:space-between;">' +
					'  <strong class="wa-text-primary" style="font-size:12.5px;">⏰ Scheduler</strong>' +
					'</div>' +
					'<div class="wa-text-muted" style="font-size:11px;">Send to a phone number at a set time. Fires while the app runs with this profile loaded — profiles holding schedules stay awake (no hibernate).</div>' +
					'<label style="display:flex;align-items:center;gap:8px;cursor:pointer;user-select:none;">' +
					'  <input type="checkbox" id="wa-sched-kill" style="width:14px;height:14px;accent-color:#f15c6d;cursor:pointer;margin:0;" />' +
					'  <span class="wa-text-muted" style="font-size:11px;">Emergency stop: cancel any pending send and halt all scheduled sending</span>' +
					'</label>' +
					'<div id="wa-sched-list" style="display:flex;flex-direction:column;gap:6px;"></div>' +
					'<div style="display:flex;gap:6px;flex-wrap:wrap;">' +
					'  <input id="wa-sched-phone" placeholder="number e.g. 62812..." inputmode="tel" style="width:130px;padding:6px 8px;border-width:1px;border-style:solid;border-radius:6px;font-size:11.5px;font-family:monospace;background:transparent;color:inherit;outline:none;" />' +
					'  <input id="wa-sched-text" placeholder="Message..." maxlength="2000" style="flex:1;min-width:140px;padding:6px 8px;border-width:1px;border-style:solid;border-radius:6px;font-size:11.5px;background:transparent;color:inherit;outline:none;" />' +
					'</div>' +
					'<div style="display:flex;gap:6px;align-items:center;">' +
					'  <input id="wa-sched-whentext" placeholder="or type: 2h, 21:00, tomorrow 08:00, fri 18:00" style="flex:1;min-width:0;padding:6px 8px;border-width:1px;border-style:solid;border-radius:6px;font-size:11.5px;background:transparent;color:inherit;outline:none;" />' +
					'</div>' +
					'<div id="wa-sched-whenpreview" class="wa-text-muted" style="font-size:11px;display:none;"></div>' +
					'<div style="display:flex;gap:6px;align-items:center;">' +
					'  <input id="wa-sched-when" type="datetime-local" style="flex:1;min-width:0;padding:6px 8px;border-width:1px;border-style:solid;border-radius:6px;font-size:11.5px;background:transparent;color:inherit;outline:none;" />' +
					'  <select id="wa-sched-repeat" style="padding:6px 8px;border-width:1px;border-style:solid;border-radius:6px;font-size:11.5px;background:transparent;color:inherit;outline:none;flex-shrink:0;">' +
					'    <option value="once">Once</option>' +
					'    <option value="daily">Daily</option>' +
					'    <option value="weekly">Weekly</option>' +
					'    <option value="monthly">Monthly</option>' +
					'    <option value="yearly">Yearly</option>' +
					'  </select>' +
					'  <select id="wa-sched-repday" title="Weekly: which day" style="display:none;padding:6px 8px;border-width:1px;border-style:solid;border-radius:6px;font-size:11.5px;background:transparent;color:inherit;outline:none;flex-shrink:0;">' +
					'    <option value="0">Sun</option>' +
					'    <option value="1">Mon</option>' +
					'    <option value="2">Tue</option>' +
					'    <option value="3">Wed</option>' +
					'    <option value="4">Thu</option>' +
					'    <option value="5">Fri</option>' +
					'    <option value="6">Sat</option>' +
					'  </select>' +
					'  <button id="wa-sched-add" class="wa-card-btn" style="padding:6px 12px;border-radius:6px;font-size:11.5px;font-weight:600;cursor:pointer;border-width:1px;border-style:solid;flex-shrink:0;">Add</button>' +
					'</div>' +
					'<div style="display:flex;align-items:center;justify-content:space-between;gap:8px;">' +
					'  <span class="wa-text-muted" style="font-size:11px;">Recent sends:</span>' +
					'  <button id="wa-sched-logcopy" class="wa-card-btn" style="padding:2px 8px;border-radius:6px;font-size:10.5px;cursor:pointer;border-width:1px;border-style:solid;">Copy log</button>' +
					'</div>' +
					'<div id="wa-sched-log" style="display:flex;flex-direction:column;gap:4px;"></div>';
				setBody.appendChild(schedSection);

				// Section 2d: AFK auto-reply (one reply per chat while away).
				var afkSection = document.createElement('div');
				afkSection.className = 'wa-modal-card';
				afkSection.setAttribute('data-cat', 'automation');
				afkSection.setAttribute('data-name', 'afk auto reply away cooldown groups emergency stop hours schedule night');
				afkSection.innerHTML = '' +
					'<div style="display:flex;align-items:center;justify-content:space-between;">' +
					'  <strong class="wa-text-primary" style="font-size:12.5px;">🤖 AFK Auto-Reply</strong>' +
					'</div>' +
					'<div class="wa-text-muted" style="font-size:11px;">While armed, a chat with new unread gets one automatic reply per cooldown. Turns itself off the moment you send a message. While armed this profile stays awake.</div>' +
					'<div id="wa-afk-status" class="wa-text-muted" style="font-size:11px;"></div>' +
					'<label style="display:flex;align-items:center;gap:8px;cursor:pointer;user-select:none;">' +
					'  <input type="checkbox" id="wa-afk-enable" style="width:14px;height:14px;accent-color:#00a884;cursor:pointer;margin:0;" />' +
					'  <span class="wa-text-muted" style="font-size:11px;">Enable AFK auto-reply</span>' +
					'</label>' +
					'<textarea id="wa-afk-text" rows="2" maxlength="500" placeholder="Reply message... variables: {date} {time} {name}" style="width:100%;padding:6px 8px;border-width:1px;border-style:solid;border-radius:6px;font-size:11.5px;background:transparent;color:inherit;outline:none;resize:vertical;font-family:inherit;"></textarea>' +
					'<div style="display:flex;gap:10px;align-items:center;flex-wrap:wrap;">' +
					'  <span class="wa-text-muted" style="font-size:11px;">Cooldown per chat:</span>' +
					'  <select id="wa-afk-cooldown" style="padding:5px 8px;border-width:1px;border-style:solid;border-radius:6px;font-size:11.5px;background:transparent;color:inherit;outline:none;">' +
					'    <option value="5">5 min</option>' +
					'    <option value="10">10 min</option>' +
					'    <option value="30">30 min</option>' +
					'    <option value="60">60 min</option>' +
					'  </select>' +
					'  <label style="display:flex;align-items:center;gap:6px;cursor:pointer;user-select:none;">' +
					'    <input type="checkbox" id="wa-afk-groups" style="width:14px;height:14px;accent-color:#00a884;cursor:pointer;margin:0;" />' +
					'    <span class="wa-text-muted" style="font-size:11px;">Also reply in groups (best-effort)</span>' +
					'  </label>' +
					'</div>' +
					'<label style="display:flex;align-items:center;gap:8px;cursor:pointer;user-select:none;">' +
					'  <input type="checkbox" id="wa-afk-window" style="width:14px;height:14px;accent-color:#00a884;cursor:pointer;margin:0;" />' +
					'  <span class="wa-text-muted" style="font-size:11px;">Only reply between these hours (e.g. overnight 21:00–07:00)</span>' +
					'</label>' +
					'<div style="display:flex;gap:8px;align-items:center;flex-wrap:wrap;">' +
					'  <input id="wa-afk-start" type="time" value="21:00" style="padding:5px 8px;border-width:1px;border-style:solid;border-radius:6px;font-size:11.5px;background:transparent;color:inherit;outline:none;" />' +
					'  <span class="wa-text-muted" style="font-size:11.5px;">–</span>' +
					'  <input id="wa-afk-end" type="time" value="07:00" style="padding:5px 8px;border-width:1px;border-style:solid;border-radius:6px;font-size:11.5px;background:transparent;color:inherit;outline:none;" />' +
					'  <span id="wa-afk-windowhint" class="wa-text-muted" style="font-size:11px;"></span>' +
					'</div>' +
					'<label style="display:flex;align-items:center;gap:8px;cursor:pointer;user-select:none;">' +
					'  <input type="checkbox" id="wa-afk-allowonly" style="width:14px;height:14px;accent-color:#00a884;cursor:pointer;margin:0;" />' +
					'  <span class="wa-text-muted" style="font-size:11px;">Only reply to the chats/numbers below</span>' +
					'</label>' +
					'<textarea id="wa-afk-allow" rows="3" maxlength="2000" placeholder="One per line: name or number — e.g. ibnu, kopikilo.id, 62812345678" style="width:100%;padding:6px 8px;border-width:1px;border-style:solid;border-radius:6px;font-size:11.5px;background:transparent;color:inherit;outline:none;resize:vertical;font-family:inherit;"></textarea>' +
					'<div class="wa-text-muted" style="font-size:11px;">Names match partially (case-insensitive); numbers match by digits, with or without +62 / leading 0.</div>' +
					'<label style="display:flex;align-items:center;gap:8px;cursor:pointer;user-select:none;">' +
					'  <input type="checkbox" id="wa-afk-kill" style="width:14px;height:14px;accent-color:#f15c6d;cursor:pointer;margin:0;" />' +
					'  <span class="wa-text-muted" style="font-size:11px;">Emergency stop: halt all AFK auto-replies</span>' +
					'</label>' +
					'<div class="wa-text-muted" style="font-size:11px;">Recent activity:</div>' +
					'<div id="wa-afk-log" style="display:flex;flex-direction:column;gap:4px;"></div>' +
					'<div style="display:flex;align-items:center;gap:8px;flex-wrap:wrap;">' +
					'  <button id="wa-afk-probe" class="wa-card-btn" style="padding:4px 10px;border-radius:6px;font-size:11px;cursor:pointer;border-width:1px;border-style:solid;flex-shrink:0;">Diagnose scan</button>' +
					'  <span class="wa-text-muted" style="font-size:11px;">No unread found? This shows what the scan sees.</span>' +
					'</div>' +
					'<div id="wa-afk-probeout" class="wa-text-muted" style="display:none;font-size:10.5px;font-family:monospace;white-space:pre-wrap;word-break:break-word;user-select:text;-webkit-user-select:text;"></div>';
				setBody.appendChild(afkSection);

				// Section 3: Maintenance & Update Actions
				var actionsSection = document.createElement('div');
				actionsSection.className = 'wa-modal-card';
				actionsSection.setAttribute('data-cat', 'help');
				actionsSection.setAttribute('data-name', 'maintenance update reload cache welcome guide restart');
				actionsSection.innerHTML = '' +
					'<strong class="wa-text-primary" style="font-size:12.5px;">Maintenance</strong>' +
					'<div style="display:grid;grid-template-columns:1fr 1fr;gap:6px;">' +
					'  <button id="wa-btn-check-updates-modal" class="wa-card-btn" style="padding:6px 8px;border-radius:6px;font-size:11.5px;font-weight:500;cursor:pointer;border-width:1px;border-style:solid;text-align:center;">Check for updates</button>' +
					'  <button id="wa-btn-reload-modal" class="wa-card-btn" style="padding:6px 8px;border-radius:6px;font-size:11.5px;font-weight:500;cursor:pointer;border-width:1px;border-style:solid;text-align:center;">Reload chat</button>' +
					'  <button id="wa-btn-hardref-modal" class="wa-card-btn" style="padding:6px 8px;border-radius:6px;font-size:11.5px;font-weight:500;cursor:pointer;border-width:1px;border-style:solid;text-align:center;">Clear cache</button>' +
					'  <button id="wa-btn-onboard-modal" class="wa-card-btn" style="padding:6px 8px;border-radius:6px;font-size:11.5px;font-weight:500;cursor:pointer;border-width:1px;border-style:solid;text-align:center;">View welcome guide</button>' +
					'</div>';
				setBody.appendChild(actionsSection);

				// Section 4: Help & local diagnostics. This intentionally performs no
				// network request and never reads chat data; it only validates the
				// small native bridge surface used by the application.
				var helpSection = document.createElement('div');
				helpSection.className = 'wa-modal-card';
				helpSection.setAttribute('data-cat', 'help');
				helpSection.setAttribute('data-name', 'help diagnostics shortcuts check bridge storage keyboard');
				helpSection.innerHTML = '' +
					'<div style="display:flex;align-items:center;justify-content:space-between;gap:12px;">' +
					'  <div><strong class="wa-text-primary" style="font-size:12.5px;">Help & diagnostics</strong><div class="wa-text-muted" style="font-size:11px;margin-top:2px;">Check the app surface locally. No chats or files are sent.</div></div>' +
					'  <button id="wa-btn-run-diagnostics" class="wa-card-btn" style="padding:6px 10px;border-radius:6px;font-size:11.5px;font-weight:500;cursor:pointer;border-width:1px;border-style:solid;white-space:nowrap;">Run quick check</button>' +
					'</div>' +
					'<div id="wa-diagnostics-result" class="wa-text-muted" aria-live="polite" style="display:none;font-size:10.5px;line-height:1.45;border-radius:6px;padding:7px 8px;"></div>' +
					'<button id="wa-btn-show-shortcuts" style="align-self:flex-start;background:transparent;border:none;color:#00a884;font-size:11px;cursor:pointer;padding:2px 0;">View keyboard shortcuts</button>' +
					'<div id="wa-shortcuts-list" class="wa-text-muted" style="display:none;font-size:10.5px;line-height:1.65;"></div>';
				setBody.appendChild(helpSection);

				// Footer rail (disclaimer + actions) — always visible at the
				// end of the scroll, outside the tab-filtered body.
				var footWrap = document.createElement('div');
				footWrap.className = 'wa-set-foot';
				modal.appendChild(footWrap);

				// Disclaimer
				var disclaimer = document.createElement('div');
				disclaimer.className = 'wa-text-muted';
				disclaimer.style.cssText = 'font-size:10px;line-height:1.4;border-top-width:1px;border-top-style:solid;padding-top:8px;margin-bottom:8px;';
				disclaimer.innerHTML = '<strong>WAtchful</strong> is an independent application and is not affiliated with Meta.';
				footWrap.appendChild(disclaimer);

				// Footer
				var footer = document.createElement('div');
				footer.style.cssText = 'display:flex;justify-content:space-between;align-items:center;margin-top:2px;';
				footer.innerHTML = '<span class="wa-text-muted" style="font-size:10.5px;">Press <kbd style="padding:1px 3px;border-radius:3px;font-family:monospace;">Esc</kbd> to close</span>';
				var footLeft = document.createElement('div');
				footLeft.style.cssText = 'display:flex;align-items:center;gap:8px;';
				var btnReport = document.createElement('button');
				btnReport.textContent = '🐞 Report issue';
				btnReport.id = 'wa-btn-report';
				btnReport.title = 'Open a pre-filled GitHub issue with recent errors (nothing is sent automatically)';
				btnReport.style.cssText = 'background:transparent;border:none;color:#8696a0;font-size:11px;cursor:pointer;padding:5px 8px;';
				btnReport.onclick = function() {
					closeSettings();
					if (window.reportIssueNow) window.reportIssueNow();
				};
				var btnDone = document.createElement('button');
				btnDone.textContent = 'Done';
				btnDone.id = 'wa-btn-done';
				btnDone.style.cssText = 'padding:5px 16px;border-radius:6px;font-size:11.5px;font-weight:600;cursor:pointer;border-width:1px;border-style:solid;';
				footLeft.appendChild(btnReport);
				footLeft.appendChild(btnDone);
				footer.appendChild(footLeft);
				footWrap.appendChild(footer);

				overlay.appendChild(modal);
				document.body.appendChild(overlay);
				modal.addEventListener('pointerdown', function(e) { e.stopPropagation(); });
				modal.addEventListener('click', function(e) { e.stopPropagation(); });

				function closeSettings() {
					window.removeEventListener('keydown', onKeyClose);
					window.removeEventListener('resize', reflowTools);
					try { if (afkStatusTimer) clearInterval(afkStatusTimer); } catch (e) {}
					afkStatusTimer = null;
					window.syncModalTheme = null;
					if (overlay.parentNode) overlay.parentNode.removeChild(overlay);
				}
				function onKeyClose(e) {
					if (e.key === 'Escape') closeSettings();
				}
				window.addEventListener('keydown', onKeyClose);
				btnDone.onclick = closeSettings;
				document.getElementById('wa-settings-close-x').onclick = closeSettings;
				overlay.onclick = function(e) {
					if (e.target === overlay) closeSettings();
				};

				// Category tabs + live search. Sections carry data-cat; only
				// the active tab's sections show (the panel stays scannable
				// instead of one long scroll). Search overrides tabs: while a
				// query is typed every matching section shows with a small
				// category label, so users find "zoom" or "startup" without
				// knowing which tab it lives in.
				var setTabs = Array.prototype.slice.call(modal.querySelectorAll('.wa-set-tab'));
				var setSections = Array.prototype.slice.call(setBody.querySelectorAll('[data-cat]'));
				var setNoResult = document.createElement('div');
				setNoResult.className = 'wa-text-muted';
				setNoResult.style.cssText = 'font-size:11.5px;padding:8px 0;display:none;';
				setNoResult.textContent = 'No settings match your search.';
				setBody.appendChild(setNoResult);
				var activeTab = 'general';
				function applyTab() {
					for (var i = 0; i < setSections.length; i++) {
						setSections[i].style.display =
							(setSections[i].getAttribute('data-cat') === activeTab) ? '' : 'none';
					}
					for (var j = 0; j < setTabs.length; j++) {
						setTabs[j].classList.toggle('wa-on',
							setTabs[j].getAttribute('data-cat') === activeTab);
					}
					setNoResult.style.display = 'none';
				}
				function applySearch(q) {
					q = (q || '').trim().toLowerCase();
					if (!q) { applyTab(); return; }
					var anyVisible = false;
					for (var i = 0; i < setSections.length; i++) {
						var hay = (setSections[i].textContent || '').toLowerCase();
						var match = hay.indexOf(q) !== -1;
						setSections[i].style.display = match ? '' : 'none';
						if (match) anyVisible = true;
					}
					for (var j = 0; j < setTabs.length; j++) {
						setTabs[j].classList.remove('wa-on');
					}
					setNoResult.style.display = anyVisible ? 'none' : '';
				}
				for (var ti = 0; ti < setTabs.length; ti++) {
					(function(tab) {
						tab.onclick = function() {
							activeTab = tab.getAttribute('data-cat');
							var searchInput = document.getElementById('wa-settings-search');
							if (searchInput) searchInput.value = '';
							applyTab();
						};
					})(setTabs[ti]);
				}
				var setSearch = document.getElementById('wa-settings-search');
				if (setSearch) {
					setSearch.oninput = function() { applySearch(setSearch.value); };
				}
				applyTab();
				// Pin the tool rail right below the sticky header (and keep the
				// offset fresh if the header reflows on a narrow window).
				var reflowTools = function() { toolsRow.style.top = header.offsetHeight + 'px'; };
				reflowTools();
				window.addEventListener('resize', reflowTools);

				// Styling Synchronizer for Modal (Dark / Light Theme).
				// One pass writes CSS custom properties on the panel; the
				// scoped stylesheet derives cards, pills, inputs, switches
				// and hover states from them, so the new layout repaints
				// without per-element inline overrides fighting the CSS.
				window.syncModalTheme = function(isThemeDark) {
					var bg = isThemeDark ? '#111b21' : '#ffffff';
					var surface = isThemeDark ? '#18262e' : '#f7f8fa';
					var border = isThemeDark ? '#2a3942' : '#d1d7db';
					var textPri = isThemeDark ? '#e9edef' : '#111b21';
					var textMut = isThemeDark ? '#8696a0' : '#667781';
					var accent = isThemeDark ? '#00a884' : '#008069';
					var field = isThemeDark ? '#111b21' : '#ffffff';

					modal.style.setProperty('--w-bg', bg);
					modal.style.setProperty('--w-surface', surface);
					modal.style.setProperty('--w-border', border);
					modal.style.setProperty('--w-text', textPri);
					modal.style.setProperty('--w-muted', textMut);
					modal.style.setProperty('--w-accent', accent);
					modal.style.setProperty('--w-field', field);
					modal.style.setProperty('--w-hover', isThemeDark ? '#1f2f39' : '#eef1f4');
					modal.style.setProperty('--w-accent-soft', isThemeDark ? 'rgba(0,168,132,.55)' : 'rgba(0,128,105,.5)');
					modal.style.colorScheme = isThemeDark ? 'dark' : 'light';
					modal.style.background = bg;
					modal.style.border = '1px solid ' + border;
					header.style.borderBottomColor = border;
					header.style.background = bg;
					document.getElementById('wa-modal-title').style.color = textPri;
					document.getElementById('wa-modal-sub').style.color = textMut;
					document.getElementById('wa-modal-icon-wrap').style.background = accent;
					document.getElementById('wa-modal-icon-wrap').style.color = accent;
					document.getElementById('wa-settings-close-x').style.color = textMut;

					// Text helpers anywhere inside the panel (including the
					// Profiles card injected later).
					document.querySelectorAll('#wa-settings-overlay .wa-text-primary').forEach(function(el) {
						el.style.color = textPri;
					});
					document.querySelectorAll('#wa-settings-overlay .wa-text-muted').forEach(function(el) {
						el.style.color = textMut;
					});

					// The Profiles card is injected after this pass with its
					// own inline colors — repaint it so light theme doesn't
					// keep the hardcoded dark surface.
					var profilesCard = document.getElementById('wa-profiles-card');
					if (profilesCard) {
						profilesCard.style.background = surface;
						profilesCard.style.borderColor = border;
					}

					var fBox = document.getElementById('wa-folder-box');
					if (fBox) {
						fBox.style.background = field;
						fBox.style.borderColor = border;
					}
					var fPath = document.getElementById('wa-folder-path');
					if (fPath) fPath.style.color = textMut;

					var btnOpen = document.getElementById('wa-btn-open-folder');
					if (btnOpen) {
						btnOpen.style.background = accent;
						btnOpen.style.color = isThemeDark ? '#111b21' : '#ffffff';
					}

					btnDone.style.background = isThemeDark ? '#202c33' : '#e9edef';
					btnDone.style.borderColor = border;
					btnDone.style.color = textPri;

					// Theme segment: active pill only (colors come from CSS vars).
					['dark', 'light', 'system'].forEach(function(mode) {
						var tBtn = document.getElementById('wa-theme-btn-' + mode);
						if (tBtn) tBtn.classList.toggle('wa-on', currentTheme === mode);
					});
				};

				// Synchronize Toggle Badges & Button States
				function updateBadges() {
					var isThemeDark = currentTheme === 'system' ?
						(window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches) :
						(currentTheme === 'dark');
					var accent = isThemeDark ? '#00a884' : '#008069';

					var privActive = window.isPrivacyModeActive ? window.isPrivacyModeActive() : false;
					var badgePriv = document.getElementById('wa-badge-priv');
					var btnPriv = document.getElementById('wa-action-toggle-priv');
					if (badgePriv && btnPriv) {
						badgePriv.textContent = privActive ? 'Enabled' : 'Disabled';
						badgePriv.style.background = privActive ? (isThemeDark ? 'rgba(0,168,132,0.15)' : 'rgba(0,128,105,0.15)') : 'transparent';
						badgePriv.style.color = privActive ? accent : '#8696a0';
						btnPriv.classList.toggle('wa-on', privActive);
						btnPriv.setAttribute('aria-checked', privActive ? 'true' : 'false');
					}

					var pinActive = window.isAlwaysOnTopActive ? window.isAlwaysOnTopActive() : false;
					var badgePin = document.getElementById('wa-badge-pin');
					var btnPin = document.getElementById('wa-action-toggle-pin');
					if (badgePin && btnPin) {
						badgePin.textContent = pinActive ? 'Pinned' : 'Unpinned';
						badgePin.style.background = pinActive ? (isThemeDark ? 'rgba(0,168,132,0.15)' : 'rgba(0,128,105,0.15)') : 'transparent';
						badgePin.style.color = pinActive ? accent : '#8696a0';
						btnPin.classList.toggle('wa-on', pinActive);
						btnPin.setAttribute('aria-checked', pinActive ? 'true' : 'false');
					}

					var muteActive = window.isAudioMuted ? window.isAudioMuted() : false;
					var badgeMute = document.getElementById('wa-badge-mute');
					var btnMute = document.getElementById('wa-action-toggle-mute');
					if (badgeMute && btnMute) {
						badgeMute.textContent = muteActive ? 'Muted' : 'Unmuted';
						badgeMute.style.background = muteActive ? 'rgba(234,0,56,0.15)' : 'transparent';
						badgeMute.style.color = muteActive ? '#ff5252' : accent;
						btnMute.classList.toggle('wa-on', muteActive);
						btnMute.setAttribute('aria-checked', muteActive ? 'true' : 'false');
					}

					var notifActive = window.isNotificationsEnabled ? window.isNotificationsEnabled() : true;
					var badgeNotif = document.getElementById('wa-badge-notif');
					var btnNotif = document.getElementById('wa-action-toggle-notif');
					if (badgeNotif && btnNotif) {
						badgeNotif.textContent = notifActive ? 'Enabled' : 'Disabled';
						badgeNotif.style.background = notifActive ? (isThemeDark ? 'rgba(0,168,132,0.15)' : 'rgba(0,128,105,0.15)') : 'transparent';
						badgeNotif.style.color = notifActive ? accent : '#8696a0';
						btnNotif.classList.toggle('wa-on', notifActive);
						btnNotif.setAttribute('aria-checked', notifActive ? 'true' : 'false');
					}

					var dlNotifActive = window.isNotifyOnDownload ? window.isNotifyOnDownload() : true;
					var badgeDlNotif = document.getElementById('wa-badge-dlnotif');
					var btnDlNotif = document.getElementById('wa-action-toggle-dlnotif');
					if (badgeDlNotif && btnDlNotif) {
						badgeDlNotif.textContent = dlNotifActive ? 'Enabled' : 'Disabled';
						badgeDlNotif.style.background = dlNotifActive ? (isThemeDark ? 'rgba(0,168,132,0.15)' : 'rgba(0,128,105,0.15)') : 'transparent';
						badgeDlNotif.style.color = dlNotifActive ? accent : '#8696a0';
						btnDlNotif.classList.toggle('wa-on', dlNotifActive);
						btnDlNotif.setAttribute('aria-checked', dlNotifActive ? 'true' : 'false');
					}

					var autoActive = window.isAutoStartActive ? window.isAutoStartActive() : false;
					var badgeAuto = document.getElementById('wa-badge-auto');
					var btnAuto = document.getElementById('wa-action-toggle-auto');
					if (badgeAuto && btnAuto) {
						badgeAuto.textContent = autoActive ? 'Enabled' : 'Disabled';
						badgeAuto.style.background = autoActive ? (isThemeDark ? 'rgba(0,168,132,0.15)' : 'rgba(0,128,105,0.15)') : 'transparent';
						badgeAuto.style.color = autoActive ? accent : '#8696a0';
						btnAuto.classList.toggle('wa-on', autoActive);
						btnAuto.setAttribute('aria-checked', autoActive ? 'true' : 'false');
					}

					window.syncModalTheme(isThemeDark);
				}
				updateBadges();

				// Hook Theme Segmented Control
				document.getElementById('wa-theme-btn-dark').onclick = function() {
					window.setAppTheme('dark');
					updateBadges();
				};
				document.getElementById('wa-theme-btn-light').onclick = function() {
					window.setAppTheme('light');
					updateBadges();
				};
				document.getElementById('wa-theme-btn-system').onclick = function() {
					window.setAppTheme('system');
					updateBadges();
				};

				// Hook Display Zoom stepper (drives the same persisted path as
				// the keyboard shortcuts, so both always agree on the level).
				window.syncZoomLabel = function() {
					var label = document.getElementById('wa-zoom-label');
					if (label && window.getPageZoom) {
						label.textContent = Math.round(window.getPageZoom() * 100) + '%';
					}
				};
				document.getElementById('wa-zoom-out').onclick = function() {
					if (window.stepPageZoom) window.stepPageZoom(-1, true);
				};
				document.getElementById('wa-zoom-in').onclick = function() {
					if (window.stepPageZoom) window.stepPageZoom(1, true);
				};
				document.getElementById('wa-zoom-reset').onclick = function() {
					if (window.setPageZoom) window.setPageZoom(1.0, true);
				};
				window.syncZoomLabel();

				// Hook Click Actions
				document.getElementById('wa-action-toggle-priv').onclick = function() {
					if (window.togglePrivacyMode) window.togglePrivacyMode();
					updateBadges();
				};
				var autoLockBox = document.getElementById('wa-priv-autolock');
				if (autoLockBox) {
					autoLockBox.checked = !!(window.isPrivacyAutoLock && window.isPrivacyAutoLock());
					autoLockBox.onchange = function() {
						if (window.setPrivacyAutoLock) window.setPrivacyAutoLock(autoLockBox.checked);
						showFloatingToast(autoLockBox.checked ?
							'🔒 Privacy auto-lock: on (blurs after 60s idle)' :
							'🔓 Privacy auto-lock: off');
					};
				}
				var avatarBox = document.getElementById('wa-blur-avatars');
				if (avatarBox) {
					avatarBox.checked = !!(window.isBlurAvatars && window.isBlurAvatars());
					avatarBox.onchange = function() {
						if (window.setBlurAvatars) window.setBlurAvatars(avatarBox.checked);
						showFloatingToast(avatarBox.checked ?
							'🙈 Profile photos: blurred (hover to peek)' :
							'🙉 Profile photos: visible');
					};
				}
				document.getElementById('wa-action-toggle-pin').onclick = function() {
					if (window.toggleAlwaysOnTop) {
						window.toggleAlwaysOnTop().then(function() { updateBadges(); });
					}
				};
				document.getElementById('wa-action-toggle-mute').onclick = function() {
					if (window.toggleMuteAudio) window.toggleMuteAudio();
					updateBadges();
				};
				document.getElementById('wa-action-toggle-notif').onclick = function() {
					if (window.setNotificationsEnabled && window.isNotificationsEnabled) {
						window.setNotificationsEnabled(!window.isNotificationsEnabled()).then(function() {
							showFloatingToast(window.isNotificationsEnabled() ? '🔔 Desktop notifications: on' : '🔕 Desktop notifications: off');
							updateBadges();
						});
					}
				};
				document.getElementById('wa-action-toggle-dlnotif').onclick = function() {
					if (window.setNotifyOnDownload && window.isNotifyOnDownload) {
						window.setNotifyOnDownload(!window.isNotifyOnDownload()).then(function() {
							updateBadges();
						});
					}
				};
				document.getElementById('wa-action-toggle-auto').onclick = function() {
					if (window.toggleAutoStart) {
						window.toggleAutoStart().then(function() { updateBadges(); });
					}
				};
				document.getElementById('wa-action-open-directchat').onclick = function() {
					if (window.openDirectChatModal) window.openDirectChatModal();
				};

				document.getElementById('wa-btn-check-updates-modal').onclick = function() {
					closeSettings();
					if (window.triggerCheckForUpdate) window.triggerCheckForUpdate();
				};
				document.getElementById('wa-btn-reload-modal').onclick = function() {
					if (window.reloadWhatsApp) window.reloadWhatsApp();
				};
				document.getElementById('wa-btn-hardref-modal').onclick = function() {
					if (window.hardRefreshWhatsApp) window.hardRefreshWhatsApp();
				};
				document.getElementById('wa-btn-onboard-modal').onclick = function() {
					closeSettings();
					if (window.showOnboardingModal) window.showOnboardingModal();
				};

				var shortcutsBtn = document.getElementById('wa-btn-show-shortcuts');
				var shortcutsList = document.getElementById('wa-shortcuts-list');
				if (shortcutsBtn && shortcutsList) {
					var modifier = isMac ? 'Cmd' : 'Ctrl';
					var tabRows = '';
					if (typeof window.cycleTabNative === 'function') {
						tabRows =
							'<div><strong class="wa-text-primary">' + modifier + '+Tab / ' + modifier + '+Shift+Tab</strong> &mdash; Next / previous account tab</div>' +
							'<div><strong class="wa-text-primary">' + modifier + '+1 &hellip; ' + modifier + '+9</strong> &mdash; Jump to account tab</div>';
					}
					shortcutsList.innerHTML =
						'<div><strong class="wa-text-primary">' + modifier + ',</strong> &mdash; Settings &amp; Controls</div>' +
						tabRows +
						'<div><strong class="wa-text-primary">' + modifier + '+Shift+C</strong> &mdash; Direct chat (new number)</div>' +
						'<div><strong class="wa-text-primary">' + modifier + '+Shift+D</strong> &mdash; Open downloads folder</div>' +
					'<div><strong class="wa-text-primary">' + modifier + '+Shift+H</strong> &mdash; Help &amp; onboarding</div>' +
						'<div><strong class="wa-text-primary">' + modifier + '+Shift+U</strong> &mdash; Check for updates</div>' +
						'<div><strong class="wa-text-primary">' + modifier + '+Shift+P / T / M / S</strong> &mdash; Privacy / on top / mute / startup</div>' +
						'<div><strong class="wa-text-primary">Esc</strong> &mdash; Close this window</div>';
					shortcutsBtn.onclick = function() {
						var open = shortcutsList.style.display !== 'none';
						shortcutsList.style.display = open ? 'none' : 'block';
						shortcutsBtn.textContent = open ? 'View keyboard shortcuts' : 'Hide keyboard shortcuts';
					};
				}

				var diagnosticsBtn = document.getElementById('wa-btn-run-diagnostics');
				var diagnosticsResult = document.getElementById('wa-diagnostics-result');
				if (diagnosticsBtn && diagnosticsResult) {
					diagnosticsBtn.onclick = function() {
						var checks = [];
						var requiredBindings = ['getDownloadDirNative', 'openDownloadDirNative', 'checkForUpdateNative', 'checkFileExistsNative'];
						var missing = requiredBindings.filter(function(name) { return typeof window[name] !== 'function'; });
						checks.push(missing.length ? 'Native bridge: unavailable (' + missing.join(', ') + ')' : 'Native bridge: ready');
						checks.push(navigator.onLine === false ? 'Network: offline (chat may not refresh)' : 'Network: available');
						try {
							var key = 'wa-desk-diagnostic-probe';
							localStorage.setItem(key, '1'); localStorage.removeItem(key);
							checks.push('Local settings storage: ready');
						} catch (e) { checks.push('Local settings storage: unavailable'); }
						var healthy = !missing.length && navigator.onLine !== false;
						diagnosticsResult.style.display = 'block';
						diagnosticsResult.style.background = healthy ? 'rgba(0,168,132,.10)' : 'rgba(234,0,56,.10)';
						diagnosticsResult.innerHTML = '<strong class="wa-text-primary">' + (healthy ? 'Quick check complete' : 'Attention needed') + '</strong><br>' + checks.map(function(line) { return '• ' + line; }).join('<br>');
					};
				}

				// Populate current download dir
				var pathLabel = document.getElementById('wa-folder-path');
				if (window.getDownloadDirNative) {
					window.getDownloadDirNative().then(function(dir) {
						if (pathLabel) pathLabel.textContent = dir;
					});
				}

				// Change folder action
				document.getElementById('wa-btn-change-folder').onclick = function() {
					if (window.chooseDownloadDirNative) {
						window.chooseDownloadDirNative().then(function(newDir) {
							if (newDir && pathLabel) {
								pathLabel.textContent = newDir;
								showFloatingToast('📁 Downloads folder updated!');
							}
						});
					}
				};

				// Open folder action
				document.getElementById('wa-btn-open-folder').onclick = function() {
					if (window.openDownloadDirNative) {
						window.openDownloadDirNative();
						showFloatingToast('📁 Opening folder in file manager...');
					}
				};

				// Reset folder action
				document.getElementById('wa-btn-reset-folder').onclick = function() {
					if (window.resetDownloadDirNative) {
						window.resetDownloadDirNative().then(function(defDir) {
							if (pathLabel) pathLabel.textContent = defDir;
							showFloatingToast('📁 Downloads folder reset to default.');
						});
					}
				};

				var organizeBox = document.getElementById('wa-organize-month');
				if (organizeBox) {
					if (window.getOrganizeByMonthNative) {
						window.getOrganizeByMonthNative().then(function(on) {
							organizeBox.checked = !!on;
						}).catch(function() {});
					}
					organizeBox.onchange = function() {
						if (!window.setOrganizeByMonthNative) return;
						window.setOrganizeByMonthNative(organizeBox.checked).then(function(applied) {
							showFloatingToast(applied ?
								'🗂️ Downloads will be organized into monthly folders.' :
								'🗂️ Downloads save directly to the folder again.');
						}).catch(function() {});
					};
				}

				// Quick Replies CRUD. User strings hit textContent only, never
				// innerHTML, so a template cannot inject markup into Settings.
				var qrList = document.getElementById('wa-qr-list');
				var qrTrigger = document.getElementById('wa-qr-trigger');
				var qrText = document.getElementById('wa-qr-text');
				function renderQuickReplies() {
					if (!qrList || !window.getQuickReplies) return;
					qrList.textContent = '';
					var list = window.getQuickReplies();
					if (!list.length) {
						var empty = document.createElement('div');
						empty.className = 'wa-text-muted';
						empty.style.cssText = 'font-size:11px;padding:2px 0;';
						empty.textContent = 'No quick replies yet — add one below.';
						qrList.appendChild(empty);
						return;
					}
					for (var i = 0; i < list.length; i++) {
						(function(item) {
							var row = document.createElement('div');
							row.style.cssText = 'display:flex;align-items:center;gap:8px;';
							var chip = document.createElement('span');
							chip.style.cssText = 'font-family:monospace;font-size:11px;font-weight:700;color:#00a884;flex-shrink:0;';
							chip.textContent = '/' + item.trigger;
							var preview = document.createElement('span');
							preview.className = 'wa-text-muted';
							preview.style.cssText = 'font-size:11px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;flex:1;min-width:0;';
							preview.textContent = item.text;
							var del = document.createElement('button');
							del.className = 'wa-card-btn';
							del.style.cssText = 'padding:3px 9px;border-radius:6px;font-size:11px;cursor:pointer;border-width:1px;border-style:solid;flex-shrink:0;';
							del.textContent = 'Delete';
							del.onclick = function() {
								if (window.deleteQuickReply) window.deleteQuickReply(item.trigger);
								renderQuickReplies();
							};
							row.appendChild(chip);
							row.appendChild(preview);
							row.appendChild(del);
							qrList.appendChild(row);
						})(list[i]);
					}
				}
				renderQuickReplies();
				var qrAdd = document.getElementById('wa-qr-add');
				if (qrAdd) {
					qrAdd.onclick = function() {
						if (!window.addQuickReply) return;
						var saved = window.addQuickReply(qrTrigger ? qrTrigger.value : '', qrText ? qrText.value : '');
						if (!saved) {
							showFloatingToast('⚠️ Trigger a-z/0-9 and non-empty text required');
							return;
						}
						if (qrTrigger) qrTrigger.value = '';
						if (qrText) qrText.value = '';
						renderQuickReplies();
						showFloatingToast('⚡ Quick reply /' + saved + ' saved');
					};
				}

				// Scheduler CRUD + log. User strings hit textContent only.
				var schedList = document.getElementById('wa-sched-list');
				var schedPhone = document.getElementById('wa-sched-phone');
				var schedText = document.getElementById('wa-sched-text');
				var schedWhen = document.getElementById('wa-sched-when');
				var schedRepeat = document.getElementById('wa-sched-repeat');
				var schedRepDay = document.getElementById('wa-sched-repday');
				// Weekly needs a weekday: show the picker only then, and
				// seed it from the parsed time until the user picks a
				// day themselves.
				var repDayTouched = false;
				var SCHED_DAYS_SHORT = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
				function syncRepDayVisibility() {
					if (schedRepDay) {
						schedRepDay.style.display =
							(schedRepeat && schedRepeat.value === 'weekly') ? '' : 'none';
					}
				}
				function seedRepDay(parsedMs) {
					if (!schedRepDay || repDayTouched || !(parsedMs > 0)) return;
					if (!schedRepeat || schedRepeat.value !== 'weekly') return;
					try { schedRepDay.value = String(new Date(parsedMs).getDay()); } catch (e) {}
				}
				if (schedRepeat) {
					schedRepeat.onchange = function() {
						syncRepDayVisibility();
						refreshSchedWhenPreview();
					};
				}
				if (schedRepDay) {
					schedRepDay.onchange = function() { repDayTouched = true; };
				}
				syncRepDayVisibility();
				var schedLog = document.getElementById('wa-sched-log');
				if (schedWhen) {
					try {
						var minD = new Date(Date.now() + 60000);
						var pad = function(v) { return (v < 10 ? '0' : '') + v; };
						schedWhen.min = minD.getFullYear() + '-' + pad(minD.getMonth() + 1) + '-' + pad(minD.getDate()) + 'T' + pad(minD.getHours()) + ':' + pad(minD.getMinutes());
					} catch (e) {}
				}
				function fmtSchedWhen(ms) {
					try {
						return new Date(ms).toLocaleString();
					} catch (e) {
						return String(ms);
					}
				}
				function renderSchedules() {
					if (!schedList || !window.getSchedules) return;
					schedList.textContent = '';
					var list = window.getSchedules();
					if (!list.length) {
						var empty = document.createElement('div');
						empty.className = 'wa-text-muted';
						empty.style.cssText = 'font-size:11px;padding:2px 0;';
						empty.textContent = 'No scheduled messages — add one below.';
						schedList.appendChild(empty);
					}
					for (var i = 0; i < list.length; i++) {
						(function(item) {
							var row = document.createElement('div');
							row.style.cssText = 'display:flex;align-items:center;gap:8px;';
							var chip = document.createElement('span');
							chip.style.cssText = 'font-family:monospace;font-size:11px;font-weight:700;color:#00a884;flex-shrink:0;';
							chip.textContent = '+' + item.phone;
							var mid = document.createElement('span');
							mid.className = 'wa-text-muted';
							mid.style.cssText = 'font-size:11px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;flex:1;min-width:0;';
							var rlabel = item.repeat;
							if (item.repeat === 'weekly' && typeof item.repeatDay === 'number') {
								rlabel += ' ' + (SCHED_DAYS_SHORT[item.repeatDay] || '');
							}
							mid.textContent = item.text + ' · ' + fmtSchedWhen(item.nextFire) + ' · ' + rlabel + (item.enabled ? '' : ' (off)');
							var tog = document.createElement('button');
							tog.className = 'wa-card-btn';
							tog.style.cssText = 'padding:3px 9px;border-radius:6px;font-size:11px;cursor:pointer;border-width:1px;border-style:solid;flex-shrink:0;';
							tog.textContent = item.enabled ? 'On' : 'Off';
							tog.onclick = function() {
								if (window.toggleSchedule) window.toggleSchedule(item.id, !item.enabled);
								renderSchedules();
							};
							var del = document.createElement('button');
							del.className = 'wa-card-btn';
							del.style.cssText = 'padding:3px 9px;border-radius:6px;font-size:11px;cursor:pointer;border-width:1px;border-style:solid;flex-shrink:0;';
							del.textContent = 'Delete';
							del.onclick = function() {
								if (window.deleteSchedule) window.deleteSchedule(item.id);
								renderSchedules();
								renderSchedLog();
							};
							row.appendChild(chip);
							row.appendChild(mid);
							row.appendChild(tog);
							row.appendChild(del);
							schedList.appendChild(row);
						})(list[i]);
					}
				}
				function renderSchedLog() {
					if (!schedLog || !window.getScheduleLog) return;
					schedLog.textContent = '';
					var log = window.getScheduleLog().slice(0, 10);
					if (!log.length) {
						var empty = document.createElement('div');
						empty.className = 'wa-text-muted';
						empty.style.cssText = 'font-size:11px;padding:2px 0;';
						empty.textContent = 'Nothing sent yet.';
						schedLog.appendChild(empty);
						return;
					}
					for (var i = 0; i < log.length; i++) {
						var line = document.createElement('div');
						line.className = 'wa-text-muted';
						line.style.cssText = 'font-size:11px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;';
						line.style.cssText = 'font-size:11px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;user-select:text;-webkit-user-select:text;cursor:text;';
						line.textContent = fmtSchedWhen(log[i].at) + ' · +' + log[i].phone + ' · ' + log[i].status + ' · ' + log[i].text;
						schedLog.appendChild(line);
					}
				}
				var schedLogCopy = document.getElementById('wa-sched-logcopy');
				if (schedLogCopy) {
					schedLogCopy.addEventListener('click', function() {
						try {
							var all = (window.getScheduleLog ? window.getScheduleLog() : []);
							var txt = '';
							for (var i = 0; i < all.length; i++) {
								txt += fmtSchedWhen(all[i].at) + ' · +' + all[i].phone + ' · ' + all[i].status + ' · ' + all[i].text + '\n';
							}
							if (!txt) txt = '(empty log)';
							var done = function() { showFloatingToast('📋 Scheduler log copied'); };
							if (navigator.clipboard && navigator.clipboard.writeText) {
								navigator.clipboard.writeText(txt).then(done, function() {});
							}
						} catch (e) {}
					});
				}
				renderSchedules();
				renderSchedLog();
				var schedKill = document.getElementById('wa-sched-kill');
				if (schedKill) {
					try {
						schedKill.checked = localStorage.getItem('wa_desk_sched_killed') === '1';
					} catch (e) {}
					schedKill.onchange = function() {
						if (window.setSchedulerKilled) window.setSchedulerKilled(schedKill.checked);
						showFloatingToast(schedKill.checked ?
							'🛑 Scheduler halted — no scheduled sends until re-enabled' :
							'⏰ Scheduler running again');
					};
				}
				var schedAdd = document.getElementById('wa-sched-add');
				var schedWhenText = document.getElementById('wa-sched-whentext');
				var schedWhenPreview = document.getElementById('wa-sched-whenpreview');
				function refreshSchedWhenPreview() {
					if (!schedWhenPreview) return;
					var raw = schedWhenText ? schedWhenText.value : '';
					if (!raw.trim()) {
						schedWhenPreview.style.display = 'none';
						return;
					}
					var parsed = window.parseSchedWhen ? window.parseSchedWhen(raw) : 0;
					schedWhenPreview.style.display = 'block';
					seedRepDay(parsed);
					var then = '';
					if (schedRepeat && schedRepeat.value && schedRepeat.value !== 'once') {
						then = ' · then ' + schedRepeat.value;
						if (schedRepeat.value === 'weekly' && schedRepDay) {
							then += ' ' + (SCHED_DAYS_SHORT[Number(schedRepDay.value)] || '');
						}
					}
					try {
						schedWhenPreview.textContent = parsed > Date.now() ?
							'→ ' + new Date(parsed).toLocaleString() + then :
							'not understood — try "2h", "21:00", "tomorrow 08:00" or "fri 18:00"';
					} catch (e) {
						schedWhenPreview.textContent = 'not understood';
					}
				}
				if (schedWhenText) {
					schedWhenText.addEventListener('input', refreshSchedWhenPreview);
				}
				if (schedAdd) {
					schedAdd.onclick = function() {
						if (!window.addSchedule) return;
						// Typed natural time wins over the picker when present.
						var typed = schedWhenText ? schedWhenText.value.trim() : '';
						var whenMs = 0;
						if (typed) {
							whenMs = window.parseSchedWhen ? window.parseSchedWhen(typed) : 0;
							if (!(whenMs > Date.now())) {
								showFloatingToast('⚠️ Time not understood — try "2h", "21:00", "tomorrow 08:00" or "fri 18:00"');
								return;
							}
						} else {
							whenMs = schedWhen && schedWhen.value ? new Date(schedWhen.value).getTime() : 0;
						}
						if (whenMs > 0) seedRepDay(whenMs);
						var id = window.addSchedule(
							schedPhone ? schedPhone.value : '',
							schedText ? schedText.value : '',
							whenMs,
							schedRepeat ? schedRepeat.value : 'once',
							schedRepDay ? schedRepDay.value : '');
						if (!id) {
							showFloatingToast('⚠️ Valid number, message and future time required');
							return;
						}
						if (schedPhone) schedPhone.value = '';
						if (schedText) schedText.value = '';
						if (schedWhenText) {
							schedWhenText.value = '';
							refreshSchedWhenPreview();
						}
						renderSchedules();
						showFloatingToast('⏰ Message scheduled');
					};
				}

				// AFK auto-reply settings + activity log. User strings hit
				// textContent only.
				var afkStatusTimer = null;
				var afkEnable = document.getElementById('wa-afk-enable');
				var afkText = document.getElementById('wa-afk-text');
				var afkCooldown = document.getElementById('wa-afk-cooldown');
				var afkGroups = document.getElementById('wa-afk-groups');
				var afkWindow = document.getElementById('wa-afk-window');
				var afkStart = document.getElementById('wa-afk-start');
				var afkEnd = document.getElementById('wa-afk-end');
				var afkHint = document.getElementById('wa-afk-windowhint');
				var afkKill = document.getElementById('wa-afk-kill');
				var afkLogBox = document.getElementById('wa-afk-log');
				var afkAllowOnly = document.getElementById('wa-afk-allowonly');
				var afkAllow = document.getElementById('wa-afk-allow');
				var afkStatusBox = document.getElementById('wa-afk-status');
				var afkProbeBtn = document.getElementById('wa-afk-probe');
				var afkProbeOut = document.getElementById('wa-afk-probeout');
				function renderAfkStatus() {
					if (!afkStatusBox || !window.__waAfkStatus) return;
					try {
						var st = window.__waAfkStatus();
						afkStatusBox.textContent = (st.armed ? '● ' : '○ ') + st.reason;
					} catch (e) {}
				}
				function renderAfkLog() {
					if (!afkLogBox || !window.getAfkLog) return;
					afkLogBox.textContent = '';
					var log = window.getAfkLog().slice(0, 8);
					if (!log.length) {
						var empty = document.createElement('div');
						empty.className = 'wa-text-muted';
						empty.style.cssText = 'font-size:11px;padding:2px 0;';
						empty.textContent = 'No auto-replies yet.';
						afkLogBox.appendChild(empty);
						return;
					}
					for (var i = 0; i < log.length; i++) {
						var line = document.createElement('div');
						line.className = 'wa-text-muted';
						line.style.cssText = 'font-size:11px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;';
						try {
							line.textContent = new Date(log[i].at).toLocaleString() + ' · ' + (log[i].chat || '—') + ' · ' + log[i].status;
						} catch (e) {
							line.textContent = String(log[i].status || '');
						}
						afkLogBox.appendChild(line);
					}
				}
				if (window.getAfkConfig) {
					var afkCfg = window.getAfkConfig();
					if (afkEnable) afkEnable.checked = !!afkCfg.enabled;
					if (afkText) afkText.value = afkCfg.text;
					if (afkCooldown) afkCooldown.value = String(afkCfg.cooldownMin);
					if (afkGroups) afkGroups.checked = !!afkCfg.groups;
					if (afkAllowOnly) afkAllowOnly.checked = !!afkCfg.allowOnly;
					if (afkAllow) afkAllow.value = (afkCfg.allowed || []).join('\n');
					// Reply window: seed the time pickers from stored minutes
					// (or the 21:00-07:00 default) and dim them while the
					// window is off.
					function minsToHM(m) {
						m = (m >= 0 && m < 1440) ? Math.floor(m) : 0;
						var h = Math.floor(m / 60), mm = m % 60;
						return (h < 10 ? '0' : '') + h + ':' + (mm < 10 ? '0' : '') + mm;
					}
					function syncAfkWindowUI(cfg) {
						if (afkWindow) afkWindow.checked = !!cfg.useWindow;
						if (afkStart) {
							afkStart.value = minsToHM(cfg.startMin);
							afkStart.disabled = !cfg.useWindow;
						}
						if (afkEnd) {
							afkEnd.value = minsToHM(cfg.endMin);
							afkEnd.disabled = !cfg.useWindow;
						}
						if (afkHint) {
							if (!cfg.useWindow) {
								afkHint.textContent = '';
							} else {
								var active = window.__waAfkInWindow ?
									window.__waAfkInWindow(cfg, Date.now()) : true;
								afkHint.textContent = (active ? '● active now' : '○ outside hours') +
									' ' + minsToHM(cfg.startMin) + '–' + minsToHM(cfg.endMin) +
									((cfg.startMin > cfg.endMin) ? ' (overnight)' : '');
							}
						}
					}
					syncAfkWindowUI(afkCfg);
					var syncAfk = function() {
						if (!window.setAfkConfig) return;
						var saved = window.setAfkConfig({
							enabled: !!(afkEnable && afkEnable.checked),
							text: afkText ? afkText.value : '',
							cooldownMin: parseInt(afkCooldown && afkCooldown.value, 10),
							groups: !!(afkGroups && afkGroups.checked),
							useWindow: !!(afkWindow && afkWindow.checked),
							startMin: afkStart ? afkStart.value : '',
							endMin: afkEnd ? afkEnd.value : '',
							allowOnly: !!(afkAllowOnly && afkAllowOnly.checked),
							allowed: afkAllow ? afkAllow.value : ''
						});
						syncAfkWindowUI(saved);
						if (afkEnable && afkEnable.checked && !saved.enabled) {
							afkEnable.checked = false;
							showFloatingToast('⚠️ AFK reply message is empty');
						}
					};
					if (afkEnable) afkEnable.onchange = syncAfk;
					if (afkText) afkText.onchange = syncAfk;
					if (afkCooldown) afkCooldown.onchange = syncAfk;
					if (afkGroups) afkGroups.onchange = syncAfk;
					if (afkAllowOnly) afkAllowOnly.onchange = syncAfk;
					if (afkAllow) afkAllow.onchange = syncAfk;
					if (afkWindow) afkWindow.onchange = syncAfk;
					if (afkStart) afkStart.onchange = syncAfk;
					if (afkEnd) afkEnd.onchange = syncAfk;
					renderAfkLog();
					renderAfkStatus();
					if (afkProbeBtn) {
						afkProbeBtn.onclick = function() {
							if (!window.__waAfkProbe || !afkProbeOut) return;
							var p = {};
							try { p = window.__waAfkProbe(); } catch (e) {}
							var lines = [
								'pane-side: ' + (p.paneSide ? 'yes' : 'MISSING (body fallback)'),
								'chat rows: ' + p.rows,
								'unread-pill badges: ' + p.pills,
								'aria unread: ' + p.ariaUnread + ' · belum-dibaca: ' + p.ariaID,
								'titled row buttons: ' + p.titledButtons,
								'tab title: ' + (p.docTitle || '(empty)'),
								'candidates: ' + ((p.candidates && p.candidates.length) ? p.candidates.join(', ') : '(none)')
							];
							for (var i = 0; i < ((p.samples && p.samples.length) || 0); i++) {
								lines.push('row' + i + ' buttons: ' + (p.samples[i].buttons.join(' | ') || '(none)'));
								lines.push('row' + i + ' testids: ' + (p.samples[i].testids.join(', ') || '(none)'));
							}
							for (var u2 = 0; u2 < ((p.unreadEls && p.unreadEls.length) || 0); u2++) {
								var ue = p.unreadEls[u2];
								lines.push('unread' + u2 + ': <' + ue.tag + '> tid=' + (ue.testid || '-') +
									' aria="' + ue.aria + '" row=' + ue.row + ' <- ' + ue.path.join(' < '));
							}
							for (var r2 = 0; r2 < ((p.rowDetail && p.rowDetail.length) || 0); r2++) {
								var rd = p.rowDetail[r2];
								lines.push('detail' + r2 + ' row: ' + rd.rowAttrs +
									' nameLen=' + rd.nameLen + ' titleTids=' + rd.titleTidCount);
								lines.push('detail' + r2 + ' click: ' + (rd.clickables.join(' | ') || '(none)'));
							}
							if (p.error) lines.push('error: ' + p.error);
							lines.push('main: found=' + (p.mainFound ? 'yes' : 'no') +
								' footer=' + (p.mainFooter ? 'yes' : 'no') +
								' editables=' + p.mainEditables +
								' header="' + (p.mainHeader || '-') + '"');
							if (p.mainHeaderAll && p.mainHeaderAll.length) {
								lines.push('main-headers: ' + p.mainHeaderAll.join(' | '));
							}
							if (p.mainHeaderText) lines.push('main-header-text: ' + p.mainHeaderText);
							if (p.mainHeaderHTML) lines.push('main-header-html: ' + p.mainHeaderHTML);
							lines.push('probe: ' + (p.probe || '?'));
							lines.push('app: ' + ((p.appShell && p.appShell.join(' ')) || '(no #app)'));
							lines.push('app-deep: ' + ((p.appDeep && p.appDeep.join(' ')) || '-'));
							lines.push('doc-wide: footers=' + p.docFooters + ' editables=' + p.docEditables);
							if (p.titleHTML) lines.push('titleHTML: ' + p.titleHTML);
							afkProbeOut.style.display = 'block';
							afkProbeOut.textContent = lines.join('\n');
						};
					}
					// Keep the status line live while the panel is open; the
					// closer below stops the timer with the overlay.
					afkStatusTimer = setInterval(function() {
						renderAfkStatus();
						renderAfkLog();
					}, 3000);
				}
				if (afkKill) {
					try { afkKill.checked = localStorage.getItem('wa_desk_afk_killed') === '1'; } catch (e) {}
					afkKill.onchange = function() {
						if (window.setAFKKilled) window.setAFKKilled(afkKill.checked);
						showFloatingToast(afkKill.checked ?
							'🛑 AFK auto-reply halted — no replies until re-enabled' :
							'🤖 AFK auto-reply ready');
					};
				}
			};

			// Keyboard Shortcut: Cmd/Ctrl + , (Settings) and Cmd/Ctrl + Shift + D (Open Download Folder)
			// Match the physical Comma key too (e.code / keyCode 188): some
			// keyboard layouts report '<' or another glyph in e.key for the
			// same key, which previously made Ctrl+, silently do nothing.
			window.addEventListener('keydown', function(e) {
				var isCommaKey = (e.key === ',' || e.key === '<') ||
					(e.code === 'Comma') || (e.keyCode === 188) || (e.which === 188);
				if ((e.metaKey || e.ctrlKey) && isCommaKey) {
					e.preventDefault();
					if (typeof window.showSettingsModal === 'function') {
						window.showSettingsModal();
					}
				} else if ((e.metaKey || e.ctrlKey) && e.shiftKey && (e.key === 'd' || e.key === 'D')) {
					e.preventDefault();
					if (window.openDownloadDirNative) {
						window.openDownloadDirNative();
						showFloatingToast('📁 Opening downloads folder...');
					}
				}
			}, true);

			// Account tabs (tabbed shell; every bridge call is guarded so
			// single-view builds and other platforms simply ignore these).
			window.addEventListener('keydown', function(e) {
				if (!(e.metaKey || e.ctrlKey) || e.altKey) return;
				if (e.key === 'Tab') {
					if (typeof window.cycleTabNative !== 'function') return;
					e.preventDefault();
					window.cycleTabNative(e.shiftKey ? -1 : 1);
				} else if (!e.shiftKey && e.key >= '1' && e.key <= '9' && e.key.length === 1) {
					if (typeof window.activateTabNative !== 'function') return;
					e.preventDefault();
					window.activateTabNative(parseInt(e.key, 10) - 1);
				}
			}, true);
		})();
	// The profiles UI must be injected LAST: it wraps window.showSettingsModal
	// (defined in the settings block above) to add the Profiles card. Injecting
	// it before the settings block would capture an undefined function and leave
	// Settings unreachable from the shortcut, toolbar button and tray menu.
	` + "\n" + getOnboardingScript() + "\n" + getProfilesUIScript()
	// Single source of truth: every UI version string flows from appVersion
	// (overridable at link time via -ldflags "-X main.appVersion=...").
	return strings.ReplaceAll(script, "__WA_APP_VERSION__", appVersion)
}

type WindowState struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	// Screens maps a stable display identifier (macOS NSScreenNumber) to the
	// frame the window had on that monitor. Only macOS populates it; other
	// platforms round-trip it unchanged.
	Screens map[string]WindowState `json:"screens,omitempty"`
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			writeCrashReport("main", r)
		}
	}()
	// Multi-profile: parse --profile <name> before anything else so the active
	// profile is known before the webview engine (and its user-data folder) is
	// created. The picker is enabled for webview-based platforms only.
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		if args[i] == "--profile" && i+1 < len(args) {
			profileFlagPending = args[i+1]
			i++
		}
	}
	if runtime.GOOS != "windows" {
		os.Setenv("WA_DESK_PICKER", "1")
	}
	initActiveProfile()
	runApp()
}
