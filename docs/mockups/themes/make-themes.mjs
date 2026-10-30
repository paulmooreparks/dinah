// Builds candidate Dinah themes from published palettes, then nudges the
// lightness of any colour that fails Dinah's own contrast rules (copied from
// cmd/dinah-desktop/frontend/appearance.js, contrast()) until it passes.
// Usage: node make-themes.mjs <out.js>
import { writeFileSync } from 'node:fs';

const rgb = v => [1, 3, 5].map(i => parseInt(v.slice(i, i + 2), 16) / 255);
const hex = c => '#' + c.map(v => Math.round(Math.min(1, Math.max(0, v)) * 255).toString(16).padStart(2, '0')).join('');
const lum = c => c.reduce((s, n, i) => s + (n <= .04045 ? n / 12.92 : ((n + .055) / 1.055) ** 2.4) * [.2126, .7152, .0722][i], 0);
const ratio = (a, b) => { const x = lum(a), y = lum(b); return (Math.max(x, y) + .05) / (Math.min(x, y) + .05); };
// HSL for nudging lightness while keeping the hue.
function toHsl([r, g, b]) { const mx = Math.max(r, g, b), mn = Math.min(r, g, b), l = (mx + mn) / 2; let h = 0, s = 0; if (mx !== mn) { const d = mx - mn; s = l > .5 ? d / (2 - mx - mn) : d / (mx + mn); h = mx === r ? (g - b) / d + (g < b ? 6 : 0) : mx === g ? (b - r) / d + 2 : (r - g) / d + 4; h /= 6; } return [h, s, l]; }
function fromHsl([h, s, l]) { if (!s) return [l, l, l]; const q = l < .5 ? l * (1 + s) : l + s - l * s, p = 2 * l - q; const f = t => { t = (t + 1) % 1; return t < 1 / 6 ? p + (q - p) * 6 * t : t < .5 ? q : t < 2 / 3 ? p + (q - p) * (2 / 3 - t) * 6 : p; }; return [f(h + 1 / 3), f(h), f(h - 1 / 3)]; }
const nudge = (v, dl) => { const h = toHsl(rgb(v)); h[2] = Math.min(1, Math.max(0, h[2] + dl)); return hex(fromHsl(h)); };

const TEXT = ['text', 'text-muted', 'accent', 'warn', 'danger', 'positive', 'dinah-skill', 'dinah-title'];
function failures(p) {
  const out = [], lit = parseFloat(p.lit) / 100;
  const input = rgb(p.light).map((v, i) => v * lit + rgb(p.surface)[i] * (1 - lit));
  const bgs = [rgb(p.bg), rgb(p.surface), rgb(p['surface-alt']), input];
  Object.keys(p).filter(k => TEXT.includes(k) || k.startsWith('syntax-')).forEach(k => { if (bgs.some(bg => ratio(rgb(p[k]), bg) < 4.5)) out.push(k); });
  if (ratio(rgb(p['on-accent']), rgb(p.accent)) < 4.5) out.push('on-accent/accent');
  if (ratio(rgb(p['on-warn']), rgb(p.warn)) < 4.5) out.push('on-warn/warn');
  if (ratio(rgb(p.border), input) < 3) out.push('border');
  const ib = rgb(p.text).map((v, i) => v * .55 + rgb(p.surface)[i] * .45);
  if (ratio(ib, input) < 3) out.push('derived-input-border');
  if (ratio(rgb(p['on-accent']), rgb(p['accent-hover'])) < 4.5) out.push('accent-hover');
  const d = rgb(p['danger-hover']);
  if (ratio(rgb(p['on-accent']), d) < 4.5 || ratio(rgb(p['on-accent']), d.map(v => v * .78 + .22)) < 4.5) out.push('danger-hover');
  if (ratio(rgb(p['tb-fg']), rgb(p['tb-bg'])) < 4.5) out.push('tb-fg');
  return out;
}
// Moves each failing colour away from its backgrounds until the palette
// passes; light mode darkens, dark mode lightens. Reports what moved.
function settle(p, mode) {
  const step = mode === 'light' ? -0.01 : 0.01, moved = {};
  for (let i = 0; i < 200; i++) {
    const f = failures(p);
    if (!f.length) return moved;
    for (const k of f) {
      const key = k === 'on-accent/accent' || k === 'accent-hover' ? (k === 'accent-hover' ? 'accent-hover' : 'accent')
        : k === 'on-warn/warn' ? 'warn' : k === 'border' ? 'border' : k === 'derived-input-border' ? 'text' : k === 'tb-fg' ? 'tb-fg' : k;
      if (!(key in moved)) moved[key] = p[key];
      // The accent and danger shades carry light text in light mode and dark
      // text in dark mode, so they move with the text colours. The roles
      // filled from the body text move with it, so a theme keeps one text
      // colour.
      if (key === 'text') ['tb-fg', 'dinah-title', 'syntax-heading'].forEach(k => { if (p[k] === p.text) p[k] = nudge(p[k], step); });
      p[key] = nudge(p[key], step);
    }
  }
  throw new Error(mode + ' palette does not settle: ' + failures(p).join(', '));
}

// Fills the syntax and title roles from a palette's named hues. A palette
// whose published syntax colours differ from its status hues gives them as
// keyword, string and number.
function palette(base, hue, mode) {
  return {
    bg: base.bg, surface: base.surface, 'surface-alt': base.alt, text: base.text, 'text-muted': base.muted, border: base.border,
    accent: hue.accent, 'accent-hover': hue.accentHover, 'on-accent': base.onAccent, warn: hue.warn, 'on-warn': base.onAccent,
    danger: hue.danger, 'danger-hover': hue.dangerHover, positive: hue.positive, light: base.light, shade: base.shade,
    lit: mode === 'light' ? '42%' : '8%', depth: mode === 'light' ? '17%' : '55%', 'tb-bg': base.alt, 'tb-fg': base.text,
    'dinah-skill': hue.magenta, 'dinah-title': base.text,
    'syntax-keyword': hue.keyword || hue.magenta, 'syntax-string': hue.string || hue.positive, 'syntax-number': hue.number || hue.warn, 'syntax-comment': base.muted,
    'syntax-name': hue.accent, 'syntax-tag': hue.violet, 'syntax-attr': hue.warn, 'syntax-heading': base.text,
    'syntax-link': hue.accent, 'syntax-error': hue.danger
  };
}
const fonts = { interface: { families: ['Inter'], fallback: 'system-ui' }, heading: { families: [], fallback: 'inherit' }, reading: { families: [], fallback: 'inherit' }, monospace: { families: ['Cascadia Code', 'Consolas'], fallback: 'monospace' } };

const S = { base03: '#002b36', base02: '#073642', base01: '#586e75', base00: '#657b83', base0: '#839496', base1: '#93a1a1', base2: '#eee8d5', base3: '#fdf6e3',
  yellow: '#b58900', orange: '#cb4b16', red: '#dc322f', magenta: '#d33682', violet: '#6c71c4', blue: '#268bd2', cyan: '#2aa198', green: '#859900' };
const N = { n0: '#2e3440', n1: '#3b4252', n2: '#434c5e', n3: '#4c566a', s0: '#d8dee9', s1: '#e5e9f0', s2: '#eceff4', f0: '#8fbcbb', f1: '#88c0d0', f2: '#81a1c1', f3: '#5e81ac',
  red: '#bf616a', orange: '#d08770', yellow: '#ebcb8b', green: '#a3be8c', purple: '#b48ead' };
const G = { bg0h: '#1d2021', bg0: '#282828', bg1: '#3c3836', bg2: '#504945', fg: '#ebdbb2', fg4: '#a89984', lbg0: '#fbf1c7', lbg1: '#ebdbb2', lbg2: '#d5c4a1', lfg: '#3c3836', lfg4: '#7c6f64',
  red: '#cc241d', green: '#98971a', yellow: '#d79921', blue: '#458588', purple: '#b16286', aqua: '#689d6a', orange: '#d65d0e',
  dred: '#9d0006', dgreen: '#79740e', dyellow: '#b57614', dblue: '#076678', dpurple: '#8f3f71', daqua: '#427b58',
  bred: '#fb4934', bgreen: '#b8bb26', byellow: '#fabd2f', bblue: '#83a598', bpurple: '#d3869b', baqua: '#8ec07c' };

const specs = {
  solarized: { name: 'Solarized', source: 'Ethan Schoonover\'s Solarized (MIT)',
    light: palette({ bg: S.base2, surface: S.base3, alt: '#e8e1cc', text: S.base02, muted: S.base01, border: S.base00, onAccent: S.base3, light: '#fffdf6', shade: S.base03 },
      { accent: S.blue, accentHover: '#1f72ad', warn: S.yellow, danger: S.red, dangerHover: '#a9221f', positive: S.green, magenta: S.magenta, violet: S.violet }, 'light'),
    dark: palette({ bg: '#00212b', surface: S.base03, alt: S.base02, text: S.base2, muted: S.base1, border: S.base0, onAccent: S.base03, light: S.base2, shade: '#000d12' },
      { accent: S.blue, accentHover: '#5aa9e6', warn: S.yellow, danger: S.red, dangerHover: '#f07a78', positive: S.green, magenta: S.magenta, violet: S.violet }, 'dark') },
  nord: { name: 'Nord', source: 'Arctic Ice Studio\'s Nord (MIT)',
    light: palette({ bg: '#dfe4ec', surface: N.s2, alt: N.s1, text: N.n0, muted: N.n3, border: '#7b8698', onAccent: N.s2, light: '#f8fafc', shade: N.n0 },
      { accent: N.f3, accentHover: '#4c6d94', warn: '#a07425', danger: N.red, dangerHover: '#9a3d47', positive: '#5f8048', magenta: N.purple, violet: N.f3 }, 'light'),
    dark: palette({ bg: '#272c36', surface: N.n0, alt: N.n1, text: N.s2, muted: N.s0, border: '#7b8698', onAccent: N.n0, light: N.s2, shade: '#161a20' },
      { accent: N.f1, accentHover: N.f0, warn: N.yellow, danger: '#e0868e', dangerHover: '#eba5ab', positive: N.green, magenta: N.purple, violet: N.f2 }, 'dark') },
  gruvbox: { name: 'Gruvbox', source: 'Pavel Pertsev\'s gruvbox (MIT)',
    light: palette({ bg: G.lbg1, surface: G.lbg0, alt: '#f2e5bc', text: G.lfg, muted: G.lfg4, border: '#928374', onAccent: G.lbg0, light: '#fffbe8', shade: G.bg0h },
      { accent: G.dblue, accentHover: '#05505e', warn: G.dyellow, danger: G.dred, dangerHover: '#7a0005', positive: G.dgreen, magenta: G.dpurple, violet: G.daqua }, 'light'),
    dark: palette({ bg: G.bg0h, surface: G.bg0, alt: G.bg1, text: G.fg, muted: '#d5c4a1', border: '#928374', onAccent: G.bg0, light: G.fg, shade: '#0e0f0f' },
      { accent: G.bblue, accentHover: '#a5bfb7', warn: G.byellow, danger: G.bred, dangerHover: '#fc7a6a', positive: G.bgreen, magenta: G.bpurple, violet: G.baqua }, 'dark') },
  graphite: { name: 'Graphite', source: 'Dinah, neutral greys',
    light: palette({ bg: '#e4e5e7', surface: '#f6f6f7', alt: '#e9eaec', text: '#16181b', muted: '#4a4f57', border: '#6e737b', onAccent: '#ffffff', light: '#ffffff', shade: '#050607' },
      { accent: '#2759b8', accentHover: '#1e4793', warn: '#8a5a00', danger: '#b42318', dangerHover: '#8c1b12', positive: '#2c6e3f', magenta: '#8a3d8f', violet: '#5b4cb3' }, 'light'),
    dark: palette({ bg: '#0f1012', surface: '#1b1d20', alt: '#26292d', text: '#f3f4f6', muted: '#c4c8ce', border: '#80868f', onAccent: '#0f1012', light: '#f3f4f6', shade: '#000000' },
      { accent: '#8db4ff', accentHover: '#adc8ff', warn: '#f2c25c', danger: '#ff9a8f', dangerHover: '#ffb8b0', positive: '#8fd6a0', magenta: '#e3a6e8', violet: '#b9adff' }, 'dark') },
  // One Dark Pro has no light variant, so light mode uses Atom's One Light,
  // the palette One Dark was paired with.
  'one-dark-pro': { name: 'One Dark Pro', source: 'Binaryify\'s One Dark Pro (MIT), with Atom\'s One Light (MIT) for light mode',
    light: palette({ bg: '#eaeaeb', surface: '#fafafa', alt: '#f0f0f1', text: '#383a42', muted: '#696c77', border: '#a0a1a7', onAccent: '#fafafa', light: '#ffffff', shade: '#202227' },
      { accent: '#4078f2', accentHover: '#2a5fcf', warn: '#986801', danger: '#e45649', dangerHover: '#b8392e', positive: '#50a14f', magenta: '#a626a4', violet: '#0184bc' }, 'light'),
    dark: palette({ bg: '#21252b', surface: '#282c34', alt: '#2c313a', text: '#abb2bf', muted: '#9da5b4', border: '#7f848e', onAccent: '#21252b', light: '#abb2bf', shade: '#0e1013' },
      { accent: '#61afef', accentHover: '#8cc4f3', warn: '#e5c07b', danger: '#e06c75', dangerHover: '#ea959c', positive: '#98c379', magenta: '#c678dd', violet: '#56b6c2', number: '#d19a66' }, 'dark') },
  dracula: { name: 'Dracula', source: 'Dracula Theme (MIT), with its Alucard palette for light mode',
    light: palette({ bg: '#f0ebd8', surface: '#fffbeb', alt: '#f7f2e0', text: '#1f1f1f', muted: '#6c664b', border: '#8a8467', onAccent: '#fffbeb', light: '#ffffff', shade: '#1f1f1f' },
      { accent: '#644ac9', accentHover: '#4e37a6', warn: '#a34d14', danger: '#cb3a2a', dangerHover: '#a12c1f', positive: '#14710a', magenta: '#a3144d', violet: '#036a96', string: '#846e15', number: '#644ac9' }, 'light'),
    dark: palette({ bg: '#21222c', surface: '#282a36', alt: '#343746', text: '#f8f8f2', muted: '#6272a4', border: '#6272a4', onAccent: '#282a36', light: '#f8f8f2', shade: '#0b0b0f' },
      { accent: '#bd93f9', accentHover: '#d0b0fb', warn: '#ffb86c', danger: '#ff5555', dangerHover: '#ffa3a3', positive: '#50fa7b', magenta: '#ff79c6', violet: '#8be9fd', string: '#f1fa8c', number: '#bd93f9' }, 'dark') },
  catppuccin: { name: 'Catppuccin', source: 'Catppuccin (MIT), Latte for light mode and Mocha for dark',
    light: palette({ bg: '#dce0e8', surface: '#eff1f5', alt: '#e6e9ef', text: '#4c4f69', muted: '#5c5f77', border: '#8c8fa1', onAccent: '#eff1f5', light: '#ffffff', shade: '#11111b' },
      { accent: '#1e66f5', accentHover: '#1550c7', warn: '#df8e1d', danger: '#d20f39', dangerHover: '#a80c2e', positive: '#40a02b', magenta: '#8839ef', violet: '#ea76cb', number: '#fe640b' }, 'light'),
    dark: palette({ bg: '#181825', surface: '#1e1e2e', alt: '#313244', text: '#cdd6f4', muted: '#bac2de', border: '#7f849c', onAccent: '#1e1e2e', light: '#cdd6f4', shade: '#0b0b12' },
      { accent: '#89b4fa', accentHover: '#b4befe', warn: '#f9e2af', danger: '#f38ba8', dangerHover: '#f5a6bd', positive: '#a6e3a1', magenta: '#cba6f7', violet: '#f5c2e7', number: '#fab387' }, 'dark') },
  'tokyo-night': { name: 'Tokyo Night', source: 'Enkia\'s Tokyo Night (MIT), Day for light mode and Night for dark',
    light: palette({ bg: '#d0d5e3', surface: '#e1e2e7', alt: '#d8dbe5', text: '#3760bf', muted: '#6172b0', border: '#848cb5', onAccent: '#e1e2e7', light: '#f4f5f8', shade: '#0f0f14' },
      { accent: '#2e7de9', accentHover: '#0f4a96', warn: '#8c6c3e', danger: '#f52a65', dangerHover: '#c71e50', positive: '#587539', magenta: '#9854f1', violet: '#007197', number: '#b15c00' }, 'light'),
    dark: palette({ bg: '#16161e', surface: '#1a1b26', alt: '#292e42', text: '#c0caf5', muted: '#a9b1d6', border: '#737aa2', onAccent: '#1a1b26', light: '#c0caf5', shade: '#0a0a0f' },
      { accent: '#7aa2f7', accentHover: '#9fbbf9', warn: '#e0af68', danger: '#f7768e', dangerHover: '#f99cad', positive: '#9ece6a', magenta: '#bb9af7', violet: '#7dcfff', number: '#ff9e64' }, 'dark') },
  github: { name: 'GitHub', source: 'GitHub\'s GitHub Theme for VS Code (MIT), Light Default and Dark Default',
    light: palette({ bg: '#eaeef2', surface: '#ffffff', alt: '#f6f8fa', text: '#1f2328', muted: '#59636e', border: '#818b98', onAccent: '#ffffff', light: '#ffffff', shade: '#010409' },
      { accent: '#0969da', accentHover: '#0550ae', warn: '#9a6700', danger: '#cf222e', dangerHover: '#a40e26', positive: '#1a7f37', magenta: '#8250df', violet: '#116329', keyword: '#cf222e', string: '#0a3069', number: '#0550ae' }, 'light'),
    dark: palette({ bg: '#010409', surface: '#0d1117', alt: '#161b22', text: '#e6edf3', muted: '#9198a1', border: '#656c76', onAccent: '#0d1117', light: '#e6edf3', shade: '#000000' },
      { accent: '#4493f8', accentHover: '#79c0ff', warn: '#d29922', danger: '#f85149', dangerHover: '#ff7b72', positive: '#3fb950', magenta: '#a371f7', violet: '#7ee787', keyword: '#ff7b72', string: '#a5d6ff', number: '#79c0ff' }, 'dark') }
};

const out = {}, report = [];
for (const [id, spec] of Object.entries(specs)) {
  const moved = { light: settle(spec.light, 'light'), dark: settle(spec.dark, 'dark') };
  out[id] = { format: 'dinah-theme', version: 1, name: spec.name, typography: fonts, palettes: { light: spec.light, dark: spec.dark } };
  report.push(spec.name + ' (' + spec.source + '): ' + ['light', 'dark'].map(m => m + ' adjusted ' + (Object.keys(moved[m]).map(k => k + ' ' + moved[m][k] + '->' + spec[m][k]).join(', ') || 'nothing')).join('; '));
}
writeFileSync(process.argv[2], 'window.themeCandidates = ' + JSON.stringify(out, null, 1) + ';\n');
console.log(report.join('\n'));
