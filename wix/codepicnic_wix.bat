et GOPATH=%CD%
cd wix
"C:\Program Files (x86)\WiX Toolset v3.10\bin\candle.exe" codepicnic.wxs
"C:\Program Files (x86)\WiX Toolset v3.10\bin\light.exe" codepicnic.wixobj
"C:\Program Files (x86)\WiX Toolset v3.10\bin\candle.exe"  -ext WixBalExtension -out codepicnic_bundle.wixobj codepicnic_bundle.wxs
"C:\Program Files (x86)\WiX Toolset v3.10\bin\light.exe" -ext WixBalExtension -out codepicnic_setup.exe codepicnic_bundle.wixobj 