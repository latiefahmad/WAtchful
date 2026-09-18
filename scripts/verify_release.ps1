# scripts/verify_release.ps1
#
# Release verification for WAtchful on Windows. Designed to be re-run for every
# release (including on a locked desktop, where synthetic keyboard input is
# blocked by the OS but PostMessage to the app's own window still works).
#
# Two modes:
#
#   1. Default (isolated launch) - launches the given .exe with an isolated
#      user data dir (temp folder + temp APPDATA; real profile data is never
#      touched) and verifies it. NOTE: WAtchful is single-instance on Windows;
#      if any WAtchful is already running, the second launch just focuses the
#      running one and exits - close it first, or use -AttachLive instead.
#
#   2. -AttachLive - attaches to the ALREADY RUNNING WAtchful window and runs
#      the same checks through its production tray path. Starts/stops nothing,
#      touches no profile data; it only opens and closes the Settings modal
#      exactly as a right-click on the tray icon would.
#
# What is verified (both modes):
#   - Settings modal opens via the production tray path: the same WM_COMMAND
#     message the tray icon's right-click menu sends (msg 0x8400, id 0x8485
#     -> openSettingsFromTray() -> window.showSettingsModal()).
#   - Build version from the modal subtitle
#     ("Application settings - version X.Y.Z(.W)").
#   - Profiles card inside the modal via UI Automation
#     (marker: "Each profile is a separate WhatsApp account...").
#   - A real Ctrl+, keystroke toggles the modal (isolated mode only, and only
#     when the interactive desktop is available and the window can be focused).
#   - A second tray command toggles the modal closed.
#
# Usage:
#   powershell -NoProfile -ExecutionPolicy Bypass -File scripts\verify_release.ps1 ^
#     -ExePath .\WAtchful.exe [-ExpectedVersion 2.0.0] [-SkipCleanup] [-SkipKeys]
#   powershell -NoProfile -ExecutionPolicy Bypass -File scripts\verify_release.ps1 ^
#     -AttachLive [-ExpectedVersion 2.0.0]
#
# Exit code 0 = all requested checks passed; 1 = a check failed.

param(
  [string]$ExePath = "",
  [string]$ExpectedVersion = "",
  [switch]$AttachLive,
  [switch]$SkipCleanup,
  [switch]$SkipKeys
)

$ErrorActionPreference = 'Stop'

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
$script:Failures = 0

function Write-Check([string]$name, [bool]$ok, [string]$detail = "") {
  if ($ok) {
    Write-Host ("[PASS] " + $name + $(if ($detail) { "  --  " + $detail } else { "" }))
  } else {
    Write-Host ("[FAIL] " + $name + $(if ($detail) { "  --  " + $detail } else { "" }))
    $script:Failures++
  }
}

# Marker text of the Profiles card description inside the Settings modal
# (profiles_ui.go). ASCII-only so it is safe in PowerShell 5.1 (no BOM issues).
$ProfileCardMarker = 'Each profile is a separate WhatsApp account'
# Regex for the modal subtitle: 'Application settings - version X.Y.Z(.W)'
$VersionPattern = 'version\s+([0-9]+\.[0-9]+\.[0-9]+(\.[0-9]+)?)'

Add-Type -AssemblyName UIAutomationClient | Out-Null
Add-Type -AssemblyName UIAutomationTypes | Out-Null

Add-Type @'
using System;
using System.Runtime.InteropServices;
public static class WAtch {
  [DllImport("user32.dll", CharSet = CharSet.Unicode)]
  public static extern IntPtr FindWindow(string cls, string title);
  [DllImport("user32.dll")]
  public static extern IntPtr PostMessage(IntPtr hWnd, uint msg, IntPtr wp, IntPtr lp);
  [DllImport("user32.dll")]
  public static extern bool SetForegroundWindow(IntPtr hWnd);
  [DllImport("user32.dll")]
  public static extern IntPtr GetForegroundWindow();
  [DllImport("user32.dll")]
  public static extern uint GetWindowThreadProcessId(IntPtr hWnd, out uint pid);
  [DllImport("user32.dll", SetLastError = true)]
  public static extern IntPtr OpenInputDesktop(uint flags, bool inherit, uint access);
  [DllImport("kernel32.dll")]
  public static extern bool CloseHandle(IntPtr h);
  [DllImport("user32.dll")]
  public static extern void keybd_event(byte vk, byte scan, uint flags, UIntPtr extra);
  [DllImport("user32.dll")]
  public static extern bool AttachThreadInput(uint idAttach, uint idAttachTo, bool attach);
  [DllImport("user32.dll")]
  public static extern bool BringWindowToTop(IntPtr hWnd);
  // AutoHotkey-style foreground grab: attach our input queue to the target
  // window's thread so Windows allows us to bring it to the foreground.
  public static bool TryFocus(IntPtr hwnd) {
    IntPtr cur = GetForegroundWindow();
    if (cur == hwnd) return true;
    uint curPid = 0, tgtPid = 0;
    uint curTid = GetWindowThreadProcessId(cur, out curPid);
    uint tgtTid = GetWindowThreadProcessId(hwnd, out tgtPid);
    uint thisTid = Kernel32.GetCurrentThreadId();
    bool attached = false;
    if (curTid != 0 && curTid != thisTid) attached |= AttachThreadInput(thisTid, curTid, true);
    if (tgtTid != 0 && tgtTid != thisTid) attached |= AttachThreadInput(thisTid, tgtTid, true);
    BringWindowToTop(hwnd);
    SetForegroundWindow(hwnd);
    if (attached) AttachThreadInput(thisTid, curTid, true);
    if (attached) AttachThreadInput(thisTid, tgtTid, false);
    return GetForegroundWindow() == hwnd;
  }
}
public static class Kernel32 {
  [DllImport("kernel32.dll")]
  public static extern uint GetCurrentThreadId();
}
'@

function Test-InteractiveDesktop {
  try {
    $h = [WAtch]::OpenInputDesktop(0, $false, 0x0001) # DESKTOP_READOBJECTS
    if ($h -ne [IntPtr]::Zero) { [void][WAtch]::CloseHandle($h); return $true }
    return $false
  } catch { return $false }
}

function Send-CtrlComma {
  # Real keystroke through the hardware input pipeline.
  # VK_OEM_COMMA = 0x21, scan code 0x33; keybd_event needs the scan code or
  # Chromium ignores it.
  [WAtch]::keybd_event(0x11, 0x1D, 0, [UIntPtr]::Zero)          # Ctrl down
  Start-Sleep -Milliseconds 60
  [WAtch]::keybd_event(0x21, 0x33, 0, [UIntPtr]::Zero)          # ',' down
  Start-Sleep -Milliseconds 60
  [WAtch]::keybd_event(0x21, 0x33, 2, [UIntPtr]::Zero)          # ',' up (KEYEVENTF_KEYUP)
  Start-Sleep -Milliseconds 60
  [WAtch]::keybd_event(0x11, 0x1D, 2, [UIntPtr]::Zero)          # Ctrl up
}

# Find a UIA element whose accessible Name equals $name exactly (fast
# PropertyCondition; must be the FULL accessible name).
function Find-Exact([System.Windows.Automation.AutomationElement]$scope, [string]$name, [int]$timeoutSec = 10) {
  $deadline = (Get-Date).AddSeconds($timeoutSec)
  $cond = New-Object System.Windows.Automation.PropertyCondition([System.Windows.Automation.AutomationElement]::NameProperty, $name)
  while ($true) {
    $el = $scope.FindFirst([System.Windows.Automation.TreeScope]::Descendants, $cond)
    if ($el -ne $null) { return $el }
    if ((Get-Date) -gt $deadline) { return $null }
    Start-Sleep -Milliseconds 300
  }
}

$script:UiaWalker = [System.Windows.Automation.TreeWalker]::ControlViewWalker

# Bounded breadth-first search under $root for an element whose Name contains
# $needle. Never walks more than $maxNodes nodes, so it cannot hang on the
# huge WhatsApp page DOM.
function Find-Under([System.Windows.Automation.AutomationElement]$root, [string]$needle, [int]$maxNodes = 800) {
  $q = New-Object System.Collections.Queue
  $q.Enqueue($root)
  $visited = 0
  $pat = "*" + $needle + "*"
  while ($q.Count -gt 0 -and $visited -lt $maxNodes) {
    $el = $q.Dequeue()
    $child = $script:UiaWalker.GetFirstChild($el)
    while ($child -ne $null) {
      $visited++
      $nm = ""
      try { $nm = $child.Current.Name } catch {}
      if ($nm -and $nm -like $pat) { return $child }
      $q.Enqueue($child)
      if ($visited -ge $maxNodes) { break }
      $child = $script:UiaWalker.GetNextSibling($child)
    }
  }
  return $null
}

# Find an element by substring near the modal anchor: walk up from the
# 'Check for updates' button (whose exact accessible name is stable) and
# probe each ancestor's subtree with a bounded search. The nearest common
# ancestor of the button, the subtitle and the Profiles card is the modal
# panel, so the hit comes from a small subtree.
function Find-NearAnchor([System.Windows.Automation.AutomationElement]$scope, [string]$needle, [int]$timeoutSec = 10) {
  $deadline = (Get-Date).AddSeconds($timeoutSec)
  while ($true) {
    $anchor = Find-Exact $scope 'Check for updates' 2
    if ($anchor -ne $null) {
      $cur = $anchor
      for ($up = 0; $up -lt 10; $up++) {
        try { $cur = $script:UiaWalker.GetParent($cur) } catch { return $null }
        if ($cur -eq $null) { break }
        $hit = Find-Under $cur $needle 800
        if ($hit -ne $null) { return $hit }
      }
      return $null
    }
    if ((Get-Date) -gt $deadline) { return $null }
    Start-Sleep -Milliseconds 300
  }
}

# The verification sequence shared by both modes.
function Invoke-Verification([System.Windows.Automation.AutomationElement]$scope, [IntPtr]$hwnd, [bool]$allowKeyTest) {
  $trayCmdMsg = 0x8400        # trayMenuBase(0x8100) + 0x0300 -> tray command
  $trayCmdSettings = 0x8485   # trayMenuBase + 901 -> openSettingsFromTray()

  # 1. First tray command -> Settings modal opens -> verify version
  [void][WAtch]::PostMessage($hwnd, $trayCmdMsg, [IntPtr]$trayCmdSettings, [IntPtr]::Zero)
  Start-Sleep -Seconds 2

  $checkEl = Find-Exact $scope 'Check for updates' 8
  $modalOpen = ($checkEl -ne $null)
  Write-Check "Tray command opens the Settings modal" $modalOpen

  # 2. Build version from the modal subtitle
  #    ("Application settings <middle-dot> version X.Y.Z(.W)")
  if ($modalOpen) {
    $subEl = Find-NearAnchor $scope 'Application settings' 10
    $sub = ""
    if ($subEl -ne $null) { $sub = $subEl.Current.Name }
    $version = ""
    if ($sub -match $VersionPattern) { $version = $Matches[1] }
    if ($ExpectedVersion -ne "") {
      Write-Check "Modal reports expected version" ($version -eq $ExpectedVersion) ("got '" + $version + "'  (subtitle: '" + $sub + "')")
    } else {
      Write-Check "Modal reports a version" ($version -ne "") ("got '" + $version + "'  (subtitle: '" + $sub + "')")
    }
  } else {
    Write-Check "Modal reports expected version" $false "modal did not open"
  }

  # 3. Profiles card inside the modal (UI Automation)
  if ($modalOpen) {
    $cardEl = Find-NearAnchor $scope $ProfileCardMarker 10
    Write-Check "Profiles card present inside the Settings modal" ($cardEl -ne $null)
  } else {
    Write-Check "Profiles card present inside the Settings modal" $false "modal did not open"
  }

  # 3. Real Ctrl+, keystroke toggles the modal (keyboard path)
  if ($allowKeyTest -and -not $SkipKeys) {
    $interactive = Test-InteractiveDesktop
    if ($interactive) {
      [void][WAtch]::TryFocus($hwnd)
      Send-CtrlComma
      Start-Sleep -Seconds 2
      $checkAfterKeys = Find-Exact $scope 'Check for updates' 5
      $stillOpen = ($checkAfterKeys -ne $null)
      Write-Check "Real Ctrl+, keystroke toggles the Settings modal" (-not $stillOpen) ("modal open before keys: " + $modalOpen + ", after keys: " + $stillOpen)
      if (-not $stillOpen) {
        # Re-open for the final toggle test below.
        [void][WAtch]::PostMessage($hwnd, $trayCmdMsg, [IntPtr]$trayCmdSettings, [IntPtr]::Zero)
        Start-Sleep -Seconds 2
        $modalOpen = (Find-ByName $scope 'Check for updates' 5) -ne $null
      }
    } else {
      Write-Host "[SKIP] Ctrl+, keystroke test (interactive desktop is locked/hidden)"
    }
  }

  # 4. Second tray command -> modal toggles closed
  [void][WAtch]::PostMessage($hwnd, $trayCmdMsg, [IntPtr]$trayCmdSettings, [IntPtr]::Zero)
  Start-Sleep -Seconds 2
  $gone = (Find-Exact $scope 'Check for updates' 5) -eq $null
  Write-Check "Second tray command toggles the Settings modal closed" $gone
}

# ---------------------------------------------------------------------------
# Mode 2: attach to the running instance
# ---------------------------------------------------------------------------
if ($AttachLive) {
  Write-Host "=============================================================="
  Write-Host " WAtchful release verification (attach to running instance)"
  Write-Host "=============================================================="
  $live = Get-Process | Where-Object { $_.ProcessName -in @('WAtchful', 'whatsapp-desktop') -and $_.MainWindowHandle -ne 0 } | Select-Object -First 1
  if ($live -eq $null) {
    Write-Host "[FAIL] No running WAtchful window found (launch the app first, or run without -AttachLive)."
    exit 1
  }
  Write-Host ("Attached: pid " + $live.Id + "  exe: " + $live.Path + "  title: '" + $live.MainWindowTitle + "'")
  try {
    $scope = [System.Windows.Automation.AutomationElement]::FromHandle($live.MainWindowHandle)
    # Attach mode belongs to the user's live session: never steal focus, so
    # the synthetic-keys check is skipped (the modal is verified via the tray
    # path; the keyboard path is covered by the CDP smoke test in CI).
    Invoke-Verification $scope $live.MainWindowHandle $false
  } catch {
    Write-Host ("[FAIL] UIA error: " + $_.Exception.Message)
    $script:Failures++
  }
  Write-Host "=============================================================="
  if ($script:Failures -eq 0) { Write-Host " RESULT: ALL CHECKS PASSED" }
  else { Write-Host (" RESULT: " + $script:Failures + " CHECK(S) FAILED") }
  Write-Host "=============================================================="
  if ($script:Failures -gt 0) { exit 1 }
  exit 0
}

# ---------------------------------------------------------------------------
# Mode 1: isolated launch
# ---------------------------------------------------------------------------
if ($ExePath -eq "") { throw "Provide -ExePath (isolated launch mode) or -AttachLive." }
$ExePath = (Resolve-Path $ExePath).Path
if (-not (Test-Path $ExePath)) { throw "Exe not found: $ExePath" }

$tempRoot = Join-Path $env:TEMP ("WAtchfulVerify-" + [Guid]::NewGuid().ToString('N').Substring(0, 8))
$profileName = "VerifyRelease"
New-Item -ItemType Directory -Path $tempRoot | Out-Null

Write-Host "=============================================================="
Write-Host " WAtchful release verification (isolated launch)"
Write-Host " exe      : $ExePath"
Write-Host " temp env : $tempRoot"
Write-Host "=============================================================="

# IMPORTANT: APPDATA must be set in OUR environment before Start-Process so
# the child inherits it (Start-Process has no -Environment on PS 5.1).
$env:APPDATA = $tempRoot

$proc = Start-Process -FilePath $ExePath -ArgumentList @("--profile", $profileName) -PassThru
Write-Host ("Launched pid " + $proc.Id + " (profile '$profileName', isolated APPDATA)")

# Find the main window. Also detect the single-instance hand-off: when
# another WAtchful is already running, this process exits 0 immediately
# after focusing the running one (claimTabbedSingleInstance).
$hwnd = [IntPtr]::Zero
for ($i = 0; $i -lt 60; $i++) {
  Start-Sleep -Seconds 1
  $p = Get-Process -Id $proc.Id -ErrorAction SilentlyContinue
  if ($p -eq $null) { break }
  if ($p.MainWindowHandle -ne 0) { $hwnd = $p.MainWindowHandle; break }
}
Write-Check "App launched and created a main window" ($hwnd -ne [IntPtr]::Zero) ("pid " + $proc.Id)

if ($hwnd -eq [IntPtr]::Zero) {
  $running = Get-Process | Where-Object { $_.ProcessName -in @('WAtchful', 'whatsapp-desktop') -and $_.Id -ne $proc.Id } | Select-Object -First 1
  if ($running -ne $null) {
    Write-Host ("[FAIL] WAtchful is single-instance: the launched process handed off to the already-running instance (pid " + $running.Id + ", '" + $running.Path + "').")
    Write-Host "       Close that instance first, or verify against it directly with:  -AttachLive"
  } else {
    Write-Host "[FAIL] No main window appeared."
  }
  if (-not $SkipCleanup) {
    try { Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue } catch {}
    Start-Sleep -Seconds 2
    Remove-Item -Recurse -Force $tempRoot -ErrorAction SilentlyContinue
  }
  exit 1
}

try {
  $scope = [System.Windows.Automation.AutomationElement]::FromHandle($hwnd)
  Invoke-Verification $scope $hwnd (-not $SkipKeys)
  Write-Host "=============================================================="
  if ($script:Failures -eq 0) { Write-Host " RESULT: ALL CHECKS PASSED" }
  else { Write-Host (" RESULT: " + $script:Failures + " CHECK(S) FAILED") }
  Write-Host "=============================================================="
} finally {
  if (-not $SkipCleanup) {
    try { Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue } catch {}
    Start-Sleep -Seconds 2
    Remove-Item -Recurse -Force $tempRoot -ErrorAction SilentlyContinue
    Write-Host "Cleanup: app stopped, temp profile removed."
  } else {
    Write-Host ("SkipCleanup: app still running (pid " + $proc.Id + "), temp profile at " + $tempRoot)
  }
}

if ($script:Failures -gt 0) { exit 1 }
exit 0
