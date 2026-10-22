my live neocities page & posting automation

## posting

`go run ./poster` opens a local form for weblog posts, gallery frames and memos. It writes `post.json`, updates the indexes, converts images to webp (`magick` on PATH) and rebuilds the static blog and story pages, `blog/index.html`, `stories/index.html`, `sitemap.xml`, `feed.xml` (weblog posts and stories, newest 20), `stories.xml` (every story) and `llms.txt`. `go run ./poster -build` rebuilds without posting; run it after hand-editing a `post.json`, `stories/all.json`, `memos/index.json` or the templates in `internal/site/`.

Memos are short asides (1-1000 characters of markdown) kept newest first in `memos/index.json`; the homepage places each one between the weblog posts around its timestamp. The build fills each memo's `html` from its `body`, so edit `body` and rebuild. That `html` is the same goldmark + bluemonday output the weblog pages carry, which is why `site.js` inserts it with `innerHTML`; nothing else on the homepage does. A memo sits directly before the first post older than it, so it never moves a post to another page: newer than every post puts it on top of page 1, older than every post puts it at the end of the last page. `#memo-N` opens the page holding memo N and scrolls to it; `#stories` opens the stories tab.

Post a memo from the poster's `memo` tab: body, live preview, commit/push checkboxes. It commits as `memo: NNN`.

`index.html` on `file://` shows "signal lost" (fetch is blocked); use the poster's `view site` link. Tick `git commit` / `git push` in the form; pushing `main` deploys everything except `poster/`, `internal/`, `writer/`, `cf-worker/` and `webring/`.

The deploy workflow uploads `index.html`, `shrine.html`, `robots.txt`, `sitemap.xml`, `feed.xml`, `llms.txt`, `stories.xml` and `assets/`, `blog/`, `gallery/`, `stories/`, `memos/`, and prunes remote files only inside those five directories.

`internal/site` holds what both binaries share: the build and its templates, atomic json writes, image conversion, the git commit/push step and the loopback guard.

## stories

`go run ./writer` opens a local app for short fiction, with the poster's security model: it listens on `127.0.0.1` with a per-run token, and every write checks the token and the `Host` header. The sidebar lists drafts (title, last save, word count, `edits ::STORY.NNN` when it's editing a published story) and published stories from `stories/index.json`. The editor has title, tags, summary, author, a cover image, a markdown body and a live preview with word count and read time (words / 230, at least 1 min).

Drafts live in `writer/drafts/<id>.json` (plus `<id>.webp` for a cover). That directory is gitignored because the repo is public: an unpublished story never gets committed. Draft ids are 16 hex chars and every route checks them before touching disk.

Autosave fires 1 s after the last keystroke, every 30 s while there are unsaved changes, and when the tab is hidden or closed (keepalive fetch). The status bar shows `saved HH:MM:SS`, `saving...`, `unsaved changes` or the error. Each save carries the draft's rev; a stale rev (the same draft open in two tabs) gets a 409, autosave pauses and you pick `reload saved copy` or `keep mine`.

| action | does |
|---|---|
| publish | needs title and body. A new story gets the next id, `stories/NNN-slug/`, a date and its `post.json` (+ `image.webp`); the draft is deleted. Commits as `story: stories/<dir>` |
| publish update | a draft opened from a published story keeps its id, dir and date, sets `updated` and replaces the cover or drops it |
| edit | on a published story: reopens the draft already linked to it, or copies `post.json` and the cover into a new one |
| unpublish | moves the story back into a draft, removes its dir and index entries, rebuilds. Second click to confirm |
| delete | removes a draft and its cover. Second click to confirm |

A failed publish restores `stories/index.json`, `stories/all.json` and the story dir. Scene breaks are markdown `---`, rendered as `◈ ◈ ◈`.

The build turns `stories/all.json` into `stories/<dir>/index.html` (reading layout, newer/older links, `ShortStory` JSON-LD), the list at `stories/index.html`, `stories.xml` (every story, full text) and `/stories/` entries in `sitemap.xml` and `llms.txt`. `feed.xml` mixes weblog posts and stories by date, newest 20, story items tagged `story`. The homepage `stories` tab reads `stories/index.json`.

## old assets

`old-assets/` holds retired images that stay in the repo but never deploy: the workflow only uploads the paths it lists, so nothing there reaches Neocities. Move a file there from `assets/` and the next deploy prunes it from the site.

## effects budget

`assets/js/effects.js` measures fps over 2 s windows. Two windows under 40 fps drop one effect tier (`data-fx` on `<html>`); the tier is kept in `sessionStorage` for the rest of the visit. If even tier 3 stays under 40, the effects aren't the bottleneck (a 30 Hz power-saving cap, say): it goes back to the richest tier within 2 fps of the best and stops probing. The homepage side note shows `fps / fx tier`.

| tier | drops |
|---|---|
| 1 | cursor trails, cloud glow, cloud rate halved, animated static gif (swapped for a jittered still tile), ring and subtitle color cycling |
| 2 | static and wallpaper glitch animation, cloud motion, flag color cycling, card backdrop blur (cards go more opaque), shrine canvas at 1x |
| 3 | static overlay, every CSS animation (paused in place), shrine trail and mote counts |

To preview a tier: `sessionStorage.setItem('fx-tier', '2'); sessionStorage.setItem('fx-lock', '1')` in the console, then reload. `sessionStorage.clear()` re-enables probing.

## api worker

`cf-worker/` runs on `api.requiem.moe`. CORS is `https://requiem.moe` only. Don't set `Access-Control-Allow-Origin` in zone header rules, they overwrite the Worker.

**views.** A cron samples the Neocities info API every 10 minutes into D1; `GET /views` returns the latest Monday snapshot plus the change since the Monday before, cached per location until next Monday.

**anonymous q&a.** The homepage `::ASK` card shows the latest answer; its overlay takes questions and lists every answer, 20 per page.

| route | auth | does |
|---|---|---|
| `GET /qa?before=<cursor>` | public | answered questions, newest first: `{ total, items, next }`, `next` is the cursor for the next page or `null` |
| `POST /qa` | public | `{ "question": "..." }`, 1-500 characters, 3 per minute per IP. A filled `website` honeypot gets a silent 204 and nothing is stored |
| `GET /qa/inbox` | bearer | the newest 200 questions, unanswered first |
| `PUT /qa/<id>` | bearer | `{ "answer": "..." }`, 1-4000 characters; editing an answer keeps its original answered time |
| `DELETE /qa/<id>` | bearer | removes the question |

Only the question text and timestamps are stored; the IP is used for rate limiting and never written to D1. Answer from the poster's `q&a` tab: copy `poster/.env.example` to `poster/.env` (gitignored), set `QA_TOKEN`, then `go run ./poster`. Real environment variables override the file; `QA_API=http://127.0.0.1:8787` points it at `npm run dev`. The token is set once with `npx wrangler secret put QA_TOKEN`; for Miniflare put `QA_TOKEN=...` in `cf-worker/.dev.vars` (gitignored).

Deploy from `cf-worker/`: `npm install`, `npx wrangler login`, `npm run deploy` (applies new D1 migrations first). `npm run dev` for Miniflare after `npx wrangler d1 migrations apply requiem-api --local`; fire the cron with `curl "http://127.0.0.1:8787/__scheduled?cron=*/10+*+*+*+*"`. An empty `package.json` at the repo root breaks esbuild.

Zone security rules for `api.requiem.moe` (dashboard, free plan): block methods other than `GET`/`POST`/`PUT`/`DELETE`/`OPTIONS`, block paths other than `/views`, `/qa` and `/qa/*`, block requests without `origin: https://requiem.moe` except paths starting with `/qa/` (the poster sends no origin; disable while debugging), rate limit 20 req / 10 s per IP. Don't use Under Attack mode; its JS challenge breaks `fetch`.

## webring

`webring/webring.json` is the member list. To add yours, open a PR appending:

```json
{ "url": "https://your.site/", "banner": "https://your.site/banner.png" }
```

`banner` is an 88x31 image, an absolute URL or a file you add under `webring/`. Merged changes show up on the homepage without a deploy.
