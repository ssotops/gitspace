# gitspace

gitspace is a CLI tool designed to manage and organize your git repositories efficiently.

## Installation

### Quick Install

To quickly install gitspace, you can use our install script:

```bash
curl -sSL https://raw.githubusercontent.com/ssotops/gitspace/main/install.sh | bash
```

This script will automatically download and install the latest version of gitspace.

### Manual Installation

If you prefer to install manually:

1. Go to the [releases page](https://github.com/ssotops/gitspace/releases) and download the latest version for your operating system.
2. Rename the downloaded file to `gitspace` (or `gitspace.exe` on Windows).
3. Make the file executable: `chmod +x gitspace` (not needed on Windows).
4. Move the file to a directory in your PATH, e.g., `/usr/local/bin` on Unix-like systems.

## Uninstallation

To uninstall gitspace, you can use our uninstall script:

```bash
curl -sSL https://raw.githubusercontent.com/ssotops/gitspace/main/uninstall.sh | bash
```

This script will remove gitspace and its configuration files.

For manual uninstallation:

1. Remove the gitspace executable from your PATH.
2. Optionally, remove the configuration directory (usually `~/.ssot/gitspace`).

## Getting Started with gitspace

1. Create a configuration file named `gs.toml` in your project directory with the following content:

```toml
[global]
path = "gs"
labels = ["feature", "bug"]
scm = "github.com"
owner = "ssotops"

[auth]
type = "ssh"
key_path = "~/.ssh/my-key"

[groups.git]
match = "startsWith"
values = ["git"]
type = "gitops"
labels = ["backend", "core"]

[groups.space]
match = "endsWith"
values = ["space"]
type = "solution"
labels = ["frontend", "experimental"]

[groups.sso]
match = "includes"
values = ["sso"]
type = "ssot"
labels = ["auth", "security"]

[groups.scmany]
match = "isExactly"
values = ["scmany"]
type = "helper"
labels = ["utility"]
```

2. Set up your GitHub token:
   ```bash
   export GITHUB_TOKEN=your_github_token_here
   ```

3. Run gitspace:
   ```bash
   gitspace
   ```

4. Follow the prompts to specify the path to your config file (or press Enter to use the default `./gs.toml`).

5. gitspace will clone the repositories matching your configuration and create symlinks.

## Configuration Explanation

- `[global]`: Global settings for gitspace.
  - `path`: The base directory where gitspace will create symlinks to your cloned repositories.
  - `labels`: Global labels to be applied to all repositories.
  - `scm`: The source control management system (e.g., "github.com").
  - `owner`: The GitHub organization or user owning the repositories.
- `[auth]`: Authentication settings.
  - `type`: The authentication method (e.g., "ssh").
  - `key_path`: Path to your SSH key. Can be a direct path (e.g., "~/.ssh/my-key") or an environment variable prefixed with "$" (e.g., "$SSH_KEY_PATH").
- `[groups.<name>]`: Repository grouping and filtering rules.
  - `match`: The matching method ("startsWith", "endsWith", "includes", or "isExactly").
  - `values`: Array of strings to match against repository names.
  - `type`: Type of the repository for this group.
  - `labels`: Labels specific to repositories in this group.

## Features

- Clones multiple repositories based on specified criteria.
- Creates symlinks for easy access to cloned repositories.
- Applies labels to repositories based on global and group-specific configurations.
- Provides a summary of cloning and symlinking operations.
- Supports plugins for extending functionality.

### Plugins
Gitspace supports plugins to extend its functionality. You can install, uninstall, and run plugins using the built-in plugin management system.

### Gitspace Catalog
Gitspace includes a catalog feature that allows you to easily install pre-defined plugins and templates. You can browse [Gitspace Catalog](https://github.com/ssotops/gitspace-catalog).

### Upgrading Gitspace
You can upgrade Gitspace to the latest version using the built-in upgrade functionality.

## Additional Configuration

In the `[global]` section of your `gs.toml` file, you can also set:
- `empty_repo_initial_branch`: Specifies the initial branch name for empty repositories (default is "master").
