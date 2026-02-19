# Full Stack
Run the full stack (DB + Backend + UI) using docker-compose.

## Prerequisites
Authenticate github to access and pull Docker images from GitHub Container Registry (GHCR) where CI IOC images are hosted.

Authorise cisco-eti docker hub account to pull images:
- go to https://github.com/settings/tokens
- If you don't have a token yet, create a new one with `read:packages` scope
- If you already have a token, ensure it has `read:packages` scope
- Click on "Configure SSO" beside delete button
- Authorize the token for cisco-eti organization
- Use the token to login to Docker Hub:
```bash
export GITHUB_TOKEN="xxxxxxxxxxxxx"  # replace with your token
echo "$GITHUB_TOKEN" | docker login ghcr.io -u "YOUR_GITHUB_USERNAME" --password-stdin
```

## Steps
- Uses `env.config` file for environment variables. You can modify as needed.

- Pull images
This takes a while to download all images. ~10 GB
Note: Rerun this command to update images.
```bash
task docker-compose-pull-images
```

- Start services
```bash
task docker-compose-up
```

- Stop services
```bash
task docker-compose-down
```

Volumes are preserved when stopping services. To remove volumes, use:
```bash
task docker-compose-down-with-volumes
```