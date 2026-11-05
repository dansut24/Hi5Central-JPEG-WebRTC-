# Hi5Central JPEG WebRTC — user-session tray controller.
# Deliberately visible in the notification area; local user can pause or exit.
# Runs under the installing user's Windows account, without elevation.
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing

$root = Join-Path $env:LOCALAPPDATA 'Hi5Central\JPEGWebRTC'
$executable = Join-Path $root 'Hi5-JPEG-ScreenShare-GUI.exe'
$statusFile = Join-Path $root 'status.txt'
if (!(Test-Path -LiteralPath $executable)) { exit 2 }

$created = $false
$mutex = New-Object System.Threading.Mutex($true, 'Local\Hi5JPEGWebRTCTray', [ref]$created)
if (!$created) { $mutex.Dispose(); exit 0 }

$context = New-Object System.Windows.Forms.ApplicationContext
$icon = New-Object System.Windows.Forms.NotifyIcon
$icon.Icon = [System.Drawing.SystemIcons]::Information
$icon.Text = 'Hi5Central JPEG - Starting'
$icon.Visible = $true
$menu = New-Object System.Windows.Forms.ContextMenuStrip
$portalItem = $menu.Items.Add('Open protected device portal')
$pauseItem = $menu.Items.Add('Pause sharing')
$exitItem = $menu.Items.Add('Exit agent (stop sharing)')
$icon.ContextMenuStrip = $menu

$script:agent = $null
$script:paused = $false
$script:stopping = $false
$script:lastStatus = ''
$script:lastLaunch = [datetime]::MinValue
$script:lastBalloon = [datetime]::MinValue
$script:context = $context
$script:icon = $icon

function Start-Agent {
    if ($script:paused -or $script:stopping) { return }
    if (!(Test-Path -LiteralPath $executable)) { return }
    if ($script:agent -and !$script:agent.HasExited) { return }
    if (((Get-Date) - $script:lastLaunch).TotalSeconds -lt 5) { return }
    $script:lastLaunch = Get-Date
    try {
        Set-Content -LiteralPath $statusFile -Value 'connecting' -Encoding ascii
        $script:agent = Start-Process -FilePath $executable -WorkingDirectory $root -PassThru -WindowStyle Hidden
    } catch {
        $script:icon.Text = 'Hi5Central JPEG - Agent failed'
    }
}
function Stop-Agent {
    if ($script:agent) {
        try {
            if (!$script:agent.HasExited) {
                Stop-Process -Id $script:agent.Id -Force -ErrorAction SilentlyContinue
            }
        } catch {}
    }
    $script:agent = $null
    Set-Content -LiteralPath $statusFile -Value 'paused' -Encoding ascii
}
$portalItem.add_Click({
    try { Start-Process 'https://jpeg.hi5central.com/portal' } catch {}
})
$pauseItem.add_Click({
    if ($script:paused) {
        $script:paused = $false
        $pauseItem.Text = 'Pause sharing'
        Start-Agent
    } else {
        $script:paused = $true
        Stop-Agent
        $pauseItem.Text = 'Resume sharing'
        $script:icon.Text = 'Hi5Central JPEG - Paused by user'
        $script:icon.Icon = [System.Drawing.SystemIcons]::Application
    }
})
$exitItem.add_Click({
    $script:stopping = $true
    Stop-Agent
    $script:context.ExitThread()
})
$icon.add_DoubleClick({
    try { Start-Process 'https://jpeg.hi5central.com/portal' } catch {}
})
$timer = New-Object System.Windows.Forms.Timer
$timer.Interval = 1500
$timer.add_Tick({
    if (!$script:paused -and !$script:stopping) { Start-Agent }
    $state = 'offline'
    if ($script:paused) {
        $state = 'paused'
    } elseif ($script:agent -and !$script:agent.HasExited) {
        if (Test-Path -LiteralPath $statusFile) {
            try { $state = (Get-Content -LiteralPath $statusFile -TotalCount 1 -ErrorAction Stop).Trim() } catch {}
        } else { $state = 'connecting' }
    }
    if ($state -ne $script:lastStatus) {
        $script:lastStatus = $state
        switch ($state) {
          'streaming' {
            $script:icon.Icon = [System.Drawing.SystemIcons]::Warning
            $script:icon.Text = 'Hi5Central JPEG - Screen is being viewed'
            if (((Get-Date) - $script:lastBalloon).TotalSeconds -gt 30) {
                $script:lastBalloon = Get-Date
                $script:icon.ShowBalloonTip(5000, 'Hi5Central JPEG sharing', 'A remote viewer is watching your screen.', [System.Windows.Forms.ToolTipIcon]::Warning)
            }
          }
          'ready' {
            $script:icon.Icon = [System.Drawing.SystemIcons]::Information
            $script:icon.Text = 'Hi5Central JPEG - Online (not streaming)'
          }
          'paused' {
            $script:icon.Icon = [System.Drawing.SystemIcons]::Application
            $script:icon.Text = 'Hi5Central JPEG - Paused locally'
          }
          default {
            $script:icon.Icon = [System.Drawing.SystemIcons]::Application
            $script:icon.Text = 'Hi5Central JPEG - Connecting / Offline'
          }
        }
    }
})
try {
    Start-Agent
    $timer.Start()
    [System.Windows.Forms.Application]::Run($context)
} finally {
    $script:stopping = $true
    $timer.Stop()
    Stop-Agent
    $icon.Visible = $false
    $icon.Dispose()
    $timer.Dispose()
    $menu.Dispose()
    $context.Dispose()
    try { $mutex.ReleaseMutex() } catch {}
    $mutex.Dispose()
}
