# Cockpit SLURM

A [Cockpit](https://cockpit-project.org/) module for managing Slurm state.

# Development dependencies

On Debian/Ubuntu:

    sudo apt install gettext nodejs npm make golang-go

On Fedora:

    sudo dnf install gettext nodejs npm make golang

On openSUSE Tumbleweed and Leap:

    sudo zypper in gettext-runtime nodejs npm make go

This repository also includes a Go backend, so the Go toolchain and an editor language server are recommended.

After installing Go, configure the module proxy and install `gopls` locally:

    go env -w GOPROXY=https://goproxy.io,direct
    go install golang.org/x/tools/gopls@latest

These commands are local environment setup and are not tracked by the repository.

# Getting and building the source

These commands check out the source and build it into the `dist/` directory:

```
git clone https://github.com/lingweicai/cockpit-slurm.git
cd cockpit-slurm
make
```

# Installing

## Install for system wide

`make install` compiles and installs the package of dist built with directory of src for frontend files in `/usr/local/share/cockpit`. 

install backend executable of cockpit-slurm-bridge and cockpit-slurm-channel for in path below: 
`/usr/local/sbin/cockpit-slurm-bridge`
`/usr/local/libexec/cockpit-slurm-channel`

and socket file to the path:
`/run/cockpit-slurm/bridge.sock`

## Install with RPM package
For Redhat/RockyLinux RPM package of `production` mode, install front end dist directory to: 
`/usr/share/cockpit` 
and backend files to:
`/usr/sbin/cockpit-slurm-bridge`
`/usr/libexec/cockpit-slurm-channel`
`/run/cockpit-slurm/bridge.sock`

To start cockpit-slurm-bridge service in backend, run command 
`sudo systemctl start cockpit-slurm-bridge`

## Install for development user

For development, you usually want to run your module straight out of the git
tree. To do that, run `make devel-install`, which links your checkout to the
location were cockpit-bridge looks for packages, installs bridge/channel under
`~/.local/libexec/cockpit-slurm`, and creates `~/.local/bin` symlinks. It also
configures `COCKPIT_SLURM_BRIDGE_SOCKET_PATH` to:

`/run/user/<uid>/cockpit-slurm/bridge.sock`

If you prefer to do this manually:

```
mkdir -p ~/.local/share/cockpit
ln -s `pwd`/dist ~/.local/share/cockpit/cockpit-slurm
```

After changing the code and running `make` again, reload the Cockpit page in
your browser.

You can also use
[watch mode](https://esbuild.github.io/api/#watch) to
automatically update the bundle on every code change with

    ./build.js -w

or

    make watch

When developing against a virtual machine, watch mode can also automatically upload
the code changes by setting the `RSYNC` environment variable to
the remote hostname.

If you are testing locally with a custom bridge socket path, the frontend now supports
a runtime override via a browser global. Set it to the same path used by the bridge process:

```js
window.COCKPIT_SLURM_BRIDGE_SOCKET_PATH = "/run/user/1000/cockpit-slurm/bridge.sock";
```

This browser global is only required for local development testing where the frontend is
running against a non-standard bridge socket path. If that override is not set, the client
falls back to the default socket path at `/run/cockpit-slurm/bridge.sock`.

    RSYNC=c make watch

When developing against a remote host as a normal user, `RSYNC_DEVEL` can be
set to upload code changes to `~/.local/share/cockpit/` instead of
`/usr/local`.

    RSYNC_DEVEL=example.com make watch

To "uninstall" the locally installed version, run `make devel-uninstall`, or
remove manually the symlink:

    rm ~/.local/share/cockpit/cockpit-slurm

# Running eslint

Cockpit SLURM uses [ESLint](https://eslint.org/) to automatically check
JavaScript/TypeScript code style in `.js[x]` and `.ts[x]` files.

eslint is executed as part of `test/static-code`, aka. `make codecheck`.

For developer convenience, the ESLint can be started explicitly by:

    npm run eslint

Violations of some rules can be fixed automatically by:

    npm run eslint:fix

Rules configuration can be found in the `.eslintrc.json` file.

## Running stylelint

Cockpit uses [Stylelint](https://stylelint.io/) to automatically check CSS code
style in `.css` and `scss` files.

styleint is executed as part of `test/static-code`, aka. `make codecheck`.

For developer convenience, the Stylelint can be started explicitly by:

    npm run stylelint

Violations of some rules can be fixed automatically by:

    npm run stylelint:fix

Rules configuration can be found in the `.stylelintrc.json` file.

# Running tests locally

Run `make check` to build an RPM, install it into a standard Cockpit test VM
(centos-9-stream by default), and run the test/check-application integration test on
it. This uses Cockpit's Chrome DevTools Protocol based browser tests, through a
Python API abstraction. Note that this API is not guaranteed to be stable, so
if you run into failures and don't want to adjust tests, consider checking out
Cockpit's test/common from a tag instead of main (see the `test/common`
target in `Makefile`).

your project after forking from starter-kit.
After the test VM is prepared, you can manually run the test without rebuilding
the VM, possibly with extra options for tracing and halting on test failures
(for interactive debugging):

    TEST_OS=centos-9-stream test/check-application -tvs

It is possible to setup the test environment without running the tests:

    TEST_OS=centos-9-stream make prepare-check

You can also run the test against a different Cockpit image, for example:

    TEST_OS=fedora-40 make check

# Running tests in CI

These tests can be run in [Cirrus CI](https://cirrus-ci.org/), on their free
[Linux Containers](https://cirrus-ci.org/guide/quick-start/) environment which
explicitly supports `/dev/kvm`. Please see [Quick
Start](https://cirrus-ci.org/guide/quick-start/) how to set up Cirrus CI for
your project. The Cockpit starter-kit template was helpful during initial
bootstrapping but this repository is now a project-specific implementation.

The included [.cirrus.yml](./.cirrus.yml) runs the integration tests for two
operating systems (Fedora and CentOS 8). Note that if/once your project grows
bigger, or gets frequent changes, you may need to move to a paid account, or
different infrastructure with more capacity.

Tests also run in [Packit](https://packit.dev/) for all currently supported
Fedora releases; see the [packit.yaml](./packit.yaml) control file. You need to
[enable Packit-as-a-service](https://packit.dev/docs/packit-service/) in your GitHub project to use this.
To run the tests in the exact same way for upstream pull requests and for
[Fedora package update gating](https://docs.fedoraproject.org/en-US/ci/), the
tests are wrapped in the [FMF metadata format](https://github.com/teemtee/fmf)
for using with the [tmt test management tool](https://docs.fedoraproject.org/en-US/ci/tmt/).
Note that Packit tests can *not* run their own virtual machine images, thus
they only run [@nondestructive tests](https://github.com/cockpit-project/cockpit/blob/main/test/common/testlib.py).

# Customizing

This repository is tailored for `cockpit-slurm`. If you started from the
Cockpit starter-kit template, the template references have been updated to use
`cockpit-slurm` where appropriate; a few historical references to the template
remain in the development notes. To find any remaining template markers, run:

    find -iname '*starter*'
    git grep -i starter

# Automated release

Once your project is ready for a release, consider automating the process. The
intended release workflow is to create a signed tag with a changelog entry, and
let CI build and publish the release tarball. The existing `.github/workflows`
and `packit.yaml` files in this repository are configured for this purpose.

# Automated maintenance

Keep your `package.json` dependencies up to date to receive security and bug
fixes. Dependabot configuration is included in `.github/dependabot.yml`.

# Further reading

 * The [Starter Kit announcement](https://cockpit-project.org/blog/cockpit-starter-kit.html) (upstream template)
 * [Cockpit Deployment and Developer documentation](https://cockpit-project.org/guide/latest/)
 * [Make your project easily discoverable](https://cockpit-project.org/blog/making-a-cockpit-application.html)
