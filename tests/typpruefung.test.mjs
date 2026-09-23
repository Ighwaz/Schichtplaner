// Stimmen die Typen, die die Oberflaeche tsc erzaehlt?
//
// dom.eingabe(id) sagt tsc: "hier steht ein <input>". Stimmt das nicht, hat
// tsc nichts geprueft, sondern etwas Falsches geglaubt - und .value auf einem
// <div> ist dann ein Fehler, den kein Compiler mehr findet. Diese Tests
// gleichen deshalb jeden Griff mit festem id gegen das Element im Markup ab.
//
// Dazu ein paar Zusicherungen ueber die Typpruefung selbst, damit sie nicht
// still aufweicht: kein @ts-ignore, der Regelkern ohne DOM.
import { test, describe } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

const hier = dirname(fileURLToPath(import.meta.url));
const html = readFileSync(join(hier, '..', 'frontend', 'index.html'), 'utf8').replace(/\r\n/g, '\n');

// Welcher Griff welches Element verspricht.
const GRIFFE = {
  eingabe: 'input',
  auswahl: 'select',
  textfeld: 'textarea',
  knopf: 'button',
};

// Alle Elemente mit diesem id - im Markup und in Template-Strings. Das Tag
// ist das naechste "<name" vor dem Attribut.
function elementeMitId(id) {
  const muster = new RegExp(`\\bid=\\\\?(["'])${id.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}\\\\?\\1`, 'g');
  const tags = [];
  for (const treffer of html.matchAll(muster)) {
    const davor = html.slice(0, treffer.index);
    const auf = davor.lastIndexOf('<');
    const tag = davor.slice(auf + 1).match(/^([a-zA-Z][\w-]*)/);
    if (tag) tags.push(tag[1].toLowerCase());
  }
  return tags;
}

// Jeder Aufruf eines Griffs mit festem id: dom.eingabe('x') oder "x".
function aufrufe() {
  const raus = [];
  const muster = /\bdom\.(eingabe|auswahl|textfeld|knopf)\(\s*(['"])([^'"`]+)\2\s*\)/g;
  for (const t of html.matchAll(muster)) {
    const zeile = html.slice(0, t.index).split('\n').length;
    raus.push({ griff: t[1], id: t[3], zeile });
  }
  return raus;
}

describe('dom-Griffe passen zum Markup', () => {
  const alle = aufrufe();

  test('es gibt Griffe mit festem id - sonst prueft dieser Test nichts', () => {
    assert.ok(alle.length > 20, `nur ${alle.length} Griffe gefunden - hat sich die Schreibweise geaendert?`);
  });

  test('jeder Griff zeigt auf ein Element, das es gibt', () => {
    const fehlend = alle.filter(a => elementeMitId(a.id).length === 0)
      .map(a => `Zeile ${a.zeile}: dom.${a.griff}('${a.id}') - kein Element mit diesem id`);
    assert.deepEqual(fehlend, []);
  });

  test('jeder Griff verspricht das Tag, das dort steht', () => {
    const falsch = [];
    for (const a of alle) {
      const tags = elementeMitId(a.id);
      const erwartet = GRIFFE[a.griff];
      for (const tag of tags) {
        if (tag !== erwartet)
          falsch.push(`Zeile ${a.zeile}: dom.${a.griff}('${a.id}') erwartet <${erwartet}>, im Markup steht <${tag}>`);
      }
    }
    assert.deepEqual(falsch, []);
  });

  test('ein id steht nur an einem Element - sonst ist der Griff Glueckssache', () => {
    const doppelt = [...new Set(alle.map(a => a.id))]
      .map(id => ({ id, tags: elementeMitId(id) }))
      .filter(e => e.tags.length > 1)
      .map(e => `${e.id}: ${e.tags.length}× (${e.tags.join(', ')})`);
    assert.deepEqual(doppelt, []);
  });
});

describe('Typpruefung bleibt ehrlich', () => {
  test('kein @ts-ignore, @ts-expect-error oder @ts-nocheck', () => {
    // Wer einen Fehler zum Schweigen bringt, statt ihn zu beheben, soll es
    // hier begruenden muessen - als Ausnahme in diesem Test, nicht still.
    const stellen = [...html.matchAll(/@ts-(ignore|expect-error|nocheck)/g)]
      .map(t => `Zeile ${html.slice(0, t.index).split('\n').length}: @ts-${t[1]}`);
    assert.deepEqual(stellen, []);
  });

  test('kein Cast auf any', () => {
    const stellen = [...html.matchAll(/@type\s*\{\s*any\s*\}/g)]
      .map(t => `Zeile ${html.slice(0, t.index).split('\n').length}`);
    assert.deepEqual(stellen, []);
  });

  test('der Regelkern greift nicht ins DOM', () => {
    // tsconfig.regelkern.json laedt nur die ES-Bibliothek, tsc wuerde also
    // meckern. Dieser Test sagt es frueher und ohne tsc.
    const block = html.match(/<script id="regelkern">\n([\s\S]*?)\n<\/script>/)[1]
      .replace(/\/\*[\s\S]*?\*\//g, '').replace(/\/\/.*$/gm, '');
    const griffe = [...block.matchAll(/\b(document|window|localStorage|fetch|navigator)\b/g)].map(t => t[1]);
    assert.deepEqual(griffe, []);
  });
});
