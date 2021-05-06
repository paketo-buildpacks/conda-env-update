

set -ex



python -V
pydoc -h
python-config --help
export PATH=${CONDA_PREFIX}/${HOST}/sysroot/bin:${CONDA_PREFIX}/${HOST}/sysroot/usr/bin:${PATH}
export LD_LIBRARY_PATH=${CONDA_PREFIX}/lib:${CONDA_PREFIX}/${HOST}/sysroot/lib64:${CONDA_PREFIX}/${HOST}/sysroot/usr/lib:/usr/lib64:/usr/lib
export DISPLAY=localhost:1
bash -x ${CONDA_PREFIX}/*-linux-gnu/sysroot/usr/bin/xvfb-run python $PWD/tests.py
python -c "import sysconfig; print sysconfig.get_config_var('CC')"
_PYTHON_SYSCONFIGDATA_NAME=_sysconfigdata_x86_64_conda_cos6_linux_gnu python -c "import sysconfig; print sysconfig.get_config_var('CC')"
python -c 'import ssl; print(ssl.get_default_verify_paths())' | grep ${CONDA_PREFIX}
exit 0
