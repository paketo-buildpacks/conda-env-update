#!/usr/bin/env bash

VER=${PKG_VERSION%.*}

make install

env

set -x
# Remove the regression test-suite to save space
# Though keep `support` as some things use that.
# TODO :: Make a subpackage for this once we implement multi-level testing.

pushd ${PREFIX}/lib/python${VER}
  cp -rf test ${SRC_DIR}/test.copied.by.install-python.sh && rm -rf test
  mkdir test
  mv ${SRC_DIR}/test.copied.by.install-python.sh/{__init__.py,test_support*,script_helper*} test
popd
# pushd ${PREFIX}/lib/python${VER}
#   mkdir test_keep
#   mv test/__init__.py test/test_support* test/script_helper* test_keep/
#   rm -rf test */test
#   mv test_keep test
# popd

# Size reductions:
pushd ${PREFIX}
  found_lib_libpython_a=no
  if [[ -f lib/libpython${VER}.a ]]; then
    found_lib_libpython_a=yes
    chmod +w lib/libpython${VER}.a
    if [[ -n ${HOST} ]]; then
      ${HOST}-strip -S lib/libpython${VER}.a
    else
      strip -S lib/libpython${VER}.a
    fi
  fi
  CONFIG_LIBPYTHON=$(find lib/python${VER}/config -name "libpython${VER}.a")
  if [[ -f ${CONFIG_LIBPYTHON} ]]; then
    if [[ ${found_lib_libpython_a} == yes ]]; then
      chmod +w ${CONFIG_LIBPYTHON}
      rm ${CONFIG_LIBPYTHON}
      ln -s ../../libpython${VER}.a ${CONFIG_LIBPYTHON}
    else
      chmod +w ${CONFIG_LIBPYTHON}
      if [[ -n ${HOST} ]]; then
        ${HOST}-strip -S ${CONFIG_LIBPYTHON}
      else
        strip -S ${CONFIG_LIBPYTHON}
      fi
    fi
  fi
popd

# Move the _sysconfigdata.py file and replace with a version with a
# configuration more typical of what a user would expect to be in a "standard"
# python build. This results in the system toolchain and
# The original configuration with the crosstool-ng compilers from the conda
# package can be selected by setting the _PYTHON_SYSCONFIGDATA_NAME
# environmental variable to _sysconfigdata_$HOST
#   using the new compilers with python will require setting _PYTHON_SYSCONFIGDATA_NAME
#   to the name of this file (minus the .py extension)
pushd $PREFIX/lib/python${VER}
  # On Python 3.5 _sysconfigdata.py was getting copied in here and compiled for some reason.
  # This breaks our attempt to find the right one as recorded_name.
  find lib-dynload -name "_sysconfigdata*.py*" -exec rm {} \;
  recorded_name=$(find . -name "_sysconfigdata*.py")
  our_compilers_name=_sysconfigdata_$(echo ${HOST} | sed -e 's/[.-]/_/g').py
  mv ${recorded_name} ${our_compilers_name}

  # Copy all "${RECIPE_DIR}"/sysconfigdata/*.py. This is to support cross-compilation. They will be
  # from the previous build unfortunately so care must be taken at version bumps and flag changes.
  cp -rf "${RECIPE_DIR}"/sysconfigdata/*.py ${PREFIX}/lib/python${VER}/

  if [[ ${HOST} =~ .*darwin.* ]]; then
    cp ${RECIPE_DIR}/sysconfigdata/default/_sysconfigdata_osx.py ${recorded_name}
  else
    if [[ ${HOST} =~ x86_64.* ]]; then
      PY_ARCH=x86_64
    elif [[ ${HOST} =~ i686.* ]]; then
      PY_ARCH=i386
    elif [[ ${HOST} =~ powerpc64le.* ]]; then
      PY_ARCH=powerpc64le
    else
      echo "ERROR: Cannot determine PY_ARCH for host ${HOST}"
      exit 1
    fi
    cat ${RECIPE_DIR}/sysconfigdata/default/_sysconfigdata_linux.py | sed "s|@ARCH@|${PY_ARCH}|g" > ${recorded_name}
    mkdir -p ${PREFIX}/compiler_compat
    cp ${LD} ${PREFIX}/compiler_compat/ld
    echo "Files in this folder are to enhance backwards compatibility of anaconda software with older compilers."   > ${PREFIX}/compiler_compat/README
    echo "See: https://github.com/conda/conda/issues/6030 for more information."                                   >> ${PREFIX}/compiler_compat/README
  fi

  # Copy the latest sysconfigdata for this platform back to the recipe so we can do full cross-compilation
  [[ -f "${RECIPE_DIR}"/sysconfigdata/${our_compilers_name} ]] && rm -f "${RECIPE_DIR}"/sysconfigdata/${our_compilers_name}
  cat ${our_compilers_name} | sed "s|${PREFIX}|/opt/anaconda1anaconda2anaconda3|g" > "${RECIPE_DIR}"/sysconfigdata/${our_compilers_name}
popd

# https://github.com/ContinuumIO/anaconda-issues/issues/6424
# TODO :: Move this into conda-build as an error with an override (same for openssl-feedstock)
if [[ ${HOST} =~ .*linux.* ]]; then
  for _SO in lib/python2.7/lib-dynload/_ssl.so ./lib/python2.7/lib-dynload/_hashlib.so; do
    if execstack -q "${PREFIX}"/${_SO} | grep -e '^X '; then
      echo "Error, executable stack found in ${_SO}"
      exit 1
    fi
  done
fi
