# git-sw

A quality-of-life, zero-dependency fuzzy branch switcher for Git.

`git-sw` acts as a drop-in replacement for `git switch`. When passed arguments, it executes a standard `git switch` identically. When run with no arguments, it opens an inline fuzzy picker to quickly search and switch between your local branches.

- Prioritizes speed: limits fetches to ensure instant loads even in massive monorepos.
- Defaults to "Recently Checked Out" order based on your `git reflog`.
- Press `Tab` to toggle to "Recently Updated" order based on commit date.
- Renders inline (capping at 40% of terminal height) to keep your terminal history clean.

## Installation

Because Git natively resolves `git <command>` to binaries named `git-<command>` in your `PATH`, simply having this binary available as `git-sw` is enough to make `git sw` work.

**Important**: Ensure you don't have an existing `sw` alias in your `~/.gitconfig` (`sw = switch`), as Git aliases override `PATH` binaries.

### Nix (Flakes)

If you use Nix Flakes / Home Manager, add it to your `flake.nix` inputs:

```nix
inputs.git-sw.url = "github:dnswd/git-sw";
```

Then add the package to your configuration:

```nix
home.packages = [
  inputs.git-sw.packages.${pkgs.system}.default
];
```

### Go Install

If you have Go installed, you can build and install it directly:

```sh
go install github.com/dnswd/git-sw@latest
```

*(Ensure your `~/go/bin` is in your system `$PATH`)*

### Manual

Clone this repository, build the binary, and place it anywhere in your `PATH`:

```sh
git clone https://github.com/dnswd/git-sw.git
cd git-sw
go build -o git-sw main.go
sudo mv git-sw /usr/local/bin/
```