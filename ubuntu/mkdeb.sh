#!/bin/bash
# Configure your paths and filenames
SOURCEBINPATH=/opt/build
SOURCEBIN=codepicnic
SOURCEICON=codepicnic.png
DEBFOLDER=codepicnic
DEBVERSION=$1
export USER="root"
cp .devscripts /root/
DEBFOLDERNAME=$DEBFOLDER-$DEBVERSION

# Create your scripts source dir
mkdir $DEBFOLDERNAME

# Copy your script to the source dir
cp $SOURCEBINPATH/$SOURCEBIN $DEBFOLDERNAME
cp $SOURCEBINPATH/$SOURCEICON $DEBFOLDERNAME
cd $DEBFOLDERNAME

# Create the packaging skeleton (debian/*)
echo "yes" | dh_make -n -s --indep --createorig

# Remove make calls
grep -v makefile debian/rules > debian/rules.new
mv debian/rules.new debian/rules

# Add control/changelog files
cp ../control debian/control
cp ../changelog debian/changelog

# debian/install must contain the list of scripts to install 
# as well as the target directory
echo $SOURCEBIN usr/bin > debian/install
echo $SOURCEICON usr/share/codepicnic >> debian/install

# Remove the example files
#rm debian/*.ex

# Build the package.
# You  will get a lot of warnings and ../somescripts_0.1-1_i386.deb
debuild

#rsync -av $DEBFOLDER_$DEBVERSION-1_all.deb ec2-user@jenkins.codepicnic.com:/tmp/

