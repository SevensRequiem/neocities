(function () {
  document.querySelectorAll('.flag-col').forEach((col, i) => {
    col.style.animationDelay = `${(6 - i) * 0.4}s`;
  });

  var alchemySymbols = [0x2604, 0x260a, 0x260b, 0x260c, 0x260d, 0x2620, 0x2622, 0x2624, 0x262a, 0x262f, 0x2656, 0x265c, 0x2666, 0x2672, 0x269b].map(function (cp) {
    return String.fromCodePoint(cp);
  });
  [[0x263c, 24], [0x26b3, 10], [0x1f700, 116]].forEach(function (r) {
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
  var memos = [];
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

  function memoStrips(slot) {
    return memos.filter(function (memo) {
      return memo.slot === slot;
    }).map(function (memo) {
      var strip = el('aside', 'memo-strip');
      strip.id = 'memo-' + memo.id;
      var head = el('a', 'memo-head', '◈ MEMO.' + String(memo.id).padStart(3, '0') + ' :: ' + memo.date.slice(0, 16).replace(/-/g, '.').replace('T', ' '));
      head.href = '#memo-' + memo.id;
      var body = el('div', 'memo-body');
      body.innerHTML = memo.html;
      strip.append(head, body);
      return strip;
    });
  }

  function showPage(n) {
    var pages = Math.ceil(posts.length / PAGE_SIZE);
    page = Math.min(Math.max(n, 1), pages);
    var first = (page - 1) * PAGE_SIZE;
    var cards = [];
    posts.slice(first, page * PAGE_SIZE).forEach(function (post, i) {
      var card = postCard(post, post === posts[0]);
      card.append(el('p', 'entry-para', post.excerpt), postFoot('read more', './blog/' + post.dir + '/'));
      cards.push.apply(cards, memoStrips(first + i));
      cards.push(card);
    });
    if (page === pages) cards.push.apply(cards, memoStrips(posts.length));
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
    var exportLink = el('a', 'blog-export blog-export-json', 'export');
    exportLink.href = './blog/all.json';
    exportLink.download = 'all.json';
    var rssLink = el('a', 'blog-export blog-export-rss', 'rss');
    rssLink.href = './feed.xml';
    var feeds = el('div', 'weblog-feeds');
    feeds.append(exportLink, rssLink);
    var tools = el('div', 'weblog-tools');
    tools.append(pager, feeds);
    weblog.replaceChildren.apply(weblog, [tools].concat(cards));
  }

  function route(e) {
    var stories = location.hash === '#stories';
    if (stories) selectTab(document.getElementById('tab-stories'));
    if (!posts.length) return;
    var m = location.hash.match(/^#(post|page|entry|memo)-(\d+)$/);
    var list = m && m[1] === 'memo' ? memos : posts;
    var idx = m && m[1] !== 'page' ? list.findIndex(function (p) {
      return p.id === Number(m[2]);
    }) : -1;
    if (m && m[1] === 'post' && idx >= 0) return location.replace('./blog/' + posts[idx].dir + '/');
    var slot = list === memos && idx >= 0 ? Math.min(memos[idx].slot, posts.length - 1) : idx;
    var target = stories ? page : m && m[1] === 'page' ? Number(m[2]) : slot >= 0 ? Math.floor(slot / PAGE_SIZE) + 1 : 1;
    if (m && m[1] === 'entry') history.replaceState(null, '', '#page-' + target);
    showPage(target);
    if (stories) return;
    selectTab(document.getElementById('tab-weblog'));
    if (list === memos && idx >= 0) return document.getElementById('memo-' + memos[idx].id).scrollIntoView();
    if (e) document.querySelector('.tab-bar').scrollIntoView();
  }

  if (location.hash === '#stories') route();
  addEventListener('hashchange', route);

  Promise.all([
    fetch('./blog/index.json').then(json),
    fetch('./memos/index.json').then(json).catch(function () {
      return [];
    }),
  ])
    .then(function (data) {
      posts = data[0];
      memos = data[1].map(function (memo) {
        var at = Date.parse(memo.date);
        var slot = posts.findIndex(function (post) {
          return Date.parse(post.date) < at;
        });
        memo.slot = slot < 0 ? posts.length : slot;
        return memo;
      });
      route();
    })
    .catch(function () {
      weblog.replaceChildren(notice('signal lost: could not load ./blog/index.json'));
    });

  var storiesPanel = document.getElementById('panel-stories');
  fetch('./stories/index.json')
    .then(json)
    .then(function (index) {
      var cards = index.map(function (story) {
        var card = el('article', 'entry-card post-card story-card');
        var head = el('header', 'entry-head');
        head.append(
          el('h2', 'panel-title', '::STORY.' + String(story.id).padStart(3, '0')),
          el('span', 'entry-tag', story.date.slice(0, 10).replace(/-/g, '.'))
        );
        card.append(head);
        if (story.image) {
          var frame = el('div', 'mosaic-frame post-frame');
          var img = el('img', 'mosaic-img');
          img.src = './stories/' + story.dir + '/' + story.image;
          img.alt = story.title;
          img.loading = 'lazy';
          frame.append(img);
          card.append(frame);
        }
        var heading = el('div', 'post-heading');
        heading.append(
          el('h3', 'post-title', story.title),
          el('span', 'entry-tag story-meta', story.words + (story.words === 1 ? ' word' : ' words') + ' :: ' + Math.max(1, Math.floor(story.words / 230)) + ' min read')
        );
        if (story.tags.length) {
          heading.append(el('span', 'entry-tag post-tags', story.tags.map(function (tag) {
            return '#' + tag.toLowerCase().replace(/\s+/g, '-');
          }).join(' ')));
        }
        card.append(heading);
        if (story.summary) card.append(el('p', 'entry-para', story.summary));
        card.append(postFoot('read', './stories/' + story.dir + '/'));
        return card;
      });
      var tools = el('div', 'weblog-tools');
      var indexLink = el('a', 'panel-link', 'story index');
      indexLink.href = './stories/';
      var rssLink = el('a', 'blog-export blog-export-rss', 'rss');
      rssLink.href = './stories.xml';
      var feeds = el('div', 'weblog-feeds');
      feeds.append(rssLink);
      tools.append(indexLink, feeds);
      storiesPanel.replaceChildren.apply(storiesPanel, [tools].concat(cards.length ? cards : [notice('no stories yet. the first one is still being written.')]));
    })
    .catch(function () {
      storiesPanel.replaceChildren(notice('signal lost: could not load ./stories/index.json'));
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

  var repoCache = null;
  try {
    repoCache = JSON.parse(sessionStorage.getItem('gh-repos'));
  } catch (e) {}

  (repoCache && Date.now() - repoCache.at < 600000
    ? Promise.resolve(repoCache.rows)
    : fetch('https://api.github.com/users/SevensRequiem/repos?sort=pushed&per_page=3')
        .then(json)
        .then(function (repos) {
          return Promise.all(
            repos.map(function (repo) {
              return fetch(repo.url + '/commits/' + repo.default_branch)
                .then(json)
                .then(function (commit) {
                  return {
                    message: commit.commit.message.split('\n')[0],
                    url: commit.html_url,
                    additions: commit.stats.additions,
                    deletions: commit.stats.deletions,
                    files: commit.files.length,
                  };
                })
                .catch(function () {
                  return null;
                })
                .then(function (commit) {
                  return {
                    name: repo.name,
                    url: repo.html_url,
                    pushed: repo.pushed_at,
                    stars: repo.stargazers_count,
                    forks: repo.forks_count,
                    language: repo.language,
                    commit: commit,
                  };
                });
            })
          );
        })
        .then(function (rows) {
          try {
            sessionStorage.setItem('gh-repos', JSON.stringify({ at: Date.now(), rows: rows }));
          } catch (e) {}
          return rows;
        })
  )
    .then(function (rows) {
      var items = rows.map(function (repo) {
        var hours = (Date.now() - Date.parse(repo.pushed)) / 3600000;
        var age = hours < 1 ? 'now' : hours < 24 ? Math.floor(hours) + 'h' : Math.floor(hours / 24) + 'd';
        var item = el('li', 'repo');
        var head = el('div', 'repo-head');
        head.append(
          link('repo-name', repo.name, repo.url),
          el('span', 'entry-tag', '★ ' + repo.stars + ' / ' + age)
        );
        item.append(head);
        if (repo.language) item.append(el('span', 'entry-tag', repo.language.toLowerCase() + ' / ' + repo.forks + ' forks'));
        if (repo.commit) {
          var diff = el('span', 'entry-tag');
          diff.append(
            el('span', 'diff-add', '+' + repo.commit.additions),
            ' ',
            el('span', 'diff-del', '-' + repo.commit.deletions),
            ' / ' + repo.commit.files + (repo.commit.files === 1 ? ' file' : ' files')
          );
          item.append(link('repo-commit', repo.commit.message, repo.commit.url), diff);
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

  var QA_API = 'https://api.requiem.moe/qa';
  var qaDialog = document.getElementById('qa-dialog');
  var qaLatest = document.getElementById('qa-latest');
  var qaList = document.getElementById('qa-list');
  var qaMore = document.getElementById('qa-more');
  var qaForm = document.getElementById('qa-form');
  var qaStatus = document.getElementById('qa-status');
  var qaNext = null;

  function qaEntry(item) {
    var entry = el('article', 'qa-entry');
    entry.append(
      el('p', 'qa-question', item.question),
      el('p', 'qa-answer', item.answer),
      el('span', 'entry-tag', item.answered.slice(0, 10).replace(/-/g, '.'))
    );
    return entry;
  }

  function loadQa() {
    return fetch(qaNext ? QA_API + '?before=' + qaNext : QA_API)
      .then(json)
      .then(function (page) {
        qaNext = page.next;
        qaMore.hidden = !page.next;
        qaList.append.apply(qaList, page.items.map(qaEntry));
        return page;
      });
  }

  loadQa()
    .then(function (page) {
      document.getElementById('qa-count').textContent = String(page.total).padStart(2, '0') + ' answered';
      qaLatest.replaceChildren(page.items.length ? qaEntry(page.items[0]) : el('span', 'entry-tag', 'nothing answered yet. ask away.'));
    })
    .catch(function () {
      qaLatest.replaceChildren(el('span', 'entry-tag', 'signal lost: the q&a is not answering.'));
    });

  document.getElementById('qa-open').addEventListener('click', function () {
    qaDialog.showModal();
  });
  qaDialog.addEventListener('click', function (e) {
    if (e.target === qaDialog) qaDialog.close();
  });
  qaMore.addEventListener('click', function () {
    qaMore.disabled = true;
    loadQa()
      .catch(function () {
        qaStatus.textContent = 'signal lost: could not load older answers.';
      })
      .finally(function () {
        qaMore.disabled = false;
      });
  });
  qaForm.addEventListener('submit', function (e) {
    e.preventDefault();
    var send = document.getElementById('qa-send');
    send.disabled = true;
    qaStatus.textContent = 'transmitting...';
    fetch(QA_API, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ question: qaForm.elements.question.value, website: qaForm.elements.website.value }),
    })
      .then(function (res) {
        if (res.status === 429) return 'too many questions, try again in a minute.';
        if (!res.ok) return 'signal lost: the question did not go through.';
        qaForm.reset();
        return 'received. it shows up below once answered.';
      }, function () {
        return 'signal lost: the question did not go through.';
      })
      .then(function (message) {
        qaStatus.textContent = message;
      })
      .finally(function () {
        send.disabled = false;
      });
  });

  var viewsNote = document.getElementById('views');
  fetch('https://api.requiem.moe/views')
    .then(json)
    .then(function (data) {
      viewsNote.replaceChildren(
        data.views.toLocaleString('en-US') + ' views ',
        el('span', 'diff-add', '+' + data.week.views.toLocaleString('en-US')),
        ' / wk'
      );
      viewsNote.title = data.hits.toLocaleString('en-US') + ' hits, +' + data.week.hits.toLocaleString('en-US') + ' since ' + data.week.since.slice(0, 10);
    })
    .catch(function () {
      viewsNote.textContent = 'views: signal lost';
    });
})();
