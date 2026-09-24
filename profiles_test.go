package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// isolateProfilesHome points the shared profile registry and per-profile
// folders at a temp dir for the duration of one test.
func isolateProfilesHome(t *testing.T) {
	t.Helper()
	base := t.TempDir()
	setAppBaseDirForTest(base)
	t.Cleanup(func() { setAppBaseDirForTest("") })
}

func TestProfileRegistryDefaultsOnFirstRun(t *testing.T) {
	isolateProfilesHome(t)

	reg := loadProfileRegistry()
	if len(reg.Profiles) != 1 {
		t.Fatalf("expected exactly 1 seeded profile, got %d", len(reg.Profiles))
	}
	if reg.Profiles[0].ID != defaultProfileID || reg.Profiles[0].Name != defaultProfileName {
		t.Fatalf("seeded profile = %+v, want default", reg.Profiles[0])
	}

	// The Default profile must own the legacy UserData dir so existing
	// sessions survive the multi-profile upgrade untouched.
	if dir := profileDirFor(reg.Profiles[0]); filepath.Base(dir) != legacyDataDirName {
		t.Fatalf("default profile dir = %q, want legacy UserData dir", dir)
	}

	// Second load must re-read from disk (and stay stable).
	again := loadProfileRegistry()
	if len(again.Profiles) != 1 || again.Profiles[0].ID != defaultProfileID {
		t.Fatalf("registry not persisted correctly: %+v", again.Profiles)
	}
}

func TestCreateProfileSanitizesAndDeduplicates(t *testing.T) {
	isolateProfilesHome(t)

	p1, err := createProfile("Team  Work")
	if err != nil {
		t.Fatalf("createProfile: %v", err)
	}
	if p1.ID != "team work" {
		t.Fatalf("sanitized id = %q, want %q", p1.ID, "team work")
	}
	if dir := profileDirFor(p1); strings.Contains(dir, "Team") || filepath.Base(dir) != p1.ID {
		t.Fatalf("profile dir %q does not match sanitized id %q", dir, p1.ID)
	}

	// Same name again is idempotent.
	p2, err := createProfile("Team Work")
	if err != nil {
		t.Fatalf("createProfile (idempotent): %v", err)
	}
	if p2.ID != p1.ID {
		t.Fatalf("duplicate create returned different profile: %v vs %v", p1, p2)
	}

	// A differently-cased duplicate maps onto the same id.
	p3, err := createProfile("TEAM WORK")
	if err != nil || p3.ID != p1.ID {
		t.Fatalf("case-variant duplicate not deduplicated: %v err=%v", p3, err)
	}

	// A new distinct name must get a distinct id (no silent merge).
	p4, err := createProfile("Team  Work 2")
	if err != nil {
		t.Fatalf("createProfile 2: %v", err)
	}
	if p4.ID == p1.ID {
		t.Fatal("distinct profile names must not collide after sanitizing")
	}

	if got := len(listProfiles()); got != 3 {
		t.Fatalf("registry size = %d, want 3", got)
	}
}

func TestCreateProfileRejectsEmptyAndUnsanitizable(t *testing.T) {
	isolateProfilesHome(t)

	if _, err := createProfile("   "); err == nil {
		t.Fatal("empty name must be rejected")
	}
	if _, err := createProfile("///"); err == nil {
		t.Fatal("name with no usable characters must be rejected")
	}
}

func TestProfileNameSanitizerStripsUnsafeChars(t *testing.T) {
	cases := map[string]string{
		`a<b>:c/d\e|f?g*h`: "a b c d e f g h",
		"  spaced   out  ": "spaced out",
		`quote"name`:       `quote name`,
	}
	for in, want := range cases {
		if got := sanitizeProfileID(in); got != want {
			t.Errorf("sanitizeProfileID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDeleteProfileRules(t *testing.T) {
	isolateProfilesHome(t)

	p, err := createProfile("Temp")
	if err != nil {
		t.Fatalf("createProfile: %v", err)
	}

	// Default profile is permanent.
	if err := deleteProfile(defaultProfileID); err == nil {
		t.Fatal("deleting the Default profile must fail")
	}

	// Unknown ids fail.
	if err := deleteProfile("ghost"); err == nil {
		t.Fatal("deleting an unknown profile must fail")
	}

	// Deleting an unused profile removes it and wipes its data dir.
	dir := profileDirFor(p)
	if err := os.MkdirAll(filepath.Join(dir, "EBWebView"), 0755); err != nil {
		t.Fatalf("prepare data dir: %v", err)
	}
	if err := deleteProfile(p.ID); err != nil {
		t.Fatalf("deleteProfile: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("profile data dir still exists after delete: %v", err)
	}
	if got := listProfiles(); len(got) != 1 {
		t.Fatalf("registry after delete = %+v, want only default", got)
	}
}

func TestDeleteProfileRefusesActiveProfile(t *testing.T) {
	isolateProfilesHome(t)

	p, err := createProfile("InUse")
	if err != nil {
		t.Fatalf("createProfile: %v", err)
	}
	profileMu.Lock()
	prev := activeProfile
	activeProfile = p
	profileMu.Unlock()
	t.Cleanup(func() {
		profileMu.Lock()
		activeProfile = prev
		profileMu.Unlock()
	})

	if err := deleteProfile(p.ID); err == nil {
		t.Fatal("deleting the active profile must fail")
	}
}

func TestRenameProfile(t *testing.T) {
	isolateProfilesHome(t)

	p, err := createProfile("Old Name")
	if err != nil {
		t.Fatalf("createProfile: %v", err)
	}
	renamed, err := renameProfile(p.ID, "New Name")
	if err != nil {
		t.Fatalf("renameProfile: %v", err)
	}
	if renamed.Name != "New Name" || renamed.ID != p.ID {
		t.Fatalf("rename result = %+v", renamed)
	}
	if _, err := renameProfile("ghost", "X"); err == nil {
		t.Fatal("renaming an unknown profile must fail")
	}
	if _, err := renameProfile(p.ID, "  "); err == nil {
		t.Fatal("renaming to an empty name must fail")
	}
	// Renaming the ACTIVE profile is registry-only and must succeed: the
	// backend has no in-use guard, and the tab shell syncs the name after.
	if _, err := renameProfile(getActiveProfile().ID, "Active Renamed"); err != nil {
		t.Fatalf("renaming the active profile must be allowed: %v", err)
	}
}

func TestTouchProfileLastUsed(t *testing.T) {
	isolateProfilesHome(t)

	p, err := createProfile("Recent")
	if err != nil {
		t.Fatalf("createProfile: %v", err)
	}
	touchProfileLastUsed(p.ID)
	reg := loadProfileRegistry()
	found := false
	for _, q := range reg.Profiles {
		if q.ID == p.ID {
			found = true
			if q.LastUsed == 0 {
				t.Fatal("LastUsed was not updated")
			}
		}
	}
	if !found {
		t.Fatal("profile disappeared from registry")
	}
}

func TestProfileDirForNonDefaultUsesProfilesFolder(t *testing.T) {
	isolateProfilesHome(t)

	p := Profile{ID: "work", Name: "Work"}
	want := filepath.Join(appBaseDir(), profilesDirName, "work")
	if got := profileDirFor(p); got != want {
		t.Fatalf("profileDirFor = %q, want %q", got, want)
	}
}

func TestFindProfileByIDOrNameIsCaseInsensitiveForNames(t *testing.T) {
	reg := profileRegistry{Profiles: []Profile{
		{ID: "default", Name: "Default"},
		{ID: "work", Name: "Team Work"},
	}}
	if p := findProfileByIDOrName(reg, "work"); p == nil {
		t.Fatal("lookup by id failed")
	}
	if p := findProfileByIDOrName(reg, "TEAM WORK"); p == nil {
		t.Fatal("lookup by name must be case-insensitive")
	}
	if p := findProfileByIDOrName(reg, "nope"); p != nil {
		t.Fatal("unknown key must not match")
	}
}

func TestUniqueProfileIDAvoidsCollisions(t *testing.T) {
	isolateProfilesHome(t)
	if _, err := createProfile("Dup"); err != nil {
		t.Fatalf("createProfile: %v", err)
	}
	p2, err := createProfile("dUp") // sanitizes to the same id "dup"
	if err != nil {
		t.Fatalf("createProfile variant: %v", err)
	}
	// Idempotent path returns the existing profile instead of a new one, so
	// uniqueness only matters for names that sanitize to a *taken* id while
	// the original lookup missed (covered by case-variant test above). Here
	// we just assert the registry stays consistent.
	if p2.ID == "" {
		t.Fatal("empty id returned")
	}
	ids := map[string]bool{}
	for _, q := range listProfiles() {
		if ids[q.ID] {
			t.Fatalf("duplicate id in registry: %q", q.ID)
		}
		ids[q.ID] = true
	}
}

func TestProfileRegistryJSONRoundTrip(t *testing.T) {
	isolateProfilesHome(t)
	if _, err := createProfile("Round Trip"); err != nil {
		t.Fatalf("createProfile: %v", err)
	}
	data, err := os.ReadFile(getProfilesFilePath())
	if err != nil {
		t.Fatalf("read profiles.json: %v", err)
	}
	var reg profileRegistry
	if err := json.Unmarshal(data, &reg); err != nil {
		t.Fatalf("profiles.json is not valid JSON: %v", err)
	}
	if reg.Version != 1 {
		t.Fatalf("registry version = %d, want 1", reg.Version)
	}
}

func TestWindowTitleForProfile(t *testing.T) {
	// Default profile keeps the plain app title.
	profileMu.Lock()
	prev := activeProfile
	activeProfile = Profile{ID: defaultProfileID, Name: defaultProfileName}
	profileMu.Unlock()
	if got := windowTitleForProfile(); got != windowTitle {
		t.Fatalf("default title = %q, want %q", got, windowTitle)
	}

	// Non-default profiles get "<base> — <name>".
	profileMu.Lock()
	activeProfile = Profile{ID: "work", Name: "Work"}
	profileMu.Unlock()
	want := windowTitle + " — Work"
	if got := windowTitleForProfile(); got != want {
		t.Fatalf("profile title = %q, want %q", got, want)
	}

	// Missing name falls back to the base title instead of a dangling dash.
	profileMu.Lock()
	activeProfile = Profile{ID: "noname", Name: "   "}
	profileMu.Unlock()
	if got := windowTitleForProfile(); got != windowTitle {
		t.Fatalf("blank-name title = %q, want %q", got, windowTitle)
	}

	profileMu.Lock()
	activeProfile = prev
	profileMu.Unlock()
}

func TestProfileInfosWireFormat(t *testing.T) {
	profiles := []Profile{
		{ID: "default", Name: "Default"},
		{ID: "work", Name: "Work", LastUsed: 42},
	}
	infos := profileInfos(profiles, false)
	if infos[1].Active {
		t.Fatal("active flag must be off when includeActive=false")
	}
	profileMu.Lock()
	prev := activeProfile
	activeProfile = Profile{ID: "work", Name: "Work"}
	profileMu.Unlock()
	defer func() {
		profileMu.Lock()
		activeProfile = prev
		profileMu.Unlock()
	}()
	infos = profileInfos(profiles, true)
	if !infos[1].Active || infos[0].Active {
		t.Fatalf("active flag mismatch: %+v", infos)
	}
}

func TestFormatBytes(t *testing.T) {
	cases := map[int64]string{
		0:         "0 B",
		999:       "999 B",
		1024:      "1.0 KB",
		1536:      "1.5 KB",
		842 * 1e6: "803.0 MB",
		5 * 1e9:   "4.7 GB",
	}
	for in, want := range cases {
		if got := formatBytes(in); got != want {
			t.Errorf("formatBytes(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestProfileDataBytes(t *testing.T) {
	isolateProfilesHome(t)

	p, err := createProfile("Sized")
	if err != nil {
		t.Fatalf("createProfile: %v", err)
	}
	dir := profileDirFor(p)
	if err := os.MkdirAll(filepath.Join(dir, "EBWebView", "Default"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	blob := filepath.Join(dir, "EBWebView", "Default", "Cookies")
	if err := os.WriteFile(blob, make([]byte, 4096), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if got := profileDataBytes(p); got != 4096 {
		t.Fatalf("profileDataBytes = %d, want 4096", got)
	}

	// A profile whose folder does not exist yet reports zero, not an error.
	ghost := Profile{ID: "ghost", Name: "Ghost"}
	if got := profileDataBytes(ghost); got != 0 {
		t.Fatalf("profileDataBytes(missing dir) = %d, want 0", got)
	}

	// The wire format must carry the computed size.
	infos := profileInfos(listProfiles(), false)
	for _, info := range infos {
		if info.ID == p.ID && info.DataBytes != 4096 {
			t.Fatalf("ProfileInfo.DataBytes = %d, want 4096", info.DataBytes)
		}
	}
}

func TestResetProfileData(t *testing.T) {
	isolateProfilesHome(t)

	p, err := createProfile("Resettable")
	if err != nil {
		t.Fatalf("createProfile: %v", err)
	}
	dir := profileDirFor(p)
	if err := os.MkdirAll(filepath.Join(dir, "EBWebView"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Cookies"), []byte("session"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Unknown id fails.
	if err := resetProfileData("ghost"); err == nil {
		t.Fatal("resetting an unknown profile must fail")
	}

	if err := resetProfileData(p.ID); err != nil {
		t.Fatalf("resetProfileData: %v", err)
	}

	// Profile stays registered, but its data folder is emptied and recreated.
	found := false
	for _, q := range listProfiles() {
		if q.ID == p.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("profile must remain registered after reset")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read reset dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("reset dir not empty: %d entries", len(entries))
	}
	if got := profileDataBytes(p); got != 0 {
		t.Fatalf("profileDataBytes after reset = %d, want 0", got)
	}

	// The active profile refuses a reset: this process would keep writing
	// into the wiped folder.
	profileMu.Lock()
	prev := activeProfile
	activeProfile = p
	profileMu.Unlock()
	t.Cleanup(func() {
		profileMu.Lock()
		activeProfile = prev
		profileMu.Unlock()
	})
	if err := resetProfileData(p.ID); err == nil {
		t.Fatal("resetting the active profile must fail")
	}
}

func TestRemoveProfileFromRegistryKeepsGuards(t *testing.T) {
	isolateProfilesHome(t)
	profileMu.Lock()
	prev := activeProfile
	activeProfile = Profile{ID: defaultProfileID, Name: defaultProfileName}
	profileMu.Unlock()
	t.Cleanup(func() {
		profileMu.Lock()
		activeProfile = prev
		profileMu.Unlock()
	})

	p, err := createProfile("Temp")
	if err != nil {
		t.Fatal(err)
	}
	dir := profileDirFor(p)

	if err := removeProfileFromRegistry(defaultProfileID); err == nil {
		t.Fatal("removing Default must fail")
	}
	profileMu.Lock()
	activeProfile = p
	profileMu.Unlock()
	if err := removeProfileFromRegistry(p.ID); err == nil {
		t.Fatal("removing the active profile must fail")
	}
	if err := removeProfileFromRegistry("missing"); err == nil {
		t.Fatal("removing an unknown profile must fail")
	}
	profileMu.Lock()
	activeProfile = Profile{ID: defaultProfileID, Name: defaultProfileName}
	profileMu.Unlock()
	if err := removeProfileFromRegistry(p.ID); err != nil {
		t.Fatalf("removing inactive profile: %v", err)
	}
	for _, q := range listProfiles() {
		if q.ID == p.ID {
			t.Fatal("profile must be unregistered")
		}
	}
	// Registry-only: the data folder must still be there for the wipe step.
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("data dir must survive registry removal: %v", err)
	}
}

func TestWipeProfileDataDir(t *testing.T) {
	isolateProfilesHome(t)
	p, err := createProfile("Temp")
	if err != nil {
		t.Fatal(err)
	}
	dir := profileDirFor(p)
	if err := os.WriteFile(filepath.Join(dir, "probe.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := wipeProfileDataDir(p.ID); err != nil {
		t.Fatalf("wipe: %v", err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatal("wipe must empty the data dir")
	}
	if err := wipeProfileDataDir(""); err == nil {
		t.Fatal("empty id must be refused")
	}
	if err := wipeProfileDataDir(".."); err == nil {
		t.Fatal("unsafe id must be refused")
	}
}

// Click-to-chat numbers: optional single leading '+', 8-15 digits (E.164
// with country code); formatting noise ignored, anything else refused so a
// crafted string can never smuggle a URL into the navigation target.
func TestNormalizeChatPhone(t *testing.T) {
	valid := map[string]string{
		"6281234567890":      "6281234567890",
		"+6281234567890":     "6281234567890",
		"+62 812-3456-7890":  "6281234567890",
		"(62) 812.3456.7890": "6281234567890",
		"  62812345  ":       "62812345",
		"12345678":           "12345678",
		"123456789012345":    "123456789012345",
	}
	for in, want := range valid {
		got, err := normalizeChatPhone(in)
		if err != nil {
			t.Errorf("normalizeChatPhone(%q) errored: %v", in, err)
		} else if got != want {
			t.Errorf("normalizeChatPhone(%q) = %q, want %q", in, got, want)
		}
	}
	for _, in := range []string{
		"", "   ", "abc", "62abc812", "1234567", "1234567890123456",
		"++6281234567890", "62+81234567890", "62812*345678", "62812#345678",
		"https://web.whatsapp.com/send?phone=6281234567890",
		"6281234567890?text=hi", "62812\n345678",
	} {
		if got, err := normalizeChatPhone(in); err == nil {
			t.Errorf("normalizeChatPhone(%q) = %q, want error", in, got)
		}
	}
}

func TestDirectChatURLUsesOfficialDeepLink(t *testing.T) {
	if got := directChatURL("6281234567890"); got != "https://web.whatsapp.com/send?phone=6281234567890" {
		t.Fatalf("directChatURL = %q", got)
	}
}
