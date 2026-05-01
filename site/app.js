// ---------------------------------------------------------------------------
// Copy buttons on every <pre> code block (skips the API index, which is
// itself a list of links rather than copy-pasteable code).
// ---------------------------------------------------------------------------
function initCopyButtons() {
  document.querySelectorAll("pre").forEach((pre) => {
    if (pre.classList.contains("api-index")) return;
    if (pre.querySelector(".copy-btn")) return;

    const btn = document.createElement("button");
    btn.type = "button";
    btn.className = "copy-btn";
    btn.textContent = "copy";
    btn.setAttribute("aria-label", "copy code to clipboard");

    btn.addEventListener("click", async () => {
      const code = pre.querySelector("code") || pre;
      try {
        await navigator.clipboard.writeText(code.textContent);
        btn.textContent = "copied";
        btn.classList.add("copied");
      } catch {
        btn.textContent = "failed";
      }
      setTimeout(() => {
        btn.textContent = "copy";
        btn.classList.remove("copied");
      }, 1500);
    });

    pre.appendChild(btn);
  });
}

// ---------------------------------------------------------------------------
// Heading anchors. For every h2/h3 with an id inside <main>, append a small
// "#" link. Clicking copies the absolute URL to the clipboard and gives
// brief inline feedback.
// ---------------------------------------------------------------------------
function initHeadingAnchors() {
  const headings = document.querySelectorAll(
    "main section h2, main section h3"
  );
  headings.forEach((h) => {
    if (h.querySelector(".heading-anchor")) return;

    // Prefer the heading's own id; fall back to the parent section's id
    // (most h2s are titled by the section's id rather than their own).
    let id = h.id;
    if (!id) {
      const section = h.closest("section");
      if (section && section.id) id = section.id;
    }
    if (!id) return;

    const a = document.createElement("a");
    a.className = "heading-anchor";
    a.href = "#" + id;
    a.textContent = "#";
    a.setAttribute("aria-label", "copy link to section");
    a.title = "copy link to this section";

    a.addEventListener("click", (e) => {
      e.preventDefault();
      const url =
        window.location.origin + window.location.pathname + "#" + id;
      history.replaceState(null, "", "#" + id);
      navigator.clipboard
        .writeText(url)
        .then(() => {
          a.classList.add("copied");
          setTimeout(() => a.classList.remove("copied"), 1500);
        })
        .catch(() => {});
    });

    h.appendChild(a);
  });
}

// ---------------------------------------------------------------------------
// Theme toggle. Defaults to the OS preference; user choice is persisted in
// localStorage and applied via [data-theme] on <html>. The pre-paint script
// in <head> sets the attribute before first render to avoid a light->dark
// flash for users who chose dark.
// ---------------------------------------------------------------------------
function initThemeToggle() {
  const btn = document.getElementById("theme-toggle");
  if (!btn) return;

  const apply = (theme) => {
    if (theme === "dark" || theme === "light") {
      document.documentElement.setAttribute("data-theme", theme);
    } else {
      document.documentElement.removeAttribute("data-theme");
    }
  };

  btn.addEventListener("click", () => {
    const explicit = document.documentElement.getAttribute("data-theme");
    const systemDark = window.matchMedia("(prefers-color-scheme: dark)").matches;
    const current = explicit || (systemDark ? "dark" : "light");
    const next = current === "dark" ? "light" : "dark";
    apply(next);
    localStorage.setItem("theme", next);
  });
}

document.addEventListener("DOMContentLoaded", () => {
  initCopyButtons();
  initThemeToggle();
  initHeadingAnchors();
});
