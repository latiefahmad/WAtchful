package main

// In-page profile manager, injected with the rest of the init script. It adds
// a "Profiles" card to the WAtchful settings modal listing every account
// profile, with switch/create/rename/delete. Switching and creating relaunch
// the app into the chosen profile (each profile owns a separate browser user
// data dir, so accounts never log each other out).
//
// NOTE: window.confirm/prompt are not shown on the WKWebView build (the
// UIDelegate does not implement runJavaScriptConfirmPanelWithMessage), so
// destructive actions use inline two-step confirmation instead of dialogs.

func getProfilesUIScript() string {
	return `
		// --- Profile Manager (multi-account) ---
		(function() {
			function notify(msg) {
				// showFloatingToast is defined later in the init script; guard
				// so early or failed loads stay silent instead of throwing.
				if (typeof showFloatingToast === 'function') showFloatingToast(msg);
			}

			var state = {
				profiles: [],
				active: null,
				confirmDeleteId: null,
				confirmResetId: null
			};

			function esc(s) {
				return String(s).replace(/[&<>"']/g, function(c) {
					return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c];
				});
			}

			function sizeLabel(bytes) {
				var n = Number(bytes) || 0;
				if (n < 1024) return n + ' B';
				var units = ['KB', 'MB', 'GB', 'TB'];
				var v = n;
				var i = -1;
				do { v /= 1024; i++; } while (v >= 1024 && i < units.length - 1);
				return v.toFixed(1) + ' ' + units[i];
			}

			function refreshProfiles() {
				if (!window.listProfilesNative) return Promise.resolve();
				return Promise.resolve(window.listProfilesNative()).then(function(list) {
					state.profiles = Array.isArray(list) ? list : [];
					state.active = state.profiles.filter(function(p) { return p && p.active; })[0] || null;
				}).catch(function() {});
			}

			window.__refreshProfilesState = refreshProfiles;

			// Repaint hook for async native ops (tabbed delete/reset/rename):
			// the bridge resolves immediately and the shell repaints here once
			// the slow work completes, instead of blocking the UI thread.
			window.__repaintProfiles = function() {
				try {
					refreshProfiles().then(function() { renderProfiles(); });
				} catch (e) {}
			};

			function actionButtons(p) {
				// Rename is safe for the active profile too (it only edits the
				// registry name; the backend has no active-profile guard and
				// the tab strip refreshes afterwards). Switch is pointless for
				// the running profile, and Reset/Delete are refused by the
				// backend because this process still writes to that folder.
				if (p.active) {
					return '<div style="display:flex;align-items:center;gap:6px;">' +
						'<span class="wa-text-muted" style="font-size:10px;font-weight:600;padding:0 8px;">IN USE</span>' +
						'<button class="wa-card-btn" data-profile-rename="' + esc(p.id) + '" title="Rename" aria-label="Rename ' + esc(p.name) + '" style="padding:4px 8px;border-radius:6px;font-size:11px;cursor:pointer;border-width:1px;border-style:solid;">✏️</button>' +
						'</div>';
				}
				return '<div style="display:flex;gap:6px;">' +
					'<button class="wa-card-btn" data-profile-switch="' + esc(p.id) + '" style="padding:4px 10px;border-radius:6px;font-size:11px;font-weight:600;cursor:pointer;border-width:1px;border-style:solid;">Switch</button>' +
					'<button class="wa-card-btn" data-profile-reset="' + esc(p.id) + '" title="Reset session data (logs this account out)" aria-label="Reset data for ' + esc(p.name) + '" style="padding:4px 8px;border-radius:6px;font-size:11px;cursor:pointer;border-width:1px;border-style:solid;">♻️</button>' +
					'<button class="wa-card-btn" data-profile-rename="' + esc(p.id) + '" title="Rename" aria-label="Rename ' + esc(p.name) + '" style="padding:4px 8px;border-radius:6px;font-size:11px;cursor:pointer;border-width:1px;border-style:solid;">✏️</button>' +
					(p.id === 'default' ? '' :
						'<button class="wa-card-btn" data-profile-delete="' + esc(p.id) + '" title="Delete" aria-label="Delete ' + esc(p.name) + '" style="padding:4px 8px;border-radius:6px;font-size:11px;cursor:pointer;border-width:1px;border-style:solid;">🗑️</button>') +
					'</div>';
			}

			function renderProfiles() {
				var wrap = document.getElementById('wa-profile-list');
				if (!wrap) return;
				var rows = state.profiles.map(function(p) {
					var isConfirming = state.confirmDeleteId === p.id;
					var isConfirmingReset = state.confirmResetId === p.id;
					return '<div style="display:flex;align-items:center;justify-content:space-between;gap:10px;padding:8px 10px;border-radius:8px;" class="wa-profile-row">' +
						'<div style="display:flex;align-items:center;gap:10px;min-width:0;">' +
						'  <span style="width:28px;height:28px;border-radius:50%;background:#00a884;color:#111b21;display:flex;align-items:center;justify-content:center;font-weight:700;font-size:12px;flex:none;">' + esc(String(p.name || '?').trim().split(/\s+/).slice(0, 2).map(function(w) { return w[0]; }).join('').toUpperCase()) + '</span>' +
						'  <div style="min-width:0;">' +
						'    <div class="wa-text-primary" style="font-size:12.5px;font-weight:600;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;">' + esc(p.name) + '</div>' +
						'    <div class="wa-text-muted" style="font-size:10.5px;">' + (p.active ? 'Active session' : 'Separate logged-in account') +
						' · ' + sizeLabel(p.data_bytes) + (p.active ? ' (live)' : '') + '</div>' +
						'  </div>' +
						'</div>' +
						(isConfirming ?
							'<div style="display:flex;align-items:center;gap:6px;">' +
							'  <span class="wa-text-muted" style="font-size:10.5px;">Delete all data?</span>' +
							'  <button class="wa-card-btn" data-profile-delete-yes="' + esc(p.id) + '" style="padding:4px 10px;border-radius:6px;font-size:11px;font-weight:600;cursor:pointer;border-width:1px;border-style:solid;color:#f15c6d;">Yes</button>' +
							'  <button class="wa-card-btn" data-profile-delete-no style="padding:4px 10px;border-radius:6px;font-size:11px;cursor:pointer;border-width:1px;border-style:solid;">No</button>' +
							'</div>' :
						(isConfirmingReset ?
							'<div style="display:flex;align-items:center;gap:6px;">' +
							'  <span class="wa-text-muted" style="font-size:10.5px;">Reset session? This logs the account out.</span>' +
							'  <button class="wa-card-btn" data-profile-reset-yes="' + esc(p.id) + '" style="padding:4px 10px;border-radius:6px;font-size:11px;font-weight:600;cursor:pointer;border-width:1px;border-style:solid;color:#f15c6d;">Yes</button>' +
							'  <button class="wa-card-btn" data-profile-reset-no style="padding:4px 10px;border-radius:6px;font-size:11px;cursor:pointer;border-width:1px;border-style:solid;">No</button>' +
							'</div>' :
							actionButtons(p))) +
						'</div>';
				});
				wrap.innerHTML = rows.join('') ||
					'<div class="wa-text-muted" style="font-size:11px;padding:6px 2px;">No profiles yet.</div>';
			}

			function bindProfileActions(modal) {
				modal.addEventListener('click', function(e) {
					var t = e.target;
					if (!t || !t.closest) return;
					var sw = t.closest('[data-profile-switch]');
					if (sw) {
						var id = sw.getAttribute('data-profile-switch');
						var target = state.profiles.filter(function(p) { return p.id === id; })[0];
						if (target && window.switchProfileNative) {
							sw.disabled = true;
							var r = window.switchProfileNative(target.name);						if (r && r.then) r.then(function() {}, function() {});
						notify('🔄 Switching to "' + target.name + '"…');
					}
						return;
					}
					var rn = t.closest('[data-profile-rename]');
					if (rn) {
						var rid = rn.getAttribute('data-profile-rename');
						var row = rn.closest('.wa-profile-row');
						var nameEl = row ? row.querySelector('.wa-text-primary') : null;
						var oldName = nameEl ? nameEl.textContent : '';
						var target = state.profiles.filter(function(p) { return p.id === rid; })[0];
						if (!target) return;
						// Inline prompt row (no window.prompt on WKWebView).
						var input = document.createElement('input');
						input.type = 'text';
						input.maxLength = 40;
						input.value = oldName;
						input.style.cssText = 'flex:1;min-width:0;background:rgba(134,150,160,.15);border:1px solid rgba(134,150,160,.4);border-radius:6px;color:inherit;padding:4px 8px;font-size:12px;outline:none;';
						row.replaceChildren(input);
						input.focus();
						input.select();
						function save() {
							var newName = (input.value || '').trim();
							if (newName && newName !== oldName && window.renameProfileNative) {
								Promise.resolve(window.renameProfileNative(rid, newName)).then(function(ok) {
									if (ok === false) { notify('⚠️ Could not rename profile.'); return refreshProfiles().then(renderProfiles); }
									target.name = newName;
									renderProfiles();
									notify('✏️ Profile renamed to "' + newName + '".');
								}).catch(function() {});
							} else {
								renderProfiles();
							}
						}
						input.addEventListener('keydown', function(ev) {
							if (ev.key === 'Enter') { ev.preventDefault(); save(); }
							if (ev.key === 'Escape') { ev.stopPropagation(); renderProfiles(); }
						});
						input.addEventListener('blur', save);
						return;
					}
					var rst = t.closest('[data-profile-reset]');
					if (rst) {
						state.confirmDeleteId = null;
						state.confirmResetId = rst.getAttribute('data-profile-reset');
						renderProfiles();
						return;
					}
					if (t.closest('[data-profile-reset-no]')) {
						state.confirmResetId = null;
						renderProfiles();
						return;
					}
					var rstYes = t.closest('[data-profile-reset-yes]');
					if (rstYes) {
						var rsid = rstYes.getAttribute('data-profile-reset-yes');
						state.confirmResetId = null;
						var rsTarget = state.profiles.filter(function(p) { return p.id === rsid; })[0];
						if (window.resetProfileNative) {
							Promise.resolve(window.resetProfileNative(rsid)).then(function(ok) {
								if (ok === false) { notify('⚠️ Cannot reset this profile (in use or not found).'); return refreshProfiles().then(renderProfiles); }
								notify('♻️ Session data cleared' + (rsTarget ? ' for "' + rsTarget.name + '"' : '') + '.');
								return refreshProfiles().then(renderProfiles);
							}).catch(function() {});
						}
						return;
					}
					var del = t.closest('[data-profile-delete]');
					if (del) {
						state.confirmResetId = null;
						state.confirmDeleteId = del.getAttribute('data-profile-delete');
						renderProfiles();
						return;
					}
					if (t.closest('[data-profile-delete-no]')) {
						state.confirmDeleteId = null;
						renderProfiles();
						return;
					}
					var yes = t.closest('[data-profile-delete-yes]');
					if (yes) {
						var did = yes.getAttribute('data-profile-delete-yes');
						state.confirmDeleteId = null;
						if (window.deleteProfileNative) {
							Promise.resolve(window.deleteProfileNative(did)).then(function(ok) {
							if (ok === false) { notify('⚠️ Cannot delete this profile (in use or last profile).'); return; }
							notify('🗑️ Profile deleted.');
								return refreshProfiles().then(renderProfiles);
							}).catch(function() {});
						}
						return;
					}
				});
			}

			function ensureCreateRow(modal) {
				var createBtn = modal.querySelector('#wa-btn-create-profile');
				if (!createBtn || createBtn.dataset.wired) return;
				createBtn.dataset.wired = '1';
				createBtn.onclick = function() {
					var input = modal.querySelector('#wa-input-profile-name');
					if (!input) return;
					var name = (input.value || '').trim();
					if (!name) { input.focus(); return; }
					if (!window.createProfileNative) return;
					createBtn.disabled = true;
					Promise.resolve(window.createProfileNative(name)).then(function() {
						notify('🔄 Opening new profile "' + name + '"…');
					}).catch(function() {}).then(function() {
						setTimeout(function() { createBtn.disabled = false; }, 3000);
					});
				};
				var input = modal.querySelector('#wa-input-profile-name');
				if (input && !input.dataset.wired) {
					input.dataset.wired = '1';
					input.addEventListener('keydown', function(e) {
						if (e.key === 'Enter') { e.preventDefault(); createBtn.click(); }
					});
				}
			}

			// Wrap showSettingsModal to inject the Profiles card every time the
			// modal is (re)built, without duplicating the original implementation.
			var originalShowSettings = window.showSettingsModal;
			function showSettingsModalWithProfiles() {
				var existed = !!document.getElementById('wa-settings-overlay');
				if (originalShowSettings) originalShowSettings();
				if (existed) return; // toggle-close: original already removed it
				var overlay = document.getElementById('wa-settings-overlay');
				if (!overlay) return;
				// NOTE: the card must go into #wa-settings-container (the panel),
				// never into the overlay itself — the overlay is a centered flex
				// row, so appending there renders the card detached beside the
				// panel instead of inside it.
				var container = document.getElementById('wa-settings-container') || overlay;
				if (!container) return;
				refreshProfiles().then(function() {
					// The modal may have been closed while profiles were loading.
					if (!document.getElementById('wa-settings-overlay')) return;

					var card = document.createElement('div');
					card.className = 'wa-modal-card';
					card.id = 'wa-profiles-card';
					// Opaque from birth (same reason as the modal container): the
					// theme sync below repaints it right away, but the card must
					// never flash transparent if that pass is ever skipped.
					card.style.cssText = 'display:flex;flex-direction:column;gap:8px;border-radius:0;border-width:0 0 1px;border-style:solid;padding:12px 0;background:#111b21;border-color:#2a3942;';
					card.innerHTML = '' +
						'<div style="display:flex;align-items:center;justify-content:space-between;gap:16px;">' +
						'  <div>' +
						'    <strong class="wa-text-primary" style="font-size:12.5px;display:block;">Profiles</strong>' +
						'    <span class="wa-text-muted" style="font-size:11px;">Each profile is a separate WhatsApp account that stays logged in</span>' +
						'  </div>' +
						'</div>' +
						'<div id="wa-profile-list" style="display:flex;flex-direction:column;gap:4px;"></div>' +
						'<div style="display:flex;gap:8px;align-items:center;">' +
						'  <input id="wa-input-profile-name" type="text" maxlength="40" placeholder="New profile name…" style="flex:1;min-width:0;background:rgba(134,150,160,.15);border:1px solid rgba(134,150,160,.4);border-radius:6px;color:inherit;padding:6px 10px;font-size:12px;outline:none;">' +
						'  <button id="wa-btn-create-profile" class="wa-card-btn" style="padding:6px 12px;border-radius:6px;font-size:11.5px;font-weight:600;cursor:pointer;border-width:1px;border-style:solid;">＋ Create &amp; open</button>' +
						'</div>';

				// Identity first: the Profiles card leads the settings, directly
				// under the header, so the active account is unmistakable before
				// any setting is touched (the pattern Chrome/Edge/VS Code use).
				// Daily toggles (Appearance, Privacy, …) keep their order below.
				var head = container.querySelector('#wa-modal-header');
				if (head && head.parentNode === container && head.nextSibling) {
					container.insertBefore(card, head.nextSibling);
				} else {
					container.insertBefore(card, container.firstChild);
				}

				renderProfiles();
				bindProfileActions(card);
				ensureCreateRow(container);
					// The card is injected asynchronously, after the modal's own
					// theme sync already ran — repaint so it is opaque and
					// matches the current theme instead of staying transparent.
					try {
						if (typeof window.syncModalTheme === 'function') {
							var themeNow = (typeof window.getAppTheme === 'function') ? window.getAppTheme() : 'dark';
							var darkNow = themeNow === 'dark' ||
								(themeNow === 'system' && window.matchMedia &&
									window.matchMedia('(prefers-color-scheme: dark)').matches);
							window.syncModalTheme(darkNow);
						}
					} catch (e) {}
				});
			}
			window.showSettingsModal = showSettingsModalWithProfiles;
		})();
	`
}
