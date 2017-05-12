cd wix
candle codepicnic.wxs
light codepicnic.wixobj
candle  -ext WixBalExtension -out codepicnic_bundle.wixobj codepicnic_bundle.wxs
light -ext WixBalExtension -out codepicnic_setup.exe codepicnic_bundle.wixobj 