# alfred-ssh-iterm-workflow
Query `~/.ssh/config` and open ssh host with iTerm badge

## Workflow

search ~/.ssh/config host name, then open in iTerm, set host name as badge, and then login the ssh host.

## SSH configuration includes

Hosts are also discovered in files referenced by `Include` directives, including
nested includes, multiple paths, wildcards, quoted paths and `~` home paths.
Relative include paths are resolved from `~/.ssh`, as in OpenSSH. Missing include
files are ignored; repeated files, symbolic-link cycles and duplicate aliases do
not create duplicate results. Each literal alias on a `Host` line is listed
separately; wildcard and negated patterns are not offered as connection targets.

For example:

```sshconfig
Host *
    Include ~/.ssh/net01/config
    Include ~/.ssh/net02/config
```

This is static alias discovery: it does not evaluate `Host`/`Match` conditions
around includes or execute `Match exec` commands. OpenSSH evaluates the actual
connection settings when a host is selected. Host-dependent include tokens such
as `%h` are not expanded.

Include paths are only read as filesystem paths: shell substitutions, backticks
and command operators are not executed. Invalid UTF-8, control characters and
invisible formatting characters in paths are rejected. Non-regular files are
rejected, and discovery is limited to 32 nested includes, 1,024 files, 1 MiB per
file and 4 MiB in total. Aliases with whitespace, shell metacharacters, hidden
characters or leading option markers are not listed. The generated SSH command
also quotes its arguments and separates the hostname from options with `--`.

This does not sandbox OpenSSH: when connecting, SSH still uses directives such
as `ProxyCommand` and `Match exec` from your configuration. Only use trusted SSH
configuration files.

For test coverage and instructions, see [Testing](test/README.md).

## How to use

1. [Download](https://github.com/hanjm/alfred-ssh-iterm-workflow/raw/master/build/alfred-open-ssh-host-with-iterm-badge-workflow.alfredworkflow), and import to alfred.

2. Set badge of current iTerm profile. learn more about iTerm badge <https://www.iterm2.com/documentation-badges.html>

    ![iTerm badge setting](./image/iterm_profile_setting.png)

3. Then
    ![gif](./image/ssh2iterm.gif)
