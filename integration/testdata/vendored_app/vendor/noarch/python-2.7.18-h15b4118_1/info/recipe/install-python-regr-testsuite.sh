#!/usr/bin/env bash

pushd ${PREFIX}/lib/python${VER}
  mv ${SRC_DIR}/test.copied.by.install-python.sh/* test/
popd
