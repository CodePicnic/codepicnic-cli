#!/bin/bash
echo "deb http://apt.codepicnic.com/ stable main" > /etc/apt/sources.list.d/codepicnic.list
cd /etc/apt/trusted.gpg.d/
wget http://apt.codepicnic.com/codepicnic.gpg
apt-get update
apt-get -y  install codepicnic-cli fuse
mkdir ~/.codepicnic
touch ~/.codepicnic/credentials
