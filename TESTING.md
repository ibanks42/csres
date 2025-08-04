# Testing the Release Process

This document explains how to test the release process for CS Resolution Monitor.

## Creating a Release

The workflow is now configured to trigger only on tag pushes. Here's how to create a release:

### 1. Create and Push a Tag

```bash
# Create a new tag (replace with your version)
git tag v1.0.0

# Push the tag to trigger the release workflow
git push origin v1.0.0
```

### 2. What Happens Automatically

The GitHub Actions workflow will:

1. **Trigger**: When a tag matching `v*` is pushed
2. **Extract Version**: From the tag name (e.g., `v1.0.0` → `1.0.0`)
3. **Build**: Compile the application with embedded version
4. **Test**: Verify the version is correctly embedded
5. **Release**: Create a GitHub release with assets

### 3. Manual Testing

To test version injection locally:

```bash
# Build with custom version
go build -ldflags "-X main.Version=1.0.0-test" -o csres-test.exe

# Verify version
./csres-test.exe --version
# Should output: CS Resolution Monitor v1.0.0-test
```

### 4. Version Format

- **Tags**: Use semantic versioning like `v1.0.0`, `v1.2.3`, `v2.0.0-beta`
- **Embedded**: The `v` prefix is stripped automatically
- **Display**: Shows as `CS Resolution Monitor v1.0.0`

### 5. Release Assets

Each release automatically includes:
- `csres.exe` - Main executable with embedded version
- `config.example.json` - Configuration example

### 6. Workflow Permissions

Make sure your GitHub repository has:
- Actions enabled
- Write permissions for releases (usually enabled by default)

## Troubleshooting

### Common Issues

1. **HTTP 403 Error**: Check repository permissions for Actions
2. **Version not embedded**: Ensure `Version` is declared as `var`, not `const`
3. **Workflow not triggering**: Verify tag format matches `v*` pattern

### Debug Steps

1. Check Actions tab in GitHub repository
2. View workflow run logs for detailed error messages
3. Test version injection locally before pushing tags