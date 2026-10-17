my live neocities page & posting automation

## posting

`go run ./poster` from the repo root opens a local form for weblog posts and gallery frames. It writes the post directory with a `post.json` (metadata, plus the markdown `body` for weblog posts) and updates `blog/index.json` or `gallery/index.json`. Blog posts also update `blog/all.json`, an array of every full `post.json` plus its `dir`, in the same newest-first order as the blog index. Images in any format ImageMagick reads are converted to webp with EXIF stripped; `magick` must be on the PATH.

Every blog post also rebuilds the crawlable side of the weblog from `blog/all.json` and `blog/index.json`: a static page per post at `blog/<dir>/index.html` (rendered markdown, canonical, Open Graph, `BlogPosting` JSON-LD), the archive at `blog/index.html`, `sitemap.xml`, the RSS feed `feed.xml` and `llms.txt`. Only files whose bytes change are rewritten and committed. `go run ./poster -build` rebuilds them all without posting; run it after editing a `post.json` by hand or changing `poster/page.html` or `poster/archive.html`. The homepage's "read more" links open the static pages, and old `#post-N` links redirect to them.

The site fetches `index.json`, which browsers block on `file://`, so opening `index.html` from disk shows "signal lost". Use the `view site` link in the poster instead; it serves the site at `/site/`.

Tick `git commit` to commit the new post directory, its index and (for blog posts) `blog/all.json` as `post: <dir>`, and `git push` to push the current branch to `origin`. Pushing `main` publishes. The deploy workflow syncs `index.html`, `shrine.html`, `robots.txt`, `sitemap.xml`, `feed.xml`, `llms.txt`, `assets/`, `blog/` and `gallery/` to neocities; `poster/` stays in the repo.

## view counter api

`cf-worker/` is a Cloudflare Worker on `api.requiem.moe`. Every 10 minutes its cron fetches `https://neocities.org/api/info?sitename=sevensrequiem` and stores `views` and `hits` in the D1 database `requiem-api`. The Neocities API sends no CORS headers, so the homepage cannot call it directly.

`GET https://api.requiem.moe/views` returns the latest sample and the change against the newest sample at least a week older (the oldest sample while there is less than a week of history):

```json
{ "views": 73560, "hits": 284480, "at": "2026-10-17T06:40:00.000Z", "week": { "views": 1560, "hits": 4480, "since": "2026-10-10T06:40:00.000Z" } }
```

Responses carry `Access-Control-Allow-Origin: https://requiem.moe` and are cached at the edge for 60 seconds, with the query string dropped from the cache key. Two rate limit bindings guard it: 30 requests a minute per IP and 1200 a minute across all callers. Cloudflare counts both per location, not worldwide. Any other path is 404 and any method other than `GET` and `OPTIONS` is 405. `workers.dev` and preview URLs are off, so the custom domain, and the zone's security rules, are the only way in.

From `cf-worker/`: `npm install`, `npx wrangler login` once, then `npm run deploy`. The script applies `migrations/` to the remote D1 database `a05da497-0427-45bb-a704-3b5fedadd897`, then deploys, which creates the `api.requiem.moe` DNS record on the first run. esbuild reads every `package.json` above `cf-worker/`, so an empty one at the repo root fails the build with `Unexpected end of file in JSON`. `npm run dev` runs it locally on Miniflare, and `curl "http://127.0.0.1:8787/__scheduled?cron=*/10+*+*+*+*"` fires the cron. The site deploy workflow does not upload `cf-worker/`.

### security rules

Zone rules run before the Worker, so blocked requests never cost a Worker invocation. Set them in the dashboard under Security for `requiem.moe` (all fit the free plan):

| kind | expression | action |
|---|---|---|
| custom rule | `http.host eq "api.requiem.moe" and not http.request.method in {"GET" "OPTIONS"}` | block |
| custom rule | `http.host eq "api.requiem.moe" and http.request.uri.path ne "/views"` | block |
| custom rule | `http.host eq "api.requiem.moe" and not any(http.request.headers["origin"][*] eq "https://requiem.moe")` | block |
| rate limiting rule | `http.host eq "api.requiem.moe"`, counted per IP, 20 requests / 10 s | block for 10 s |

The origin rule drops anything that is not a browser fetch from the homepage, which includes curl and most bots; it also blocks a direct look at the endpoint, so disable it while debugging. Bot Fight Mode (Security, Bots) is zone-wide and also covers the main site if its record is proxied. The HTTP DDoS managed ruleset is on for every plan; raising its sensitivity to high is safe for an endpoint this small. Under Attack mode serves a JavaScript challenge that `fetch` cannot pass, so it breaks the counter.

## webring

`webring/webring.json` is the member list: an array of `{ "url", "banner" }`. `banner` is an 88x31 image, either an absolute URL or a path relative to `webring/` for banners kept in this repo. The homepage fetches the list and its banners from `raw.githubusercontent.com` on `main`, so a pushed change shows up without a deploy and `webring/` is not synced to neocities.
