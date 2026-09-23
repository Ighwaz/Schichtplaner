// CSV-Ausgabe und zwei Meldungen, die etwas anderes sagten als sie meinten.
//
// Alle vier Fälle hier fielen bei der Typisierung auf, nicht im Betrieb: sie
// treten nur mit Namen auf, die Anführungszeichen enthalten, oder mit einem
// Plan im alten Format.
import { test, describe } from 'node:test';
import assert from 'node:assert/strict';
import { starteOberflaeche, klick, ruhe, warteBis } from './lade-oberflaeche.mjs';

const TEAM = [
  { name: 'Bauer, Martin', team: 'DE', color: '#4a9eff', icon: '', prefs: {} },
  { name: 'Krüger, Sina', team: 'DE', color: '#22c55e', icon: '', prefs: {} },
];

// Faengt ab, was ein Export in einen Blob schreiben wuerde. jsdom kennt
// createObjectURL nicht, deshalb wird auch das ersetzt.
function fangeAusgabe(o) {
  const teile = [];
  const echt = o.fenster.Blob;
  o.fenster.Blob = function (stuecke, opts) { teile.push(stuecke.join('')); return new echt(stuecke, opts); };
  o.fenster.URL.createObjectURL = () => 'blob:test';
  o.fenster.URL.revokeObjectURL = () => {};
  return teile;
}

describe('CSV-Ausgabe', () => {
  test('ein Feld verdoppelt Anführungszeichen', async () => {
    const o = await starteOberflaeche({ mitarbeiter: TEAM });
    assert.equal(o.fenster.csvFeld('Max "Mo" Muster'), '"Max ""Mo"" Muster"');
    assert.equal(o.fenster.csvFeld('ohne'), '"ohne"');
    assert.equal(o.fenster.csvFeld(null), '""');
    assert.equal(o.fenster.csvFeld(3), '"3"');
  });

  test('die Tagesliste maskiert Namen mit Anführungszeichen', async () => {
    const name = 'Max "Mo" Muster';
    const o = await starteOberflaeche({
      mitarbeiter: [{ name, team: 'DE', color: '#4a9eff', icon: '', prefs: {} }],
      schichten: {
        '2026-01-05': { frueh: [], normal: [], spaet: [], rufbereitschaft: [name] },
      },
    });
    const teile = fangeAusgabe(o);
    o.$('#rufkw-year').value = '2026';
    o.fenster.exportRufCSV();
    assert.equal(teile.length, 1, 'es wurde nichts ausgegeben');
    assert.match(teile[0], /"Max ""Mo"" Muster"/,
      'die Anführungszeichen im Namen sind nicht verdoppelt');
  });

  test('der Wochenplan stürzt bei einem alten Eintrag nicht ab', async () => {
    // Im alten Format steht ein einzelner Name als Zeichenkette statt als
    // Liste. Vorher lief der Export darauf in names.forEach und warf.
    const o = await starteOberflaeche({ mitarbeiter: TEAM });
    o.api.zustand.ruf_kw = { '2026-W03': 'Anna "A"' };
    await o.fenster.refreshData();
    klick(o.$('.vtab[data-view="ruf"]'));   // füllt die Jahresauswahl
    await ruhe();

    const teile = fangeAusgabe(o);
    o.$('#rufkw-year').value = '2026';
    o.fenster.exportRufKWCSV();
    assert.equal(teile.length, 1, 'es wurde nichts ausgegeben');
    assert.match(teile[0], /KW 3,.*,"Anna ""A"""/, teile[0].split('\n').slice(0, 5).join(' | '));
  });
});

describe('Meldungen, die stimmen', () => {
  test('ein verdrehter Zeitraum sagt, dass er verdreht ist', async () => {
    const o = await starteOberflaeche({ mitarbeiter: TEAM });
    klick(o.$('.vtab[data-view="ruf"]'));
    await ruhe();
    o.fenster.openRufReihum();
    await ruhe();

    o.$('#reihum-von').value = '40';
    o.$('#reihum-bis').value = '10';
    o.fenster.renderReihumVorschau();

    const text = o.$('#reihum-vorschau').textContent;
    assert.match(text, /liegt hinter/, text);
    assert.doesNotMatch(text, /schon vergeben/,
      'der verdrehte Zeitraum sieht aus wie ein voller Plan');
  });

  test('nach dem Löschen holt Rückgängig niemanden zurück, den es nicht gibt', async () => {
    // saveProfile leert beim Umbenennen Undo-Stapel und kopierten Tag;
    // deleteMitarbeiter tat es nicht, und ein Schritt zurück brachte die
    // Chips der gelöschten Person wieder.
    const o = await starteOberflaeche({
      mitarbeiter: TEAM,
      schichten: {
        '2026-09-03': { frueh: ['Bauer, Martin'], normal: [], spaet: [], rufbereitschaft: [] },
      },
    });
    // Ein zweiter Schritt, damit überhaupt etwas im Undo-Stapel liegt.
    o.fenster.waehleSchicht('spaet');
    await o.fenster.applySchicht(['2026-09-04'], 'add', 'spaet', 'Krüger, Sina');
    o.fenster.pushUndo();

    await o.fenster.deleteMitarbeiter('Bauer, Martin');
    await warteBis(() => !o.$$('.chip').some(c => c.textContent.includes('Bauer')), 'das Entfernen');

    o.fenster.doUndo();
    await ruhe();
    assert.ok(!o.$$('.chip').some(c => c.textContent.includes('Bauer')),
      'Rückgängig hat die gelöschte Person zurückgeholt');
  });
});
