(function () {
  var token = document.querySelector('meta[name="poster-token"]').content;
  var gitCommit = document.getElementById('git-commit');
  var gitPush = document.getElementById('git-push');
  var draftList = document.getElementById('draft-list');
  var publishedList = document.getElementById('published-list');
  var emptyDesk = document.getElementById('empty-desk');
  var desk = document.getElementById('desk');
  var statusBar = document.getElementById('status-bar');
  var fields = {
    title: document.getElementById('f-title'),
    tags: document.getElementById('f-tags'),
    summary: document.getElementById('f-summary'),
    author: document.getElementById('f-author'),
    body: document.getElementById('f-body')
  };
  var coverInput = document.getElementById('f-cover');
  var coverRemove = document.getElementById('cover-remove');
  var pvHead = document.getElementById('pv-head');
  var pvTitle = document.getElementById('pv-title');
  var pvMeta = document.getElementById('pv-meta');
  var pvCover = document.getElementById('pv-cover');
  var pvBody = document.getElementById('pv-body');
  var wordCount = document.getElementById('word-count');
  var saveState = document.getElementById('save-state');
  var conflictBox = document.getElementById('conflict');
  var deleteBtn = document.getElementById('delete-draft');
  var publishBtn = document.getElementById('publish');
  var statusLine = document.getElementById('status-line');

  var current = null;
  var dirty = false;
  var conflict = false;
  var chain = Promise.resolve();
  var debounce = 0;

  function request(method, path, body, options) {
    var init = { method: method, headers: { 'X-Poster-Token': token } };
    if (body instanceof FormData) {
      init.body = body;
    } else if (body) {
      init.headers['Content-Type'] = 'application/json';
      init.body = JSON.stringify(body);
    }
    if (options && options.keepalive) init.keepalive = true;
    return fetch(path, init).then(function (res) {
      return res.json().then(function (out) {
        if (res.ok) return out;
        var err = new Error(out.error || 'http ' + res.status);
        err.status = res.status;
        throw err;
      });
    });
  }

  function note(text, isError) {
    statusLine.className = isError ? 'status-line status-error' : 'status-line';
    statusLine.textContent = text;
  }

  function fail(err) {
    note('signal lost: ' + err.message, true);
  }

  function setState(text, isError) {
    saveState.className = isError ? 'save-state state-error' : 'save-state';
    saveState.textContent = text;
  }

  function pad(n) {
    return String(n).padStart(3, '0');
  }

  function two(n) {
    return String(n).padStart(2, '0');
  }

  function stamp(iso) {
    var d = new Date(iso);
    return d.getFullYear() + '.' + two(d.getMonth() + 1) + '.' + two(d.getDate()) + ' ' + two(d.getHours()) + ':' + two(d.getMinutes());
  }

  function clock(iso) {
    var d = new Date(iso);
    return two(d.getHours()) + ':' + two(d.getMinutes()) + ':' + two(d.getSeconds());
  }

  function readTime(words) {
    return Math.max(1, Math.floor(words / 230)) + ' min read';
  }

  function render() {
    pvHead.textContent = current && current.story ? '::STORY.' + pad(current.story) + ' :: EDIT' : '::STORY.NEW';
    pvTitle.textContent = fields.title.value.trim() || 'untitled';
    pvBody.innerHTML = DOMPurify.sanitize(marked.parse(fields.body.value));
    var words = pvBody.innerText.split(/\s+/).filter(Boolean).length;
    var line = words + ' words :: ' + readTime(words);
    wordCount.textContent = line;
    var tags = fields.tags.value
      .split(',')
      .map(function (t) {
        return t.trim();
      })
      .filter(Boolean)
      .map(function (t) {
        return '#' + t;
      });
    pvMeta.textContent = [line].concat(tags).join(' :: ');
  }

  function shelfMeta(text, className) {
    var span = document.createElement('span');
    span.className = className || 'shelf-meta';
    span.textContent = text;
    return span;
  }

  function loadLibrary() {
    return request('GET', '/library')
      .then(function (lib) {
        draftList.replaceChildren.apply(
          draftList,
          lib.drafts.map(function (d) {
            var li = document.createElement('li');
            var btn = document.createElement('button');
            btn.type = 'button';
            btn.className = 'shelf-item';
            btn.dataset.id = d.id;
            btn.setAttribute('aria-current', String(!!current && current.id === d.id));
            btn.append(shelfMeta(d.title.trim() || 'untitled', 'shelf-title'), shelfMeta('saved ' + stamp(d.saved) + ' :: ' + d.words + ' words'));
            if (d.story) btn.append(shelfMeta('edits ::STORY.' + pad(d.story), 'shelf-meta shelf-link'));
            btn.addEventListener('click', function () {
              openDraft(d.id);
            });
            li.append(btn);
            return li;
          })
        );
        if (!lib.drafts.length) draftList.append(shelfMeta('no drafts.', 'shelf-empty'));

        publishedList.replaceChildren.apply(
          publishedList,
          lib.published.map(function (s) {
            var li = document.createElement('li');
            var item = document.createElement('div');
            item.className = 'shelf-item';
            var view = document.createElement('a');
            view.className = 'shelf-title';
            view.href = '/site/stories/' + encodeURIComponent(s.dir) + '/';
            view.target = '_blank';
            view.rel = 'noopener noreferrer';
            view.textContent = s.title;
            var row = document.createElement('div');
            row.className = 'tag-row';
            var edit = document.createElement('button');
            edit.type = 'button';
            edit.className = 'tag-chip';
            edit.textContent = 'edit';
            edit.addEventListener('click', function () {
              hold(true);
              flush()
                .then(function () {
                  return request('POST', '/stories/' + s.id + '/edit');
                })
                .then(fill)
                .then(loadLibrary)
                .catch(fail)
                .finally(function () {
                  hold(false);
                });
            });
            var pull = document.createElement('button');
            pull.type = 'button';
            pull.className = 'tag-chip';
            pull.textContent = 'unpublish';
            pull.addEventListener('click', function () {
              if (pull.textContent === 'unpublish') {
                pull.textContent = 'really unpublish';
                pull.classList.add('chip-danger');
                return;
              }
              pull.disabled = true;
              hold(true);
              note('unpublishing...');
              flush()
                .then(function () {
                  return request('POST', '/stories/' + s.id + '/unpublish', { commit: gitCommit.checked, push: gitPush.checked });
                })
                .then(function (out) {
                  note(out.done);
                  fill(out.draft);
                  return loadLibrary();
                })
                .catch(function (err) {
                  pull.disabled = false;
                  fail(err);
                })
                .finally(function () {
                  hold(false);
                });
            });
            row.append(edit, pull);
            item.append(view, shelfMeta('::STORY.' + pad(s.id) + ' :: ' + stamp(s.date)), shelfMeta(s.words + ' words :: ' + readTime(s.words)), row);
            li.append(item);
            return li;
          })
        );
        if (!lib.published.length) publishedList.append(shelfMeta('nothing published yet.', 'shelf-empty'));
      })
      .catch(fail);
  }

  function hold(on) {
    Object.keys(fields).forEach(function (key) {
      fields[key].readOnly = on;
    });
    coverInput.disabled = on;
    coverRemove.disabled = on;
    deleteBtn.disabled = on;
  }

  function loadCover() {
    if (pvCover.src) URL.revokeObjectURL(pvCover.src);
    pvCover.removeAttribute('src');
    pvCover.hidden = true;
    coverRemove.hidden = !current || !current.cover;
    if (!current || !current.cover) return;
    var id = current.id;
    fetch('/drafts/' + id + '/cover', { headers: { 'X-Poster-Token': token } })
      .then(function (res) {
        if (!res.ok) throw new Error('cover http ' + res.status);
        return res.blob();
      })
      .then(function (blob) {
        if (!current || current.id !== id) return;
        pvCover.src = URL.createObjectURL(blob);
        pvCover.hidden = false;
      })
      .catch(fail);
  }

  function fill(d) {
    current = { id: d.id, rev: d.rev, story: d.story, cover: d.cover };
    dirty = false;
    conflict = false;
    conflictBox.hidden = true;
    fields.title.value = d.title;
    fields.tags.value = d.tags;
    fields.summary.value = d.summary;
    fields.author.value = d.author;
    fields.body.value = d.body;
    coverInput.value = '';
    deleteBtn.textContent = 'delete';
    deleteBtn.classList.remove('chip-danger');
    publishBtn.textContent = d.story ? 'publish update' : 'publish';
    emptyDesk.hidden = true;
    desk.hidden = false;
    statusBar.hidden = false;
    history.replaceState(null, '', '#' + d.id);
    setState('saved ' + clock(d.saved));
    loadCover();
    render();
    Array.prototype.forEach.call(draftList.querySelectorAll('.shelf-item'), function (btn) {
      btn.setAttribute('aria-current', String(btn.dataset.id === d.id));
    });
  }

  function closeDesk() {
    current = null;
    dirty = false;
    desk.hidden = true;
    statusBar.hidden = true;
    emptyDesk.hidden = false;
    history.replaceState(null, '', location.pathname);
    loadCover();
  }

  function save(keepalive) {
    clearTimeout(debounce);
    if (!current || conflict || !dirty) return chain;
    dirty = false;
    setState('saving...');
    var snap = {
      id: current.id,
      draft: {
        title: fields.title.value,
        tags: fields.tags.value,
        summary: fields.summary.value,
        author: fields.author.value,
        body: fields.body.value
      }
    };
    chain = chain.then(function () {
      if (!current || current.id !== snap.id) return;
      snap.draft.rev = current.rev;
      var body = JSON.stringify(snap.draft);
      return request('PUT', '/drafts/' + snap.id, snap.draft, { keepalive: keepalive && body.length < 60000 })
        .then(function (out) {
          if (!current || current.id !== snap.id) return;
          current.rev = out.rev;
          if (!dirty) setState('saved ' + clock(out.saved));
          loadLibrary();
        })
        .catch(function (err) {
          if (err.status === 409) {
            conflict = true;
            conflictBox.hidden = false;
            setState('this draft was changed elsewhere (another tab, or its story was unpublished). reload the saved copy, or keep yours to overwrite it.', true);
            return;
          }
          dirty = true;
          setState('save failed: ' + err.message, true);
        });
    });
    return chain;
  }

  function flush() {
    return save().then(function () {
      if (conflict) throw new Error('resolve the save conflict first');
      if (dirty) throw new Error('the draft is not saved yet');
    });
  }

  function openDraft(id) {
    return flush()
      .then(function () {
        return request('GET', '/drafts/' + id);
      })
      .then(fill)
      .catch(fail);
  }

  Object.keys(fields).forEach(function (key) {
    fields[key].addEventListener('input', function () {
      if (!current) return;
      dirty = true;
      if (!conflict) setState('unsaved changes');
      render();
      clearTimeout(debounce);
      debounce = setTimeout(save, 1000);
    });
  });

  setInterval(function () {
    if (dirty) save();
  }, 30000);

  document.addEventListener('visibilitychange', function () {
    if (document.visibilityState === 'hidden' && dirty) save(true);
  });
  window.addEventListener('pagehide', function () {
    if (dirty) save(true);
  });

  document.getElementById('conflict-reload').addEventListener('click', function () {
    request('GET', '/drafts/' + current.id)
      .then(fill)
      .catch(fail);
  });

  document.getElementById('conflict-keep').addEventListener('click', function () {
    request('GET', '/drafts/' + current.id)
      .then(function (d) {
        current.rev = d.rev;
        current.story = d.story;
        publishBtn.textContent = d.story ? 'publish update' : 'publish';
        render();
        conflict = false;
        conflictBox.hidden = true;
        dirty = true;
        return save();
      })
      .catch(fail);
  });

  document.getElementById('new-draft').addEventListener('click', function () {
    flush()
      .then(function () {
        return request('POST', '/drafts');
      })
      .then(function (d) {
        fill(d);
        fields.title.focus();
        return loadLibrary();
      })
      .catch(fail);
  });

  coverInput.addEventListener('change', function () {
    var file = coverInput.files[0];
    if (!file || !current) return;
    var form = new FormData();
    form.append('image', file);
    setState('converting cover...');
    request('PUT', '/drafts/' + current.id + '/cover', form)
      .then(function () {
        current.cover = true;
        setState(dirty ? 'unsaved changes' : 'cover saved');
        loadCover();
      })
      .catch(function (err) {
        setState('cover failed: ' + err.message, true);
      })
      .finally(function () {
        coverInput.value = '';
      });
  });

  coverRemove.addEventListener('click', function () {
    request('DELETE', '/drafts/' + current.id + '/cover')
      .then(function () {
        current.cover = false;
        loadCover();
      })
      .catch(fail);
  });

  deleteBtn.addEventListener('click', function () {
    if (deleteBtn.textContent === 'delete') {
      deleteBtn.textContent = 'really delete';
      deleteBtn.classList.add('chip-danger');
      return;
    }
    var id = current.id;
    clearTimeout(debounce);
    dirty = false;
    chain
      .then(function () {
        return request('DELETE', '/drafts/' + id);
      })
      .then(function (out) {
        note(out.done);
        closeDesk();
        return loadLibrary();
      })
      .catch(fail);
  });

  publishBtn.addEventListener('click', function () {
    publishBtn.disabled = true;
    hold(true);
    note('publishing...');
    flush()
      .then(function () {
        return request('POST', '/drafts/' + current.id + '/publish', { commit: gitCommit.checked, push: gitPush.checked, rev: current.rev });
      })
      .then(function (out) {
        note(out.done);
        closeDesk();
        return loadLibrary();
      })
      .catch(function (err) {
        if (err.status === 409) {
          conflict = true;
          conflictBox.hidden = false;
        }
        fail(err);
      })
      .finally(function () {
        publishBtn.disabled = false;
        hold(false);
      });
  });

  loadLibrary();
  if (/^#[0-9a-f]{16}$/.test(location.hash)) openDraft(location.hash.slice(1));
})();
