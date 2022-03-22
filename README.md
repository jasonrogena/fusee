## Fusee

Fusee mounts a readonly [FUSE filesystem](https://www.kernel.org/doc/html/latest/filesystems/fuse.html) whose content is the output of arbitrary commands. Useful in situations where you would like to dynamically load the contents of a file from a command. Directories and files exposed by Fusee are treated as any other by the operating system.

You can use Fuse to build a FUSE filesystem with as many levels of directories and files as your operating system allows. Apart from the filesystem's root directory, directories are loaded lazily (i.e. the contents of the directory are built only when the operating system calls readdir() against the directory).

### Building

To build Fusee, run:

```sh
make build
```

### Usage

```
fusee path/to/config.toml
```

### Usage Scenarios

The following scenarios show example usages for Fusee.

#### 1. Ansible Vault Password Files

The `ansible-playbook` command accepts the `--vault-password-file` flag which you use to specify the path to a plaintext file containing the password (symetric key) Ansible should use to decrpt variable files. There are some situations where you would not want to have a plaintext file with a password hanging around in your filesystem. Use Fusee to provide the output of a command extracting the password from a secure place as a file.

This is how you woud use Fusee with Ansible fault files.

1. Put your GPG encrypted Ansible vault password files in `$HOME/encrypted-ansible-keys`.

```sh
$ ls $HOME/encrypted-ansible-passwords/
project-1-password.asc  project-2-password.asc  project-3-password.asc sub-dir-with-more-passwords/
```

2. Create a Fusee configuration file to load the encrypted password files as plaintext files in a FUSE mount:

`/etc/fusee.toml`
```toml
[mounts.ansible-vault-passwords]
path = "/tmp/ansible-password-files"
readCommand = "ls -1 $HOME/encrypted-ansible-passwords/" # The command to use to generate the list of files in the mount point's root directory
nameSeparator = "\n" # How to separate the names of the files gotten from the readCommand above
mode = 0o555
cache = true # Cache the list of files in the mount's root directory
cacheSeconds = 30 # The number of seconds to cache the mount's root directory contents

  [mounts.ansible-vault-passwords.file]
  # The command to use to decrypt an encrypted password file to expose as a plaintext file. The
  # command will be ran when any process tries to read /tmp/ansible-password-files/<password file>
  # You can provide a Go template string as the command. Check the list below of supported template
  # variables exposed by Fusee.
  readCommand = "gpg --decrypt $HOME/encrypted-ansible-passwords/{{ .RelativePath }} 2> /dev/null"
  mode = 0o555
  # Set to false so that the plaintext passwords aren't cached in memory
  # and gpg is always called when users try to access the file
  cache = false

  [mounts.mount-a.directory]
  # Tests whether direntries within $HOME/encrypted-ansible-passwords are directories
  # and, if so, provides the command (`ls -1`) to generate a list of files under these
  # directories
  readCommand = "test -d $HOME/encrypted-ansible-passwords/{{ .RelativePath }} && ls -1 $HOME/encrypted-ansible-passwords/{{ .RelativePath }}"
  nameSeparator = "\n"
  mode = 0o555
  cache = true
  cacheSeconds = 30
```

3. Run Fusee:

```sh
fusee /etc/fusee.toml
```

4. Run `ansible-playbook`:

```sh
ansible-playbook --vault-password-file=/tmp/ansible-password-files/project-1-password.asc playbook.yml
```

### Config

```toml
[mounts.mount-a]
path = "/tmp/mount-test"
# Optional. The command to use to get the list of files and directories in
# the mount's root. If not defined, .directory.read-command will be used.
# You can provide a Go template string as the command. Check the list below
# of supported template variables exposed by Fusee.
readCommand = "echo 'Downloads Documents'"
# Optional. What should be used to separate the names returned by readCommand.
# If not defined, .directory.read-command will be used
nameSeparator = " "
mode = 0o555
cache = true
# The number of seconds the list of files in the root directory should be cached before
# being rendered as stale.
cacheSeconds = 30

  [mounts.mount-a.directory]
  # The command to use to generate the contents of a file.
  # You can provide a Go template string as the command. Check the list below
  # of supported template variables exposed by Fusee.
  readCommand = "test -d $HOME/{{ .RelativePath }} && ls -1 $HOME/{{ .RelativePath }}"
  nameSeparator = "\n"
  mode = 0o555
  cache = true
  cacheSeconds = 30

  [mounts.mount-a.file]
  # The command to use to generate the list of directory entries. This command
  # should pass when ran against a directory in the mount and should fail if
  # ran against regular files. This is how Fusee tells if a direntry is a file
  # or a directory.
  #
  # You can provide a Go template string as the command. Check the list below
  # of supported template variables exposed by Fusee.
  readCommand = "echo woot"
  mode = 0o555
  cache = true
  cacheSeconds = 30
```

### Command Template Variables

The following variables are usable in the go templates defined in the `readCommand` fields:

- `MountName`: The name of the Fusee mount. 
- `MountRootDirPath`: The absolute path for the mount's root directory.
- `RelativePath`: The path, relative to the mount's root, for the file or directory being accessed.
- `Name`: The name of the file or directory being accessed.
