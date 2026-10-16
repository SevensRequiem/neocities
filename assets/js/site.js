(function () {
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

  function selectTab(tab) {
    tabs.forEach(function (t) {
      t.setAttribute('aria-selected', t === tab);
    });
    document.querySelectorAll('.tab-panel').forEach(function (panel) {
      panel.hidden = panel.id !== tab.getAttribute('aria-controls');
    });
  }

  tabs.forEach(function (tab) {
    tab.addEventListener('click', function () {
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
      img.alt = post.title;
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
      card.append(el('p', 'entry-para', post.excerpt), postFoot('read more', './blog/' + post.dir + '/'));
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
    var exportLink = el('a', 'panel-link blog-export', 'blog export json');
    exportLink.href = './blog/all.json';
    exportLink.download = 'all.json';
    var tools = el('div', 'weblog-tools');
    tools.append(pager, exportLink);
    weblog.replaceChildren.apply(weblog, [tools].concat(cards));
    selectTab(document.getElementById('tab-weblog'));
  }

  function route(e) {
    var m = location.hash.match(/^#(post|page)-(\d+)$/);
    var post = m && m[1] === 'post' && posts.find(function (p) {
      return p.id === Number(m[2]);
    });
    if (post) return location.replace('./blog/' + post.dir + '/');
    showPage(m && m[1] === 'page' ? Number(m[2]) : 1);
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

  var WEBRING = 'https://raw.githubusercontent.com/SevensRequiem/neocities/main/webring/';
  var webringList = document.getElementById('webring-list');
  fetch(WEBRING + 'webring.json')
    .then(json)
    .then(function (sites) {
      document.getElementById('webring-count').textContent =
        String(sites.length).padStart(2, '0') + ' linked';
      var items = sites.map(function (site) {
        var a = link('webring-site', '', site.url);
        var img = el('img', '');
        img.src = new URL(site.banner, WEBRING);
        img.alt = new URL(site.url).hostname;
        img.width = 88;
        img.height = 31;
        img.loading = 'lazy';
        a.append(img);
        return a;
      });
      webringList.replaceChildren.apply(webringList, items);
    })
    .catch(function () {
      webringList.replaceChildren(el('span', 'entry-tag', 'signal lost: could not load the webring.'));
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
})();
