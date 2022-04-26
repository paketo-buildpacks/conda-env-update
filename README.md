# Paketo Conda Env Update Cloud Native Buildpack

The Conda Env Update Buildpack runs commands to update the conda environment. It installs the conda environment into a
layer which makes it available for subsequent buildpacks and in the final running container.

The buildpack is published for consumption at `gcr.io/paketo-buildpacks/conda-env-update` and
`paketobuildpacks/conda-env-update`.

## Behavior

This buildpack participates when there is an `environment.yml` or
`package-list.txt` file in the app directory.

The buildpack will do the following:

* At build time:
    - Requires that conda has already been installed in the build container
    - Updates the conda environment and stores it in a layer
      - If a `package-list.txt` is in the app dir, a new environment is created
        from it
      - Otherwise, the `environment.yml` file is used to update the environment
    - Reuses the cached conda environment layer from a previous build if and
      only if a `package-list.txt` is in the app dir and it has not changed
      since the previous build
* At run time:
    - Does nothing

## Integration

The Conda Env Update Buildpack provides `conda-environment` as a dependency. Downstream buildpacks can require the
conda-environment dependency by
generating a [Build Plan TOML](https://github.com/buildpacks/spec/blob/master/buildpack.md#build-plan-toml)
file that looks like the following:

```toml
[[requires]]
# The name of the Conda Env Update dependency is "conda-environment". This value is considered
# part of the public API for the buildpack and will not change without a plan
# for deprecation.
name = "conda-environment"

# The Conda Env Update buildpack supports some non-required metadata options.
[requires.metadata]

# Setting the build flag to true will ensure that the conda environment
# layer is available for subsequent buildpacks during their build phase.
# If you are writing a buildpack that needs the conda environment
# during its build process, this flag should be set to true.
build = true

# Setting the launch flag to true will ensure that the conda environment is
# available to the running application. If you are writing an application
# that needs to use the conda environment at runtime, this flag should be set to true.
launch = true
```

## SBOM

This buildpack can generate a Software Bill of Materials (SBOM) for the dependencies of an application.

However, this feature only works if an application vendors its dependencies in
the `vendor` directory. This is due to a limitation in the upstream SBOM
generation library (Syft).

Applications that declare their dependencies via a `package-list.txt` file but
do not vendor them will result in an empty SBOM.

## Usage

To package this buildpack for consumption:

```
$ ./scripts/package.sh --version <version-number>
```

This will create a `buildpackage.cnb` file under the `build` directory which you can use to build your app as follows:
`pack build <app-name> -p <path-to-app> -b build/buildpackage.cnb -b <other-buildpacks..>`

To run the unit and integration tests for this buildpack:

```
$ ./scripts/unit.sh && ./scripts/integration.sh
```