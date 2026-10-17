const CORS = { 'Access-Control-Allow-Origin': 'https://requiem.moe' };
const HOUR = 3600;
const DAY = 24 * HOUR;
const WEEK = 7 * DAY;

const iso = (at) => new Date(at * 1000).toISOString();

export default {
  async fetch(request, env, ctx) {
    const url = new URL(request.url);
    if (url.pathname !== '/views') return new Response('not found', { status: 404 });

    if (request.method === 'OPTIONS') {
      return new Response(null, { status: 204, headers: { ...CORS, 'Access-Control-Allow-Methods': 'GET', 'Access-Control-Max-Age': '86400' } });
    }
    if (request.method !== 'GET') return new Response('method not allowed', { status: 405, headers: { Allow: 'GET, OPTIONS' } });

    const [perIp, global] = await Promise.all([
      env.PER_IP.limit({ key: request.headers.get('cf-connecting-ip') ?? 'unknown' }),
      env.GLOBAL.limit({ key: 'all' }),
    ]);
    if (!perIp.success || !global.success) {
      return new Response('slow down', { status: 429, headers: { ...CORS, 'Retry-After': '60' } });
    }

    const now = Math.floor(Date.now() / 1000);
    const today = Math.floor(now / DAY);
    const monday = (today - ((today + 3) % 7)) * DAY;
    const respond = (body, expires) =>
      new Response(body, {
        headers: { ...CORS, 'Content-Type': 'application/json', 'Cache-Control': `public, max-age=${Math.min(Math.max(expires - now, 1), HOUR)}` },
      });

    const key = new Request(`${url.origin}/views/${monday}`);
    const cached = await caches.default.match(key);
    if (cached) return respond(cached.body, Number(cached.headers.get('X-Expires')));

    const first = env.DB.prepare('SELECT at, views, hits FROM stats WHERE at >= ?1 ORDER BY at LIMIT 1');
    const [thisWeek, lastWeek, weekBefore] = (
      await env.DB.batch([first.bind(monday), first.bind(monday - WEEK), first.bind(monday - 2 * WEEK)])
    ).map((r) => r.results[0]);

    const ready = Boolean(thisWeek);
    const current = ready ? thisWeek : lastWeek;
    const base = ready ? lastWeek : weekBefore;
    if (!current) return new Response('no samples yet', { status: 503, headers: { ...CORS, 'Retry-After': '600' } });

    const body = JSON.stringify({
      views: current.views,
      hits: current.hits,
      at: iso(current.at),
      week: { views: current.views - base.views, hits: current.hits - base.hits, since: iso(base.at) },
    });
    const expires = ready ? monday + WEEK : now + 60;
    ctx.waitUntil(caches.default.put(key, new Response(body, { headers: { 'Cache-Control': `public, max-age=${expires - now}`, 'X-Expires': String(expires) } })));
    return respond(body, expires);
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
