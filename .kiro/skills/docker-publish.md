---
name: Docker Publish
trigger: publish
---

# Docker Publish Skill

When the user says "publish" or "publish <version>", execute the following Docker commands to build and publish the echolist-backend image to Docker Hub.

## Commands to Execute

1. **Login to Docker Hub:**
   ```bash
   docker login
   ```
   This will prompt for Docker Hub credentials interactively.

2. **Build the Docker image:**
   - If user provides a version (e.g., "publish 1.2.3"):
     ```bash
     docker build -t onefivefour/echolist-backend:latest -t onefivefour/echolist-backend:<version> .
     ```
   - If user just says "publish" (no version):
     ```bash
     docker build -t onefivefour/echolist-backend:latest .
     ```

3. **Push to Docker Hub:**
   ```bash
   docker push onefivefour/echolist-backend --all-tags
   ```

## Version Handling

- Extract the version from the user's message if provided (e.g., "publish 1.2.3" → version is "1.2.3")
- If no version is provided, only tag with "latest"
- Version should follow semantic versioning format (e.g., 1.0.0, 1.2.3, 2.0.0-beta)

## Notes

- The docker login command is interactive and will prompt for username and password
- The build process may take a few minutes depending on the system
- Ensure you're in the project root directory before running these commands
- All commands should be run sequentially, stopping if any command fails
