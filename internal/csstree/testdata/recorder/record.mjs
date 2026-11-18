// Records css-tree 2.2.0 behaviour (through csso 5.0.5's fork) for the
// differential tests of package csstree.
//
// Usage, from this directory, with a checkout of csso 5.0.5 (and its
// node_modules) at <csso>:
//
//   docker run --rm -v <csso>:/w:ro -v "$PWD/..":/s -w /s/recorder node:24 \
//       node record.mjs /w /s/parse.json.gz
//
// --dump as a third argument writes uncompressed JSON with full AST dumps
// and source maps instead of digests, for inspecting a divergence.
import { createHash } from 'crypto';
import { writeFileSync } from 'fs';
import { gzipSync } from 'zlib';
import { buildCases, keywordSamples } from './corpus.mjs';

const workDir = process.argv[2];
const outFile = process.argv[3];
const dumpMode = process.argv[4] === '--dump';
const syntax = await import(workDir + '/lib/syntax.js');
const csstree = await import(workDir + '/node_modules/css-tree/lib/index.js');
const nodes = await import(workDir + '/node_modules/css-tree/lib/syntax/node/index.js');
const { List, keyword, property, tokenize } = csstree;

const sha = s => createHash('sha1').update(s, 'utf8').digest('hex').slice(0, 16);
const q = s => '"' + s.replace(/[\\"]/g, m => '\\' + m) + '"';

function dumpLoc(loc) {
    if (!loc) return '-';
    return `${loc.source}:${loc.start.offset},${loc.start.line},${loc.start.column}-${loc.end.offset},${loc.end.line},${loc.end.column}`;
}

function dumpValue(v) {
    if (v === null || v === undefined) return 'null';
    if (typeof v === 'string') return q(v);
    if (typeof v === 'boolean') return String(v);
    if (v instanceof List) return '[' + v.toArray().map(dumpValue).join(' ') + ']';
    if (typeof v === 'object' && typeof v.type === 'string') return dump(v);
    return '?' + typeof v;
}

function dump(node) {
    let out = '(' + node.type + '@' + dumpLoc(node.loc);
    for (const key of Object.keys(nodes[node.type].structure)) {
        out += ' ' + key + '=' + dumpValue(node[key]);
    }
    return out + ')';
}

const errLine = e => `${e.message}@${e.offset},${e.line},${e.column}`;

function walkTrace(ast, options) {
    const out = [];
    const ctx = c => (c.stylesheet ? 'S' : '') + (c.atrule ? 'A' : '') + (c.atrulePrelude ? 'P' : '') + (c.rule ? 'R' : '') +
        (c.selector ? 'L' : '') + (c.block ? 'B' : '') + (c.declaration ? 'D' : '') + (c.function ? 'F' : '');
    syntax.walk(ast, {
        ...options,
        enter(node, item, list) {
            out.push('>' + node.type + ':' + ctx(this) + (item ? 'i' : '') + (list ? 'l' : ''));
        },
        leave(node) {
            out.push('<' + node.type + ':' + ctx(this));
        }
    });
    return out.join(' ');
}

function tokensOf(ast) {
    const tokens = syntax.generate(ast, {
        decorator(handlers) {
            const tokens = [];
            let curNode = null;
            return {
                ...handlers,
                node(node) {
                    const tmp = curNode;
                    curNode = node;
                    handlers.node.call(this, node);
                    curNode = tmp;
                },
                emit(value, type, auto) {
                    tokens.push(type + ':' + value + ':' + (auto ? '-' : curNode.type));
                },
                result() {
                    return tokens;
                }
            };
        }
    });
    return tokens.join('\u0001');
}

const cases = buildCases(workDir + '/fixtures');
const records = [];

for (const { css, o } of cases) {
    const rec = { css, o };
    const recovered = [];
    const toks = [];
    tokenize(css, (type, start, end) => toks.push(`${type}:${start}:${end}`));
    rec.tokens = sha(toks.join(' '));

    let ast;
    const comments = [];
    try {
        ast = syntax.parse(css, { ...o, onParseError: e => recovered.push(errLine(e)), onComment: (value, loc) => comments.push(q(value) + '@' + dumpLoc(loc)) });
    } catch (e) {
        if (typeof e.offset !== 'number') throw e;
        rec.error = errLine(e);
        rec.formatted = sha(e.formattedMessage);
    }
    if (recovered.length) rec.recovered = recovered.join('\n');
    if (comments.length) rec.comments = sha(comments.join(' '));

    if (ast) {
        const astDump = dump(ast);
        rec.ast = dumpMode ? astDump : sha(astDump);
        rec.css2 = syntax.generate(ast);
        rec.gentokens = sha(tokensOf(ast));
        if (o.positions) {
            const filename = o.filename || '<unknown>';
            const res = syntax.generate(ast, { sourceMap: true });
            res.map._file = filename;
            res.map.setSourceContent(filename, css);
            if (res.css !== rec.css2) throw new Error('sourcemap css differs');
            rec.map = dumpMode ? res.map.toString() : sha(res.map.toString());
        }
        if (records.length % 4 === 0) {
            const copy = csstree.clone(ast);
            const trace = [];
            let count = 0;
            syntax.walk(copy, {
                reverse: records.length % 8 === 0,
                enter(node, item, list) {
                    count++;
                    trace.push(node.type + (this.rule ? 'R' : '') + (this.declaration ? 'D' : ''));
                    if (item && count % 7 === 0) {
                        list.insertData({ type: 'Raw', loc: null, value: 'R' + count }, item);
                    }
                    if (item && count % 3 === 0) {
                        list.remove(item);
                        return this.skip;
                    }
                    if (count % 11 === 0 && item && item.next) {
                        list.remove(item.next);
                    }
                },
                leave(node, item, list) {
                    trace.push('/' + node.type);
                    if (item && count % 5 === 0) {
                        list.replace(item, List.createItem({ type: 'Raw', loc: null, value: 'L' + count }));
                    }
                }
            });
            rec.mutate = sha(trace.join(' ') + '\n' + syntax.generate(copy));
        }
        if (!o.positions || records.length % 3 === 0) {
            rec.walk = sha([
                walkTrace(ast, {}),
                walkTrace(ast, { reverse: true }),
                walkTrace(ast, { visit: 'Rule' }),
                walkTrace(ast, { visit: 'Declaration', reverse: true }),
                walkTrace(ast, { visit: 'Atrule' }),
                walkTrace(ast, { visit: 'Selector' })
            ].join('\n'));
        }
    }
    records.push(rec);
}

const names = keywordSamples.map(name => {
    const k = keyword(name);
    const p = property(name);
    return {
        name,
        keyword: [k.basename, k.name, k.prefix, k.vendor, k.custom],
        property: [p.basename, p.name, p.hack, p.vendor, p.prefix, p.custom]
    };
});

const escapes = [];
for (const s of ['', 'a', 'a b', 'a"b', "a'b", 'a\\b', 'a\nb', '\0', '\x01a', '\x1f1', '\x7f', 'é', '😀', '(x)', 'a\tb', '\x01 ', '-', '1a', '-1', '--', 'a-b_c', 'a.b', '\x01\x02']) {
    escapes.push({
        s,
        string: csstree.string.encode(s),
        stringApos: csstree.string.encode(s, true),
        url: csstree.url.encode(s),
        ident: csstree.ident.encode(s)
    });
}
for (const s of ['a', '"a"', "'a'", '"a', '"\\"', '"\\41 x"', '"a\\\nb"', 'url(a)', 'url( a )', 'url(a\\)b)', 'url(\\)', 'url(a\\', 'a\\', 'a\\31 b', '\\', '"a\\"', 'url(\\\nx)', '\\0', 'a\\\r\nb']) {
    escapes.push({
        s,
        decodeString: csstree.string.decode(s),
        decodeUrl: s.startsWith('url(') ? csstree.url.decode(s) : null,
        decodeIdent: csstree.ident.decode(s)
    });
}

function mulberry32(a) {
    return function() {
        a |= 0; a = a + 0x6D2B79F5 | 0;
        let t = Math.imul(a ^ a >>> 15, 1 | a);
        t = t + Math.imul(t ^ t >>> 7, 61 | t) ^ t;
        return ((t ^ t >>> 14) >>> 0) / 4294967296;
    };
}

const lists = [];
const modes = ['forEach', 'forEachRight', 'nextUntil', 'prevUntil', 'reduce', 'reduceRight'];
for (let seed = 1; seed <= 600; seed++) {
    const rnd = mulberry32(seed);
    const mode = modes[seed % modes.length];
    const mk = name => ({ type: 'Identifier', name });
    const list = new List().fromArray(Array.from({ length: 1 + seed % 9 }, (_, i) => mk('n' + i)));
    const visited = [];
    let counter = 0;
    let calls = 0;
    const cb = (data, item) => {
        visited.push(data.name);
        if (++calls > 40) return true;
        const op = Math.floor(rnd() * 10);
        switch (op) {
            case 0: list.remove(item); break;
            case 1: if (item.next) list.remove(item.next); break;
            case 2: if (item.prev) list.remove(item.prev); break;
            case 3: list.insertData(mk('x' + counter++), item); break;
            case 4: list.appendData(mk('x' + counter++)); break;
            case 5: list.prependData(mk('x' + counter++)); break;
            case 6: list.replace(item, List.createItem(mk('x' + counter++))); break;
            case 7: list.insertList(new List().fromArray([mk('x' + counter++), mk('x' + counter++)]), item.next); break;
            case 8: list.replace(item, new List().fromArray([mk('x' + counter++), mk('x' + counter++)])); break;
        }
        return rnd() < 0.1;
    };
    switch (mode) {
        case 'forEach': list.forEach(cb); break;
        case 'forEachRight': list.forEachRight(cb); break;
        case 'nextUntil': list.nextUntil(list.head && list.head.next, cb); break;
        case 'prevUntil': list.prevUntil(list.tail, cb); break;
        case 'reduce': list.reduce((acc, data, item) => cb(data, item) || acc, false); break;
        case 'reduceRight': list.reduceRight((acc, data, item) => cb(data, item) || acc, false); break;
    }
    const back = [];
    for (let c = list.tail; c !== null; c = c.prev) back.push(c.data.name);
    lists.push({ seed, mode, visited: visited.join(' '), final: list.toArray().map(d => d.name).join(' '), back: back.join(' ') });
}

const out = JSON.stringify({ cases: records, names, escapes, lists });
writeFileSync(outFile, dumpMode ? out : gzipSync(out, { level: 9 }));
console.log(records.length, 'cases,', records.filter(r => r.error).length, 'errors,', records.filter(r => r.recovered).length, 'with recovered errors');
