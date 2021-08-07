# Docker Recreate

Docker CLI plugin for recreating docker containers while keeping their configuration (with updated image)

## Installation

1. Download `docker-recreate` for your platform from the [releases]() page
1. Copy it to:
   - Windows: `c:\Users\<your user>\.docker\cli-plugins\docker-recreate.exe`
   - Mac and Linux: `$HOME/.docker/cli-plugins/docker-recreate`
1. On Mac and Linux make it executable with `chmod +x $HOME/.docker/cli-plugins/docker-recreate`

Or alternatively on Mac and Linux run the following:

```sh
mkdir -p ~/.docker/cli-plugins
curl https://github.com/andreccosta/docker-recreate/latest/download/docker-recreate-linux-amd64 -L -s -S -o ~/.docker/cli-plugins/docker-recreate
chmod +x ~/.docker/cli-plugins/docker-recreate
```

## Usage

```console
docker recreate NAME [-pull]
```
