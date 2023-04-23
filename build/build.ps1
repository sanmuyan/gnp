New-Item -ItemType Directory -Path gnp -Force
New-Item -ItemType Directory -Path pkg -Force
$sourcePath = "$(Get-Location)\gnp\"
$destinationPath = "$(Get-Location)\pkg"
function zip {
    param (
        $sourcePath,
        $destinationPath
    )
    Compress-Archive -Path $sourcePath -DestinationPath $destinationPath -Force
}


$os="linux"
$arch_list=@("amd64", "arm")

foreach ($arch in $arch_list) {
    $env:GOOS=$os
    $env:GOARCH=$arch
    go build -o .\gnp\gnpc ..\cmd\client
    go build -o .\gnp\gnps ..\cmd\server
    zip $sourcePath $destinationPath\gnp-$os-$arch.zip
    Remove-Item -Path $sourcePath\* -Force
}

$os = "darwin"
$arch_list = @("amd64", "arm64")

foreach ($arch in $arch_list) {
    $env:GOOS = $os
    $env:GOARCH = $arch
    go build -o .\gnp\gnpc ..\cmd\client
    go build -o .\gnp\gnps ..\cmd\server
    zip $sourcePath $destinationPath\gnp-$os-$arch.zip
    Remove-Item -Path $sourcePath\* -Force
}

$os = "windows"
$arch_list = @("amd64")

foreach ($arch in $arch_list) {
    $env:GOOS = $os
    $env:GOARCH = $arch
    go build -o .\gnp\gnpc.exe ..\cmd\client
    go build -o .\gnp\gnps.exe ..\cmd\server
    zip $sourcePath $destinationPath\gnp-$os-$arch.zip
    Remove-Item -Path $sourcePath\* -Force
}