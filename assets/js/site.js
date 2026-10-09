(function () {
  var prefersReducedMotion = window.matchMedia(
    '(prefers-reduced-motion: reduce)'
  ).matches;

  document.querySelectorAll('.flag-col').forEach((col, i) => {
    col.style.animationDelay = `${(6 - i) * 0.4}s`;
  });

  var alchemySymbols = [0x2299, 0x263c, 0x2722, 0x2723, 0x2724, 0x2725, 0x2726, 0x2727].map(function (cp) {
    return String.fromCodePoint(cp);
  });
  [[0x2630, 8], [0x4dc0, 64], [0x1d300, 60], [0x101d0, 45], [0x1f780, 40]].forEach(function (r) {
    for (var i = 0; i < r[1]; i++) alchemySymbols.push(String.fromCodePoint(r[0] + i));
  });
  var ringTrack = document.getElementById('ring-track');
  var ringSet = document.createDocumentFragment();
  for (var gi = 0; gi < 48; gi++) {
    var glyph = document.createElement('span');
    glyph.textContent = alchemySymbols[Math.floor(Math.random() * alchemySymbols.length)];
    glyph.style.animationDelay = (Math.random() * -4).toFixed(2) + 's';
    glyph.style.animationDuration = (3 + Math.random() * 3).toFixed(2) + 's';
    ringSet.appendChild(glyph);
  }
  ringTrack.appendChild(ringSet.cloneNode(true));
  ringTrack.appendChild(ringSet);

  var tabs = document.querySelectorAll('.tab-btn');
  var postTab = document.getElementById('tab-post');

  function selectTab(tab) {
    tabs.forEach(function (t) {
      t.setAttribute('aria-selected', t === tab);
    });
    document.getElementById('panel-weblog').hidden = tab.id === 'tab-gallery';
    document.getElementById('panel-gallery').hidden = tab.id !== 'tab-gallery';
  }

  tabs.forEach(function (tab) {
    tab.addEventListener('click', function () {
      if (tab.id === 'tab-weblog' && !postTab.hidden) location.hash = '#page-1';
      selectTab(tab);
    });
  });

  var PAGE_SIZE = 2;
  var weblog = document.getElementById('panel-weblog');
  var posts = [];
  var page = 1;

  function el(tag, className, text) {
    var node = document.createElement(tag);
    node.className = className;
    node.textContent = text || '';
    return node;
  }

  function postCard(post, latest) {
    var card = el('article', 'entry-card post-card');
    var head = el('header', 'entry-head');
    head.append(
      el('h2', 'panel-title', '::ENTRY.' + String(post.id).padStart(3, '0')),
      el('span', 'entry-tag', post.date.slice(0, 10).replace(/-/g, '.'))
    );
    card.append(head);
    if (post.image) {
      var frame = el('div', 'mosaic-frame post-frame');
      var img = el('img', 'mosaic-img');
      img.src = './blog/' + post.dir + '/' + post.image;
      img.alt = '';
      img.loading = 'lazy';
      frame.append(img);
      if (latest) frame.append(el('div', 'frame-tag', 'LATEST :: TRANSMISSION'));
      card.append(frame);
    }
    var heading = el('div', 'post-heading');
    heading.append(el('h3', 'post-title', post.title));
    if (post.tags.length) {
      var tagLine = post.tags.map(function (tag) {
        return '#' + tag.toLowerCase().replace(/\s+/g, '-');
      });
      heading.append(el('span', 'entry-tag post-tags', tagLine.join(' ')));
    }
    card.append(heading);
    return card;
  }

  function postFoot(label, href) {
    var foot = el('footer', 'post-foot');
    var link = el('a', 'panel-link', label);
    link.href = href;
    foot.append(link);
    return foot;
  }

  function notice(text) {
    var card = el('article', 'entry-card post-card');
    card.append(el('p', 'entry-para', text));
    return card;
  }

  function showPage(n) {
    var pages = Math.ceil(posts.length / PAGE_SIZE);
    page = Math.min(Math.max(n, 1), pages);
    var cards = posts.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE).map(function (post) {
      var card = postCard(post, post === posts[0]);
      card.append(el('p', 'entry-para', post.excerpt), postFoot('read more', '#post-' + post.id));
      return card;
    });
    var pager = el('nav', 'pager');
    pager.setAttribute('aria-label', 'weblog pages');
    var prev = el(page > 1 ? 'a' : 'span', 'panel-link pager-prev', 'newer');
    var next = el(page < pages ? 'a' : 'span', 'panel-link pager-next', 'older');
    if (page > 1) prev.href = '#page-' + (page - 1);
    if (page < pages) next.href = '#page-' + (page + 1);
    var picker = el('input', 'pager-input');
    picker.inputMode = 'numeric';
    picker.maxLength = 3;
    picker.value = page;
    picker.addEventListener('change', function () {
      var n = Math.min(Math.max(parseInt(picker.value, 10) || page, 1), pages);
      picker.value = n;
      location.hash = '#page-' + n;
    });
    var jump = el('label', 'entry-tag pager-jump', 'page');
    jump.append(picker, '/ ' + pages);
    pager.append(prev, jump, next);
    weblog.replaceChildren.apply(weblog, [pager].concat(cards));
    postTab.hidden = true;
    selectTab(document.getElementById('tab-weblog'));
  }

  function showPost(id) {
    var post = posts.find(function (p) {
      return p.id === id;
    });
    if (!post) return showPage(1);
    var hash = location.hash;
    var card = postCard(post, false);
    var body = el('div', 'post-body', 'receiving...');
    card.append(body, postFoot('back to weblog', '#page-' + page));
    weblog.replaceChildren(card);
    postTab.hidden = false;
    selectTab(postTab);

    fetch('./blog/' + post.dir + '/post.json')
      .then(json)
      .then(function (full) {
        if (location.hash !== hash) return;
        body.innerHTML = DOMPurify.sanitize(marked.parse(full.body));
        body.querySelectorAll('a[href]').forEach(function (a) {
          a.target = '_blank';
          a.rel = 'noopener noreferrer';
        });
      })
      .catch(function () {
        body.textContent = 'signal lost: could not load this entry.';
      });
  }

  function route(e) {
    var m = location.hash.match(/^#(post|page)-(\d+)$/);
    if (m && m[1] === 'post') showPost(Number(m[2]));
    else showPage(m ? Number(m[2]) : 1);
    if (e) document.querySelector('.tab-bar').scrollIntoView();
  }

  fetch('./blog/index.json')
    .then(json)
    .then(function (index) {
      posts = index;
      route();
      addEventListener('hashchange', route);
    })
    .catch(function () {
      weblog.replaceChildren(notice('signal lost: could not load ./blog/index.json'));
    });

  var repoList = document.getElementById('repo-list');

  function json(res) {
    if (!res.ok) throw new Error(res.status);
    return res.json();
  }

  function link(className, text, href) {
    var a = el('a', className, text);
    a.href = href;
    a.target = '_blank';
    a.rel = 'noopener noreferrer';
    return a;
  }

  fetch('https://api.github.com/users/SevensRequiem/repos?sort=pushed&per_page=4')
    .then(json)
    .then(function (repos) {
      return Promise.all(
        repos.map(function (repo) {
          return fetch(repo.url + '/commits/' + repo.default_branch)
            .then(json)
            .catch(function () {
              return null;
            })
            .then(function (commit) {
              return { repo: repo, commit: commit };
            });
        })
      );
    })
    .then(function (rows) {
      var items = rows.map(function (row) {
        var repo = row.repo;
        var hours = (Date.now() - Date.parse(repo.pushed_at)) / 3600000;
        var age = hours < 1 ? 'now' : hours < 24 ? Math.floor(hours) + 'h' : Math.floor(hours / 24) + 'd';
        var item = el('li', 'repo');
        var head = el('div', 'repo-head');
        head.append(
          link('repo-name', repo.name, repo.html_url),
          el('span', 'entry-tag', '★ ' + repo.stargazers_count + ' / ' + age)
        );
        item.append(head);
        if (repo.language) item.append(el('span', 'entry-tag', repo.language.toLowerCase() + ' / ' + repo.forks_count + ' forks'));
        if (row.commit) {
          var diff = el('span', 'entry-tag');
          diff.append(
            el('span', 'diff-add', '+' + row.commit.stats.additions),
            ' ',
            el('span', 'diff-del', '-' + row.commit.stats.deletions),
            ' / ' + row.commit.files.length + (row.commit.files.length === 1 ? ' file' : ' files')
          );
          item.append(link('repo-commit', row.commit.commit.message.split('\n')[0], row.commit.html_url), diff);
        }
        return item;
      });
      repoList.replaceChildren.apply(repoList, items);
    })
    .catch(function () {
      repoList.replaceChildren(el('li', 'entry-tag', 'signal lost: github is not answering.'));
    });

  var lightbox = document.getElementById('lightbox');
  var lightboxImg = document.getElementById('lightbox-img');
  var galleryGrid = document.getElementById('gallery-grid');
  fetch('./gallery/index.json')
    .then(json)
    .then(function (index) {
      document.getElementById('gallery-count').textContent =
        String(index.length).padStart(2, '0') + ' frames';
      index.forEach(function (item) {
        var base = './gallery/' + item.dir + '/';
        var stamp = item.date.slice(0, 10).replace(/-/g, '.');
        var tile = el('button', 'gallery-tile');
        tile.type = 'button';
        var img = el('img', '');
        img.src = base + item.thumb;
        img.alt = item.caption;
        img.loading = 'lazy';
        tile.append(img, el('span', 'gallery-meta', stamp));
        tile.addEventListener('click', function () {
          lightboxImg.src = base + item.image;
          lightboxImg.alt = item.caption;
          document.getElementById('lightbox-caption').textContent = item.caption;
          document.getElementById('lightbox-meta').textContent = stamp;
          lightbox.showModal();
        });
        galleryGrid.append(tile);
      });
    })
    .catch(function () {
      galleryGrid.replaceChildren(notice('signal lost: could not load ./gallery/index.json'));
    });
  lightbox.addEventListener('click', function (e) {
    if (e.target === lightbox) lightbox.close();
  });

  var clouds = document.querySelectorAll('.cloud');
  var CLOUD_RAMP = '.:-=+*#%@';

  function cloudHash(x, y) {
    var n = Math.sin(x * 127.1 + y * 311.7) * 43758.5453;
    return n - Math.floor(n);
  }

  function cloudNoise(x, y) {
    var xi = Math.floor(x);
    var yi = Math.floor(y);
    var u = (x - xi) * (x - xi) * (3 - 2 * (x - xi));
    var v = (y - yi) * (y - yi) * (3 - 2 * (y - yi));
    var top = cloudHash(xi, yi) + (cloudHash(xi + 1, yi) - cloudHash(xi, yi)) * u;
    var bottom = cloudHash(xi, yi + 1) + (cloudHash(xi + 1, yi + 1) - cloudHash(xi, yi + 1)) * u;
    return top + (bottom - top) * v;
  }

  function drawClouds() {
    var cols = Math.min(160, Math.ceil((innerWidth / 2 - 642) / 5));
    if (cols <= 0) return;
    var rows = Math.floor((innerHeight * 0.72) / 10);
    var scroll = performance.now() * 0.004;
    var sides = ['', ''];
    for (var y = 0; y < rows; y++) {
      var envelope = 0.5 + 0.5 * Math.sin((Math.PI * y) / (rows - 1));
      for (var c = -cols; c < cols; c++) {
        var density = cloudNoise((c + scroll) * 0.06, y * 0.16) * 0.65 + cloudNoise((c + scroll) * 0.17, y * 0.4) * 0.35;
        var level = Math.floor((density * envelope - 0.46) * 18);
        sides[c < 0 ? 0 : 1] += level < 0 ? ' ' : CLOUD_RAMP[Math.min(level, CLOUD_RAMP.length - 1)];
      }
      sides[0] += '\n';
      sides[1] += '\n';
    }
    clouds[0].textContent = sides[0];
    clouds[1].textContent = sides[1];
  }

  drawClouds();
  addEventListener('resize', drawClouds);
  if (!prefersReducedMotion) setInterval(drawClouds, 125);

  var fpsNote = document.getElementById('fps');
  var fpsFrames = 0;
  var fpsSince = performance.now();
  requestAnimationFrame(function tick(now) {
    fpsFrames++;
    if (now - fpsSince >= 1000) {
      fpsNote.textContent = Math.round((fpsFrames * 1000) / (now - fpsSince)) + ' fps';
      fpsFrames = 0;
      fpsSince = now;
    }
    requestAnimationFrame(tick);
  });

  var GHOST_COUNT = 16;
  var GHOST_INTERVAL = 35;
  var GHOST_LIFE = 800;
  var ghostPool = null;
  var ghostIdx = 0;
  var lastGhostTime = 0;
  var ghostRafId = 0;

  function makeGhostPool() {
    var pool = [];
    for (var i = 0; i < GHOST_COUNT; i++) {
      var img = document.createElement('img');
      img.src = './assets/cursors/arrow.png';
      img.className = 'cursor-ghost';
      img.style.left = '0';
      img.style.top = '0';
      img.style.opacity = '0';
      img.style.transform = 'translate3d(-100px,-100px,0)';
      document.body.appendChild(img);
      pool.push({
        el: img,
        born: 0,
        x: 0,
        y: 0,
        driftX: 0,
        driftY: 0,
        active: false,
      });
    }
    return pool;
  }

  document.addEventListener('mousemove', function (e) {
    if (prefersReducedMotion || document.body.classList.contains('low-fx'))
      return;
    var now = Date.now();
    if (now - lastGhostTime < GHOST_INTERVAL) return;
    lastGhostTime = now;
    if (!ghostPool) ghostPool = makeGhostPool();
    var g = ghostPool[ghostIdx];
    ghostIdx = (ghostIdx + 1) % GHOST_COUNT;
    g.born = now;
    g.x = e.clientX;
    g.y = e.clientY;
    g.driftX = (Math.random() - 0.5) * 8;
    g.driftY = (Math.random() - 0.5) * 8;
    g.active = true;
    g.el.style.opacity = '0.5';
    g.el.style.transform = 'translate3d(' + g.x + 'px,' + g.y + 'px,0)';
    if (!ghostRafId) ghostRafId = requestAnimationFrame(updateGhosts);
  });

  function updateGhosts() {
    var now = Date.now();
    var alive = 0;
    for (var i = 0; i < ghostPool.length; i++) {
      var g = ghostPool[i];
      if (!g.active) continue;
      var age = now - g.born;
      if (age > GHOST_LIFE) {
        g.active = false;
        g.el.style.opacity = '0';
        continue;
      }
      alive++;
      var t = age / GHOST_LIFE;
      var gx = (g.x + g.driftX * t).toFixed(1);
      var gy = (g.y + g.driftY * t).toFixed(1);
      var sc = (1 - t * 0.3).toFixed(3);
      g.el.style.opacity = (0.45 * (1 - t * t)).toFixed(3);
      g.el.style.transform =
        'translate3d(' + gx + 'px,' + gy + 'px,0) scale(' + sc + ')';
    }
    ghostRafId = alive ? requestAnimationFrame(updateGhosts) : 0;
  }
})();
