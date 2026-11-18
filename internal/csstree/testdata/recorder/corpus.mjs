// Builds the differential test cases: hand-written edge cases, csso's
// fixture stylesheets and seeded random CSS (with mutations).
import { readFileSync, readdirSync, statSync } from 'fs';
import { join } from 'path';

function mulberry32(a) {
    return function() {
        a |= 0; a = a + 0x6D2B79F5 | 0;
        let t = Math.imul(a ^ a >>> 15, 1 | a);
        t = t + Math.imul(t ^ t >>> 7, 61 | t) ^ t;
        return ((t ^ t >>> 14) >>> 0) / 4294967296;
    };
}

let rnd = mulberry32(12345);
const pick = arr => arr[Math.floor(rnd() * arr.length)];
const chance = p => rnd() < p;
const times = (n, fn) => Array.from({ length: n }, fn);

const idents = ['a', 'foo', 'Bar', '-webkit-box', '-moz-x', '--var', '_h', '\\31 23', 'é', 'ünï', '𝒜b', 'u', 'n', 'N', 'odd', 'even', 'url', 'i', 's',
    'important', 'inherit', 'from', 'to', 'and', 'not', 'only', 'or', 'screen', 'print', 'auto', 'none', 'red', 'RED', 'transparent', 'x\\:y', 'a\\ b', '\\@', '-', '--', 'e', 'E', 'of', 'İ', 'ΣΑΣ'];
const numbers = ['0', '1', '-1', '+1', '.5', '+.5', '-.5', '1e3', '1E-3', '1.5e+2', '00', '10.0', '0.0', '3', '12', '100', '1e', '-0', '+0', '1.'];
const units = ['px', 'em', 'PX', 'n', 'n-', 'n-1', 'e', 'deg', 'x', 's', 'ms', 'dppx', '\\70 x', 'é'];
const strings = ['"a"', "'b'", '"esc\\"aped"', "'it\\'s'", '"line\\\ncont"', '"\\0"', '"\\D800"', '"\\110000"', '"\\41 B"', '"\\\\"', '""', "''", '"a\'b"', "'a\"b'", '"\\\r\nx"', '"tab\\9 x"', '"ünï"', '"😀"', '"\\1F600"', '"\\"', "'\\'"];
const urls = ['url(a.png)', 'url( "x" )', "url('y z')", 'url(a b)', 'url(a"b)', 'url(\\))', 'URL(x)', 'url()', 'url( )', 'url(a\\ b)', 'url(\\61 )', 'url(x\\', 'url("a)b")', 'url(data:image/png;base64,iVBOR==)', 'url(a(b)', 'url(\\0)', 'url( x  )'];
const hashes = ['#fff', '#FFF', '#123', '#-a', '#a1b2c3', '#\\31', '#é'];
const urange = ['U+0025-00FF', 'u+4??', 'U+1F600', 'U+0-7F', 'u+0025', 'U+??????', 'U+???????', 'u+a-', 'U+12345678', 'u+1e3', 'u+1e-3', 'U+0025-00FF?', 'u+?', 'U+FF-'];
const fnNames = ['calc', 'rgb', 'rgba', 'var', 'attr', 'expression', 'linear-gradient', 'foo', 'Calc', 'VAR', 'min', 'max', 'url', 'format', 'local', 'counter', 'translate', 'selector', 'env'];
const ops = ['/', ',', '*', '+', '-', ' / ', ' + ', ' - ', ' * ', ', '];
const ws = ['', ' ', ' ', '  ', '\n', '\t', '\r\n', '\f', '/* c */', ' /**/ ', '/*!x*/'];

function sp() { return chance(0.6) ? ' ' : pick(ws); }

function value(depth = 0) {
    const n = 1 + Math.floor(rnd() * 4);
    let out = '';
    for (let i = 0; i < n; i++) {
        if (i > 0) out += chance(0.3) ? pick(ops) : chance(0.85) ? ' ' : pick(ws);
        out += term(depth);
    }
    return out;
}

function term(depth) {
    const r = rnd();
    if (r < 0.15) return pick(idents);
    if (r < 0.3) return pick(numbers);
    if (r < 0.42) return pick(numbers) + pick(units);
    if (r < 0.47) return pick(numbers) + '%';
    if (r < 0.53) return pick(strings);
    if (r < 0.58) return pick(urls);
    if (r < 0.63) return pick(hashes);
    if (r < 0.66) return pick(urange);
    if (r < 0.70 && depth < 3) return '(' + value(depth + 1) + ')';
    if (r < 0.73 && depth < 3) return '[' + value(depth + 1) + ']';
    if (r < 0.88 && depth < 3) {
        const name = pick(fnNames);
        if (name.toLowerCase() === 'var') {
            return name + '(' + sp() + pick(['--x', '--y', 'a', '1']) + (chance(0.6) ? sp() + ',' + (chance(0.3) ? sp() : value(depth + 1)) : '') + ')';
        }
        return name + '(' + (chance(0.9) ? value(depth + 1) : '') + (chance(0.95) ? ')' : '');
    }
    if (r < 0.92) return pick(['!', '#', '$', '?', '@x', '{', '}', ';', ':', '=', '<', '>', '~', '|', '&', '^', '\\', '`']);
    return pick(['progid:DXImageTransform.Microsoft.gradient(startColorstr=\'#1\', endColorstr=\'#2\')', '12px/1.5 a', '1 / 2', 'calc(1px+2px)', 'calc(1px - -2px)', 'calc( (1px + 2px) * 3 )', 'a/**/b', '-a', '-1px-2px', '1px-2px', '+-1', 'a--b', '\\', '\\\n', '-->', '<!--']);
}

function attrSel() {
    const names = ['a', 'data-x', '*|a', '|a', 'ns|a', '*', 'a|', 'x i'];
    const m = ['=', '~=', '^=', '$=', '*=', '|=', '!=', '= '];
    let out = '[' + sp() + pick(names);
    if (chance(0.7)) {
        out += sp() + pick(m) + sp() + pick(['b', '"c"', "'d e'", '1', '#x', 'é']);
        if (chance(0.3)) out += sp() + pick(['i', 's', 'I', 'x']);
    }
    return out + sp() + (chance(0.95) ? ']' : '');
}

function nthArg() {
    return pick(['odd', 'even', 'ODD', '2n+1', '2n + 1', '-n+3', '+n', 'n', '-n', 'N', '3', '+3', '-3', '2n-1', '2n- 1', '2n -1', '2n - 1', ' 2n+ 1 ', 'n-1', '-n-1', '+n-1', '2N+1', '1n', '+5n-3', '-2n+ 3', '- n', '2n+-1', 'n- -1', '0n+0', '1.5n', 'foo', '2n of .a', 'odd of a, b', 'n+1 of', '+ n', '-n- 2', '10n-', '2n+', '3n+/**/1']);
}

let selDepth = 0;
function compound() {
    let out = '';
    const n = 1 + Math.floor(rnd() * 3);
    for (let i = 0; i < n; i++) {
        const r = rnd();
        if (r < 0.2 && i === 0) out += pick(['a', 'div', 'DIV', '*', 'ns|a', '*|*', '|b', 'svg|*', 'é']);
        else if (r < 0.4) out += '.' + pick(['b', 'c-d', '\\31 x', '_e', 'é', '-x', '1', '']);
        else if (r < 0.5) out += pick(['#id', '#x-y', '#1', '#\\31']);
        else if (r < 0.6) out += attrSel();
        else if (r < 0.8 && selDepth < 2) {
            selDepth++;
            const inner = selectorList();
            selDepth--;
            out += ':' + pick(['not(' + inner + ')', 'is(' + inner + ')', 'nth-child(' + nthArg() + ' of ' + inner + ')']);
        } else if (r < 0.8) out += ':' + pick(['hover', 'first-child', 'where(a)', 'has(> a)', 'matches(a,b)', '-moz-any(a)', '-webkit-any(b)',
            'nth-child(' + nthArg() + ')', 'nth-last-child(' + nthArg() + ')', 'nth-of-type(' + nthArg() + ')', 'nth-last-of-type(' + nthArg() + ')', 'lang(en)', 'dir(rtl)', 'lang("en")', 'unknown(a b (c))', 'slotted(a)', 'NOT(a)', 'host', 'host(.a)', 'nth-child()', 'not()', 'contains("x")']);
        else if (r < 0.9) out += '::' + pick(['before', 'after', 'part(x)', 'slotted(a)', 'cue(b)', '-webkit-scrollbar', 'Before']);
        else out += pick(['50%', '0%', 'from', 'to', '.5%', '1']);
    }
    return out;
}

function selector() {
    let out = compound();
    const n = Math.floor(rnd() * 3);
    for (let i = 0; i < n; i++) {
        out += pick([' ', '>', ' > ', '+', ' + ', '~', ' ~ ', ' /deep/ ', '/deep/', '  ', '\n', ' /**/ ', '/**/', '>>', '>+']) + compound();
    }
    return out;
}

function selectorList() {
    return times(1 + Math.floor(rnd() * 2), selector).join(chance(0.5) ? ',' : ' , ');
}

const hackProps = ['*zoom', '_height', '$x', '+y', '#z', '&w', '/a', '//b', 'color', 'COLOR', '-webkit-transition', '--custom', '--Empty', 'filter', 'font', 'grid-area', 'background', 'margin', 'src', 'unicode-range', 'width', '-ms-filter', '\\63 olor', '1a', '.x'];

function declaration() {
    const prop = pick(hackProps);
    let v;
    if (prop.startsWith('--')) {
        v = pick(['', ' ', ' {a b}', 'a,b', ' [x]', ' 1px', '  ', ' (;)', ' !important', ' foo !x', ' var(--y)', ' a /*c*/ b ', ' {;}', '{}']);
    } else if (prop === 'unicode-range') {
        v = sp() + pick(urange);
    } else {
        v = sp() + (chance(0.95) ? value() : '');
    }
    let out = prop + sp() + (chance(0.97) ? ':' : '') + v;
    if (chance(0.15)) out += sp() + pick(['!important', '! important', '!IMPORTANT', '!ie', '!', '!important!', '! /**/ important', '!1']);
    return out;
}

function block(depth, declarations) {
    let out = '{' + sp();
    const n = Math.floor(rnd() * 4);
    for (let i = 0; i < n; i++) {
        if (declarations || chance(0.2)) {
            out += declaration() + (chance(0.85) ? sp() + ';' : '') + sp();
        } else if (depth < 2 && chance(0.5)) {
            out += atrule(depth + 1) + sp();
        } else {
            out += rule(depth + 1) + sp();
        }
    }
    return out + (chance(0.96) ? '}' : '');
}

function rule(depth = 0) {
    return selectorList() + sp() + block(depth, true);
}

const mediaQueries = ['screen', 'screen and (min-width: 100px)', 'print, screen', 'not screen and (color)', 'only screen', '(min-aspect-ratio: 16/9)', '(min-aspect-ratio: 16 / 9)', '(x: 0/1)', '(x:1.5/2)', '(x: -1/2)',
    '(-webkit-min-device-pixel-ratio: 1.5)', '(min-resolution: 2dppx)', '(orientation:landscape)', '(width)', '(a: "b")', 'screen and', '(a:)', '(1px)', 'all and (max-width:100px) , print', '(a: b c)', '(x: 1/ /2)', 'screen (a:b)', 'SCREEN AND (MIN-WIDTH:1PX)'];

function atrule(depth = 0) {
    const r = rnd();
    if (r < 0.2) return '@media' + sp() + pick(mediaQueries) + sp() + block(depth, false);
    if (r < 0.3) return '@import' + sp() + pick(['"a.css"', 'url(a.css)', 'url("b.css")', "'c.css' screen", 'url(x) print, (min-width:1px)', 'x', '"a" (a:b)', '"a" x y', '']) + sp() + ';';
    if (r < 0.4) return '@supports' + sp() + pick(['(display: grid)', 'not (display:grid)', '(a:b) and (c:d)', '((a:b) or (c:d))', 'selector(a > b)', '(a)', '(--x: y)', '', '(a: b) xx', 'foo(1)', '(display: grid !important)']) + sp() + block(depth, false);
    if (r < 0.48) return '@font-face' + sp() + block(depth, true);
    if (r < 0.55) return '@page' + sp() + pick(['', ':first', ':left a', 'x:right']) + sp() + block(depth, true);
    if (r < 0.65) return '@' + pick(['keyframes', '-webkit-keyframes', 'KEYFRAMES']) + ' ' + pick(['x', 'y', '"s"']) + sp() + '{' + times(Math.floor(rnd() * 3), () => pick(['from', 'to', '50%', '0%,100%', '10.5%']) + block(depth + 1, true)).join('') + '}';
    if (r < 0.7) return '@charset "utf-8";';
    if (r < 0.75) return '@namespace ' + pick(['svg url(x)', '"x"', 'a "b"']) + ';';
    if (r < 0.82) return '@' + pick(['unknown', 'x-y', 'viewport', 'document', 'layer', 'container']) + sp() + pick(['', 'foo', 'a b', '(x)', 'url(x)', '{']) + sp() + pick([block(depth, true), block(depth, false), ';', '{a:b}', '{x{}}', '{@x;}', '']);
    if (r < 0.86) return '@media' + sp() + pick(mediaQueries) + ';';
    return '@' + pick(['media', 'import', 'supports', 'font-face', 'page']) + pick([';', '{}', ' {', ' ;']);
}

function stylesheet() {
    let out = chance(0.03) ? '\uFEFF' : '';
    const n = 1 + Math.floor(rnd() * 5);
    for (let i = 0; i < n; i++) {
        const r = rnd();
        if (r < 0.55) out += rule();
        else if (r < 0.85) out += atrule();
        else out += pick(['/*! keep */', '/* drop */', '<!--', '-->', ';', '}', '{}', 'a{', '@', '/*!', '/*/', '/*! /* */', ' ', '\\', '!']);
        out += sp();
    }
    return out;
}

const mutationChars = ['{', '}', '(', ')', '[', ']', ';', ':', ',', '"', "'", '\\', '/', '*', '@', '!', '#', ' ', '\n', '-', '+', '.', '%', 'u', 'n', 'url(', '/*', '*/', '<!--', '-->', '\r', '\f', '\0', 'é'];

function mutate(s) {
    const n = 1 + Math.floor(rnd() * 3);
    for (let i = 0; i < n; i++) {
        const pos = Math.floor(rnd() * (s.length + 1));
        const r = rnd();
        if (r < 0.4) s = s.slice(0, pos) + pick(mutationChars) + s.slice(pos);
        else if (r < 0.8) s = s.slice(0, pos) + s.slice(pos + 1 + Math.floor(rnd() * 3));
        else s = s.slice(0, pos) + s.slice(pos, pos + 5) + s.slice(pos);
    }
    // never split a surrogate pair
    return s.replace(/[\uD800-\uDBFF](?![\uDC00-\uDFFF])|(?<![\uD800-\uDBFF])[\uDC00-\uDFFF]/g, '');
}

const hand = [
    '', ' ', '\uFEFF', '\uFEFFa{b:c}', '\uFFFEa{}', 'a{}', 'a{b:c}', 'a{b:c;}', 'a { b : c ; d : e }', '{}', '{a:b}', 'a{;;b:c;;}',
    'a{b:c!important}', 'a{b:c ! important}', 'a{b:c!IMPORTANT}', 'a{b:c !ie}', 'a{b:c!important;d:e}', 'a{*zoom:1;_height:1px}',
    'a{--x:}', 'a{--x: }', 'a{--x:  }', 'a{--x:{a:b}}', 'a{--x:[;]}', 'a{--x:a!important}', 'a{--x: var(--y,)}', 'a{b:var(--x,)}', 'a{b:var(--x, )}', 'a{b:var( --x , 1px )}',
    '/*! a */', '/* a */', '/*!', '/*', '/*/', '/**/', '/*!*/a{}', 'a{}/*! x */b{}', '<!-- a{} -->', '-->', '<!--',
    '@charset "x";', '@import "a.css";', '@import url(a.css) screen and (color), print;', '@import;', '@import "a" layer;',
    '@media screen{a{b:c}}', '@media (min-width:100px) and (max-width:200px){a{b:c}}', '@media{}', '@media ;', '@media screen;', '@media (a:1/2){}', '@media (a:0/2){}', '@media (a:1.5/2){}', '@media (a:-1/2){}',
    '@supports (a:b){}', '@supports not (a:b){}', '@supports (a:b) and ((c:d) or (e:f)){}', '@supports{}', '@supports selector(a){}',
    '@font-face{font-family:x;src:url(x)}', '@page{}', '@page :first{margin:0}', '@keyframes x{from{a:b}to{c:d}}', '@keyframes x{0%,50%{a:b}}',
    '@x;', '@x{}', '@x{a:b}', '@x{a{b:c}}', '@x y z{a:b}', '@x{@y;}', '@ {}', '@-{}', '@--x{}', '@1{}',
    'a,b{}', 'a , b{}', 'a>b{}', 'a > b{}', 'a+b~c{}', 'a /deep/ b{}', 'a/deep/b{}', 'a  b{}', 'a\nb{}', 'a/**/b{}', 'a /**/ b{}',
    '.a.b#c[d]{}', '[a]{}', '[a=b]{}', '[a="b"]{}', '[a=b i]{}', '[a="b" s]{}', '[a i]{}', '[*|a]{}', '[|a]{}', '[a|b]{}', '[*]{}', '[a|=b]{}', '[a~=b]{}', '[a^=b]{}', '[a$=b]{}', '[a*=b]{}', '[a!=b]{}', '[a==b]{}', '[]{}', '[a{}', '[*|=a]{}',
    ':hover{}', '::before{}', ':not(a){}', ':not(a,b){}', ':not(){}', ':nth-child(2n+1){}', ':nth-child(2n + 1){}', ':nth-child(-n+3){}', ':nth-child(n){}', ':nth-child(+n){}', ':nth-child(odd){}', ':nth-child(EVEN){}',
    ':nth-child(2n+1 of .a){}', ':nth-child(3){}', ':nth-child(+3){}', ':nth-child(-3){}', ':nth-child(n-1){}', ':nth-child(n- 1){}', ':nth-child(n -1){}', ':nth-child(n - 1){}', ':nth-child(-n- 1){}', ':nth-child(2n-){}', ':nth-child(1.5){}', ':nth-child(){}', ':nth-child(n+){}', ':nth-child(- n){}', ':nth-child(+ n){}', ':nth-child(2n+ +1){}', ':nth-child(n-a){}', ':nth-child(-n-a){}', ':nth-child(2n-a){}', ':nth-child(n1){}', ':nth-child(-n1){}', ':nth-child(+5){}', ':nth-child(n+-1){}',
    ':lang(en){}', ':dir(rtl){}', ':has(>a){}', ':is(a,b){}', ':where(){}', ':slotted(a){}', ':unknown(a(b)c){}', '::part(a){}', ':a(){}', ':a(', 'a:{}', '::{}', ':::a{}',
    '50%{}', 'from{}', '.5%{}', '1{}', '.1a{}', 'a.1{}', '#1{}', '#{}', 'a#{}', '*{}', '*|*{}', '|a{}', 'a|{}', 'a|*{}', '**{}', 'a,{}', ',a{}', 'a,,b{}', 'a{', 'a}', '}a{}', 'a{b:c}}', 'a{{}}', 'a{b{c:d}}',
    'a{b}', 'a{b:}', 'a{:b}', 'a{b c:d}', 'a{b:c d}', 'a{b:c;d}', 'a{b:c)}', 'a{b:(c}', 'a{b:[c}', 'a{b:c]}', 'a{b:"c}', "a{b:'c\nd'}", 'a{b:url(c d)}', 'a{b:url(c"d)}', 'a{b:url(c\'d)}', 'a{b:url(c(d)}', 'a{b:url(\\\n)}', 'a{b:url(}', 'a{b:url( a }', 'a{b:url("a" b)}', 'a{b:url("a"}', 'a{b:url(a)}', 'a{b:URL(a)}', 'a{b:url( "a" )}', 'a{b:url(a\\)b)}', 'a{b:url(a\\ b)}', 'a{b:url()}', 'a{b:url("")}', "a{b:url('a\"b')}", 'a{b:url("a\'b")}', 'a{b:url("very long url with spaces and (parens)")}', 'a{b:url(a,b)}',
    'a{b:"a\\"b"}', "a{b:'a\\'b'}", 'a{b:"a\'b"}', "a{b:'a\"b'}", 'a{b:"a\\\nb"}', 'a{b:"\\41"}', 'a{b:"\\41 B"}', 'a{b:"\\0"}', 'a{b:"\\D800"}', 'a{b:"\\110000"}', 'a{b:"\\"}', 'a{b:"\\', 'a{b:"a', 'a{b:"\\1"}', 'a{b:"\\1 a"}', 'a{b:"\\7f"}', 'a{b:"\\9 1"}',
    'a{b:1px+2px}', 'a{b:1px -2px}', 'a{b:1px - 2px}', 'a{b:1px-2px}', 'a{b:calc(1px + 2px)}', 'a{b:calc(1px+-2px)}', 'a{b:calc(-1px - -2px)}', 'a{b:calc( 1px+ 2px )}', 'a{b:a/b}', 'a{b:a / b}', 'a{b:a,b}', 'a{b:a , b}', 'a{b:1e3}', 'a{b:1E3}', 'a{b:1e+3}', 'a{b:1e-3px}', 'a{b:1.e3}', 'a{b:+.5}', 'a{b:-.5em}', 'a{b:1e}', 'a{b:1ex}', 'a{b:1e3e}', 'a{b:1%}', 'a{b:1.5%}', 'a{b:-1%}',
    'a{b:#fff}', 'a{b:#}', 'a{b:# a}', 'a{b:#-}', 'a{b:#\\31}', 'a{b:U+0025-00FF}', 'a{b:u+4??}', 'a{b:U+???????}', 'a{b:u+1e3}', 'a{b:u+1e-3}', 'a{b:U+12345678}', 'a{b:U+0-7F}', 'a{b:U+}', 'a{b:u+a-}', 'a{b:u+-1}', 'a{b:U+0025-00FF?}', 'a{b:U+FFFFFFF}', 'a{b:U+1-1-1}', 'a{b:u+1??-2}',
    'a{b:expression(a(b))}', 'a{b:expression(}', 'a{b:progid:DXImageTransform.Microsoft.gradient(startColorstr=\'#1\', endColorstr=\'#2\')}', 'a{filter:alpha(opacity=50)}', 'a{b:a!}', 'a{b:!}', 'a{b:c !important !important}', 'a{b:c!important x}',
    'a{\\63 olor:red}', 'a{/a:b}', 'a{//a:b}', 'a{#a:b}', 'a{$a:b}', 'a{&a:b}', 'a{+a:b}', 'a{1a:b}', 'a{-a:b}', 'a{--a:b}', 'a{-:b}', 'a{@a:b}', 'a{a :b}', 'a{a: b}', 'a{a:b ;c:d}',
    'a{b:\\}', 'a{b:\\\n}', 'a{b:-->}', 'a{b:<!--}', 'a{b:a\\:b}', 'a{b:é}', 'a{b:"😀"}', 'a{b:𝒜}', 'é{}', '.😀{}', 'a{b:\\1F600}',
    'a\r\n{\r\nb:c\r\n}', 'a\r{\rb:c\r}', 'a\f{\fb:c}', 'a\t{\tb:\tc}', '\n\n\na{b:c}\n', '/*\n\n*/a{b:c}', 'a{b:c}\r\n/*! x\r\n*/',
    'a{b:c}/*', 'a{b:c', 'a{b:c;', 'a{b:', 'a{b', 'a{', 'a', '@', '@a', '@a ', '@media', '@media screen', '@media screen{', '@media screen{a', '@media screen{a{', '@import', '@import "a"', '@supports (a',
    'a{b:c(d e f)}', 'a{b:c(,)}', 'a{b:c(()}', 'a{b:(;)}', 'a{b:[;]}', 'a{b:{}}', 'a{b:c{}}', 'a{b:c;}d{e:f}', '@media x{a{b:c}d}', '@media x{a{b:c}@y;}',
    '.a{color:red;}.b{color:red}', '.a { color: #ff0000; }\n.b { display: block; float: left; }',
];

function fixtureFiles(dir) {
    const out = [];
    for (const name of readdirSync(dir)) {
        const full = join(dir, name);
        if (statSync(full).isDirectory()) {
            out.push(...fixtureFiles(full));
        } else if (name.endsWith('.css') && !name.endsWith('.min.css')) {
            out.push(full);
        }
    }
    return out.sort();
}

export function buildCases(fixturesDir) {
    const cases = [];
    const add = (css, o) => cases.push({ css, o });

    for (const css of hand) {
        add(css, { context: 'stylesheet', positions: true, filename: 'hand.css' });
        add(css, { context: 'stylesheet' });
        add(css, { context: 'declarationList', positions: true });
    }

    for (const file of fixtureFiles(fixturesDir)) {
        const css = readFileSync(file, 'utf8');
        add(css, { context: 'stylesheet', positions: true, filename: file.slice(fixturesDir.length + 1) });
    }

    rnd = mulberry32(2024);
    for (let i = 0; i < 2500; i++) {
        let css = stylesheet();
        if (chance(0.35)) css = mutate(css);
        add(css, i % 2 ? { context: 'stylesheet', positions: true, filename: 'r' + i + '.css' } : { context: 'stylesheet' });
    }

    rnd = mulberry32(77);
    for (let i = 0; i < 800; i++) {
        let css = times(1 + Math.floor(rnd() * 4), () => declaration()).join(chance(0.8) ? ';' : pick([';;', ' ; ', '\n', ';/**/']));
        if (chance(0.3)) css = mutate(css);
        add(css, { context: 'declarationList', positions: chance(0.5) });
    }

    rnd = mulberry32(99);
    const ctxGen = {
        value: () => value(),
        selector: () => selector(),
        selectorList: () => selectorList(),
        declaration: () => declaration(),
        rule: () => rule(),
        block: () => block(0, chance(0.5)),
        atrule: () => atrule(),
        mediaQueryList: () => times(1 + Math.floor(rnd() * 2), () => pick(mediaQueries)).join(','),
        mediaQuery: () => pick(mediaQueries),
        atrulePrelude: () => pick([pick(mediaQueries), '"a.css" screen', 'url(x) (a:b)', '(a:b) and (c:d)', ':first', 'x', 'a b c', '(a: b)', 'not (a:b)'])
    };
    for (let i = 0; i < 1500; i++) {
        const context = pick(Object.keys(ctxGen));
        let css = ctxGen[context]();
        if (chance(0.25)) css = mutate(css);
        const o = { context, positions: chance(0.5) };
        if (context === 'atrulePrelude') o.atrule = pick(['media', 'import', 'supports', 'page', 'font-face', 'x', 'MEDIA', '']);
        add(css, o);
    }

    rnd = mulberry32(5);
    for (let i = 0; i < 400; i++) {
        let css = stylesheet();
        if (chance(0.3)) css = mutate(css);
        const o = { context: 'stylesheet', positions: chance(0.5) };
        if (chance(0.4)) o.parseValue = false;
        if (chance(0.4)) o.parseAtrulePrelude = false;
        if (chance(0.4)) o.parseRulePrelude = false;
        if (chance(0.5)) o.parseCustomProperty = true;
        if (chance(0.2)) { o.offset = 10; o.column = 5; }
        add(css, o);
    }

    return cases;
}

export const keywordSamples = [...new Set([...idents, ...hackProps, 'A', '-WEBKIT-BOX', '-moz-', '-', '--', '-x', '-x-', '--x-y', '-ms-filter', '*-moz-x', '_-webkit-y', '//-o-z', '/a', '/', '//', 'İ', 'Σ', 'ΑΣ', 'aΣb', 'ΑΣ.', '-é-x', '-é', '-𝒜-x', '#-a-b', '+--x', '*--x'])];
