my live neocities page & posting automation

## posting

`go run ./poster` opens a local form for weblog posts and gallery frames. It writes `post.json`, updates the indexes, converts images to webp (`magick` on PATH) and rebuilds the static blog pages, `sitemap.xml`, `feed.xml` (newest 20) and `llms.txt`. `go run ./poster -build` rebuilds without posting; run it after hand-editing a `post.json` or the poster templates.

`index.html` on `file://` shows "signal lost" (fetch is blocked); use the poster's `view site` link. Tick `git commit` / `git push` in the form; pushing `main` deploys everything except `poster/`, `cf-worker/` and `webring/`.

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

## view counter api

`cf-worker/` runs on `api.requiem.moe`. A cron samples the Neocities info API every 10 minutes into D1; `GET /views` returns the latest Monday snapshot plus the change since the Monday before, cached per location until next Monday. CORS is `https://requiem.moe` only. Don't set `Access-Control-Allow-Origin` in zone header rules, they overwrite the Worker.

Deploy from `cf-worker/`: `npm install`, `npx wrangler login`, `npm run deploy`. `npm run dev` for Miniflare; fire the cron with `curl "http://127.0.0.1:8787/__scheduled?cron=*/10+*+*+*+*"`. An empty `package.json` at the repo root breaks esbuild.

Zone security rules for `api.requiem.moe` (dashboard, free plan): block methods other than `GET`/`OPTIONS`, block paths other than `/views`, block requests without `origin: https://requiem.moe` (disable while debugging), rate limit 20 req / 10 s per IP. Don't use Under Attack mode; its JS challenge breaks `fetch`.

## webring

`webring/webring.json` is the member list. To add yours, open a PR appending:

```json
{ "url": "https://your.site/", "banner": "https://your.site/banner.png" }
```

`banner` is an 88x31 image, an absolute URL or a file you add under `webring/`. Merged changes show up on the homepage without a deploy.
