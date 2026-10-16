my live neocities page & posting automation

## posting

`go run ./poster` from the repo root opens a local form for weblog posts and gallery frames. It writes the post directory with a `post.json` (metadata, plus the markdown `body` for weblog posts) and updates `blog/index.json` or `gallery/index.json`. Blog posts also update `blog/all.json`, an array of every full `post.json` plus its `dir`, in the same newest-first order as the blog index. Images in any format ImageMagick reads are converted to webp with EXIF stripped; `magick` must be on the PATH.

Every blog post also rebuilds the crawlable side of the weblog from `blog/all.json` and `blog/index.json`: a static page per post at `blog/<dir>/index.html` (rendered markdown, canonical, Open Graph, `BlogPosting` JSON-LD), the archive at `blog/index.html`, `sitemap.xml`, the RSS feed `feed.xml` and `llms.txt`. Only files whose bytes change are rewritten and committed. `go run ./poster -build` rebuilds them all without posting; run it after editing a `post.json` by hand or changing `poster/page.html` or `poster/archive.html`. The homepage's "read more" links open the static pages, and old `#post-N` links redirect to them.

The site fetches `index.json`, which browsers block on `file://`, so opening `index.html` from disk shows "signal lost". Use the `view site` link in the poster instead; it serves the site at `/site/`.

Tick `git commit` to commit the new post directory, its index and (for blog posts) `blog/all.json` as `post: <dir>`, and `git push` to push the current branch to `origin`. Pushing `main` publishes. The deploy workflow syncs `index.html`, `shrine.html`, `robots.txt`, `sitemap.xml`, `feed.xml`, `llms.txt`, `assets/`, `blog/` and `gallery/` to neocities; `poster/` stays in the repo.

## webring

`webring/webring.json` is the member list: an array of `{ "url", "banner" }`. `banner` is an 88x31 image, either an absolute URL or a path relative to `webring/` for banners kept in this repo. The homepage fetches the list and its banners from `raw.githubusercontent.com` on `main`, so a pushed change shows up without a deploy and `webring/` is not synced to neocities.
