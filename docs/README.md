# MaaS Platform Documentation

This directory contains the documentation for the MaaS (Model-as-a-Service) Platform. This is build automatically using GitHub Actions when changes are pushed to the main branch.

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
pip install mkdocs
pip install mkdocs-material
pip install mkdocs-git-revision-date-localized-plugin
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

### Deployment

The documentation is automatically deployed to GitHub Pages when changes are pushed to the main branch. The deployment is handled by the GitHub Actions workflow in `.github/workflows/docs.yml`.