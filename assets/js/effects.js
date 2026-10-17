(function () {
  var motionQuery = window.matchMedia('(prefers-reduced-motion: reduce)');
  var coarsePointerQuery = window.matchMedia('(pointer: coarse)');
  var clouds = document.querySelectorAll('.cloud');
  var staticOverlay = document.getElementById('static-overlay');
  var cloudRamp = '.:-=+*#%@';
  var cloudTimer = 0;
  var cloudSize = '';
  var trailsEnabled = false;
  var ghostCount = 16;
  var ghostInterval = 35;
  var ghostLife = 800;
  var ghostPool = null;
  var ghostIdx = 0;
  var lastGhostTime = 0;
  var ghostSrc = new URL('../cursors/arrow.png', document.currentScript.src).href;
  var ghostRafId = 0;

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
        sides[c < 0 ? 0 : 1] += level < 0 ? ' ' : cloudRamp[Math.min(level, cloudRamp.length - 1)];
      }
      sides[0] += '\n';
      sides[1] += '\n';
    }
    clouds[0].textContent = sides[0];
    clouds[1].textContent = sides[1];
  }

  function makeGhostPool() {
    var pool = [];
    for (var i = 0; i < ghostCount; i++) {
      var img = document.createElement('img');
      img.src = ghostSrc;
      img.alt = '';
      img.setAttribute('aria-hidden', 'true');
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
    if (!trailsEnabled) return;
    var now = Date.now();
    if (now - lastGhostTime < ghostInterval) return;
    lastGhostTime = now;
    if (!ghostPool) ghostPool = makeGhostPool();
    var g = ghostPool[ghostIdx];
    ghostIdx = (ghostIdx + 1) % ghostCount;
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
      if (age > ghostLife) {
        g.active = false;
        g.el.style.opacity = '0';
        continue;
      }
      alive++;
      var t = age / ghostLife;
      var gx = (g.x + g.driftX * t).toFixed(1);
      var gy = (g.y + g.driftY * t).toFixed(1);
      var sc = (1 - t * 0.3).toFixed(3);
      g.el.style.opacity = (0.45 * (1 - t * t)).toFixed(3);
      g.el.style.transform =
        'translate3d(' + gx + 'px,' + gy + 'px,0) scale(' + sc + ')';
    }
    ghostRafId = alive ? requestAnimationFrame(updateGhosts) : 0;
  }

  function syncEffects() {
    var motion = document.body.dataset.motion;
    var animate = !document.hidden && (motion === 'running' || (motion !== 'paused' && !motionQuery.matches));
    var wide = innerWidth > 1320 && clouds.length === 2;
    trailsEnabled = animate && !coarsePointerQuery.matches && !document.body.classList.contains('low-fx');

    if (!trailsEnabled) {
      if (ghostRafId) cancelAnimationFrame(ghostRafId);
      ghostRafId = 0;
      if (ghostPool) ghostPool.forEach(function (g) {
        g.active = false;
        g.el.style.opacity = '0';
      });
    }

    if (staticOverlay) {
      staticOverlay.style.animationPlayState = document.hidden ? 'paused' : '';
      staticOverlay.style.backgroundImage = document.hidden ? "url('./assets/img/static-still.png')" : '';
    }

    if (!animate || !wide) {
      if (cloudTimer) clearInterval(cloudTimer);
      cloudTimer = 0;
    }
    if (!wide || document.hidden) return;

    var size = innerWidth + 'x' + innerHeight;
    if (cloudSize !== size) {
      drawClouds();
      cloudSize = size;
    }
    if (animate && !cloudTimer) cloudTimer = setInterval(drawClouds, 125);
  }

  motionQuery.addEventListener('change', syncEffects);
  coarsePointerQuery.addEventListener('change', syncEffects);
  document.addEventListener('visibilitychange', syncEffects);
  addEventListener('resize', syncEffects);
  new MutationObserver(syncEffects).observe(document.body, {
    attributes: true,
    attributeFilter: ['data-motion', 'class'],
  });
  syncEffects();
})();
