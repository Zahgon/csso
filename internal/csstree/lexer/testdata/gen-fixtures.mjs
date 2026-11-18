// Records the fixtures of the lexer tests with css-tree 2.2.0 and mdn-data
// 2.0.28 (as installed by csso 5.0.5's package-lock.json), under Node 24:
//
//   node testdata/gen-fixtures.mjs <node_modules> <csso fixtures dir>
//
// run from internal/csstree/lexer. It writes testdata/cases.json.gz (declarations
// and values with css-tree's tokens and match results), testdata/definitions.json.gz
// (css-tree's data.js) and ../defsyntax/testdata/syntax.json.gz (definition
// syntax parse/generate results). The corpus is deterministic.
import fs from 'fs';
import path from 'path';
import zlib from 'zlib';

const [modules, fixturesDir] = process.argv.slice(2);
const csstree = await import(path.resolve(modules, 'css-tree/lib/index.js'));
const { default: prepareTokens } = await import(path.resolve(modules, 'css-tree/lib/lexer/prepare-tokens.js'));
const { default: data } = await import(path.resolve(modules, 'css-tree/lib/data.js'));

const { lexer, parse, walk } = csstree;

function writeGz(file, text) {
    fs.writeFileSync(file, zlib.gzipSync(text, { level: 9 }));
    console.log(file, text.length, fs.statSync(file).size);
}
// ---------- corpus ----------
const decls = new Set();

function addDecl(prop, value) {
    decls.add(prop + ':' + value);
}

// (a) real-world declarations from csso's fixtures
function collectFiles(dir) {
    for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
        const p = path.join(dir, e.name);
        if (e.isDirectory()) collectFiles(p);
        else if (e.name.endsWith('.css')) {
            const css = fs.readFileSync(p, 'utf8');
            let ast;
            try { ast = parse(css, { parseValue: false, parseCustomProperty: false }); } catch (e) { continue; }
            walk(ast, { visit: 'Declaration', enter(node) {
                if (node.value.type === 'Raw') addDecl(node.property, node.value.value);
            } });
        }
    }
}
collectFiles(fixturesDir);

// (b) synthetic declarations
let seed = 12345;
function rnd(n) { seed = (seed * 1103515245 + 12345) & 0x7fffffff; return seed % n; }
function pick(a) { return a[rnd(a.length)]; }

const colors = ['red', 'RED', 'Red', 'grey', 'GrEy', 'transparent', 'currentcolor', 'currentColor', 'windowtext', 'ButtonFace',
    '#fff', '#FFF', '#ffff', '#ff0000', '#ff000080', '#ggg', '#ff', '#12345', '#1234567',
    'rgb(1,2,3)', 'rgb(1, 2, 3)', 'rgba(1,2,3,.5)', 'rgb(1 2 3 / 50%)', 'rgb(10%,20%,30%)', 'rgb(1,2)', 'RGB(1,2,3)',
    'hsl(120,50%,50%)', 'hsla(120,50%,50%,0.3)', 'hsl(120deg 50% 50%)', 'hwb(120 10% 10%)', 'lab(50% 20 30)', 'lch(50% 20 30)',
    '-webkit-link', '-moz-ButtonDefault', '-webkit-focus-ring-color', 'blue\\9', 'red\\0', 'navy', 'whitesmoke', 'rebeccapurple',
    'color-mix(in srgb, red, blue)', 'rgb(var(--a))', 'hsl(calc(1deg + 2deg), 1%, 2%)'];
const lengths = ['0', '0px', '1px', '1PX', '-1px', '+1px', '1.5em', '.5rem', '1e3px', '1E3px', '1q', '10vw', '1vmin', '2cqi', '1px\\9', '1px\\0',
    '10%', '0%', '-5%', '100%', '1.5%', 'calc(1px + 2%)', 'calc(100% - 10px)', '-webkit-calc(1px)', '-moz-calc(2em)', 'CALC(1px)',
    '1foo', '1x', '1dpi', '1s', '1ms', '1deg', '1fr', '1hz', '1db', '1st', '0.0', '00', '-0', '+0', '0e0', '0\\9', '1px\\8', '1\u212Ahz', '1Px', '1İn', '1e', '1.5.5px', '+.5em', '-.0e-0px'];
const numbers = ['0', '1', '1.5', '-2', '+3', '.5', '1e3', '1e-3', '2.', '7\\9', '1/2', '16/9', '1 / 2'];
const idents = ['auto', 'none', 'inherit', 'initial', 'unset', 'revert', 'revert-layer', 'INHERIT', 'foo', 'default', 'normal', 'bold',
    'italic', 'solid', 'dashed', 'inset', 'center', 'left', 'top', 'right', 'bottom', 'cover', 'contain', 'no-repeat', 'repeat-x',
    'ease', 'ease-in-out', 'linear', 'all', 'opacity', 'span', 'dense', 'row', 'column', 'thin', 'medium', 'thick', 'serif',
    'sans-serif', 'Arial', 'x-large', 'smaller', 'small-caps', 'block', 'flex', 'grid', 'hidden', 'visible', 'scroll',
    'border-box', 'padding-box', 'text', 'nowrap', 'pre', 'uppercase', 'underline', 'wavy', 'İnherit', '\\9', 'a\\9', 'İnitial', 'ınherit', '\u212Aeep-all', 'ﬁxed'];
const others = ['"str"', "'s'", 'url(a.png)', 'url("a b.png")', 'url()', 'var(--x)', 'VAR(--x)', 'var(--x, red)', '!important',
    'attr(data-x)', 'counter(a)', 'linear-gradient(red, blue)', 'linear-gradient(to right, #fff 0%, rgba(0,0,0,.5) 100%)',
    'radial-gradient(circle at 10% 20%, red, blue 50%)', '-webkit-linear-gradient(top, red, blue)',
    '-webkit-gradient(linear, left top, left bottom, from(#fff), to(#000))', 'repeating-linear-gradient(45deg, red 0 10px, blue 10px 20px)',
    'conic-gradient(red, blue)', 'image-set("a.png" 1x)', 'cubic-bezier(.1,.2,.3,.4)', 'steps(4, end)', 'translate(10px, 20%)',
    'rotate(45deg)', 'scale(1.5)', 'matrix(1,0,0,1,0,0)', 'blur(2px)', 'drop-shadow(0 0 2px red)', 'repeat(2, 1fr)',
    'minmax(100px, 1fr)', 'fit-content(10%)', 'rect(1px, 2px, 3px, 4px)', 'rect(1px 2px 3px 4px)', 'U+0025-00FF', 'u+4??',
    'expression(1+1)', 'progid:DXImageTransform.Microsoft.gradient(startColorstr=#80000000, endColorstr=#80000000)',
    'alpha(opacity=50)', '[a]', '[a b]', '(1)', '{a}', ',', '/', '+', '-', '*', 'a,b', 'env(safe-area-inset-top)',
    'max(1px, 2%)', 'min(1px, 2%)', 'clamp(1px, 2%, 3px)', 'element(#a)', 'cross-fade(url(a.png) 50%, red)', '#a', 'fn()',
    'symbols(cyclic "*")', 'local(Arial)', 'format("woff")', 'U+26', 'circle(50% at 50% 50%)', 'inset(10% round 5px)',
    'path("M0 0")', 'polygon(0 0, 100% 0, 50% 100%)', 'ellipse(10px 20%)', 'paint(foo)', 'calc(1 + 2)', 'calc(1)', 'calc((1px))'];
const pools = [colors, lengths, numbers, idents, others];

const props = Object.keys(lexer.properties);
const extraProps = ['-webkit-box-shadow', '-moz-border-radius', '-webkit-transition', '-ms-filter', 'filter', '*zoom', '_height',
    '*color', '_background', '--custom', '--x', 'unknown-prop', '-webkit-unknown', '-foo-color', 'COLOR', 'Background-Color',
    '-WEBKIT-BOX-SHADOW', '//color', '$color', '-moz-osx-font-smoothing', '-webkit-text-fill-color', '-webkit-text-stroke',
    '-webkit-tap-highlight-color', 'scrollbar-face-color', '-ms-scrollbar-base-color'];

function fragment() {
    const r = rnd(10);
    if (r < 3) return pick(colors);
    if (r < 6) return pick(lengths);
    if (r < 7) return pick(numbers);
    if (r < 9) return pick(idents);
    return pick(others);
}
function value() {
    const n = 1 + rnd(4);
    const parts = [];
    for (let i = 0; i < n; i++) {
        parts.push(fragment());
        if (i < n - 1) parts.push(pick([' ', ' ', ' ', ', ', ',', ' / ', '/', ' ']));
    }
    return parts.join('');
}

const colorish = props.filter(p => /color|background|border|shadow|outline|fill|stroke|caret|column-rule|text-decoration|text-emphasis|scrollbar|mask|filter/.test(p) || /percentage|length|color/.test(String(lexer.properties[p].syntax && csstree.definitionSyntax.generate(lexer.properties[p].syntax))));

{
    for (const p of props.concat(extraProps)) {
        for (let k = 0; k < 4; k++) addDecl(p, value());
    }
    for (const p of colorish) {
        for (let k = 0; k < 6; k++) addDecl(p, value());
        for (const pool of pools) addDecl(p, pick(pool));
    }
    // every fragment against a set of key properties
    const key = ['color', 'background', 'background-color', 'border', 'border-color', 'box-shadow', 'text-shadow', 'outline', 'font',
        'line-height', 'margin', 'padding', 'width', 'flex', 'transition', 'transform', 'grid-template-columns', 'grid-area',
        'background-position', 'background-size', 'fill', 'stroke', 'filter', 'mask', 'border-radius', 'font-size',
        'vertical-align', 'opacity', 'z-index', 'content', 'unicode-range', 'cursor', 'column-rule', 'text-decoration', 'caret-color',
        'border-image', 'list-style', 'animation', 'font-family', 'letter-spacing', 'clip', 'shape-outside', 'zoom', '-ms-filter'];
    for (const p of key) {
        for (const pool of pools) for (const f of pool) addDecl(p, f);
        for (let k = 0; k < 25; k++) addDecl(p, value());
    }
}


// (c) values generated from the definition syntaxes themselves, valid and
// slightly corrupted, so that most of them match
const genericSamples = {
    'length': ['0', '1px', '2em', '-3vh', 'calc(1px + 2%)', '1.5rem', '0px'], 'percentage': ['0%', '50%', '100%', '-5%'],
    'number': ['0', '1.5', '2', '-1', '.5'], 'integer': ['1', '3', '0', '-2'], 'ident': ['foo', 'bar'], 'custom-ident': ['myname', 'x'],
    'string': ['"s"', "'t'"], 'url-token': ['url(a.png)'], 'hex-color': ['#abc', '#aabbcc', '#ABCD'], 'dimension': ['1px', '2foo'],
    'angle': ['45deg', '1turn', '0'], 'time': ['1s', '200ms'], 'resolution': ['2dppx', '96dpi'], 'flex': ['1fr'], 'frequency': ['1hz'],
    'declaration-value': ['a b', 'x'], 'any-value': ['x'], 'urange': ['U+26', 'u+0-7f'], 'an-plus-b': ['2n+1', 'odd'],
    'custom-property-name': ['--x'], 'zero': ['0'], 'ident-token': ['foo'], 'hash-token': ['#a'], 'number-token': ['1'],
    'dimension-token': ['1px'], 'percentage-token': ['1%'], 'string-token': ['"s"'], 'delim-token': ['+'], 'decibel': ['1db'],
    'semitones': ['1st'], 'id-selector': ['#id'], 'comma-token': [','], 'colon-token': [':'], 'semicolon-token': [';'],
    'whitespace-token': [' '], 'function-token': ['fn('], 'at-keyword-token': ['@a'], 'url': ['url(a.png)', 'url("b.png")'],
};
class TooDeep extends Error {}
function sample(node, depth) {
    if (depth > 14) throw new TooDeep();
    switch (node.type) {
        case 'Group': {
            let terms = node.terms;
            if (node.combinator === '|') return sample(pick(terms), depth + 1);
            if (node.combinator === '&&' || node.combinator === '||') {
                terms = terms.slice();
                for (let i = terms.length - 1; i > 0; i--) { const j = rnd(i + 1); [terms[i], terms[j]] = [terms[j], terms[i]]; }
                if (node.combinator === '||') terms = terms.slice(0, 1 + rnd(terms.length));
            }
            const parts = terms.map(t => sample(t, depth + 1)).filter(x => x !== '');
            return parts.join(' ');
        }
        case 'Multiplier': {
            const max = node.max === 0 ? node.min + 2 : node.max;
            const n = node.min + rnd(Math.min(max, node.min + 3) - node.min + 1);
            const parts = [];
            for (let i = 0; i < n; i++) parts.push(sample(node.term, depth + 1));
            return parts.join(node.comma ? ', ' : ' ');
        }
        case 'Type': {
            if (genericSamples[node.name]) return pick(genericSamples[node.name]);
            const t = lexer.types[node.name];
            if (!t || !t.syntax) return 'x';
            return sample(t.syntax, depth + 1);
        }
        case 'Property': return sample(lexer.properties[node.name].syntax, depth + 1);
        case 'Keyword': return rnd(8) === 0 ? node.name.toUpperCase() : node.name;
        case 'AtKeyword': return '@' + node.name;
        case 'Function': return node.name + '(';
        case 'String': return node.value.slice(1, -1);
        case 'Token': return node.value;
        case 'Comma': return ',';
    }
    return '';
}
function sampleValue(syntax) {
    for (let k = 0; k < 20; k++) {
        try { return sample(syntax, 0); } catch (e) { if (!(e instanceof TooDeep)) throw e; }
    }
    return null;
}
{
    for (const p of props) {
        const syntax = lexer.properties[p].syntax;
        for (let k = 0; k < 7; k++) {
            const v = sampleValue(syntax);
            if (v === null) continue;
            if (k < 5) { addDecl(p, v); continue; }
            // corrupt: drop or duplicate a word
            const words = v.split(' ');
            const i = rnd(words.length);
            if (k === 5) words.splice(i, 1); else words.splice(i, 0, words[i]);
            addDecl(p, words.join(' '));
        }
    }
}
// hand-written shorthands
const handwritten = [
    'background: url(a.png) no-repeat 0 0 / cover, linear-gradient(red, blue) #fff',
    'background: #fff url(a.png) no-repeat right top',
    'background: red; background: blue',
    'background: 0 0, red',
    'background: red, blue',
    'background: none 0% 0% / auto repeat scroll padding-box border-box rgb(1,2,3)',
    'border: 1px solid red',
    'border: solid 1px rgba(0,0,0,.1)',
    'border: 0 none',
    'border: medium none currentColor',
    'border-color: red green blue yellow',
    'border-color: red green blue yellow black',
    'box-shadow: 0 0 0 1px red, inset 0 1px 2px rgba(0,0,0,.5)',
    'box-shadow: inset 0 0 0 transparent',
    'box-shadow: none',
    'box-shadow: 1px 1px red 1px',
    'text-shadow: 1px 1px 2px black, 0 0 1em blue, 0 0 0.2em blue',
    'font: italic small-caps bold condensed 16px/2 cursive',
    'font: 12px/1.5 "Helvetica Neue", Arial, sans-serif',
    'font: bold 0/0 a',
    'font: 100%/1 serif',
    'font: menu',
    'font: inherit',
    'font: -apple-system-body',
    'grid-template-columns: [full-start] minmax(1em, 1fr) [main-start] minmax(0, 40em) [main-end] minmax(1em, 1fr) [full-end]',
    'grid-template-areas: "a b" "c d"',
    'grid-template: "a a a" 40px "b c c" 40px / 1fr 1fr 1fr',
    'grid-area: 1 / 2 / 3 / 4',
    'grid-row: span 2 / 3',
    'grid: auto-flow dense / 40px 40px',
    'transition: opacity .3s ease-in-out, transform 0s 1s',
    'transition: all 0 ease',
    'transition: color 1s, background-color 2s linear 0.5s',
    'animation: 3s ease-in 1s infinite reverse both running slidein',
    'background-image: linear-gradient(45deg, red 0%, blue 100%), radial-gradient(red, transparent)',
    'background-image: -webkit-gradient(linear, 0 0, 0 100%, from(red), color-stop(50%, blue), to(white))',
    'background-image: -moz-linear-gradient(top, rgba(255,255,255,1) 0%, rgba(0,0,0,0) 100%)',
    'background-image: -o-linear-gradient(top, #fff, #000)',
    'background-image: -webkit-radial-gradient(center, ellipse cover, red 0%, blue 100%)',
    'background-position: 0% 0%',
    'background-position: right 10% bottom 0%',
    'background-size: 0% 100%',
    'filter: progid:DXImageTransform.Microsoft.gradient( startColorstr=\'#80000000\', endColorstr=\'#80000000\',GradientType=0 )',
    '-ms-filter: "progid:DXImageTransform.Microsoft.Alpha(Opacity=50)"',
    'filter: alpha(opacity=0)',
    'filter: drop-shadow(0 0 0 red) blur(0%)',
    'margin: 0% auto',
    'margin: 0 0 0 0 0',
    'padding: 0% 0% 0% 0%',
    'line-height: 0%',
    'line-height: 0',
    'flex: 1 1 0%',
    'flex: 0 0 0%',
    '-ms-flex: 1 1 0%',
    'width: 0%',
    'height: calc(100% - 0%)',
    'transform: translate(0%, 0%) rotate(0deg)',
    'transform: translateX(0%)',
    'border-radius: 0% / 50%',
    'border-radius: 50%',
    'stroke-dasharray: 0% 1%',
    'font-size: 0%',
    'vertical-align: 0%',
    'text-indent: 0%',
    'clip-path: circle(0% at 0% 0%)',
    'mask: url(a.svg) 0% 0% / 0% no-repeat',
    'color: red !important',
    'color: red\\9',
    'color: red \\9',
    'color: #fff\\9',
    'color: rgb(0,0,0)\\9',
    'color: var(--x)',
    'color: red var(--x)',
    'color: expression(foo)',
    'color: -webkit-link',
    'color: WindowText',
    'color: inherit',
    'color: red blue',
    'color: ',
    'color: ;',
    'color: rgb(1,2,3',
    'color: "red"',
    'color: url(red)',
    'color: #fff #000',
    'color: transparent',
    'color: 0',
    'background-color: rgba(0,0,0,0)',
    'outline: 0',
    'outline: thin dotted invert',
    'outline: 1px solid -webkit-focus-ring-color',
    'column-rule: 1px solid red',
    'text-decoration: underline wavy red',
    'text-decoration-color: currentcolor',
    'caret-color: auto',
    'fill: url(#a) red',
    'stroke: context-stroke',
    'border-image: url(a.png) 30 30 round',
    'border-image: linear-gradient(red, blue) 27 / 35px 0% 0 round stretch',
    'list-style: square inside url(a.png)',
    'content: "a" attr(x) counter(c, upper-roman)',
    'cursor: url(a.cur) 1 2, auto',
    'unicode-range: U+0025-00FF, u+4??',
    'quotes: "\\201C" "\\201D"',
    'clip: rect(0 0 0 0)',
    'clip: rect(0, 0, 0, 0)',
    'zoom: 1',
    'zoom: 100%',
    '*zoom: 1',
    '_height: 1%',
    'scrollbar-face-color: red',
    '-webkit-tap-highlight-color: rgba(0,0,0,0)',
    '-webkit-box-shadow: 0 0 0 red',
    '-moz-box-shadow: 0 0 0 0 red',
    '-webkit-text-fill-color: transparent',
    'image-rendering: -webkit-optimize-contrast',
    'aspect-ratio: 16 / 9',
    'aspect-ratio: 0.5',
    'inset: 0% auto',
    'gap: 0% 0%',
    'object-position: 0% 0%',
    'perspective-origin: 0% 0%',
    'transform-origin: 0% 0% 0',
    'shape-outside: circle(0%)',
    'offset: 0% 0% path("M 0 0")',
    'font-variation-settings: "wght" 400',
    'font-feature-settings: "liga" 0',
    'will-change: transform, opacity',
    'counter-reset: a 0 b',
    'paint-order: stroke fill',
    'mix-blend-mode: multiply',
    'text-emphasis: filled red',
    'accent-color: red',
    'border-inline-color: red blue',
    'outline-color: invert',
    'background: url(data:image/png;base64,AAAA) 0 0 no-repeat',
    'background: rgba(0,0,0,.5) url(a.png) repeat-x 0% 0%',
    'font-family: a, b, "c d", e f',
    'font-family: default',
    'font-family: inherit',
    'margin: -0%',
    'width: 0% !important',
    'top: 0%',
    'left: 0%\\9',
    'flex-basis: 0%',
    'max-width: 0%',
    'background: -webkit-linear-gradient(left, red 0%, blue 100%) 0% 0% / 0% 0%',
    'grid-template-columns: repeat(auto-fill, minmax(0%, 1fr))',
    'transition: margin-left 0s',
    'transition-timing-function: cubic-bezier(0, 0, 0.58, 1)',
    'speak: none',
    'voice-family: male',
    'azimuth: left-side behind',
    'pause: 1s 2s',
    'size: a4 landscape',
    'page-break-after: always',
    'src: url(a.woff) format("woff")',
    'behavior: url(a.htc)',
    'text-size-adjust: 100%',
    '-webkit-text-size-adjust: 0%',
    'background: red url(a.png) 0% 0% / 0% 0% no-repeat fixed padding-box content-box, url(b.png), url(c.png), url(d.png), url(e.png)',
    'background: a b c d e f g h i j k l m n o p',
    'box-shadow: 0 0 0 1px red, 0 0 0 1px red, 0 0 0 1px red, 0 0 0 1px red, 0 0 0 1px red, 0 0 0 1px red, 0 0 0 1px red, 0 0 0 1px red, 0 0 0 1px red, 0 0 0 1px red',
    'grid-template-areas: "a" "b" "c" "d" "e" "f" "g" "h" "i" "j" "k" "l" "m"',
    'transition: a 1s, b 1s, c 1s, d 1s, e 1s, f 1s, g 1s, h 1s, i 1s, j 1s, k 1s, l 1s, m 1s, n 1s, o 1s, p 1s, q 1s',
    'mask: a b c d e f g h i j k l',
    'font: a b c d e f g h',
];
for (const d of handwritten) decls.add(d);

// long values around css-tree's iteration limit, valid and invalid
const repeated = [
    ['grid', '"a" 1px [b]', ' ', ' / x'], ['grid-template-columns', '[a] 1px', ' ', ' x'], ['transition', 'a 1s 2s ease', ', ', ' x'],
    ['mask', 'url(a) 0 0 / 1px repeat border-box', ', ', ' x'], ['box-shadow', '1px 1px 1px 1px red inset', ', ', ' x'],
    ['text-shadow', '1px 1px 1px red', ', ', ' x'], ['animation', 'a 1s 2s ease infinite alternate both running', ', ', ' x'],
    ['background', 'red url(a) 0 0 / 1px no-repeat', ', ', ' x'], ['background', 'url(a) 0% 0% / 0% no-repeat', ', ', ''],
    ['font-family', 'a b', ', ', ' 1'], ['will-change', 'a', ', ', ' 1'], ['transform', 'translate(0%, 1px)', ' ', ' x'],
];
for (const [p, unit, sep, tail] of repeated) {
    for (const n of [3, 10, 20, 30, 45, 60]) {
        const v = Array(n).fill(unit).join(sep);
        addDecl(p, v);
        addDecl(p, v + tail);
    }
}

// ---------- recording ----------
function traceToStr(trace) {
    if (trace === null) return null;
    return trace.map(s => (s.type === 'Type' ? 'T:' : s.type === 'Property' ? 'P:' : 'K:') + s.name);
}

// nodes of a value in walk order and its tokens as [type, value, node index]
function tokensOf(value) {
    const nodes = [];
    walk(value, node => { nodes.push(node); });
    const index = new Map(nodes.map((n, i) => [n, i]));
    const tokens = prepareTokens(value, csstree).map(t => [t.type, t.value, t.node ? index.get(t.node) : -1]);
    return { nodes, tokens };
}

function describe(c, res, nodes) {
    c.m = res.matched ? 1 : 0;
    c.i = res.iterations === undefined ? -1 : res.iterations;
    if (res.error) {
        c.e = res.error.name + ': ' + (res.error.rawMessage || res.error.message);
        if (res.error.name === 'SyntaxMatchError') {
            c.em = [res.error.css, res.error.mismatchOffset, res.error.mismatchLength,
                res.error.loc.start.offset, res.error.loc.start.line, res.error.loc.start.column,
                res.error.loc.end.offset, res.error.loc.end.line, res.error.loc.end.column, res.error.syntax];
        }
    }
    c.r = nodes.map(n => traceToStr(res.getTrace(n)));
    return c;
}

function record(decl, source, mutation) {
    const { nodes, tokens } = tokensOf(decl.value);
    return describe({ d: source, x: mutation, p: decl.property, t: tokens }, lexer.matchDeclaration(decl), nodes);
}

function recordType(ty, value, source) {
    const { nodes, tokens } = tokensOf(value);
    return describe({ d: source, ty, x: -1, p: '', t: tokens }, lexer.matchType(ty, value), nodes);
}

const cases = [];
for (const d of decls) {
    let ast;
    try { ast = parse('a{' + d + '}'); } catch (e) { continue; }
    const declsInRule = [];
    walk(ast, { visit: 'Declaration', enter(node) { declsInRule.push(node); } });
    if (declsInRule.length !== 1) continue;
    const decl = declsInRule[0];
    const c = record(decl, d, -1);
    cases.push(c);
    // csso's Percentage pass: each percentage replaced by a Number 0
    let k = 0;
    const pcts = [];
    walk(decl.value, { visit: 'Percentage', enter(node, item, list) { pcts.push({ node, item, list }); } });
    for (const { node, item, list } of pcts) {
        if (!item) { k++; continue; }
        const saved = item.data;
        item.data = { type: 'Number', loc: node.loc, value: '0' };
        cases.push(record(decl, d, k));
        item.data = saved;
        k++;
    }
}

// matchType cases: the value parsed on its own and matched against a type
const typeNames = ['color', 'length', 'percentage', 'length-percentage', 'image', 'number', 'integer', 'an-plus-b', 'urange',
    'custom-ident', 'position', 'gradient', 'url', 'string', 'nth', 'angle', 'time', 'named-color', 'hex-color', 'rgb()',
    'shadow', 'bg-layer', 'final-bg-layer', 'line-names', 'track-list', 'transform-list', 'filter-function-list', 'ratio',
    'number-zero-one', 'positive-integer', 'alpha-value', 'hue', 'zero', 'declaration-value', 'any-value', 'id-selector',
    'unknown-type', 'calc-sum', 'single-transition', 'easing-function', 'font-weight-absolute', 'family-name'];
for (const ty of typeNames) {
    const values = new Set();
    for (const pool of pools) for (let k = 0; k < 6; k++) values.add(pick(pool));
    const t = lexer.types[ty];
    if (t && t.syntax) for (let k = 0; k < 8; k++) { const v = sampleValue(t.syntax); if (v !== null) values.add(v); }
    for (const v of ['', 'inherit', '2n+1', '-n+3', 'n- 1', '+n', '- n', 'n + 1', '2n -1', '-2n- 1', 'U+0-7F', 'u+?', 'u+1-2-3',
        'U+1234567', 'u+a?', 'U+ff??', '1/2', '0/0', '1 / 0']) values.add(v);
    for (const v of values) {
        let ast;
        try { ast = parse(v, { context: 'value' }); } catch (e) { continue; }
        const c = recordType(ty, ast, v);
        cases.push(c);
    }
}

writeGz('testdata/cases.json.gz', cases.map(c => JSON.stringify(c)).join('\n') + '\n');

// ---------- css-tree's data.js ----------
{
    const atrules = {};
    for (const k in data.atrules) {
        atrules[k] = { prelude: data.atrules[k].prelude, descriptors: data.atrules[k].descriptors === undefined ? null : data.atrules[k].descriptors };
    }
    writeGz('testdata/definitions.json.gz', JSON.stringify({ types: data.types, properties: data.properties, atrules }));
}

// ---------- definition syntax ----------
{
const { parse, generate } = csstree.definitionSyntax;
const sources = new Set();
for (const m of ['types', 'properties']) for (const k in data[m]) sources.add(data[m][k]);
for (const k in data.atrules) {
    const a = data.atrules[k];
    if (a.prelude) sources.add(a.prelude);
    for (const d in a.descriptors || {}) sources.add(a.descriptors[d]);
}
const extra = ['a b c', 'a | b && c || d e', '[ a | b ]! c', 'a{2} b{1,} c{0,3} d#{1,2} e#? f+# g?', '<length [0,∞]>',
    '<number [-∞,10]>', '<angle [0deg,360deg]>', '<integer [-5,-1]>', "'progid:' a", "'\\'' b", '@media', 'fn( a , b )',
    '[ , a ]', '<a()>', "<'prop'>+", '{0}', 'a{0}', 'a{1,0}', '<x [1,99999999999999999999999]>',
    '', '   ', 'a   b', '<x [ 1 , 2 ]>', 'a / b', 'a [ b c ]', '[[a]]', 'a,b', '[a | b]#', '<rgb()> | rgb( a )'];
const bad = ['a |', '| a', 'a || | b', 'a &', '<a', "<'a>", "'abc", 'a{x}', 'a{1', '<x [a,1]>', '<x [1;2]>', ']', 'a ]', '<>',
    '*', 'a{1,2', '<x [1,2>', '<x [∞,1]>', '@', '<x [-a,1]>', 'a{', '{1}', 'a!', '<x[1,2]>', "<'a'", '<a b>'];
const out = [];
for (const s of [...sources, ...extra]) {
    try {
        const ast = parse(s);
        out.push({ s, g: generate(ast), c: generate(ast, { compact: true }), f: generate(ast, { forceBraces: true }), a: JSON.stringify(ast) });
    } catch (e) { out.push({ s, err: e.message }); }
}
for (const s of bad) {
    try { const ast = parse(s); out.push({ s, g: generate(ast), c: generate(ast, { compact: true }), f: generate(ast, { forceBraces: true }), a: JSON.stringify(ast) }); }
    catch (e) { out.push({ s, err: e.message }); }
}
writeGz('../defsyntax/testdata/syntax.json.gz', out.map(o => JSON.stringify(o)).join('\n') + '\n');
}
