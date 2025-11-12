# MaaS Platform Documentation

This directory contains the documentation for the MaaS (Model-as-a-Service) Platform. Documentation is built automatically using GitHub Actions and versioned with [mike](https://github.com/jimporter/mike).

## Documentation Versioning

This project uses `mike` to maintain versioned documentation:

- **`latest`**: Always points to the most recent tagged release (default version, visible in dropdown)
- **`dev`**: Development version from the main branch (hidden from version selector, accessible at `/dev/`)
- **`v*.*.*`**: Specific version tags (e.g., v1.0.0, v1.2.3, visible in dropdown)

When a new version tag is pushed (e.g., `v1.0.0`), the docs are automatically:
1. Deployed with that version number
2. Tagged as `latest`
3. Set as the default version

**Note**: The `dev` version is intentionally hidden from the version dropdown to avoid confusion for end users. Developers can access it directly at `https://opendatahub-io.github.io/maas-billing/dev/`

## Styling Notes

The documentation is styled using the [Material for MkDocs](https://squidfunk.github.io/mkdocs-material/) theme.

### Important Differences from Github Markdown Styling

Admonitions are styled using the `!!! note` syntax, more info [here](https://squidfunk.github.io/mkdocs-material/reference/admonitions/).

## Building the Documentation Locally

To build and preview the documentation website locally, follow these steps:

### Prerequisites

- Python 3.11 or higher
- pip (Python package manager)

### Installation

1. Install MkDocs and required plugins:

```bash
pip install -r requirements.txt
```

Or install individually:

```bash
pip install mkdocs
pip install mkdocs-material
pip install mkdocs-git-revision-date-localized-plugin
pip install mike
```

### Building and Serving

1. Navigate to the docs directory (where `mkdocs.yml` is located):

```bash
cd /path/to/maas-billing/docs
```

2. Start the development server:

```bash
mkdocs serve
```

3. Open your browser and navigate to `http://127.0.0.1:8000`

The documentation will automatically reload when you make changes to the source files.

### Building for Production

To build the static site files:

```bash
mkdocs build
```

This will create a `site/` directory with the generated HTML files.

### Configuration

The documentation is configured using `mkdocs.yml` in this directory. This file contains:

- Site metadata and navigation
- Theme configuration (Material Design with Red Hat branding)
- Markdown extensions and plugins
- Build settings

### File Structure

- `index.md` - Homepage
- `*.md` - Individual documentation pages
- `mkdocs.yml` - MkDocs configuration
- `assets/` - Images and other static assets

### Contributing

When adding new documentation:

1. Create or edit Markdown files in this directory
2. Update the navigation in `mkdocs.yml` if adding new pages
3. Test locally with `mkdocs serve`
4. Commit your changes

### Working with Versioned Documentation

#### Viewing All Versions

To see all available documentation versions:

```bash
cd docs
mike list
```

#### Deploying a Specific Version Locally

To deploy a version locally (requires git repository):

```bash
cd docs
# Deploy a specific version
mike deploy v1.0.0 latest

# Set the default version
mike set-default latest

# Serve locally to test
mike serve
```

Then navigate to `http://localhost:8000` to view the versioned documentation.

#### Deleting a Version

```bash
cd docs
mike delete v1.0.0
```

### Deployment

The documentation is automatically deployed to GitHub Pages using two workflows:

1. **`.github/workflows/docs.yml`**: Deploys the `dev` version when changes are pushed to the main branch
2. **`.github/workflows/docs-version.yml`**: Deploys versioned documentation when a git tag is pushed

#### How Versioning Works

- Push to `main` branch → Deploys as `dev` version (hidden from dropdown)
- Push tag `v1.0.0` → Deploys as `v1.0.0` version AND updates `latest` alias (visible in dropdown)
- The `latest` alias always points to the most recent tagged release
- The default version shown to users is `latest`

All versions are stored in the `gh-pages` branch.

#### Accessing the Dev Version

The `dev` version doesn't appear in the version selector but can be accessed directly:
- URL: `https://opendatahub-io.github.io/maas-billing/dev/`

This keeps the version dropdown clean for end users while still making development docs available for contributors.