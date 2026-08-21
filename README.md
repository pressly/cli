# pressly/cli docs

This is the long-lived source branch for the [pressly/cli](https://github.com/pressly/cli) documentation site, published at <https://pressly.github.io/cli/>.

It is intentionally orphaned from `main` and contains only the static site. GitHub Pages serves the `docs/` folder of this branch directly: **push to `docs` branch is the deploy**.

## Layout

```
docs/
├── index.html      main page
├── flagtype/       flagtype subpackage
├── graceful/       graceful subpackage
├── xflag/          xflag subpackage
├── styles.css      all styling
├── app.js          copy buttons, theme toggle, heading anchors
└── .nojekyll       tells GitHub Pages to skip Jekyll processing
```

## Local preview

```bash
cd docs && python3 -m http.server 8000
```

Then open <http://localhost:8000>.

## GitHub Pages settings

Source: **Deploy from a branch**. Branch: **`docs`**. Folder: **`/docs`**.
