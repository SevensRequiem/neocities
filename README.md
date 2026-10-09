my live neocities page & posting automation

## posting

`go run ./poster` from the repo root opens a local form for weblog posts and gallery frames. It writes the post directory and updates `blog/index.json` or `gallery/index.json`. Images in any format ImageMagick reads are converted to webp with EXIF stripped; `magick` must be on the PATH.

The site fetches `index.json`, which browsers block on `file://`, so opening `index.html` from disk shows "signal lost". Use the `view site` link in the poster instead; it serves the site at `/site/`.

Tick `git commit` to commit the new post directory and its index as `post: <dir>`, and `git push` to push the current branch to `origin`. Pushing `main` publishes. The deploy workflow syncs `index.html`, `assets/`, `blog/` and `gallery/` to neocities; `poster/` stays in the repo.
