const CORS = { 'Access-Control-Allow-Origin': 'https://requiem.moe' };
const HOUR = 3600;
const DAY = 24 * HOUR;
const WEEK = 7 * DAY;
const QA_PAGE = 20;

const iso = (at) => new Date(at * 1000).toISOString();
const reply = (data, status = 200, headers = {}) =>
  new Response(JSON.stringify(data), { status, headers: { ...headers, 'Content-Type': 'application/json' } });

async function views(env, ctx, url) {
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
}

async function answered(env, url) {
  const [at, id] = (url.searchParams.get('before') ?? '').split('.').map(Number);
  const cursor = Number.isSafeInteger(at) && Number.isSafeInteger(id) ? [at, id] : [Number.MAX_SAFE_INTEGER, 0];
  const [count, page] = await env.DB.batch([
    env.DB.prepare('SELECT count(*) AS total FROM questions WHERE answered IS NOT NULL'),
    env.DB.prepare(
      'SELECT id, asked, question, answered, answer FROM questions WHERE answered IS NOT NULL AND (answered, id) < (?1, ?2) ORDER BY answered DESC, id DESC LIMIT ?3',
    ).bind(cursor[0], cursor[1], QA_PAGE + 1),
  ]);
  const rows = page.results.slice(0, QA_PAGE);
  const last = rows.at(-1);
  return reply(
    {
      total: count.results[0].total,
      items: rows.map((r) => ({ question: r.question, answer: r.answer, asked: iso(r.asked), answered: iso(r.answered) })),
      next: page.results.length > QA_PAGE ? `${last.answered}.${last.id}` : null,
    },
    200,
    { ...CORS, 'Cache-Control': 'public, max-age=60' },
  );
}

async function ask(request, env, ip) {
  if (!(await env.ASK.limit({ key: ip })).success) return new Response('slow down', { status: 429, headers: { ...CORS, 'Retry-After': '60' } });
  const raw = await request.text();
  if (raw.length > 4096) return new Response('too large', { status: 413, headers: CORS });
  let body;
  try {
    body = JSON.parse(raw);
  } catch {
    return new Response('bad json', { status: 400, headers: CORS });
  }
  if (body?.website) return new Response(null, { status: 204, headers: CORS });
  const question = String(body?.question ?? '').replace(/\r\n?/g, '\n').trim();
  if (!question || question.length > 500) return new Response('question must be 1-500 characters', { status: 400, headers: CORS });
  await env.DB.prepare('INSERT INTO questions (asked, question) VALUES (?1, ?2)').bind(Math.floor(Date.now() / 1000), question).run();
  return new Response(null, { status: 204, headers: CORS });
}

async function admin(request, env, url) {
  const given = new TextEncoder().encode(request.headers.get('Authorization') ?? '');
  const want = new TextEncoder().encode(`Bearer ${env.QA_TOKEN}`);
  if (!env.QA_TOKEN || given.byteLength !== want.byteLength || !crypto.subtle.timingSafeEqual(given, want)) {
    return new Response('unauthorized', { status: 401 });
  }

  if (url.pathname === '/qa/inbox') {
    if (request.method !== 'GET') return new Response('method not allowed', { status: 405, headers: { Allow: 'GET' } });
    const { results } = await env.DB.prepare(
      'SELECT id, asked, question, answered, answer FROM questions ORDER BY answered IS NOT NULL, asked DESC LIMIT 200',
    ).all();
    return reply(results.map((r) => ({ ...r, asked: iso(r.asked), answered: r.answered && iso(r.answered) })));
  }

  const id = Number(url.pathname.slice('/qa/'.length));
  if (!Number.isSafeInteger(id) || id < 1) return new Response('not found', { status: 404 });

  if (request.method === 'DELETE') {
    const { meta } = await env.DB.prepare('DELETE FROM questions WHERE id = ?1').bind(id).run();
    return meta.changes ? reply({ done: 'deleted' }) : new Response('not found', { status: 404 });
  }
  if (request.method !== 'PUT') return new Response('method not allowed', { status: 405, headers: { Allow: 'PUT, DELETE' } });

  let body;
  try {
    body = await request.json();
  } catch {
    return new Response('bad json', { status: 400 });
  }
  const answer = String(body?.answer ?? '').replace(/\r\n?/g, '\n').trim();
  if (!answer || answer.length > 4000) return new Response('answer must be 1-4000 characters', { status: 400 });
  const { meta } = await env.DB.prepare('UPDATE questions SET answer = ?2, answered = coalesce(answered, ?3) WHERE id = ?1')
    .bind(id, answer, Math.floor(Date.now() / 1000))
    .run();
  return meta.changes ? reply({ done: 'answered' }) : new Response('not found', { status: 404 });
}

export default {
  async fetch(request, env, ctx) {
    const url = new URL(request.url);
    const isPublic = url.pathname === '/views' || url.pathname === '/qa';
    if (!isPublic && !url.pathname.startsWith('/qa/')) return new Response('not found', { status: 404 });

    const methods = url.pathname === '/qa' ? 'GET, POST' : 'GET';
    if (isPublic && request.method === 'OPTIONS') {
      return new Response(null, {
        status: 204,
        headers: { ...CORS, 'Access-Control-Allow-Methods': methods, 'Access-Control-Allow-Headers': 'Content-Type', 'Access-Control-Max-Age': '86400' },
      });
    }
    if (isPublic && !methods.includes(request.method)) {
      return new Response('method not allowed', { status: 405, headers: { Allow: `${methods}, OPTIONS` } });
    }

    const ip = request.headers.get('cf-connecting-ip') ?? 'unknown';
    const [perIp, global] = await Promise.all([env.PER_IP.limit({ key: ip }), env.GLOBAL.limit({ key: 'all' })]);
    if (!perIp.success || !global.success) {
      return new Response('slow down', { status: 429, headers: { ...CORS, 'Retry-After': '60' } });
    }

    if (!isPublic) return admin(request, env, url);
    if (url.pathname === '/views') return views(env, ctx, url);
    return request.method === 'POST' ? ask(request, env, ip) : answered(env, url);
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
