#!/bin/sh
echo "Running postinstall" > /tmp/my_postinstall.log
sudo hdiutil attach -mountpoint /private/tmp/osxfuse osxfuse-3.4.2.dmg >> /tmp/my_postinstall.log

cd /tmp/osxfuse/Extras

sudo ls >> /tmp/my_postinstall.log

sudo installer -pkg  "FUSE for macOS 3.4.2.pkg" -target "/" >> /tmp/my_postinstall.log

cd /

sudo hdiutil detach /tmp/osxfuse
#sudo installer -package osxfuse-3.4.2.dmg -target / >> /tmp/my_postinstall.log



exit 0 # all good
