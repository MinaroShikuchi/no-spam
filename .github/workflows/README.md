# GitHub Actions Workflows

## CI/CD Pipeline

The `ci.yml` workflow provides continuous integration and deployment for the no-spam project.

### Triggers

- **Push**: Runs on push to `main`, `master`, or `develop` branches
- **Pull Request**: Runs on PRs targeting `main`, `master`, or `develop` branches
- **Tags**: Runs on version tags (e.g., `v1.0.0`)

### Jobs

#### 1. Test Job
- Sets up Go 1.21
- Installs dependencies
- Builds the application
- Runs unit tests with race detection
- Runs E2E tests
- Generates coverage report
- Uploads coverage as artifact

#### 2. Lint Job
- Runs golangci-lint for code quality checks

#### 3. Release Job (Tags only)
- Only runs when a tag starting with `v` is pushed
- Builds binaries for multiple platforms:
  - Linux (amd64, arm64)
  - macOS (amd64, arm64)
  - Windows (amd64)
- Creates SHA256 checksums
- Creates a GitHub release with:
  - All platform binaries
  - Checksums file
  - Auto-generated release notes

### Creating a Release

To create a new release:

```bash
# Tag the commit
git tag -a v1.0.0 -m "Release v1.0.0"

# Push the tag
git push origin v1.0.0
```

The workflow will automatically:
1. Run tests and linting
2. Build binaries for all platforms
3. Create a GitHub release
4. Upload binaries and checksums

### Local Testing

Before pushing, you can test locally:

```bash
# Run unit tests
make test-unit

# Run E2E tests
JWT_SECRET=test-secret go test -v -run TestE2E .

# Run linting
golangci-lint run
```
