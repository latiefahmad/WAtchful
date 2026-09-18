package main

// Multi-account profiles: every account gets its own browser user-data
// folder, so multiple WhatsApp accounts stay logged in side by side instead
// of logging each other out.
//
// Storage layout (base dir = settings home, e.g. %APPDATA%\WAtchful):
//
//	<base>/profiles.json        — profile registry
//	<base>/UserData             — legacy single-profile storage, still used by
//	                              the "Default" profile for compatibility with
//	                              pre-multi-profile installs
//	<base>/Profiles/<id>/       — user-data dir of every other profile
//
// The registry is a JSON file guarded by a mutex because profile manager
// calls arrive from webview bindings while a picker process may also be
// writing it.

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	defaultProfileID   = "default"
	defaultProfileName = "Default"
	profilesFileName   = "profiles.json"
	profilesDirName    = "Profiles"
	legacyDataDirName  = "UserData"

	maxProfiles = 20
)

// Profile describes one WhatsApp account workspace.
type Profile struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt int64  `json:"created_at"`
	LastUsed  int64  `json:"last_used,omitempty"`
}

type profileRegistry struct {
	Version  int       `json:"version"`
	Profiles []Profile `json:"profiles"`
}

var (
	profileMu          sync.Mutex
	activeProfile      = Profile{ID: defaultProfileID, Name: defaultProfileName}
	profileFlagPending = ""

	// appBaseDirOverride redirects all profile storage (tests only).
	appBaseDirOverride string
)

func setAppBaseDirForTest(dir string) {
	appBaseDirOverride = dir
}

// appBaseDir returns the shared WAtchful home (next to settings.json),
// where the profile registry and per-profile data dirs live.
func appBaseDir() string {
	if appBaseDirOverride != "" {
		return appBaseDirOverride
	}
	// Shared data home (legacy folder preferred when it exists) so profiles
	// keep their sessions across the app rename.
	return getSettingsBaseDir()
}

func getProfilesFilePath() string {
	return filepath.Join(appBaseDir(), profilesFileName)
}

// windowTitleForProfile appends the active profile name to the window title
// ("WAtchful — Work") so concurrently open profiles are distinguishable
// in the taskbar, window switchers, and Mission Control. The Default profile
// keeps the plain title, and profile names are unique (createProfile
// deduplicates case-insensitively), so the composed title stays unambiguous.
func windowTitleForProfile() string {
	p := getActiveProfile()
	if p.ID == defaultProfileID || strings.TrimSpace(p.Name) == "" {
		return windowTitle
	}
	return windowTitle + " — " + p.Name
}

func stripProfileNameNewlines(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' {
			return ' '
		}
		return r
	}, strings.TrimSpace(s))
}

// profileNamesForMenu feeds the native tray/menu-bar profile switcher: the
// display names joined with "\n" (newlines inside names are stripped, since
// they are the menu-side separator) plus the active profile's name, which the
// menu marks with a checkmark and disables.
func profileNamesForMenu() (joined, active string) {
	parts := make([]string, 0, 8)
	for _, p := range listProfiles() {
		name := stripProfileNameNewlines(p.Name)
		if name == "" {
			continue
		}
		parts = append(parts, name)
	}
	return strings.Join(parts, "\n"), stripProfileNameNewlines(getActiveProfile().Name)
}

// switchToProfileByName resolves a profile by display name (or id) and
// relaunches the app into it. Used by the native tray/menubar menus, which
// only carry the display name. Returns false when nothing matches.
func switchToProfileByName(name string) bool {
	// Tabbed shell (Windows): switching is an in-process tab activation.
	if tabSwitchHook != nil {
		return tabSwitchHook(name)
	}
	p := findProfileByIDOrName(loadProfileRegistry(), name)
	if p == nil {
		return false
	}
	touchProfileLastUsed(p.ID)
	relaunchWithProfile(p.Name)
	return true
}

// tabSwitchHook, when set, handles profile switches without relaunching.
// Installed by the Windows tabbed shell; nil everywhere else (macOS/Linux
// keep the relaunch model, as does the standalone profile picker).
var tabSwitchHook func(name string) bool

// setActiveProfile makes p the process-visible active profile and records
// the switch. In the tabbed shell the manager calls this on every tab
// change, so windowTitleForProfile, profileNamesForMenu and the
// delete/reset guards transparently follow the visible tab.
func setActiveProfile(p Profile) {
	profileMu.Lock()
	activeProfile = p
	profileMu.Unlock()
	touchProfileLastUsed(p.ID)
}

// getActiveProfile returns the profile selected for this process. Every call
// after initialization returns the same value, so bindings and cache paths
// stay consistent for the lifetime of the app.
func getActiveProfile() Profile {
	return activeProfile
}

// initActiveProfile resolves the active profile for this run:
// --profile <name|id> wins, then WA_DESK_PROFILE, then the last used one.
// Unknown names fall back to the last-used profile (picker processes pass
// whatever they were given; the picker itself always validates first).
func initActiveProfile() {
	// Existing installs have no profiles.json yet: bootstrap it with the
	// Default profile pointing at the legacy UserData dir.
	reg := loadProfileRegistry()

	flagName := strings.TrimSpace(profileFlagPending)
	if flagName == "" {
		flagName = strings.TrimSpace(os.Getenv("WA_DESK_PROFILE"))
	}

	var chosen *Profile
	if flagName != "" {
		chosen = findProfileByIDOrName(reg, flagName)
		if chosen == nil {
			// Creating on demand keeps the picker fast path honest: it only
			// sends names it created, but a manual `--profile Work` launch
			// should also just work.
			p, err := createProfile(flagName)
			if err == nil {
				chosen = &p
			}
		}
	}
	if chosen == nil {
		chosen = lastUsedProfile(reg)
	}

	profileMu.Lock()
	activeProfile = *chosen
	profileMu.Unlock()

	touchProfileLastUsed(chosen.ID)
}

func loadProfileRegistry() profileRegistry {
	profileMu.Lock()
	defer profileMu.Unlock()
	return loadProfileRegistryLocked()
}

func loadProfileRegistryLocked() profileRegistry {
	reg := profileRegistry{Version: 1}
	data, err := os.ReadFile(getProfilesFilePath())
	if err != nil {
		// First run of the multi-profile build: seed the registry with the
		// Default profile that owns the legacy UserData directory.
		reg.Profiles = []Profile{defaultProfile()}
		_ = saveProfileRegistryLocked(reg)
		return reg
	}
	_ = json.Unmarshal(data, &reg)
	now := time.Now().Unix()
	existing := make(map[string]bool, len(reg.Profiles))
	kept := reg.Profiles[:0]
	for _, p := range reg.Profiles {
		p.ID = sanitizeProfileID(p.ID)
		if p.ID == "" || existing[p.ID] {
			continue
		}
		if strings.TrimSpace(p.Name) == "" {
			p.Name = p.ID
		}
		if p.CreatedAt == 0 {
			p.CreatedAt = now
		}
		existing[p.ID] = true
		kept = append(kept, p)
	}
	if !existing[defaultProfileID] {
		// Always guarantee the Default profile exists.
		kept = append([]Profile{defaultProfile()}, kept...)
	}
	reg.Profiles = kept
	return reg
}

func defaultProfile() Profile {
	return Profile{ID: defaultProfileID, Name: defaultProfileName}
}

func saveProfileRegistryLocked(reg profileRegistry) error {
	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return err
	}
	_ = os.MkdirAll(appBaseDir(), 0755)
	tmp := getProfilesFilePath() + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, getProfilesFilePath())
}

func listProfiles() []Profile {
	return loadProfileRegistry().Profiles
}

// profileInfos is the wire format handed to the in-page profile manager.
// When includeActive is set, exactly one entry gets active=true (the profile
// running this process), so the UI can label it "in use".
type ProfileInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Active    bool   `json:"active"`
	LastUsed  int64  `json:"last_used"`
	DataBytes int64  `json:"data_bytes"`
}

func profileInfos(profiles []Profile, includeActive bool) []ProfileInfo {
	activeID := ""
	if includeActive {
		activeID = getActiveProfile().ID
	}
	out := make([]ProfileInfo, 0, len(profiles))
	for _, p := range profiles {
		out = append(out, ProfileInfo{
			ID:        p.ID,
			Name:      p.Name,
			Active:    includeActive && p.ID == activeID,
			LastUsed:  p.LastUsed,
			DataBytes: profileDataBytes(p),
		})
	}
	return out
}

func findProfileByIDOrName(reg profileRegistry, key string) *Profile {
	key = strings.TrimSpace(key)
	for i := range reg.Profiles {
		if reg.Profiles[i].ID == key {
			return &reg.Profiles[i]
		}
	}
	for i := range reg.Profiles {
		if strings.EqualFold(reg.Profiles[i].Name, key) {
			return &reg.Profiles[i]
		}
	}
	return nil
}

func lastUsedProfile(reg profileRegistry) *Profile {
	var best *Profile
	for i := range reg.Profiles {
		if best == nil || reg.Profiles[i].LastUsed > best.LastUsed {
			best = &reg.Profiles[i]
		}
	}
	if best == nil {
		d := defaultProfile()
		best = &d
	}
	return best
}

func touchProfileLastUsed(id string) {
	profileMu.Lock()
	defer profileMu.Unlock()
	reg := loadProfileRegistryLocked()
	for i := range reg.Profiles {
		if reg.Profiles[i].ID == id {
			reg.Profiles[i].LastUsed = time.Now().Unix()
		}
	}
	_ = saveProfileRegistryLocked(reg)
}

var profileNameSanitizer = regexp.MustCompile(`[\x00-\x1f<>:"/\\|?*]`)

func sanitizeProfileID(raw string) string {
	clean := profileNameSanitizer.ReplaceAllString(strings.TrimSpace(raw), " ")
	clean = strings.Join(strings.Fields(clean), " ")
	if len(clean) > 40 {
		clean = strings.TrimSpace(clean[:40])
	}
	return strings.ToLower(clean)
}

// walkDirSize sums the size of every regular file under dir. Locked files
// (an active browser session may hold its cache open) are skipped rather than
// failing the whole walk; the result is informational. A missing directory
// simply means the profile has no data yet.
func walkDirSize(dir string) int64 {
	var total int64
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil && info.Mode().IsRegular() {
			total += info.Size()
		}
		return nil
	})
	return total
}

// formatBytes renders a byte count for the profile UIs ("842.1 MB").
func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	v := float64(n)
	suffixes := []string{"KB", "MB", "GB", "TB"}
	i := -1
	for v >= unit && i < len(suffixes)-1 {
		v /= unit
		i++
	}
	return fmt.Sprintf("%.1f %s", v, suffixes[i])
}

// profileDataBytes returns the on-disk size of a profile's browser user-data
// directory (0 when it does not exist yet).
func profileDataBytes(p Profile) int64 {
	dir := profileDirFor(p)
	if dir == "" {
		return 0
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return 0
	}
	return walkDirSize(dir)
}

// profileDirFor returns the browser user-data dir of a profile. The Default
// profile deliberately keeps the legacy <base>/UserData path so existing
// sessions survive the upgrade untouched.
func profileDirFor(p Profile) string {
	base := appBaseDir()
	if p.ID == defaultProfileID {
		return filepath.Join(base, legacyDataDirName)
	}
	id := sanitizeProfileID(p.ID)
	if id == "" || strings.ContainsAny(id, "\"") || strings.Contains(id, "..") {
		return ""
	}
	return filepath.Join(base, profilesDirName, id)
}

// createProfile registers a new profile with a unique, sanitized name.
func createProfile(name string) (Profile, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Profile{}, fmt.Errorf("profile name cannot be empty")
	}
	profileMu.Lock()
	defer profileMu.Unlock()
	reg := loadProfileRegistryLocked()
	if len(reg.Profiles) >= maxProfiles {
		return Profile{}, fmt.Errorf("profile limit (%d) reached", maxProfiles)
	}
	if existing := findProfileByIDOrName(reg, name); existing != nil {
		return *existing, nil // idempotent: picking an existing profile
	}
	sanitized := sanitizeProfileID(name)
	if sanitized == "" {
		return Profile{}, fmt.Errorf("profile name must contain usable characters")
	}
	// Match on the sanitized id too, so "Team Work" / "TEAM WORK" /
	// "Team  Work" all map onto one profile instead of spawning
	// near-duplicates like "team work-2".
	for i := range reg.Profiles {
		if reg.Profiles[i].ID == sanitized {
			return reg.Profiles[i], nil
		}
	}
	now := time.Now().Unix()
	p := Profile{
		ID:        uniqueProfileIDLocked(reg, sanitized),
		Name:      name,
		CreatedAt: now,
	}
	reg.Profiles = append(reg.Profiles, p)
	if err := saveProfileRegistryLocked(reg); err != nil {
		return Profile{}, err
	}
	_ = os.MkdirAll(profileDirFor(p), 0755)
	return p, nil
}

func uniqueProfileIDLocked(reg profileRegistry, id string) string {
	if id == "" {
		id = "profile"
	}
	taken := make(map[string]bool, len(reg.Profiles))
	for _, p := range reg.Profiles {
		taken[p.ID] = true
	}
	if !taken[id] {
		return id
	}
	for n := 2; ; n++ {
		candidate := fmt.Sprintf("%s-%d", id, n)
		if !taken[candidate] {
			return candidate
		}
	}
}

// deleteProfile removes a profile and wipes its user-data folder. The Default
// profile is permanent. The active profile can be deleted (used by the picker,
// which runs before this process owns a webview), but a running session cannot
// delete itself.
func deleteProfile(id string) error {
	if err := removeProfileFromRegistry(id); err != nil {
		return err
	}
	return wipeProfileDataDir(id)
}

// removeProfileFromRegistry unregisters id (Default/active guards included)
// without touching its data folder. Split from deleteProfile so the tabbed
// shell can unregister on the UI thread while the (possibly locked, large)
// folder is wiped on a background thread.
func removeProfileFromRegistry(id string) error {
	profileMu.Lock()
	defer profileMu.Unlock()
	if id == defaultProfileID {
		return fmt.Errorf("the Default profile cannot be deleted")
	}
	if getActiveProfile().ID == id {
		return fmt.Errorf("cannot delete the profile currently in use")
	}
	reg := loadProfileRegistryLocked()
	kept := reg.Profiles[:0]
	found := false
	for _, p := range reg.Profiles {
		if p.ID == id {
			found = true
			continue
		}
		kept = append(kept, p)
	}
	if !found {
		return fmt.Errorf("profile not found")
	}
	reg.Profiles = kept
	return saveProfileRegistryLocked(reg)
}

// wipeProfileDataDir deletes a profile's user-data folder with the same
// safety guards deleteProfile always had. It does not touch the registry.
func wipeProfileDataDir(id string) error {
	dir := profileDirFor(Profile{ID: id})
	if dir == "" || filepath.Clean(dir) == filepath.Clean(appBaseDir()) {
		return fmt.Errorf("refusing to delete unsafe path %q", dir)
	}
	return os.RemoveAll(dir)
}

// renameProfile renames an existing profile in place.
func renameProfile(id, name string) (Profile, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Profile{}, fmt.Errorf("profile name cannot be empty")
	}
	profileMu.Lock()
	defer profileMu.Unlock()
	reg := loadProfileRegistryLocked()
	for i := range reg.Profiles {
		if reg.Profiles[i].ID == id {
			reg.Profiles[i].Name = name
			if err := saveProfileRegistryLocked(reg); err != nil {
				return Profile{}, err
			}
			return reg.Profiles[i], nil
		}
	}
	return Profile{}, fmt.Errorf("profile not found")
}

// resetProfileData wipes a profile's browser user-data folder (session,
// cookies, caches) while keeping the profile registered. The folder is
// recreated empty, matching createProfile. Deleting the running profile's
// data is refused for the same reason as deleting it outright: this process
// would keep writing to the removed folder.
func resetProfileData(id string) error {
	profileMu.Lock()
	defer profileMu.Unlock()
	if getActiveProfile().ID == id {
		return fmt.Errorf("cannot reset the profile currently in use")
	}
	reg := loadProfileRegistryLocked()
	found := false
	for i := range reg.Profiles {
		if reg.Profiles[i].ID == id {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("profile not found")
	}
	dir := profileDirFor(Profile{ID: id})
	if dir == "" || filepath.Clean(dir) == filepath.Clean(appBaseDir()) {
		return fmt.Errorf("refusing to reset unsafe path %q", dir)
	}
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	return os.MkdirAll(dir, 0755)
}

// relaunchWithProfile starts a new app process on the given profile and exits
// this one. Used by "switch profile" and by the picker page after creating a
// profile. The child inherits stdout/stderr; its lifetime is independent.
func relaunchWithProfile(profileName string) {
	profileFlagPending = ""
	exe, err := os.Executable()
	if err != nil {
		return
	}
	// The child must become a chat session, never another picker: strip the
	// picker opt-in from the inherited environment (a picker launched with
	// WA_DESK_PICKER=1 would otherwise relaunch into an endless picker loop).
	if runtime.GOOS == "darwin" {
		// Inside a .app bundle, exec the bundle so the Dock keeps a single
		// app icon instead of a bare binary.
		if idx := strings.Index(exe, ".app/"); idx != -1 {
			bundle := exe[:idx+5]
			_ = exec.Command("open", "-n", bundle, "--args", "--profile", profileName).Start()
			os.Exit(0)
		}
	}
	env := os.Environ()
	childEnv := make([]string, 0, len(env))
	for _, entry := range env {
		if !strings.HasPrefix(entry, "WA_DESK_PICKER=") {
			childEnv = append(childEnv, entry)
		}
	}
	cmd := exec.Command(exe, "--profile", profileName)
	cmd.Env = childEnv
	_ = cmd.Start()
	os.Exit(0)
}

// ---------------------------------------------------------------------------
// Profile picker
// ---------------------------------------------------------------------------

//go:embed profiles_picker.html
var profilesPickerHTML []byte

const pickerPageURL = "http://whatsapp.desk.picker/"

// isProfilePickerRequested reports whether this run should show the picker
// instead of loading WhatsApp Web directly.
func isProfilePickerRequested() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("WA_DESK_PICKER")), "1")
}

// profilePickerDataDir returns an isolated user-data dir for the standalone
// profile picker process. It must not use any profile's real folder: those
// may be locked by live sessions, and the picker never stores anything.
func profilePickerDataDir() string {
	return filepath.Join(appBaseDir(), "PickerData")
}

// pickerPageHTML renders the standalone picker page with the current profile
// list baked in (fresh data on every launch, no persistence needed).
func pickerPageHTML() string {
	type pickerProfile struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		LastUsed  int64  `json:"last_used"`
		CreatedAt int64  `json:"created_at"`
		DataBytes int64  `json:"data_bytes"`
	}
	profiles := listProfiles()
	list := make([]pickerProfile, 0, len(profiles))
	for _, p := range profiles {
		list = append(list, pickerProfile{ID: p.ID, Name: p.Name, LastUsed: p.LastUsed, CreatedAt: p.CreatedAt, DataBytes: profileDataBytes(p)})
	}
	data, _ := json.Marshal(map[string]interface{}{
		"active":    getActiveProfile(),
		"version":   appVersion,
		"platform":  runtime.GOOS,
		"profiles":  list,
		"canDelete": true,
	})
	return strings.ReplaceAll(string(profilesPickerHTML), "__WA_PROFILE_DATA__", string(data))
}

// pickerBindings attaches the picker's native bridge. All handlers relaunch or
// exit — the picker page itself never becomes the chat session.
func pickerBindings(bind func(string, interface{}) error) {
	_ = bind("pickerListNative", func() []Profile {
		return listProfiles()
	})
	_ = bind("pickerLaunchNative", func(id string) {
		if p := findProfileByIDOrName(loadProfileRegistry(), id); p != nil {
			touchProfileLastUsed(p.ID)
			relaunchWithProfile(p.Name)
		}
	})
	_ = bind("pickerCreateNative", func(name string) {
		p, err := createProfile(name)
		if err != nil {
			return
		}
		touchProfileLastUsed(p.ID)
		relaunchWithProfile(p.Name)
	})
	_ = bind("pickerDeleteNative", func(id string) bool {
		return deleteProfile(id) == nil
	})
	_ = bind("pickerResetNative", func(id string) bool {
		return resetProfileData(id) == nil
	})
	_ = bind("pickerRenameNative", func(id, name string) bool {
		_, err := renameProfile(id, name)
		return err == nil
	})
	_ = bind("pickerQuitNative", func() {
		os.Exit(0)
	})
}
