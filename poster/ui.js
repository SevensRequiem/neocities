(function () {
  var token = document.querySelector('meta[name="poster-token"]').content;
  var forms = [document.getElementById('form-blog'), document.getElementById('form-gallery')];
  var tabs = [document.getElementById('tab-blog'), document.getElementById('tab-gallery')];
  var previewImg = document.getElementById('preview-img');
  var previewBody = document.getElementById('preview-body');
  var statusLine = document.getElementById('status-line');

  var commonTags = JSON.parse(document.querySelector('meta[name="poster-tags"]').content);

  function tagChip(label, onClick) {
    var chip = document.createElement('button');
    chip.type = 'button';
    chip.className = 'tag-chip';
    chip.textContent = label;
    chip.addEventListener('click', onClick);
    return chip;
  }

  forms.forEach(function (form) {
    var picked = [];
    var fresh = form.querySelector('.tag-new');

    function renderTags() {
      form.elements.tags.value = picked.join(',');
      form.querySelector('.tag-picked').replaceChildren.apply(
        form.querySelector('.tag-picked'),
        picked.map(function (tag) {
          return tagChip('- ' + tag, function () {
            picked.splice(picked.indexOf(tag), 1);
            renderTags();
          });
        })
      );
      form.querySelector('.tag-common').replaceChildren.apply(
        form.querySelector('.tag-common'),
        commonTags
          .filter(function (tag) {
            return picked.indexOf(tag) < 0;
          })
          .map(function (tag) {
            return tagChip('+ ' + tag, function () {
              picked.push(tag);
              renderTags();
            });
          })
      );
    }

    function addFresh() {
      var tag = fresh.value.replace(/,/g, ' ').trim();
      fresh.value = '';
      if (tag && picked.indexOf(tag) < 0) picked.push(tag);
      renderTags();
    }

    form.querySelector('.tag-add').addEventListener('click', addFresh);
    fresh.addEventListener('keydown', function (e) {
      if (e.key !== 'Enter') return;
      e.preventDefault();
      addFresh();
    });
    form.addEventListener('reset', function () {
      picked = [];
      renderTags();
    });
    renderTags();
  });


  function renderPreview() {
    var form = forms[0].hidden ? forms[1] : forms[0];
    var text = form.elements.body ? form.elements.body.value : form.elements.caption.value;
    previewBody.innerHTML = DOMPurify.sanitize(marked.parse(text));
    var file = form.elements.image.files[0];
    URL.revokeObjectURL(previewImg.src);
    previewImg.hidden = !file;
    if (file) previewImg.src = URL.createObjectURL(file);
  }

  var workspace = document.querySelector('.workspace');
  var qaTab = document.getElementById('tab-qa');
  var qaInbox = document.getElementById('qa-inbox');
  var qaItems = document.getElementById('qa-items');

  tabs.forEach(function (tab, i) {
    tab.addEventListener('click', function () {
      tabs.forEach(function (t, j) {
        t.setAttribute('aria-selected', i === j);
        forms[j].hidden = i !== j;
      });
      qaTab.setAttribute('aria-selected', false);
      qaInbox.hidden = true;
      workspace.hidden = false;
      renderPreview();
    });
  });

  function qaRequest(method, path, body) {
    return fetch(path, {
      method: method,
      headers: { 'X-Poster-Token': token },
      body: body && JSON.stringify(body)
    })
      .then(function (res) {
        return res.json();
      })
      .then(function (out) {
        if (out.error) throw new Error(out.error);
        return out;
      });
  }

  function qaFail(err) {
    statusLine.className = 'status-line status-error';
    statusLine.textContent = 'signal lost: ' + err.message;
  }

  function qaItem(item) {
    var form = document.createElement('form');
    form.className = 'post-form qa-item' + (item.answered ? '' : ' qa-pending');
    var question = document.createElement('p');
    question.className = 'qa-question';
    question.textContent = item.question;
    var meta = document.createElement('span');
    meta.className = 'field-label';
    meta.textContent = 'asked ' + item.asked.slice(0, 16).replace('T', ' ') + (item.answered ? ' :: answered ' + item.answered.slice(0, 16).replace('T', ' ') : ' :: pending');
    var answer = document.createElement('textarea');
    answer.className = 'field-input qa-answer';
    answer.maxLength = 4000;
    answer.required = true;
    answer.value = item.answer || '';
    answer.setAttribute('aria-label', 'answer');
    var save = document.createElement('button');
    save.className = 'submit-btn';
    save.textContent = item.answered ? 'update' : 'answer';
    var remove = document.createElement('button');
    remove.type = 'button';
    remove.className = 'tag-chip';
    remove.textContent = 'delete';
    var row = document.createElement('div');
    row.className = 'tag-row';
    row.append(save, remove);
    form.append(question, meta, answer, row);

    form.addEventListener('submit', function (e) {
      e.preventDefault();
      save.disabled = true;
      qaRequest('PUT', '/qa/' + item.id, { answer: answer.value })
        .then(function (out) {
          statusLine.className = 'status-line';
          statusLine.textContent = out.done;
          loadInbox();
        })
        .catch(qaFail)
        .finally(function () {
          save.disabled = false;
        });
    });
    remove.addEventListener('click', function () {
      if (remove.textContent === 'delete') {
        remove.textContent = 'really delete';
        return;
      }
      qaRequest('DELETE', '/qa/' + item.id)
        .then(function (out) {
          statusLine.className = 'status-line';
          statusLine.textContent = out.done;
          form.remove();
        })
        .catch(qaFail);
    });
    return form;
  }

  function loadInbox() {
    qaRequest('GET', '/qa/inbox')
      .then(function (items) {
        qaItems.replaceChildren.apply(qaItems, items.map(qaItem));
        if (!items.length) qaItems.textContent = 'no questions yet.';
      })
      .catch(qaFail);
  }

  qaTab.addEventListener('click', function () {
    tabs.forEach(function (t) {
      t.setAttribute('aria-selected', false);
    });
    qaTab.setAttribute('aria-selected', true);
    workspace.hidden = true;
    qaInbox.hidden = false;
    loadInbox();
  });
  document.getElementById('qa-refresh').addEventListener('click', loadInbox);

  forms.forEach(function (form) {
    form.addEventListener('input', renderPreview);
    form.addEventListener('submit', function (e) {
      e.preventDefault();
      var button = form.querySelector('.submit-btn');
      button.disabled = true;
      statusLine.className = 'status-line';
      statusLine.textContent = 'writing...';
      fetch(form.getAttribute('action'), {
        method: 'POST',
        headers: { 'X-Poster-Token': token },
        body: new FormData(form)
      })
        .then(function (res) {
          return res.json();
        })
        .then(function (out) {
          if (out.error) throw new Error(out.error);
          statusLine.textContent = out.done;
          var commit = form.elements.commit.checked;
          var push = form.elements.push.checked;
          form.reset();
          form.elements.commit.checked = commit;
          form.elements.push.checked = push;
          renderPreview();
        })
        .catch(function (err) {
          statusLine.className = 'status-line status-error';
          statusLine.textContent = 'signal lost: ' + err.message;
        })
        .finally(function () {
          button.disabled = false;
        });
    });
  });
})();
