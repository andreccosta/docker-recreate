# Docker Recreate

Docker CLI plugin for recreating docker containers while keeping their configuration (but updating their image).

## Installation

1. Download `docker-recreate` for your platform from the [latest release](https://github.com/andreccosta/docker-recreate/releases/lastest)
1. Copy it to:
   - Windows: `c:\Users\<your user>\.docker\cli-plugins\docker-recreate.exe`
   - Mac and Linux: `$HOME/.docker/cli-plugins/docker-recreate`
1. On Mac and Linux make it executable with `chmod +x $HOME/.docker/cli-plugins/docker-recreate`

Or alternatively for Linux run the following:

```sh
mkdir -p ~/.docker/cli-plugins
curl https://github.com/andreccosta/docker-recreate/releases/latest/download/docker-recreate-linux-amd64 -L -s -S -o ~/.docker/cli-plugins/docker-recreate
chmod +x ~/.docker/cli-plugins/docker-recreate
```

Or for Mac:

```sh
mkdir -p ~/.docker/cli-plugins
curl https://github.com/andreccosta/docker-recreate/releases/latest/download/docker-recreate-darwin-amd64 -L -s -S -o ~/.docker/cli-plugins/docker-recreate
chmod +x ~/.docker/cli-plugins/docker-recreate
```

## Usage

```console
docker recreate [-pull] NAME
```
