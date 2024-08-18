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
2. Optionally, remove the configuration directory (usually `~/.gitspace`).

## Getting Started with gitspace

1. Create a configuration file named `gs.hcl` in your project directory with the following content:

```hcl
repositories {
  gitspace {
    path = "gs"
  }
  clone {
    scm = "github.com"
    owner = "ssotops"
    # repositories starting with "git"
    # eg. github.com/ssotops/gitspace
    startsWith = ["git"]

    # repositories ending with "space", 
    # eg. github.com/ssotops/gitspace, github.com/ssotops/k1space, github.com/ssotops/ssotspace
    endsWith = ["space"]

    # repositories containing 
    # eg. github.com/ssotops/ssotspace
    includes = ["sso"]

    # repositories named "scmany"
    # eg. github.com/ssotops/scmany
    name = ["scmany"]
    auth {
      type = "ssh"
      keyPath = "~/.ssh/my-key"
    }
  }
}
```

2. Set up your GitHub token:
   ```bash
   export GITHUB_TOKEN=your_github_token_here
   ```

3. Run gitspace:
   ```bash
   gitspace
   ```

4. Follow the prompts to specify the path to your config file (or press Enter to use the default `./gs.hcl`).

5. gitspace will clone the repositories matching your configuration and create symlinks.

## Configuration Explanation

- `path`: The base directory where gitspace will create symlinks to your cloned repositories.
- `scm`: The source control management system (e.g., "github.com").
- `owner`: The GitHub organization or user owning the repositories.
- `auth`: Specifies the authentication method (SSH in this case) and the path to your SSH key.
  - `keyPath`: Can be either a direct path to your SSH key (e.g., "~/.ssh/my-key") or an environment variable name prefixed with "$" (e.g., "$SSH_KEY_PATH"). If an environment variable is used, gitspace will read the key path from that variable.
- `endsWith`: Filters repositories to clone based on their name endings (eg. `space` will match `gitspace`).
- `startsWith`: Filters repositories to clone based on their name prefixes (eg. `git` will match `gitspace`).
- `includes`: Filters repositories to clone based on substrings in their names (eg. `sot` will match `ssotspace`).
- `name`: Filters repositories to clone based on exact name matches.

## Example Configuration

```hcl
repositories {
  gitspace {
    path = "gs"
  }
  clone {
    scm = "github.com"
    owner = "ssotops"
    startsWith = ["git"]
    endsWith = ["space"]
    includes = ["sso"]
    name = ["scmany"]
    auth {
      type = "ssh"
      keyPath = "$SSH_KEY_PATH"  # This will read the SSH key path from the SSH_KEY_PATH environment variable
    }
  }
}
```

In this example, the SSH key path is set to `$SSH_KEY_PATH`. Before running gitspace, you would set this environment variable:

```bash
export SSH_KEY_PATH=~/.ssh/my-github-key
```

Alternatively, you could specify the path directly in the configuration:

```hcl
auth {
  type = "ssh"
  keyPath = "~/.ssh/my-github-key"
}
```

## Features

- Clones multiple repositories based on specified criteria.
- Creates symlinks for easy access to cloned repositories.
- Provides a summary of cloning and symlinking operations.

## Support

If you encounter any issues or have questions, please [open an issue](https://github.com/ssotops/gitspace/issues) on our GitHub repository.
