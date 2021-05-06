# Vendored app

To recreate this vendored app:

**Prerequisites**
- Must be run on linux OS and **case-insensitive file system**
- Install conda (using the [installer from the miniconda
  buildpack](https://github.com/paketo-community/miniconda/blob/560c8d11b9f8cc8ad36eb3fcf3dda91ac946b850/buildpack.toml#L18)
- Install conda build tools: `conda install conda-build`

**Steps**
1. `cd integration/testdata/vendored_app`
1. Use the existing `environment.yml` file in the root of the app
1. `CONDA_PKGS_DIRS=vendor/noarch conda env create -f environment.yml -n vendored_app`
1. `conda index vendor`
1. `conda list -n vendored_app -e > package-list.txt`
1. Commit `environment.yml`, `vendor`, and `package-list.txt`
