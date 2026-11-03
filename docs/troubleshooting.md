# Troubleshooting

This record covers issues found during the initial environment setup. Recovery work was limited to the project's environment and preserved existing SDKs, containers, and databases.

## Node.js selects an older version

A noninteractive shell initially selected Ubuntu's Node.js 18.19.1 even though nvm and Node.js 24.20.0 were already installed. Noninteractive shells do not load `.bashrc` in the same way as interactive shells.

Load nvm explicitly and select the project's version:

```bash
export NVM_DIR="$HOME/.nvm"
. "$NVM_DIR/nvm.sh"
cd /home/jason/projects/socialai-v2
nvm use
command -v node
node --version
npm --version
```

Automation should load nvm or use the executable from the selected installation. Keep Windows and WSL dependencies separate. Removing the system Node.js installation is unnecessary.

## Go is installed but missing from PATH

The first command lookup did not find Go. A later check found the existing Go 1.25.5 installation at `/usr/local/go/bin/go`.

Go 1.27.3 was installed in the user directory, and `~/.local/bin` was added to PATH. Verify the selected installation:

```bash
command -v go
go version
go env GOPATH GOTOOLCHAIN GOOS GOARCH CGO_ENABLED
```

The executable should resolve to `~/.local/bin/go`. If a new shell selects another version, inspect PATH ordering before changing GOROOT.

## Docker Desktop runs, but WSL cannot connect

Ubuntu's individual WSL integration switch was disabled. It was enabled under Settings > Resources > WSL integration, followed by Apply & restart.

Check from Ubuntu:

```bash
docker version
docker compose version
docker info --format '{{.OSType}}'
```

Both Client and Server should be present, and the engine should report `linux`. Seeing only the client does not confirm that the engine is available. A separate dockerd service in Ubuntu is unnecessary.

A Windows command environment may fail to see a named pipe even when the desktop interface is running. Check the actual Docker Desktop state and the WSL connection before concluding that a reinstall is needed.

## Stale sockets in Docker's runtime directory

Startup logs reported an Inference manager failure while removing `dockerInference`, with the Windows error "The file cannot be accessed by the system." The runtime directory contained two inaccessible zero-byte socket reparse points.

Recovery performed on this machine:

1. Verify the exact path: `%LOCALAPPDATA%\Docker\run`.
2. Inspect Docker's process state and the runtime directory contents.
3. Preserve the directory as `run.pre-socialai-v2.<UTC-timestamp>` and create a new empty `run` directory.
4. Start Docker Desktop, check the actual engine state, and verify Ubuntu integration.

Images, containers, named volumes, and WSL virtual disks were preserved. No factory reset or global prune was performed. The runtime backups remain on the machine.

During the full restart check, `docker desktop restart` timed out while stopping Docker. The socket error then recurred. After confirming that Docker had exited, the same temporary directory was preserved and recreated. Docker started successfully, and both project services returned to `healthy`.

This is a tested recovery procedure for the observed failure, not evidence that the underlying Windows or Docker Desktop problem has been permanently fixed.

For daily work, stop the project's services with `docker compose stop`. When exiting Docker Desktop, use its normal desktop shutdown flow and avoid forcefully terminating it during startup or shutdown. If the error returns, inspect current logs and process state before repeating recovery. Do not end an ordinary user WSL terminal to work around a Docker CLI stop timeout.

Reference: [Docker's issue tracker for the same startup failure](https://github.com/docker/desktop-feedback/issues/554).

## Elasticsearch starts slowly or reports vm.max_map_count

The first startup downloads and initializes the image. Check progress with:

```bash
docker compose up -d --wait --wait-timeout 240
docker compose ps
docker compose logs --tail=100 elasticsearch
sysctl vm.max_map_count
docker compose exec -T elasticsearch cat /proc/sys/vm/max_map_count
```

The initial Ubuntu check reported `1048576`. After Docker Desktop restarted, both Ubuntu and the container reported `262144`. Saving a sysctl setting inside the Docker distribution and applying it manually worked temporarily, but restarting Docker overrode the active value.

The final local Compose configuration uses `node.store.allow_mmap: "false"`, so startup does not depend on host kernel tuning. The experimental sysctl changes were rolled back. Disabling mmap has performance implications; production deployment needs separate configuration.

Also check for out-of-memory termination, insufficient Docker memory, or port conflicts. Preserve the data volumes while diagnosing startup failures.

## Stored passwords differ from .env

Passwords are stored when the named volumes are initialized. Editing `.env` and restarting containers does not rotate existing service credentials.

Use the service's supported password update procedure, then update `.env`. Reinitializing data volumes is appropriate only after confirming that the stored data can be discarded.

## Port conflicts

The supporting services use 5432 and 9200. Ports 8080 and 5173 are reserved for future application services.

```bash
ss -ltn
docker compose ps
```

If another project uses a host port, change the corresponding value in `.env` and recreate this project's container. Leave unrelated databases running. The existing MySQL ports, 3306 and 33060, do not conflict with these defaults.

## GitHub SSH authentication fails

GitHub recognized the existing public key, but the initial noninteractive WSL session could not sign with it and returned `Permission denied (publickey)`.

A dedicated ssh-agent is now available at `~/.ssh/socialai-v2-agent.sock`. The repository's local Git configuration selects this agent and the existing `~/.ssh/id_ed25519` key. After the owner unlocked the key in their own terminal, `git ls-remote origin HEAD` succeeded.

This setting is stored only in the local `.git/config`. The global SSH configuration and existing key files were preserved. The passphrase was entered by the owner and was not written to files or logs.

The unlocked identity remains available while the agent is running. After WSL fully shuts down or restarts, start the agent and unlock the existing key again:

```bash
ssh-agent -a "$HOME/.ssh/socialai-v2-agent.sock"
SSH_AUTH_SOCK="$HOME/.ssh/socialai-v2-agent.sock" ssh-add "$HOME/.ssh/id_ed25519"
git -C /home/jason/projects/socialai-v2 ls-remote origin HEAD
```

If the socket path already exists, first check whether an agent is responding:

```bash
SSH_AUTH_SOCK="$HOME/.ssh/socialai-v2-agent.sock" ssh-add -l
```

Exit code 0 means an identity is loaded. Exit code 1 means the agent is reachable but has no identities. Exit code 2 means the agent cannot be reached. Remove a stale socket only after confirming the last case and verifying that the path is the socket created for this setup.

A standalone SSH command does not read Git's repository-specific `core.sshCommand`. To test SSH directly, select the agent explicitly:

```bash
ssh -o IdentityAgent="$HOME/.ssh/socialai-v2-agent.sock" -T git@github.com
```

GitHub's successful authentication message states that shell access is unavailable. This command may return exit code 1 even after successful authentication; read the response as well as the exit code.

Creating local commits does not contact GitHub. The repository owner handles pushes.
