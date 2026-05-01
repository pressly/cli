# pressly/cli docs

This is the long-lived source branch for the [pressly/cli](https://github.com/pressly/cli) documentation site, published at <https://pressly.github.io/cli/>.

It is intentionally orphaned from `main` and contains only the static site and the workflow that publishes it.

## Layout

- `site/` - static HTML/CSS/JS, served as-is.
- `.github/workflows/pages.yml` - on push to `docs`, copies `site/` to a `_site/` build dir and force-pushes it to the `gh-pages` branch via [peaceiris/actions-gh-pages](https://github.com/peaceiris/actions-gh-pages).

GitHub Pages serves from `gh-pages`. The `docs` branch is the editable source; `gh-pages` is the deploy artifact and is overwritten on every publish.

## Local preview

```bash
cd site && python3 -m http.server 8000
```

Then open <http://localhost:8000>.
