#!/bin/sh
echo "Running postinstall" > /tmp/codepicnic_postinstall.log
sudo ls >> /tmp/codepicnic_postinstall.log
echo "mkdir" >> /tmp/codepicnic_postinstall.log
sudo mkdir /usr/local/codepicnic 
sudo mv codepicnic.png /usr/local/codepicnic/codepicnic.png >> /tmp/codepicnic_postinstall.log
sudo hdiutil attach -mountpoint /private/tmp/osxfuse osxfuse-3.4.2.dmg >> /tmp/codepicnic_postinstall.log
cd /tmp/osxfuse/Extras
sudo ls >> /tmp/codepicnic_postinstall.log
sudo installer -pkg  "FUSE for macOS 3.4.2.pkg" -target "/" >> /tmp/codepicnic_postinstall.log
cd /
sudo hdiutil detach /tmp/osxfuse
exit 0 # all good
