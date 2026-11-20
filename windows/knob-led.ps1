# Sets the colour of the knob's lights over Bluetooth. AnticaterKnob.exe starts this
# script once, hidden, and writes one line per change to its standard input:
#
#   RRGGBB BRIGHTNESS     e.g. "00A7E1 45"   (brightness 0..100)
#   off                   lights off
#
# It answers each line with "ok ..." or "error ...". The command is the one the
# vendor's ANTICATER app sends (read from its code, 21/11/2026): a 64-byte packet
#   03 FD 00 06 R G B BRIGHTNESS ON 00 ... 00
# written without response to characteristic AE41 of the knob's own service AE40.
# One colour for the whole knob; the knob has no per-light command.
#
# Windows PowerShell 5.1 reaches the WinRT Bluetooth API directly, so no extra
# program or library is needed. The vendor app must be closed: while it runs it
# holds the service and opening it fails with SharingViolation.

$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Runtime.WindowsRuntime
$asTask = [System.WindowsRuntimeSystemExtensions].GetMethods() | Where-Object {
    $_.Name -eq 'AsTask' -and $_.GetParameters().Count -eq 1 -and $_.GetParameters()[0].ParameterType.Name -eq 'IAsyncOperation`1' } | Select-Object -First 1
function Await($op, [Type]$t) {
    $task = $asTask.MakeGenericMethod($t).Invoke($null, @($op))
    if (-not $task.Wait(10000)) { throw 'timeout' }
    $task.Result
}
$null = [Windows.Devices.Bluetooth.BluetoothLEDevice, Windows.Devices.Bluetooth, ContentType = WindowsRuntime]
$null = [Windows.Devices.Bluetooth.GenericAttributeProfile.GattCharacteristic, Windows.Devices.Bluetooth, ContentType = WindowsRuntime]
$null = [Windows.Devices.Enumeration.DeviceInformation, Windows.Devices.Enumeration, ContentType = WindowsRuntime]
$null = [Windows.Security.Cryptography.CryptographicBuffer, Windows.Security.Cryptography, ContentType = WindowsRuntime]
$null = [Windows.Storage.Streams.IBuffer, Windows.Storage.Streams, ContentType = WindowsRuntime]
$G = 'Windows.Devices.Bluetooth.GenericAttributeProfile'
$cache = [Windows.Devices.Bluetooth.BluetoothCacheMode]::Cached
$write = [Windows.Devices.Bluetooth.GenericAttributeProfile.GattCharacteristic].GetMethod('WriteValueAsync',
    [Type[]]@([Windows.Storage.Streams.IBuffer], [Windows.Devices.Bluetooth.GenericAttributeProfile.GattWriteOption]))
$script:dev = $null
$script:char = $null

function Connect {
    $sel = [Windows.Devices.Bluetooth.BluetoothLEDevice]::GetDeviceSelectorFromPairingState($true)
    $infos = Await ([Windows.Devices.Enumeration.DeviceInformation]::FindAllAsync($sel)) ([Windows.Devices.Enumeration.DeviceInformationCollection])
    foreach ($info in $infos) {
        if ($info.Name -notlike 'ANTICATER*') { continue }
        $d = Await ([Windows.Devices.Bluetooth.BluetoothLEDevice]::FromIdAsync($info.Id)) ([Windows.Devices.Bluetooth.BluetoothLEDevice])
        if (-not $d) { continue }
        $sr = Await ($d.GetGattServicesAsync($cache)) ("$G.GattDeviceServicesResult" -as [Type])
        foreach ($svc in $sr.Services) {
            if ($svc.Uuid.ToString().Substring(4, 4).ToUpper() -ne 'AE40') { continue }
            $open = Await ($svc.OpenAsync([Windows.Devices.Bluetooth.GenericAttributeProfile.GattSharingMode]::SharedReadAndWrite)) ("$G.GattOpenStatus" -as [Type])
            if ("$open" -ne 'Success' -and "$open" -ne 'AlreadyOpened') { throw "open AE40: $open (is the vendor app running?)" }
            $cr = Await ($svc.GetCharacteristicsAsync($cache)) ("$G.GattCharacteristicsResult" -as [Type])
            foreach ($c in $cr.Characteristics) {
                if ($c.Uuid.ToString().Substring(4, 4).ToUpper() -eq 'AE41') { $script:dev = $d; $script:char = $c; return }
            }
        }
        $d.Dispose()
    }
    throw 'knob not found (paired Bluetooth device named ANTICATER with service AE40)'
}

function Send([byte[]]$pkt) {
    if (-not $script:char) { Connect }
    $buf = [Windows.Security.Cryptography.CryptographicBuffer]::CreateFromByteArray($pkt)
    $st = Await ($write.Invoke($script:char, [object[]]@($buf, [Windows.Devices.Bluetooth.GenericAttributeProfile.GattWriteOption]::WriteWithoutResponse))) ("$G.GattCommunicationStatus" -as [Type])
    if ("$st" -ne 'Success') { throw "write: $st" }
}

while ($null -ne ($line = [Console]::In.ReadLine())) {
    $line = $line.Trim()
    if ($line -eq '') { continue }
    $pkt = New-Object byte[] 64
    $pkt[0] = 0x03; $pkt[1] = 0xFD; $pkt[3] = 0x06
    if ($line -eq 'off') {
        $pkt[7] = 0; $pkt[8] = 0
    } elseif ($line -match '^([0-9A-Fa-f]{6})\s+(\d{1,3})$') {
        $hex = $Matches[1]
        $pkt[4] = [Convert]::ToByte($hex.Substring(0, 2), 16)
        $pkt[5] = [Convert]::ToByte($hex.Substring(2, 2), 16)
        $pkt[6] = [Convert]::ToByte($hex.Substring(4, 2), 16)
        $pkt[7] = [byte][Math]::Min(100, [int]$Matches[2]); $pkt[8] = 1
    } else {
        [Console]::Out.WriteLine("error bad line: $line"); [Console]::Out.Flush(); continue
    }
    try {
        try { Send $pkt } catch { $script:char = $null; if ($script:dev) { $script:dev.Dispose(); $script:dev = $null }; Send $pkt }
        [Console]::Out.WriteLine("ok $line")
    } catch {
        $script:char = $null
        [Console]::Out.WriteLine("error $($_.Exception.Message)")
    }
    [Console]::Out.Flush()
}
