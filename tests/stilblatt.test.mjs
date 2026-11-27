// Fehler im Stilblatt, die kein DOM-Test sieht.
//
// Beide Fehler hier waren jahrelang da und fielen niemandem auf, weil sie
// nichts kaputtmachen: etwas sieht anders aus, oder man sieht es gar nicht.
// Ein Test, der nur Knoten zaehlt, geht daran vorbei - und ein Mensch merkt
// nicht, dass ein Tooltip fehlt, den er nie gesehen hat.
//
// jsdom rechnet die Kaskade mit, also lassen sich beide als Verhalten
// pruefen statt als Zeichenkette im Quelltext.
import { test, describe } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import { starteOberflaeche } from './lade-oberflaeche.mjs';

const hier = dirname(fileURLToPath(import.meta.url));
const html = readFileSync(join(hier, '..', 'frontend', 'index.html'), 'utf8').replace(/\r\n/g, '\n');

// Der <style>-Block ohne den Druckteil. Papier hat eigene, bewusst dichtere
// Groessen und steht darum nicht unter derselben Regel.
function stilOhneDruck() {
  const stil = html.slice(html.indexOf('<style>'), html.indexOf('</style>'));
  const di = stil.indexOf('@media print');
  if (di < 0) return stil;
  let tiefe = 0, ende = di;
  for (let k = stil.indexOf('{', di); k < stil.length; k++) {
    if (stil[k] === '{') tiefe++;
    else if (stil[k] === '}' && --tiefe === 0) { ende = k + 1; break; }
  }
  return stil.slice(0, di) + stil.slice(ende);
}

describe('Der Tooltip ist sichtbar, wenn er gezeigt wird', () => {
  // Frueher stand das Element zweimal im Stilblatt: .tooltip (Deckkraft 0,
  // .show macht 1) und weiter unten #tooltip mit display:none. Ein id wiegt
  // schwerer als zwei Klassen, also gewann display:none - showTooltip baute
  // den Inhalt, setzte .show, platzierte das Kaestchen, und zu sehen war
  // nichts.
  test('.show hebt das Verstecken wirklich auf', async () => {
    const o = await starteOberflaeche();
    const tt = o.$('#tooltip');
    const vorher = o.fenster.getComputedStyle(tt);
    assert.equal(vorher.opacity, '0', 'ohne .show soll er unsichtbar sein');

    tt.classList.add('show');
    const nachher = o.fenster.getComputedStyle(tt);
    assert.notEqual(nachher.display, 'none',
      'eine andere Regel haelt display:none fest - .show kann nur die Deckkraft');
    assert.equal(nachher.opacity, '1');
  });

  test('der Weg über showTooltip zeigt den Tag', async () => {
    const heute = new Date();
    const tag = `${heute.getFullYear()}-${String(heute.getMonth() + 1).padStart(2, '0')}-15`;
    const o = await starteOberflaeche({
      mitarbeiter: [{ name: 'Bauer, Martin', team: 'DE', color: '#4a9eff', icon: '', prefs: {} }],
      schichten: { [tag]: { frueh: ['Bauer, Martin'], normal: [], spaet: [], rufbereitschaft: [] } },
    });
    const zelle = o.$(`.day-cell[data-key="${tag}"]`);
    assert.ok(zelle, `die Zelle ${tag} fehlt`);

    o.fenster.showTooltip(zelle, tag);
    const tt = o.$('#tooltip');
    const stil = o.fenster.getComputedStyle(tt);
    assert.notEqual(stil.display, 'none', 'der Tooltip bleibt unsichtbar');
    assert.equal(stil.opacity, '1');
    assert.match(tt.textContent, /Bauer, Martin/, 'der Inhalt fehlt');

    o.fenster.hideTooltip();
    assert.equal(o.fenster.getComputedStyle(tt).opacity, '0', 'er geht nicht wieder weg');
  });
});

describe('Die Schriften werden überhaupt geladen', () => {
  // @import muss vor jeder anderen Regel stehen. Stand es dahinter - hier
  // hinter sechs @keyframes und einem @media-Block -, ist die Zeile
  // unzulaessig und der Browser verwirft sie stillschweigend. Gemessen war
  // die Textbreite in "Inter" dann auf das Zeichen gleich der in einer frei
  // erfundenen Schriftfamilie: es lief immer die Ersatzschrift.
  test('das @import steht vor allen anderen Regeln', async () => {
    const o = await starteOberflaeche();
    const blatt = o.dokument.styleSheets[0];
    const regeln = [...blatt.cssRules];
    const importe = regeln
      .map((r, i) => ({ r, i }))
      .filter(x => x.r.constructor.name === 'CSSImportRule');
    assert.equal(importe.length, 1, 'genau ein @import erwartet');
    assert.equal(importe[0].i, 0,
      `das @import steht an Stelle ${importe[0].i} - davor duerfen keine Regeln stehen, ` +
      'sonst verwirft der Browser es');
  });

  test('hell ist die Vorgabe, schon im ersten Bild', async () => {
    // Zwei Stellen entscheiden das: das Attribut im <html>-Tag malt das
    // erste Bild, bevor init() laeuft, und init() setzt es danach aus dem
    // gemerkten Wert. Standen die beiden auf verschiedenen Themen, blitzte
    // beim Start kurz das falsche auf.
    const o = await starteOberflaeche();
    assert.equal(o.dokument.documentElement.dataset.theme, 'light');
    assert.match(o.$('#btn-theme').textContent, /Theme/);
  });

  test('ohne Netz greift eine Oberflächenschrift, nicht die Vorgabe des Browsers', async () => {
    // Das @import holt die Schrift aus dem Netz. Ein Arbeitsplatz ohne
    // Verbindung bekommt sie also nicht - und fiel vorher direkt auf
    // sans-serif, was in WebView2 Arial heisst. Dazwischen gehoert die
    // Schrift des Systems, damit der unverbundene Start wie eine
    // Entscheidung aussieht und nicht wie ein Unfall.
    const o = await starteOberflaeche();
    const schrift = o.fenster.getComputedStyle(o.dokument.body).fontFamily;
    assert.match(schrift, /Inter/, 'Inter fehlt');
    assert.match(schrift, /system-ui|Segoe UI/,
      'zwischen Inter und sans-serif fehlt eine Schrift, die es auf dem Rechner gibt');
    assert.match(schrift, /sans-serif/, 'die allgemeine Ersatzschrift fehlt als letzte Stufe');
  });
});

// Achtzehn verschiedene Schriftgroessen standen im Stilblatt, zwoelf davon
// zwischen 7.5 und 13 Pixeln. Hierarchie entsteht aus Verhaeltnissen, und
// 11 zu 11.5 ist keines - das war eine Reihe von Einzelentscheidungen, kein
// System. Diese Tests halten die fuenf Stufen fest, damit es nicht wieder
// zerfasert: wer eine Groesse braucht, nimmt eine Marke oder begruendet die
// Ausnahme hier.
describe('Die Schriftskala bleibt eine Skala', () => {
  // Emoji in einem Avatar und im Auswahlgitter sind Grafik, keine Schrift.
  const GRAFIK = ['.prof-av-big', '.emoji-opt'];

  test('die fünf Stufen sind erklärt und haben Abstand', () => {
    const stufen = [...html.matchAll(/--t([1-5]):([0-9.]+)px/g)]
      .map(t => ({ nr: Number(t[1]), px: Number(t[2]) }))
      .sort((a, b) => a.nr - b.nr);
    assert.equal(stufen.length, 5, 'es sollen genau fünf Stufen sein');
    for (let i = 1; i < stufen.length; i++) {
      const faktor = stufen[i].px / stufen[i - 1].px;
      assert.ok(faktor >= 1.1,
        `--t${stufen[i].nr} (${stufen[i].px}px) steht zu dicht an --t${stufen[i - 1].nr} ` +
        `(${stufen[i - 1].px}px): Faktor ${faktor.toFixed(2)}`);
    }
  });

  test('keine feste Schriftgröße außerhalb der Skala', () => {
    const stil = stilOhneDruck();
    const roh = [];
    for (const m of stil.matchAll(/([^{}]*)\{([^{}]*)\}/g)) {
      const selektor = m[1].split('*/').pop().trim();
      if (GRAFIK.some(g => selektor.includes(g))) continue;
      for (const f of m[2].matchAll(/font-size:([0-9.]+)px/g))
        roh.push(`${selektor || '?'} → ${f[1]}px`);
    }
    assert.deepEqual(roh, []);
  });

  test('die Ausnahmen gibt es wirklich - sonst prüft die Liste nichts', () => {
    for (const g of GRAFIK)
      assert.ok(html.includes(g), `${g} steht nicht mehr im Markup - Ausnahme streichen`);
  });
});

// Farben nur noch als Marke, und die Schrift muss lesbar sein.
//
// Beides war verletzt, und zwar systematisch: 58 Farbliterale standen in
// themenneutralen Regeln, und jedes einzelne war ein Dunkel-Wert - sie
// entstanden, als es nur das dunkle Thema gab. rgba(248,113,113) hat auf
// Weiss 2.77:1. Seit hell die Vorgabe ist, war das die Normalansicht.
describe('Das Farbsystem bleibt ein System', () => {
  const marken = roh => Object.fromEntries(
    [...roh.matchAll(/--([a-z0-9-]+):\s*(#[0-9a-fA-F]{6})\b/g)].map(m => [m[1], m[2]]));

  // Jede Marke, die ein Block setzt - auch die ohne Hexwert. Die Kommentare
  // muessen vorher raus: dort stehen Markennamen als Erklaerung, nicht als
  // Deklaration.
  const namen = roh => [...roh.replace(/\/\*[\s\S]*?\*\//g, '').matchAll(/--([a-z0-9-]+)\s*:/g)]
    .map(m => m[1]);

  function block(start) {
    const i = html.indexOf(start);
    return html.slice(i, html.indexOf('\n}', i));
  }

  function kontrast(a, b) {
    const leucht = h => {
      const teile = [1, 3, 5].map(i => parseInt(h.slice(i, i + 2), 16) / 255)
        .map(c => (c <= 0.03928 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4)));
      return 0.2126 * teile[0] + 0.7152 * teile[1] + 0.0722 * teile[2];
    };
    const [hi, lo] = [leucht(a), leucht(b)].sort((x, y) => y - x);
    return (hi + 0.05) / (lo + 0.05);
  }

  for (const [thema, start] of [['hell', ':root{'], ['dunkel', '[data-theme="dark"]{']]) {
    test(`jede Schriftfarbe ist im ${thema}en Thema lesbar`, () => {
      const m = { ...marken(block(':root{')), ...marken(block(start)) };
      const schlecht = [];
      for (const t of ['text', 'text2', 'text3'])
        for (const g of ['surface', 'surface2', 'surface3', 'bg']) {
          const k = kontrast(m[t], m[g]);
          if (k < 4.5) schlecht.push(`--${t} (${m[t]}) auf --${g} (${m[g]}): ${k.toFixed(2)}:1`);
        }
      assert.deepEqual(schlecht, [], 'unter 4.5:1');
    });
  }

  test('keine Farbe außerhalb der Marken', () => {
    // Der Druckblock und die beiden Markenblöcke dürfen Werte tragen; alles
    // andere nimmt var(--…) oder color-mix(), damit es dem Thema folgt.
    let rest = stilOhneDruck();
    for (const start of [':root{', '[data-theme="dark"]{']) rest = rest.replace(block(start), '');
    const roh = [
      ...[...rest.matchAll(/#[0-9a-fA-F]{6}\b/g)].map(m => m[0]),
      ...[...rest.matchAll(/rgba?\(\s*\d+\s*,\s*\d+\s*,\s*\d+/g)].map(m => m[0] + ',…)'),
    ];
    assert.deepEqual(roh, []);
  });

  test('es gibt keine Marke, die niemand benutzt', () => {
    // Alle Marken, nicht nur die mit Hexwert: --chip-alpha und --radius-cell
    // standen jahrelang ungelesen da, weil die Probe vorher nur nach #rrggbb
    // suchte. --radius-cell hielt dabei 11px fest, waehrend die Zelle
    // 9px benutzte - eine Marke, die niemand liest, driftet.
    const alle = [...namen(block(':root{')), ...namen(block('[data-theme="dark"]{'))];
    const tot = alle.filter(t => !html.includes('var(--' + t + ')')
      && !html.includes('var(--' + t + ',') && !html.includes('var(--' + t + ' '));
    assert.deepEqual([...new Set(tot)], []);
  });
});

// Jede Regel braucht einen Nutzer.
//
// Sechsundzwanzig Klassennamen standen im Stilblatt, die das Markup nie
// vergibt. Das ist nicht nur Gewicht: .ruf-person-chip sieht dem lebenden
// .rufkw-person-chip zum Verwechseln aehnlich, .day-act-btn dem lebenden
// .dab, und .ruf-table war die Tabelle einer Ansicht, die es nicht mehr
// gibt. Wer dort etwas aendert, aendert nichts - und sucht den Fehler dann
// an der falschen Stelle.
describe('Jede Klasse im Stilblatt wird auch vergeben', () => {
  // Namen, die nie als Ganzes im Quelltext stehen, weil sie aus Stuecken
  // gebaut werden: badge-${team}, st-${stand}, s-${['f','n','s','r'][i]}.
  const GEBAUT = ['badge-de', 'badge-in', 'st-ok', 'st-offen', 's-f', 's-n', 's-s', 's-r'];

  test('kein Klassenname ohne Stelle, die ihn setzt', () => {
    const a = html.indexOf('<style>'), e = html.indexOf('</style>');
    const stil = html.slice(a, e).replace(/\/\*[\s\S]*?\*\//g, '');
    const rest = html.slice(0, a) + html.slice(e);   // Markup und Skripte
    const namen = new Set();
    for (const r of stil.matchAll(/([^{}]*)\{[^{}]*\}/g))
      for (const k of r[1].matchAll(/\.([A-Za-z][\w-]*)/g)) namen.add(k[1]);
    assert.ok(namen.size > 300, `nur ${namen.size} Klassen gefunden - greift der Ausschnitt noch?`);

    // Der Name muss als ganzes Wort dastehen - Klassennamen bestehen nur
    // aus [A-Za-z0-9_-], also ist die Grenze genau dieser Satz. Ausgeschrieben
    // statt als \w, weil ein Escape in einer Zeichenkette leicht verloren
    // geht und die Probe dann stillschweigend etwas anderes prueft.
    const RAND = '[^A-Za-z0-9_-]';
    const tot = [...namen].filter(k =>
      !GEBAUT.includes(k) && !new RegExp(RAND + k + RAND).test(rest)).sort();
    assert.deepEqual(tot, []);
  });

  test('die gebauten Namen werden wirklich gebaut - sonst prüft die Liste nichts', () => {
    for (const k of GEBAUT) {
      // Der Stamm vor dem letzten Bindestrich muss im Quelltext an einem
      // Platzhalter kleben: class="… badge-${…}".
      const stamm = k.slice(0, k.lastIndexOf('-') + 1);
      assert.ok(html.includes(stamm + '${'),
        `${k} wird nirgends zusammengesetzt - raus aus der Liste`);
    }
  });
});
