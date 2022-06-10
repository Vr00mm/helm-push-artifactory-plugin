#!/bin/sh -e

set -eu
set -x

DIST_DIRS_EXEC="find * -maxdepth 0 -type d -exec"

cd _dist
${DIST_DIRS_EXEC} mkdir -p {}/${PLUGIN_FULL_NAME} \;
${DIST_DIRS_EXEC} mkdir -p {}/${PLUGIN_FULL_NAME}/scripts \; 
${DIST_DIRS_EXEC} cp ../plugin.yaml {}/${PLUGIN_FULL_NAME} \; 
${DIST_DIRS_EXEC} cp ../README.md {}/${PLUGIN_FULL_NAME} \; 
${DIST_DIRS_EXEC} cp ../scripts/install_plugin.sh {}/${PLUGIN_FULL_NAME}/scripts \;
${DIST_DIRS_EXEC} tar -zcf ${BIN_NAME}-${VERSION}-{}.tar.gz -C {} . \;

DIST_DIRS=$(find * -maxdepth 0 -type d )

for d in ${DIST_DIRS}; do
    cd $d/${PLUGIN_FULL_NAME}
    zip -r ../../${BIN_NAME}-${VERSION}-${d}.zip ./*
    cd ../..
done
