const ORIGIN = 'https://requiem.moe';
const WEEK = 7 * 24 * 3600;

const iso = (at) => new Date(at * 1000).toISOString();

export default {
  async fetch(request, env, ctx) {
    const url = new URL(request.url);
    if (url.pathname !== '/views') return new Response('not found', { status: 404 });

    const cors = { 'Access-Control-Allow-Origin': ORIGIN };
    if (request.method === 'OPTIONS') {
      return new Response(null, { status: 204, headers: { ...cors, 'Access-Control-Allow-Methods': 'GET', 'Access-Control-Max-Age': '86400' } });
    }
    if (request.method !== 'GET') return new Response('method not allowed', { status: 405, headers: { Allow: 'GET, OPTIONS' } });

    const [perIp, global] = await Promise.all([
      env.PER_IP.limit({ key: request.headers.get('cf-connecting-ip') ?? 'unknown' }),
      env.GLOBAL.limit({ key: 'all' }),
    ]);
    if (!perIp.success || !global.success) {
      return new Response('slow down', { status: 429, headers: { ...cors, 'Retry-After': '60' } });
    }

    const key = new Request(url.origin + url.pathname);
    const cached = await caches.default.match(key);
    if (cached) return cached;

    const [latestRows, baseRows] = await env.DB.batch([
      env.DB.prepare('SELECT at, views, hits FROM stats ORDER BY at DESC LIMIT 1'),
      env.DB.prepare(
        'SELECT at, views, hits FROM stats WHERE at <= (SELECT MAX(at) FROM stats) - ?1 OR at = (SELECT MIN(at) FROM stats) ORDER BY at DESC LIMIT 1'
      ).bind(WEEK),
    ]);
    const latest = latestRows.results[0];
    const base = baseRows.results[0];
    if (!latest) return new Response('no samples yet', { status: 503, headers: { ...cors, 'Retry-After': '600' } });

    const res = Response.json(
      {
        views: latest.views,
        hits: latest.hits,
        at: iso(latest.at),
        week: { views: latest.views - base.views, hits: latest.hits - base.hits, since: iso(base.at) },
      },
      { headers: { ...cors, 'Cache-Control': 'public, max-age=60' } }
    );
    ctx.waitUntil(caches.default.put(key, res.clone()));
    return res;
  },

  async scheduled(event, env) {
    const res = await fetch('https://neocities.org/api/info?sitename=sevensrequiem', {
      headers: { 'User-Agent': 'requiem-api (+https://requiem.moe)' },
    });
    if (!res.ok) throw new Error(`neocities answered ${res.status}`);
    const data = await res.json();
    if (data.result !== 'success') throw new Error(`neocities answered ${data.result}`);
    await env.DB.prepare('INSERT OR REPLACE INTO stats (at, views, hits) VALUES (?1, ?2, ?3)')
      .bind(Math.floor(event.scheduledTime / 1000), data.info.views, data.info.hits)
      .run();
  },
};
