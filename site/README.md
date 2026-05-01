# site

Source for the [pressly/cli docs site](https://pressly.github.io/cli/).

Plain HTML, CSS, and a small bit of JavaScript. No framework, no build step.
The deploy workflow pushes `site/` to the `gh-pages` branch.

## Layout

- `index.html`    - main page
- `flagtype.html` - flagtype subpackage page
- `graceful.html` - graceful subpackage page
- `xflag.html`    - xflag subpackage page
- `styles.css`    - all styling
- `app.js`        - copy buttons, theme toggle, heading anchors

## Local preview

```bash
cd site
python3 -m http.server 8000
# open http://localhost:8000
```

## Deploy

Pushes to the `docs` branch trigger `.github/workflows/pages.yml`, which
publishes to the `gh-pages` branch. GitHub Pages must be configured to serve
from that branch (Settings, Pages).
