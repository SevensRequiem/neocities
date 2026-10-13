my live neocities page & posting automation

## posting

`go run ./poster` from the repo root opens a local form for weblog posts and gallery frames. It writes the post directory with a `post.json` (metadata, plus the markdown `body` for weblog posts) and updates `blog/index.json` or `gallery/index.json`. Blog posts also update `blog/all.json`, an array of every full `post.json` plus its `dir`, in the same newest-first order as the blog index. Images in any format ImageMagick reads are converted to webp with EXIF stripped; `magick` must be on the PATH.

The site fetches `index.json`, which browsers block on `file://`, so opening `index.html` from disk shows "signal lost". Use the `view site` link in the poster instead; it serves the site at `/site/`.

Tick `git commit` to commit the new post directory, its index and (for blog posts) `blog/all.json` as `post: <dir>`, and `git push` to push the current branch to `origin`. Pushing `main` publishes. The deploy workflow syncs `index.html`, `shrine.html`, `robots.txt`, `sitemap.xml`, `assets/`, `blog/` and `gallery/` to neocities; `poster/` stays in the repo.

## webring

`webring/webring.json` is the member list: an array of `{ "url", "banner" }`. `banner` is an 88x31 image, either an absolute URL or a path relative to `webring/` for banners kept in this repo. The homepage fetches the list and its banners from `raw.githubusercontent.com` on `main`, so a pushed change shows up without a deploy and `webring/` is not synced to neocities.
