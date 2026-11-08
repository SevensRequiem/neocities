(function () {
  var ROOT = '../../';
  var RING = 'https://raw.githubusercontent.com/SevensRequiem/neocities/main/webring/';
  var K = 0.03;
  var FONT = 'Departure, ui-monospace, monospace';
  var SM = '11px ' + FONT;
  var BODY = '14px ' + FONT;
  var TITLE = '17px ' + FONT;
  var H2 = '16px ' + FONT;
  var BIG = '22px ' + FONT;
  var C = {
    ink: '#d7e6ff',
    dim: '#7d8bb0',
    faint: 'rgba(160,180,235,0.22)',
    vio: '#b48cff',
    teal: '#57cbde',
    pink: '#ff6b9d',
    panel: 'rgba(4,6,15,0.64)',
    card: 'rgba(120,140,220,0.035)',
    hot: 'rgba(180,140,255,0.11)'
  };
  var SECTIONS = ['weblog', 'memos', 'gallery', 'stories', 'webring', 'uplink', 'about'];
  var UPLINKS = [
    ['github', 'browse the code', 'https://github.com/SevensRequiem', 'github.png'],
    ['youtube', 'watch the channel', 'https://www.youtube.com/@SevensRequiem', 'youtube_small.png'],
    ['steam', 'view my profile', 'https://steamcommunity.com/id/SevensRequiem', 'steam.png'],
    ['x', 'follow the feed', 'https://x.com/SevensRequiem', 'x.png'],
    ['discord', 'join the server', 'https://discord.gg/smEZfvE9Mg', 'discord.png'],
    ['ko-fi', 'support my work', 'https://ko-fi.com/sevensrequiem', 'kofi.png'],
    ['rss', 'weblog + stories feed', ROOT + 'feed.xml'],
    ['archive', 'every weblog post', ROOT + 'blog/'],
    ['shrine', 'the shrine', ROOT + 'shrine.html'],
    ['surface', 'back to the classic theme', ROOT]
  ];

  var VS = 'attribute vec2 p;varying vec2 v;void main(){v=p*.5+.5;gl_Position=vec4(p,0.,1.);}';

  var BG = [
    'precision highp float;',
    'uniform vec2 res;uniform float t;uniform vec2 m;',
    'float h1(float x){return fract(sin(x*91.317)*43758.5453);}',
    'float h2(vec2 p){return fract(sin(dot(p,vec2(127.1,311.7)))*43758.5453);}',
    'float ns(vec2 p){vec2 i=floor(p),f=fract(p);f=f*f*(3.-2.*f);',
    '  return mix(mix(h2(i),h2(i+vec2(1,0)),f.x),mix(h2(i+vec2(0,1)),h2(i+vec2(1,1)),f.x),f.y);}',
    'float fbm(vec2 p){float s=0.,a=.5;for(int i=0;i<5;i++){s+=a*ns(p);p=p*2.02+vec2(3.1,1.7);a*=.5;}return s;}',
    'void main(){',
    '  vec2 p=(gl_FragCoord.xy-.5*res)/res.y;',
    '  float px=1./res.y;',
    '  float hz=-.16;',
    '  vec3 col=mix(vec3(.05,.035,.09),vec3(.006,.008,.02),smoothstep(hz-.05,.5,p.y));',
    '  col+=vec3(.12,.06,.17)*exp(-abs(p.y-hz)*9.)*.8;',
    '  vec2 q=p*vec2(1.6,3.)+vec2(t*.015,0.)+m*.03;',
    '  float w=fbm(q+fbm(q*1.7-vec2(0.,t*.02))*1.4);',
    '  col+=mix(vec3(.04,.12,.16),vec3(.14,.07,.2),w)*smoothstep(.35,.85,w)*.55;',
    '  vec2 hc=p-vec2(.55,.24)-m*.012;',
    '  float r=length(hc),a=atan(hc.y,hc.x);',
    '  float beat=.7+.3*sin(t*.7);',
    '  col+=vec3(.55,.45,1.)*exp(-abs(r-.17)*90.)*.22*beat;',
    '  col+=vec3(.35,.8,.95)*exp(-abs(r-.235)*160.)*.12;',
    '  col+=vec3(.6,.5,1.)*step(.8,fract(a*9.549+t*.02))*smoothstep(.007,0.,abs(r-.205))*.35;',
    '  col+=vec3(.25,.2,.45)*exp(-r*7.)*.25*beat;',
    '  col=mix(col,vec3(.008,.008,.018),smoothstep(hz,hz-.03,p.y)*.85);',
    '  float gx=p.x*90.,id=floor(gx);',
    '  float ly=abs(p.y-(hz-.006-h1(id+2.)*.012));',
    '  col+=vec3(1.,.72,.55)*.55*step(.9,h1(id))*smoothstep(.14,0.,abs(fract(gx)-.5))*smoothstep(px*1.5,0.,ly)*(.6+.4*sin(t*3.+id));',
    '  for(int i=0;i<2;i++){',
    '    float fi=float(i);',
    '    vec2 g=p*(14.+fi*10.)+vec2(t*(.3+fi*.2),-t*(.12+fi*.1))+m*(.4+fi*.6);',
    '    vec2 cid=floor(g),f=fract(g)-.5;',
    '    float rnd=h2(cid+fi*7.);',
    '    vec2 o=vec2(h2(cid+3.),h2(cid+5.))-.5;',
    '    float d=length(f-o*.7);',
    '    col+=vec3(.6,.75,1.)*smoothstep(.07,0.,d)*step(.86,rnd)*(.4+.6*sin(t*2.+rnd*40.))*(.5-fi*.2);',
    '  }',
    '  for(int i=0;i<4;i++){',
    '    float fl=float(3-i);',
    '    float s=1./(1.+fl*.9);',
    '    float fog=fl/4.;',
    '    vec2 q=(p-vec2(-m.x*.03*s,hz))/s;',
    '    q.x+=t*.06+fl*3.7;',
    '    float S=1.3,cell=floor(q.x/S),u=fract(q.x/S);',
    '    float lp=px/s;',
    '    float ta=.62+.1*h1(cell+fl*13.),tb=.62+.1*h1(cell+1.+fl*13.);',
    '    float top=u<.5?ta:tb;',
    '    float xd=min(u,1.-u)*S;',
    '    vec3 sil=mix(vec3(.006,.006,.014),vec3(.07,.045,.11),fog);',
    '    float pole=smoothstep(.012+lp,.012,xd)*step(q.y,top+.02);',
    '    float cb=step(xd,.08)*smoothstep(.007+lp,.007,abs(q.y-(top-.04)));',
    '    float ins=step(xd,.07)*step(.05,xd)*smoothstep(.014+lp,.014,abs(q.y-(top-.025)))*step(.5,fract(xd*28.));',
    '    col=mix(col,sil,max(max(pole,cb),ins));',
    '    for(int k=0;k<3;k++){',
    '      float fk=float(k);',
    '      float ya=ta-.04-fk*.03,yb=tb-.04-fk*.03;',
    '      float sag=.09+.03*fk+.01*sin(t*.6+cell*1.7+fl+fk);',
    '      float y=mix(ya,yb,u)-sag*4.*u*(1.-u);',
    '      float sl=(yb-ya-sag*4.*(1.-2.*u))/S;',
    '      float d=abs(q.y-y)/sqrt(1.+sl*sl);',
    '      col=mix(col,sil,smoothstep(lp*1.3,0.,d)*.9);',
    '      col+=vec3(.2,.25,.45)*exp(-d/lp)*.06*(1.-fog);',
    '      float ph=fract(q.x*.18-t*(.22+.09*fk)+h1(fk+fl*5.));',
    '      float pulse=exp(-pow((ph-.5)*38.,2.));',
    '      vec3 pc=mix(vec3(.35,.85,1.),vec3(.78,.55,1.),h1(fk*3.+fl));',
    '      col+=pc*pulse*(exp(-d/(lp*2.2))*1.4+exp(-d/(lp*14.))*.35)*(1.-fog*.6);',
    '    }',
    '  }',
    '  gl_FragColor=vec4(col,1.);',
    '}'
  ].join('\n');

  var POST = [
    'precision highp float;',
    'varying vec2 v;',
    'uniform sampler2D bg,ui;',
    'uniform vec2 res;uniform float t,k,dpr,glitch;',
    'vec2 warp(vec2 u){vec2 c=u-.5;return .5+c*(1.+k*dot(c,c));}',
    'vec3 comp(vec2 u){',
    '  vec4 f=texture2D(ui,u);',
    '  vec2 o=3.*dpr/res;',
    '  vec3 b=texture2D(bg,u).rgb;',
    '  vec3 bb=(texture2D(bg,u+vec2(o.x*2.,o.y)).rgb+texture2D(bg,u-vec2(o.x*2.,o.y)).rgb',
    '    +texture2D(bg,u+vec2(-o.x,o.y*2.)).rgb+texture2D(bg,u+vec2(o.x,-o.y*2.)).rgb)*.25;',
    '  b=mix(b,bb*.75,smoothstep(0.,.4,f.a));',
    '  return b*(1.-f.a)+f.rgb;',
    '}',
    'float hash(vec2 p){return fract(sin(dot(p,vec2(12.9898,78.233)))*43758.5453);}',
    'void main(){',
    '  vec2 u=warp(v);',
    '  float band=step(.997,hash(vec2(floor(v.y*48.),floor(t*7.))))*glitch;',
    '  u.x+=band*.004;',
    '  vec2 ca=(u-.5)*.004;',
    '  vec3 c=vec3(comp(u+ca).r,comp(u).g,comp(u-ca).b);',
    '  vec2 d=2.5*dpr/res;',
    '  c+=(texture2D(ui,u+d).rgb+texture2D(ui,u-d).rgb+texture2D(ui,u+vec2(d.x,-d.y)).rgb+texture2D(ui,u+vec2(-d.x,d.y)).rgb)*.08;',
    '  c*=step(0.,u.x)*step(u.x,1.)*step(0.,u.y)*step(u.y,1.);',
    '  c*=.93+.07*sin(gl_FragCoord.y/dpr*3.14159);',
    '  c+=.012*smoothstep(.96,1.,fract(v.y*.6-t*.04));',
    '  c+=(hash(gl_FragCoord.xy+fract(t)*91.)-.5)*.035;',
    '  c*=1.-.55*pow(length(v-.5)*1.25,2.5);',
    '  gl_FragColor=vec4(c,1.);',
    '}'
  ].join('\n');

  var canvas = document.getElementById('wire');
  var gl = canvas.getContext('webgl', { alpha: false, antialias: false, depth: false, stencil: false });
  if (!gl) {
    document.getElementById('nogl').className = 'plain shown';
    canvas.remove();
    return;
  }
  var ui = document.createElement('canvas');
  var g = ui.getContext('2d');

  var motion = matchMedia('(prefers-reduced-motion: reduce)').matches ? 0.2 : 1;
  var W = 0, H = 0, dpr = 1;
  var L = {};
  var D = {};
  var pending = {};
  var imgs = {};
  var items = [], contentH = 0, maxScroll = 0, hits = [];
  var S = { sec: 'weblog', open: null, scroll: 0, target: 0, hot: null, saved: {} };
  var rebuild = true, dirty = true;
  var GL = {};
  var bgScale = 0.5, tier = 0, lows = 0, fps = 60, frames = 0, fpsT = 0, lastSec = -1;
  var mouse = [0, 0], mouseT = [0, 0];

  function shader(type, src) {
    var s = gl.createShader(type);
    gl.shaderSource(s, src);
    gl.compileShader(s);
    if (!gl.getShaderParameter(s, gl.COMPILE_STATUS) && !gl.isContextLost()) throw new Error(gl.getShaderInfoLog(s));
    return s;
  }

  function program(fs) {
    var p = gl.createProgram();
    gl.attachShader(p, shader(gl.VERTEX_SHADER, VS));
    gl.attachShader(p, shader(gl.FRAGMENT_SHADER, fs));
    gl.bindAttribLocation(p, 0, 'p');
    gl.linkProgram(p);
    var u = {};
    for (var i = 0, n = gl.getProgramParameter(p, gl.ACTIVE_UNIFORMS); i < n; i++) {
      var name = gl.getActiveUniform(p, i).name;
      u[name] = gl.getUniformLocation(p, name);
    }
    return { p: p, u: u };
  }

  function texture() {
    var tex = gl.createTexture();
    gl.bindTexture(gl.TEXTURE_2D, tex);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
    return tex;
  }

  function initGL() {
    GL.bg = program(BG);
    GL.post = program(POST);
    gl.bindBuffer(gl.ARRAY_BUFFER, gl.createBuffer());
    gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1, -1, 3, -1, -1, 3]), gl.STATIC_DRAW);
    gl.enableVertexAttribArray(0);
    gl.vertexAttribPointer(0, 2, gl.FLOAT, false, 0, 0);
    GL.bgTex = texture();
    GL.uiTex = texture();
    GL.fbo = gl.createFramebuffer();
    gl.pixelStorei(gl.UNPACK_FLIP_Y_WEBGL, true);
    gl.pixelStorei(gl.UNPACK_PREMULTIPLY_ALPHA_WEBGL, true);
    sizeBg();
    rebuild = true;
  }

  function sizeBg() {
    GL.bw = Math.max(1, Math.round(W * dpr * bgScale));
    GL.bh = Math.max(1, Math.round(H * dpr * bgScale));
    gl.bindTexture(gl.TEXTURE_2D, GL.bgTex);
    gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, GL.bw, GL.bh, 0, gl.RGBA, gl.UNSIGNED_BYTE, null);
    gl.bindFramebuffer(gl.FRAMEBUFFER, GL.fbo);
    gl.framebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, GL.bgTex, 0);
    gl.bindFramebuffer(gl.FRAMEBUFFER, null);
  }

  function load(key, url) {
    if (pending[key]) return;
    pending[key] = true;
    fetch(url)
      .then(function (r) {
        if (!r.ok) throw new Error(r.status);
        return r.json();
      })
      .then(function (d) {
        D[key] = Array.isArray(d) ? d : null;
      }, function () {
        D[key] = null;
      })
      .then(function () {
        rebuild = true;
      });
  }

  function pic(src) {
    if (imgs[src]) return imgs[src];
    var im = new Image();
    im.crossOrigin = 'anonymous';
    im.onload = function () {
      rebuild = true;
    };
    im.onerror = function () {
      im.failed = true;
      dirty = true;
    };
    im.src = src;
    imgs[src] = im;
    return im;
  }

  function go(url) {
    if (url[0] === '#') {
      location.hash = url;
      return;
    }
    if (/^https?:\/\//i.test(url)) {
      window.open(url, '_blank', 'noopener,noreferrer');
      return;
    }
    if (/^mailto:/i.test(url) || !/^[a-z][a-z0-9+.-]*:/i.test(url)) location.href = url;
  }

  function str(x) {
    return x == null ? '' : String(x);
  }

  function pad3(n) {
    return ('00' + n).slice(-3);
  }

  function day(d) {
    return str(d).slice(0, 10).replace(/-/g, '.');
  }

  function stamp(d) {
    return day(d) + ' ' + str(d).slice(11, 16);
  }

  function tagline(tags) {
    return Array.isArray(tags) && tags.length ? ' · ' + tags.join(' / ').toLowerCase() : '';
  }

  function host(url) {
    var h = str(url).replace(/^https?:\/\//, '').replace(/\/.*$/, '');
    try {
      return decodeURIComponent(h);
    } catch (e) {
      return h;
    }
  }

  function wrap(text, font, w) {
    g.font = font;
    var lines = [];
    str(text).replace(/\r/g, '').split('\n').forEach(function (para) {
      var line = '';
      para.split(' ').forEach(function (word) {
        var next = line ? line + ' ' + word : word;
        if (g.measureText(next).width <= w) {
          line = next;
          return;
        }
        if (line) lines.push(line);
        line = word;
        while (line.length > 1 && g.measureText(line).width > w) {
          var cut = line.length - 1;
          while (cut > 1 && g.measureText(line.slice(0, cut)).width > w) cut--;
          lines.push(line.slice(0, cut));
          line = line.slice(cut);
        }
      });
      lines.push(line);
    });
    return lines;
  }

  function fit(text, font, w) {
    g.font = font;
    if (g.measureText(text).width <= w) return text;
    while (text.length > 1 && g.measureText(text + '…').width > w) text = text.slice(0, -1);
    return text + '…';
  }

  function rows(text, font, color, lh, w, x, mark) {
    return wrap(text, font, w - (x || 0)).map(function (t, i) {
      return { t: t, f: font, c: color, lh: lh, x: x || 0, mark: i ? null : mark };
    });
  }

  function drawRow(r, x, y, w) {
    if (!r.t && !r.mark) return;
    g.font = r.f;
    g.fillStyle = r.c;
    var base = y + r.lh * 0.72;
    if (r.mark) g.fillText(r.mark, x + r.x - 16, base);
    g.fillText(r.t, r.center ? x + (w - g.measureText(r.t).width) / 2 : x + r.x, base);
  }

  function height(rs) {
    return rs.reduce(function (a, r) {
      return a + r.lh;
    }, 0);
  }

  function add(h, paint, act) {
    items.push({ y: contentH, h: h, paint: paint, act: act });
    contentH += h;
  }

  function cells(h, list) {
    items.push({ y: contentH, h: h, cells: list });
    contentH += h;
  }

  function gap(h) {
    contentH += h;
  }

  function flow(rs) {
    rs.forEach(function (r) {
      add(r.lh, function (x, y, w) {
        drawRow(r, x, y, w);
      });
    });
  }

  function card(rs, act, accent) {
    var h = height(rs) + 22;
    add(h, function (x, y, w, hot) {
      g.fillStyle = hot ? C.hot : C.card;
      g.fillRect(x, y, w, h - 10);
      g.fillStyle = hot ? accent || C.vio : C.faint;
      g.fillRect(x, y, 2, h - 10);
      rs.forEach(function (r) {
        drawRow(r, x + 14, y + 6, w - 28);
        y += r.lh;
      });
    }, act);
  }

  function heading(title, tag) {
    add(46, function (x, y, w) {
      g.font = BIG;
      g.fillStyle = C.vio;
      g.fillText('::' + title.toUpperCase(), x, y + 24);
      if (tag) {
        g.font = SM;
        g.fillStyle = C.dim;
        g.fillText(tag, x + w - g.measureText(tag).width, y + 24);
      }
      g.fillStyle = C.faint;
      g.fillRect(x, y + 34, w, 1);
    });
  }

  function notice(text) {
    flow(rows(text, BODY, C.dim, 22, L.view.w));
  }

  function link(label, act, color) {
    add(26, function (x, y, w, hot) {
      g.font = BODY;
      g.fillStyle = hot ? C.pink : color || C.teal;
      g.fillText(fit(label, BODY, w), x, y + 17);
      if (hot) g.fillRect(x, y + 21, Math.min(g.measureText(label).width, w), 1);
    }, act);
  }

  function ready(d, path) {
    if (d === undefined) notice('receiving ' + path + ' ...');
    else if (d === null) notice('signal lost: could not load ' + path);
    else if (!d.length) notice('nothing on the wire here yet.');
    else return true;
    return false;
  }

  function parse(md) {
    var out = [], para = [], code = null, links = [];
    function inline(s) {
      return s
        .replace(/<[^>]+>/g, '')
        .replace(/!?\[([^\]]*)\]\(([^)\s]+)[^)]*\)/g, function (_, text, url) {
          links.push(url);
          return (text || 'link') + ' [' + links.length + ']';
        })
        .replace(/(^|\s)(https?:\/\/\S+)/g, function (_, sp, url) {
          links.push(url);
          return sp + host(url) + ' [' + links.length + ']';
        })
        .replace(/(\*\*|__|\*|`|~~)(\S(?:.*?\S)?)\1/g, '$2')
        .replace(/&amp;/g, '&').replace(/&lt;/g, '<').replace(/&gt;/g, '>').replace(/&quot;/g, '"').replace(/&#39;/g, "'");
    }
    function flush() {
      if (para.length) out.push({ k: 'p', t: inline(para.join(' ')) });
      para = [];
    }
    str(md).replace(/\r/g, '').split('\n').forEach(function (line) {
      var m;
      if (/^\s*```/.test(line)) {
        if (code) {
          out.push({ k: 'code', t: code.join('\n') });
          code = null;
        } else {
          flush();
          code = [];
        }
        return;
      }
      if (code) code.push(line);
      else if (!line.trim()) flush();
      else if ((m = /^#{1,6}\s+(.*)/.exec(line))) {
        flush();
        out.push({ k: 'h', t: inline(m[1]) });
      } else if (/^\s*([-*_])(\s*\1){2,}\s*$/.test(line)) {
        flush();
        out.push({ k: 'hr' });
      } else if ((m = /^>\s?(.*)/.exec(line))) {
        flush();
        out.push({ k: 'q', t: inline(m[1]) });
      } else if ((m = /^\s*(?:[-*+]|\d+\.)\s+(.*)/.exec(line))) {
        flush();
        out.push({ k: 'li', t: inline(m[1]) });
      } else para.push(line.trim());
    });
    if (code) out.push({ k: 'code', t: code.join('\n') });
    flush();
    return { blocks: out, links: links };
  }

  function blockRows(bs, w) {
    var out = [];
    bs.forEach(function (b, i) {
      if (i) out.push({ t: '', lh: b.k === 'li' && bs[i - 1].k === 'li' ? 3 : 12 });
      if (b.k === 'h') out = out.concat(rows(b.t, H2, C.vio, 24, w));
      else if (b.k === 'p') out = out.concat(rows(b.t, BODY, C.ink, 22, w));
      else if (b.k === 'q') out = out.concat(rows(b.t, BODY, C.teal, 22, w, 18, '›'));
      else if (b.k === 'li') out = out.concat(rows(b.t, BODY, C.ink, 22, w, 18, '◇'));
      else if (b.k === 'code') out = out.concat(rows(b.t, SM, C.teal, 17, w, 16));
      else out.push({ t: '◈ ◈ ◈', f: BODY, c: C.dim, lh: 26, x: 0, center: true });
    });
    return out;
  }

  function refs(links, indent) {
    links.forEach(function (u, i) {
      add(22, function (x, y, w, hot) {
        g.font = SM;
        g.fillStyle = hot ? C.pink : C.teal;
        g.fillText(fit('[' + (i + 1) + '] ' + u, SM, w - indent), x + indent, y + 15);
      }, u);
    });
  }

  function frameImg(im, x, y, w, h, cover, hot) {
    g.fillStyle = 'rgba(2,3,8,0.7)';
    g.fillRect(x, y, w, h);
    if (im && im.naturalWidth) {
      var s = (cover ? Math.max : Math.min)(w / im.naturalWidth, h / im.naturalHeight);
      var dw = im.naturalWidth * s, dh = im.naturalHeight * s;
      g.save();
      g.beginPath();
      g.rect(x, y, w, h);
      g.clip();
      g.drawImage(im, x + (w - dw) / 2, y + (h - dh) / 2, dw, dh);
      g.restore();
    } else {
      g.font = SM;
      g.fillStyle = C.dim;
      var label = im && im.failed ? 'no carrier' : 'receiving';
      g.fillText(label, x + (w - g.measureText(label).width) / 2, y + h / 2 + 4);
    }
    g.strokeStyle = hot ? C.vio : C.faint;
    g.lineWidth = 1;
    g.strokeRect(x + 0.5, y + 0.5, w - 1, h - 1);
  }

  function hero(src, maxH) {
    var im = pic(src), w = L.view.w;
    var h = im.naturalWidth ? Math.min(w * im.naturalHeight / im.naturalWidth, maxH) : Math.min(200, maxH);
    add(h + 18, function (x, y) {
      frameImg(im, x, y, w, h, false);
    });
  }

  function back(sec) {
    link('‹ back to ' + sec, '#/' + sec, C.dim);
    gap(8);
  }

  function pager(list, i, kind) {
    var newer = list[i - 1], older = list[i + 1], half = L.view.w / 2;
    gap(14);
    cells(28, [
      newer && { x: 0, w: half, act: '#/' + kind + '/' + newer.id, label: '‹ newer' },
      older && { x: half, w: half, act: '#/' + kind + '/' + older.id, label: 'older ›', right: true }
    ].filter(Boolean).map(function (c) {
      c.paint = function (x, y, hot) {
        g.font = BODY;
        g.fillStyle = hot ? C.pink : C.teal;
        g.fillText(c.label, c.right ? x + c.w - g.measureText(c.label).width : x, y + 18);
      };
      return c;
    }));
  }

  function find(list, id) {
    for (var i = 0; list && i < list.length; i++) if (list[i].id === id) return i;
    return -1;
  }

  var VIEWS = {
    weblog: function () {
      var d = D.posts;
      heading('weblog', d ? d.length + ' posts' : '');
      if (!ready(d, 'blog/index.json')) return;
      var w = L.view.w - 28;
      d.forEach(function (p) {
        card(rows('#' + pad3(p.id) + ' · ' + day(p.date) + tagline(p.tags), SM, C.dim, 18, w)
          .concat(rows(p.title, TITLE, C.ink, 24, w))
          .concat(rows(p.excerpt, BODY, C.dim, 20, w)), '#/post/' + p.id);
      });
      gap(6);
      link('download every post as json ↗', ROOT + 'blog/all.json');
    },

    memos: function () {
      var d = D.memos;
      heading('memos', d ? d.length + ' asides' : '');
      if (!ready(d, 'memos/index.json')) return;
      var w = L.view.w - 28;
      d.forEach(function (m) {
        var md = parse(m.body);
        card(rows('memo ' + pad3(m.id) + ' · ' + stamp(m.date), SM, C.teal, 18, w).concat(blockRows(md.blocks, w)), null, C.teal);
        if (md.links.length) {
          contentH -= 6;
          refs(md.links, 14);
          gap(10);
        }
      });
    },

    gallery: function () {
      var d = D.gallery;
      heading('gallery', d ? d.length + ' frames' : '');
      if (!ready(d, 'gallery/index.json')) return;
      var n = Math.max(2, Math.floor((L.view.w + 12) / 172)), cw = (L.view.w - (n - 1) * 12) / n;
      for (var i = 0; i < d.length; i += n) {
        cells(cw + 38, d.slice(i, i + n).map(function (f, j) {
          var im = pic(ROOT + 'gallery/' + f.dir + '/' + f.thumb);
          return {
            x: j * (cw + 12), w: cw, act: '#/frame/' + f.id,
            paint: function (x, y, hot) {
              frameImg(im, x, y, cw, cw, true, hot);
              g.font = SM;
              g.fillStyle = hot ? C.ink : C.dim;
              g.fillText(fit(day(f.date) + ' ' + str(f.caption), SM, cw), x, y + cw + 16);
            }
          };
        }));
      }
    },

    stories: function () {
      var d = D.stories;
      heading('stories', d ? d.length + ' told' : '');
      if (!ready(d, 'stories/index.json')) return;
      var w = L.view.w - 28;
      d.forEach(function (s) {
        card(rows('::STORY.' + pad3(s.id) + ' · ' + day(s.date) + ' · ' + (s.words || 0) + ' words' + tagline(s.tags), SM, C.dim, 18, w)
          .concat(rows(s.title, TITLE, C.ink, 24, w))
          .concat(rows(s.summary, BODY, C.dim, 20, w)), '#/story/' + s.id, C.pink);
      });
    },

    webring: function () {
      var d = D.ring;
      heading('webring', d ? pad3(d.length).slice(1) + ' linked' : '');
      if (!ready(d, 'webring.json')) return;
      var n = Math.max(1, Math.floor((L.view.w + 14) / 214)), cw = (L.view.w - (n - 1) * 14) / n, bh = Math.round(cw * 31 / 88);
      d = d.filter(function (s) {
        return /^https?:\/\//.test(str(s.url));
      });
      for (var i = 0; i < d.length; i += n) {
        cells(bh + 34, d.slice(i, i + n).map(function (s, j) {
          var b = str(s.banner);
          var im = /^https:\/\//.test(b) ? pic(b) : pic(RING + b);
          return {
            x: j * (cw + 14), w: cw, act: s.url,
            paint: function (x, y, hot) {
              frameImg(im, x, y, cw, bh, false, hot);
              g.font = SM;
              g.fillStyle = hot ? C.pink : C.dim;
              g.fillText(fit(host(s.url) + ' ↗', SM, cw), x, y + bh + 16);
            }
          };
        }));
      }
      gap(8);
      link('add yours ↗', 'https://github.com/SevensRequiem/neocities#webring');
    },

    uplink: function () {
      heading('uplink', 'off-node');
      UPLINKS.forEach(function (u) {
        var icon = u[3] && pic(ROOT + 'assets/icons/' + u[3]);
        add(46, function (x, y, w, hot) {
          g.fillStyle = hot ? C.hot : C.card;
          g.fillRect(x, y, w, 38);
          g.fillStyle = hot ? C.vio : C.faint;
          g.fillRect(x, y, 2, 38);
          if (icon && icon.naturalWidth) {
            g.imageSmoothingEnabled = false;
            g.drawImage(icon, x + 14, y + 7, 24, 24);
            g.imageSmoothingEnabled = true;
          } else {
            g.font = BODY;
            g.fillStyle = C.teal;
            g.fillText('◈', x + 20, y + 24);
          }
          g.font = BODY;
          g.fillStyle = hot ? C.ink : C.vio;
          g.fillText(u[0], x + 50, y + 18);
          g.font = SM;
          g.fillStyle = C.dim;
          g.fillText(fit(u[1] + (/^https?:/.test(u[2]) ? ' ↗' : ''), SM, w - 64), x + 50, y + 32);
        }, u[2]);
      });
    },

    about: function () {
      var w = L.view.w;
      heading('about', 'self-taught dev');
      flow(rows("I'm just a self-taught dev who loves the soul of the machine. I'm really focusing on my projects & healing.", BODY, C.ink, 22, w));
      gap(14);
      [['handle', 'requiem', C.ink], ['status', 'growing', C.teal], ['theme', 'aether :: pure webgl', C.ink]].forEach(function (r) {
        add(24, function (x, y) {
          g.font = SM;
          g.fillStyle = C.dim;
          g.fillText(r[0], x, y + 16);
          g.font = BODY;
          g.fillStyle = r[2];
          g.fillText(r[1], x + 90, y + 16);
        });
      });
      add(24, function (x, y, w2, hot) {
        g.font = SM;
        g.fillStyle = C.dim;
        g.fillText('contact', x, y + 16);
        g.font = BODY;
        g.fillStyle = hot ? C.pink : C.vio;
        g.fillText('admin@requiem.moe', x + 90, y + 16);
      }, 'mailto:admin@requiem.moe');
      gap(18);
      flow(rows('keys :: 1-7 sections · j / k or arrows scroll · space / shift+space page · esc back', SM, C.dim, 18, w));
      gap(10);
      link('ai directed dev :: <2506.14376> ↗', 'https://arxiv.org/abs/2506.14376');
    }
  };

  var READERS = {
    post: function (id) {
      back('weblog');
      if (!ready(D.posts, 'blog/index.json')) return;
      var i = find(D.posts, id);
      if (i < 0) return notice('no post #' + id + ' on the wire.');
      var p = D.posts[i], w = L.view.w;
      load('full', ROOT + 'blog/all.json');
      flow(rows('#' + pad3(p.id) + ' · ' + day(p.date) + tagline(p.tags), SM, C.dim, 18, w));
      flow(rows(p.title, BIG, C.ink, 30, w));
      gap(10);
      if (p.image) hero(ROOT + 'blog/' + p.dir + '/' + p.image, 460);
      var fi = find(D.full, id);
      if (fi < 0) {
        if (D.full === undefined) notice('receiving blog/all.json ...');
        else flow(rows(p.excerpt, BODY, C.ink, 22, w));
      } else {
        var full = D.full[fi];
        if (full.quote) {
          flow(rows(full.quote, BODY, C.teal, 22, w, 18, '›'));
          gap(12);
        }
        var md = parse(full.body);
        flow(blockRows(md.blocks, w));
        if (md.links.length) {
          gap(14);
          refs(md.links, 0);
        }
      }
      gap(16);
      link('open on the surface ↗', ROOT + 'blog/' + p.dir + '/');
      pager(D.posts, i, 'post');
    },

    story: function (id) {
      back('stories');
      if (!ready(D.stories, 'stories/index.json')) return;
      var i = find(D.stories, id);
      if (i < 0) return notice('no story ' + id + ' on the wire.');
      var s = D.stories[i], w = L.view.w;
      load('storiesFull', ROOT + 'stories/all.json');
      flow(rows('::STORY.' + pad3(s.id) + ' · ' + day(s.date) + ' · ' + (s.words || 0) + ' words · ' + Math.max(1, Math.round((s.words || 0) / 230)) + ' min', SM, C.dim, 18, w));
      flow(rows(s.title, BIG, C.ink, 30, w));
      gap(10);
      if (s.image) hero(ROOT + 'stories/' + s.dir + '/' + s.image, 420);
      var fi = find(D.storiesFull, id);
      if (fi < 0) notice(D.storiesFull === undefined ? 'receiving stories/all.json ...' : str(s.summary));
      else flow(blockRows(parse(D.storiesFull[fi].body).blocks, w));
      gap(16);
      link('open on the surface ↗', ROOT + 'stories/' + s.dir + '/');
      pager(D.stories, i, 'story');
    },

    frame: function (id) {
      back('gallery');
      if (!ready(D.gallery, 'gallery/index.json')) return;
      var i = find(D.gallery, id);
      if (i < 0) return notice('no frame ' + id + ' on the wire.');
      var f = D.gallery[i], src = ROOT + 'gallery/' + f.dir + '/' + f.image;
      hero(src, Math.max(220, L.view.h - 150));
      flow(rows(f.caption, BODY, C.ink, 22, L.view.w));
      flow(rows(day(f.date) + tagline(f.tags), SM, C.dim, 18, L.view.w));
      gap(10);
      link('open full image ↗', src);
      pager(D.gallery, i, 'frame');
    }
  };

  function navLabel(k, i) {
    if (k === 'surface') return L.wide ? '    surface ↗' : 'surface ↗';
    return L.wide ? '0' + (i + 1) + '  ' + k : k;
  }

  function layout() {
    W = innerWidth;
    H = innerHeight;
    dpr = Math.min(devicePixelRatio || 1, 2);
    var m = W < 600 ? 16 : 28;
    L.wide = W >= 860;
    L.head = { x: m, y: m, w: W - 2 * m, h: L.wide ? 60 : 46 };
    L.foot = { x: m, y: H - m - 22, w: W - 2 * m, h: 22 };
    var top = L.head.y + L.head.h + 10;
    var keys = SECTIONS.concat('surface');
    L.nav = [];
    if (L.wide) {
      keys.forEach(function (k, i) {
        L.nav.push({ x: m, y: top + i * 36 + (k === 'surface' ? 14 : 0), w: 176, h: 30, k: k, label: navLabel(k, i) });
      });
      var x = m + 176 + 22;
      L.panel = { x: x, y: top, w: Math.min(W - x - m, 880), h: L.foot.y - 12 - top };
    } else {
      g.font = BODY;
      var nx = m, ny = top;
      keys.forEach(function (k, i) {
        var label = navLabel(k, i), w = g.measureText(label).width + 20;
        if (nx + w > W - m && nx > m) {
          nx = m;
          ny += 34;
        }
        L.nav.push({ x: nx, y: ny, w: w, h: 28, k: k, label: label });
        nx += w + 6;
      });
      top = ny + 28 + 10;
      L.panel = { x: m, y: top, w: W - 2 * m, h: L.foot.y - 10 - top };
    }
    var pad = L.wide ? 22 : 14;
    L.view = { x: L.panel.x + pad, y: L.panel.y + pad, w: L.panel.w - 2 * pad - 6, h: L.panel.h - 2 * pad };
    var cw = Math.round(W * dpr), ch = Math.round(H * dpr);
    if (ui.width !== cw || ui.height !== ch) {
      ui.width = canvas.width = cw;
      ui.height = canvas.height = ch;
      if (GL.fbo) sizeBg();
    }
  }

  function clamp(y) {
    return Math.max(0, Math.min(maxScroll, y));
  }

  function build() {
    rebuild = false;
    dirty = true;
    layout();
    items = [];
    contentH = 0;
    if (S.open) READERS[S.open.kind](S.open.id);
    else VIEWS[S.sec]();
    gap(12);
    maxScroll = Math.max(0, contentH - L.view.h);
    S.target = clamp(S.target);
    S.scroll = clamp(S.scroll);
  }

  function corners(r, color) {
    g.fillStyle = color;
    [[r.x, r.y, 1, 1], [r.x + r.w, r.y, -1, 1], [r.x, r.y + r.h, 1, -1], [r.x + r.w, r.y + r.h, -1, -1]].forEach(function (c) {
      g.fillRect(c[0], c[1], 12 * c[2], 1 * c[3]);
      g.fillRect(c[0], c[1], 1 * c[2], 12 * c[3]);
    });
  }

  function hit(x, y, w, h, key, act) {
    var y0 = Math.max(y, L.view.y), y1 = Math.min(y + h, L.view.y + L.view.h);
    if (y1 > y0) hits.push({ x: x, y: y0, w: w, h: y1 - y0, key: key, act: act });
  }

  function paint() {
    dirty = false;
    hits = [];
    g.setTransform(dpr, 0, 0, dpr, 0, 0);
    g.clearRect(0, 0, W, H);
    g.textBaseline = 'alphabetic';

    var hd = L.head;
    g.font = (L.wide ? '38px' : '28px') + ' Esoterik, ' + FONT;
    g.fillStyle = C.ink;
    g.fillText('::requiem', hd.x, hd.y + hd.h - 16);
    var tw = g.measureText('::requiem').width;
    g.font = SM;
    var sub = 'aether relay · present on the wire';
    if (hd.x + tw + 24 + g.measureText(sub).width < hd.x + hd.w - (L.wide ? 170 : 0)) {
      g.fillStyle = C.dim;
      g.fillText(sub, hd.x + tw + 18, hd.y + hd.h - 18);
    }
    if (L.wide) {
      var bars = Math.max(1, Math.min(5, Math.round(fps / 12)));
      var sig = 'signal ' + '◆◆◆◆◆'.slice(0, bars) + '◇◇◇◇◇'.slice(bars) + '  linked';
      g.fillStyle = C.teal;
      g.fillText(sig, hd.x + hd.w - g.measureText(sig).width, hd.y + hd.h - 18);
    }
    g.fillStyle = C.faint;
    g.fillRect(hd.x, hd.y + hd.h - 4, hd.w, 1);

    L.nav.forEach(function (b) {
      var key = 'n' + b.k;
      var active = b.k === S.sec && b.k !== 'surface', hot = S.hot === key;
      hits.push({ x: b.x, y: b.y, w: b.w, h: b.h, key: key, act: b.k === 'surface' ? ROOT : '#/' + b.k });
      g.fillStyle = active ? 'rgba(180,140,255,0.16)' : hot ? C.hot : 'rgba(4,6,15,0.5)';
      g.fillRect(b.x, b.y, b.w, b.h);
      if (active) corners(b, C.vio);
      else {
        g.strokeStyle = C.faint;
        g.lineWidth = 1;
        g.strokeRect(b.x + 0.5, b.y + 0.5, b.w - 1, b.h - 1);
      }
      g.font = BODY;
      g.fillStyle = active ? C.ink : hot ? C.vio : b.k === 'surface' ? C.teal : C.dim;
      g.fillText(b.label, b.x + 10, b.y + b.h / 2 + 5);
    });

    var p = L.panel, v = L.view;
    g.fillStyle = C.panel;
    g.fillRect(p.x, p.y, p.w, p.h);
    g.strokeStyle = C.faint;
    g.lineWidth = 1;
    g.strokeRect(p.x + 0.5, p.y + 0.5, p.w - 1, p.h - 1);
    corners(p, C.vio);

    g.save();
    g.beginPath();
    g.rect(v.x - 4, v.y, v.w + 8, v.h);
    g.clip();
    var y0 = v.y - S.scroll;
    items.forEach(function (it, i) {
      var y = y0 + it.y;
      if (y + it.h < v.y || y > v.y + v.h) return;
      if (it.cells) {
        it.cells.forEach(function (c, j) {
          var key = 'c' + i + ':' + j;
          hit(v.x + c.x, y, c.w, it.h, key, c.act);
          c.paint(v.x + c.x, y, S.hot === key);
        });
        return;
      }
      var key = 'c' + i;
      if (it.act) hit(v.x, y, v.w, it.h, key, it.act);
      it.paint(v.x, y, v.w, S.hot === key);
    });
    g.globalCompositeOperation = 'destination-out';
    var fade = 18;
    var gt = g.createLinearGradient(0, v.y, 0, v.y + fade);
    gt.addColorStop(0, 'rgba(0,0,0,1)');
    gt.addColorStop(1, 'rgba(0,0,0,0)');
    if (S.scroll > 1) {
      g.fillStyle = gt;
      g.fillRect(v.x - 4, v.y, v.w + 8, fade);
    }
    var gb = g.createLinearGradient(0, v.y + v.h - fade, 0, v.y + v.h);
    gb.addColorStop(0, 'rgba(0,0,0,0)');
    gb.addColorStop(1, 'rgba(0,0,0,1)');
    if (S.scroll < maxScroll - 1) {
      g.fillStyle = gb;
      g.fillRect(v.x - 4, v.y + v.h - fade, v.w + 8, fade);
    }
    g.restore();

    if (maxScroll > 0) {
      var tx = p.x + p.w - 8, th = Math.max(24, v.h * v.h / contentH);
      g.fillStyle = C.faint;
      g.fillRect(tx, v.y, 1, v.h);
      g.fillStyle = C.vio;
      g.fillRect(tx - 1, v.y + (v.h - th) * (S.scroll / maxScroll), 3, th);
    }

    var f = L.foot, now = new Date();
    var clock = now.getFullYear() + '.' + ('0' + (now.getMonth() + 1)).slice(-2) + '.' + ('0' + now.getDate()).slice(-2) + ' ' + now.toTimeString().slice(0, 8);
    var path = '/aether/' + (S.open ? S.open.kind + '/' + S.open.id : S.sec);
    g.font = SM;
    g.fillStyle = C.faint;
    g.fillRect(f.x, f.y, f.w, 1);
    g.fillStyle = C.dim;
    g.fillText(L.wide ? Math.round(fps) + ' fps · bg ' + bgScale.toFixed(2) + 'x' : path, f.x, f.y + 16);
    if (L.wide) g.fillText(path, f.x + (f.w - g.measureText(path).width) / 2, f.y + 16);
    g.fillStyle = C.teal;
    g.fillText(clock, f.x + f.w - g.measureText(clock).width, f.y + 16);

    gl.bindTexture(gl.TEXTURE_2D, GL.uiTex);
    gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, gl.RGBA, gl.UNSIGNED_BYTE, ui);
  }

  function route() {
    var parts = location.hash.replace(/^#\/?/, '').split('/');
    var kind = parts[0], id = parseInt(parts[1], 10);
    var parent = { post: 'weblog', story: 'stories', frame: 'gallery' }[kind];
    if (!S.open) S.saved[S.sec] = S.target;
    if (parent && id >= 0) {
      S.sec = parent;
      S.open = { kind: kind, id: id };
      S.scroll = S.target = 0;
    } else {
      S.sec = SECTIONS.indexOf(kind) >= 0 ? kind : 'weblog';
      S.open = null;
      S.scroll = S.target = S.saved[S.sec] || 0;
    }
    S.hot = null;
    rebuild = true;
  }

  function scrollBy(dy) {
    S.target = clamp(S.target + dy);
  }

  function pointer(e) {
    var u = e.clientX / W - 0.5, v = e.clientY / H - 0.5, s = 1 + K * (u * u + v * v);
    var x = (0.5 + u * s) * W, y = (0.5 + v * s) * H;
    for (var i = hits.length - 1; i >= 0; i--) {
      var h = hits[i];
      if (x >= h.x && x < h.x + h.w && y >= h.y && y < h.y + h.h) return h;
    }
    return null;
  }

  canvas.addEventListener('pointermove', function (e) {
    mouseT = [e.clientX / W * 2 - 1, 1 - e.clientY / H * 2];
    var h = pointer(e), key = h ? h.key : null;
    if (key !== S.hot) {
      S.hot = key;
      dirty = true;
      canvas.style.cursor = h ? 'pointer' : 'default';
    }
  });

  canvas.addEventListener('click', function (e) {
    var h = pointer(e);
    if (h) go(h.act);
  });

  canvas.addEventListener('wheel', function (e) {
    e.preventDefault();
    scrollBy(e.deltaY * (e.deltaMode === 1 ? 18 : e.deltaMode === 2 ? L.view.h : 1));
  }, { passive: false });

  var touchY = null, touchV = 0;
  canvas.addEventListener('touchstart', function (e) {
    touchY = e.touches[0].clientY;
    touchV = 0;
  }, { passive: true });
  canvas.addEventListener('touchmove', function (e) {
    var y = e.touches[0].clientY;
    touchV = touchY - y;
    S.scroll = S.target = clamp(S.target + touchV);
    touchY = y;
    dirty = true;
    e.preventDefault();
  }, { passive: false });
  canvas.addEventListener('touchend', function () {
    scrollBy(touchV * 14);
  });

  addEventListener('keydown', function (e) {
    if (e.ctrlKey || e.metaKey || e.altKey) return;
    var k = e.key, page = L.view.h - 60;
    if (k === 'ArrowDown' || k === 'j') scrollBy(70);
    else if (k === 'ArrowUp' || k === 'k') scrollBy(-70);
    else if (k === 'PageDown' || (k === ' ' && !e.shiftKey)) scrollBy(page);
    else if (k === 'PageUp' || (k === ' ' && e.shiftKey)) scrollBy(-page);
    else if (k === 'Home') S.target = 0;
    else if (k === 'End') S.target = maxScroll;
    else if ((k === 'Escape' || k === 'Backspace') && S.open) go('#/' + S.sec);
    else if (k >= '1' && k <= '7') go('#/' + SECTIONS[k - 1]);
    else return;
    e.preventDefault();
  });

  addEventListener('hashchange', route);
  addEventListener('resize', function () {
    rebuild = true;
  });

  canvas.addEventListener('webglcontextlost', function (e) {
    e.preventDefault();
  });
  canvas.addEventListener('webglcontextrestored', function () {
    initGL();
  });

  function frame(now) {
    requestAnimationFrame(frame);
    if (gl.isContextLost()) return;

    frames++;
    if (!fpsT) fpsT = now;
    if (now - fpsT >= 2000) {
      fps = frames * 1000 / (now - fpsT);
      frames = 0;
      fpsT = now;
      lows = fps < 40 ? lows + 1 : 0;
      if (lows >= 2 && tier < 2) {
        tier++;
        lows = 0;
        bgScale = [0.5, 0.35, 0.25][tier];
        sizeBg();
      }
    }

    var diff = S.target - S.scroll;
    if (diff) {
      S.scroll = Math.abs(diff) < 0.5 ? S.target : S.scroll + diff * 0.22;
      dirty = true;
    }
    var sec = Math.floor(now / 1000);
    if (sec !== lastSec) {
      lastSec = sec;
      dirty = true;
    }
    if (rebuild) build();
    if (dirty) paint();

    mouse[0] += (mouseT[0] - mouse[0]) * 0.04;
    mouse[1] += (mouseT[1] - mouse[1]) * 0.04;
    var t = now / 1000 * motion;

    gl.bindFramebuffer(gl.FRAMEBUFFER, GL.fbo);
    gl.viewport(0, 0, GL.bw, GL.bh);
    gl.useProgram(GL.bg.p);
    gl.uniform2f(GL.bg.u.res, GL.bw, GL.bh);
    gl.uniform1f(GL.bg.u.t, t);
    gl.uniform2f(GL.bg.u.m, mouse[0], mouse[1]);
    gl.drawArrays(gl.TRIANGLES, 0, 3);

    gl.bindFramebuffer(gl.FRAMEBUFFER, null);
    gl.viewport(0, 0, canvas.width, canvas.height);
    gl.useProgram(GL.post.p);
    gl.activeTexture(gl.TEXTURE0);
    gl.bindTexture(gl.TEXTURE_2D, GL.bgTex);
    gl.activeTexture(gl.TEXTURE1);
    gl.bindTexture(gl.TEXTURE_2D, GL.uiTex);
    gl.activeTexture(gl.TEXTURE0);
    gl.uniform1i(GL.post.u.bg, 0);
    gl.uniform1i(GL.post.u.ui, 1);
    gl.uniform2f(GL.post.u.res, canvas.width, canvas.height);
    gl.uniform1f(GL.post.u.t, t);
    gl.uniform1f(GL.post.u.k, K);
    gl.uniform1f(GL.post.u.dpr, dpr);
    gl.uniform1f(GL.post.u.glitch, motion < 1 ? 0 : 1);
    gl.drawArrays(gl.TRIANGLES, 0, 3);
  }

  [['Departure', 'assets/fonts/DepartureMono-Regular.woff2'], ['Esoterik', 'assets/fonts/Esoterik-pLDr.ttf']].forEach(function (f) {
    new FontFace(f[0], 'url(' + ROOT + f[1] + ')').load().then(function (face) {
      document.fonts.add(face);
      rebuild = true;
    }, function () {});
  });

  load('posts', ROOT + 'blog/index.json');
  load('memos', ROOT + 'memos/index.json');
  load('gallery', ROOT + 'gallery/index.json');
  load('stories', ROOT + 'stories/index.json');
  load('ring', RING + 'webring.json');

  layout();
  initGL();
  route();
  requestAnimationFrame(frame);
})();
