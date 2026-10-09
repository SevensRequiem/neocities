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

  tabs.forEach(function (tab, i) {
    tab.addEventListener('click', function () {
      tabs.forEach(function (t, j) {
        t.setAttribute('aria-selected', i === j);
        forms[j].hidden = i !== j;
      });
      renderPreview();
    });
  });

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
