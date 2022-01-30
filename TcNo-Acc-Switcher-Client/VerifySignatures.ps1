
function RunSign {
    param (
        [string]$p
    )
    Write-Host $p
    
    $args = 'sign /tr http://timestamp.sectigo.com?td=sha256 /td SHA256 /fd SHA256 /a "' + $p + '"'
    Start-Process "C:\Program Files (x86)\Windows Kits\10\bin\10.0.22000.0\x64\signtool.exe" -ArgumentList $args
    Write-Host $args
}

cd "C:\Users\TCNO\Documents\GitHub\TcNo-Acc-Switcher\TcNo-Acc-Switcher-Client\bin\x64\Release\TcNo-Acc-Switcher"

Get-ChildItem TcNo*.exe,TcNo*.dll,runas.exe,_First* -Recurse | ForEach-object {Get-AuthenticodeSignature $_} | Where-Object {$_.status -eq "NotSigned"} | ForEach-Object { RunSign($_.path)}