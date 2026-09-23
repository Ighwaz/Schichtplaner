// Die Anleitung aus dem Mehr-Menü: dass man sie aufbekommt, dass man sich
// darin bewegen kann - und vor allem, dass sie nicht veraltet.
//
// Der letzte Teil ist der eigentliche Grund für diese Datei. Eine Anleitung
// rottet leise: jemand baut eine Ansicht dazu, ein Menü bekommt einen neuen
// Eintrag, und der Text bleibt stehen. Die beiden Vollständigkeitstests
// weiter unten lassen das nicht durchgehen - wer etwas hinzufügt, muss die
// Anleitung anfassen, sonst schlägt der Lauf fehl.
import { test, describe } from 'node:test';
import assert from 'node:assert/strict';
import { starteOberflaeche, klick, ruhe } from './lade-oberflaeche.mjs';

const taste = (o, key, ziel = null) =>
  (ziel || o.dokument.body).dispatchEvent(
    new o.fenster.KeyboardEvent('keydown', { key, bubbles: true }));

const offen = o => !o.$('#hilfe-modal').classList.contains('hidden');
const text = o => o.$('#hilfe-inhalt').textContent;
const sichtbareKapitel = o =>
  o.$$('#hilfe-inhalt section').filter(a => !a.classList.contains('weg')).map(a => a.id);

// Jeder Knopf im Mehr-Menü und das Stichwort, unter dem die Anleitung ihn
// erklärt. Kommt ein Knopf dazu, fällt er hier auf, bevor er unerklärt bleibt.
const MENUE = {
  'btn-ics': 'ICS',
  'btn-print': 'Drucken',
  'btn-export-data': 'Sicherung',
  'btn-refresh': 'Neu laden',
  'btn-folder': 'Ordner wechseln',
  'btn-zoom-out': 'Anzeigegröße',
  'btn-zoom-in': 'Anzeigegröße',
  'btn-theme': 'Theme',
  'btn-hilfe': null,   // die Anleitung selbst - sie erklärt sich nicht noch einmal
};

async function oeffneAnleitung(o) {
  klick(o.$('#btn-more'));
  klick(o.$('#btn-hilfe'));
  await ruhe();
  return o;
}

describe('Anleitung aufschlagen', () => {
  test('das Mehr-Menü führt hinein', async () => {
    const o = await starteOberflaeche();
    assert.ok(!offen(o), 'sie darf nicht von selbst aufgehen');
    await oeffneAnleitung(o);
    assert.ok(offen(o), 'nach dem Klick steht sie offen');
    assert.ok(o.$('#more-menu').classList.contains('hidden'), 'das Menü schließt sich dabei');
  });

  test('F1 geht auch aus einem Eingabefeld heraus', async () => {
    const o = await starteOberflaeche();
    taste(o, 'F1', o.$('#sb-search'));
    await ruhe();
    assert.ok(offen(o), 'F1 ist keine Taste, die in ein Feld schreibt');
  });

  test('sie öffnet beim Schnellstart', async () => {
    const o = await oeffneAnleitung(await starteOberflaeche());
    assert.equal(o.$('#hilfe-nav button.aktiv').dataset.zu, 'hilfe-start');
  });
});

describe('Bewegen in der Anleitung', () => {
  test('jeder Navigationspunkt führt zu einem Kapitel und umgekehrt', async () => {
    const o = await oeffneAnleitung(await starteOberflaeche());
    const punkte = o.$$('#hilfe-nav button').map(b => b.dataset.zu);
    const kapitel = o.$$('#hilfe-inhalt section').map(a => a.id);
    assert.ok(punkte.length >= 8, 'es soll eine Gliederung geben, keine Handvoll');
    assert.deepEqual(punkte, kapitel, 'Reihenfolge und Bestand müssen sich decken');
  });

  test('ein Klick markiert den Punkt', async () => {
    const o = await oeffneAnleitung(await starteOberflaeche());
    klick(o.$('#hilfe-nav button[data-zu="hilfe-ruf"]'));
    await ruhe();
    assert.equal(o.$('#hilfe-nav button.aktiv').dataset.zu, 'hilfe-ruf');
    assert.equal(o.$$('#hilfe-nav button.aktiv').length, 1, 'immer nur einer');
  });

  test('die Suche blendet aus, was nicht passt', async () => {
    const o = await oeffneAnleitung(await starteOberflaeche());
    const feld = o.$('#hilfe-suche');
    feld.value = 'Reihum';
    feld.dispatchEvent(new o.fenster.Event('input'));
    const bleibt = sichtbareKapitel(o);
    assert.deepEqual(bleibt, ['hilfe-ruf'], 'nur die Rufbereitschaft kennt "Reihum"');
    assert.ok(o.$('#hilfe-nav button[data-zu="hilfe-cal"]').classList.contains('weg'),
      'der Navigationspunkt geht mit');
    assert.ok(o.$('#hilfe-leer').classList.contains('hidden'), 'es gibt ja einen Treffer');
  });

  test('ohne Treffer steht ein Hinweis statt einer leeren Seite', async () => {
    const o = await oeffneAnleitung(await starteOberflaeche());
    const feld = o.$('#hilfe-suche');
    feld.value = 'Schichtplanung auf dem Mars';
    feld.dispatchEvent(new o.fenster.Event('input'));
    assert.equal(sichtbareKapitel(o).length, 0);
    assert.ok(!o.$('#hilfe-leer').classList.contains('hidden'));
  });

  test('ein zweites Öffnen beginnt wieder von vorn', async () => {
    const o = await oeffneAnleitung(await starteOberflaeche());
    const feld = o.$('#hilfe-suche');
    feld.value = 'Reihum';
    feld.dispatchEvent(new o.fenster.Event('input'));
    taste(o, 'Escape');
    await ruhe();
    await oeffneAnleitung(o);
    assert.equal(feld.value, '', 'die alte Suche darf nicht stehen bleiben');
    assert.equal(sichtbareKapitel(o).length, o.$$('#hilfe-inhalt section').length);
  });
});

describe('Anleitung bleibt vollständig', () => {
  test('jede Ansicht hat ihr Kapitel', async () => {
    const o = await oeffneAnleitung(await starteOberflaeche());
    const ansichten = o.$$('#view-tabs .vtab').map(t => t.dataset.view);
    assert.ok(ansichten.length >= 5, 'die Reiter sollen gefunden werden');
    for (const v of ansichten) {
      assert.ok(o.$(`#hilfe-inhalt section#hilfe-${v}`),
        `die Ansicht "${v}" hat kein Kapitel #hilfe-${v}`);
    }
  });

  test('jeder Knopf im Mehr-Menü kommt vor', async () => {
    const o = await oeffneAnleitung(await starteOberflaeche());
    const ids = o.$$('#more-menu button').map(b => b.id);
    assert.deepEqual(ids.slice().sort(), Object.keys(MENUE).sort(),
      'ein Knopf im Mehr-Menü ist weder erklärt noch hier vermerkt');
    const inhalt = text(o);
    for (const [id, stichwort] of Object.entries(MENUE)) {
      if (stichwort === null) continue;
      assert.ok(inhalt.includes(stichwort), `"${stichwort}" (${id}) fehlt in der Anleitung`);
    }
  });

  test('die vier Schichten stehen mit ihren Zeiten da', async () => {
    const o = await oeffneAnleitung(await starteOberflaeche());
    const inhalt = text(o);
    for (const wort of ['Früh', 'Normaldienst', 'Spät', 'Rufbereitschaft'])
      assert.ok(inhalt.includes(wort), `${wort} fehlt`);
    assert.ok(inhalt.includes('06–14') && inhalt.includes('14–22') && inhalt.includes('22–06'),
      'die Zeiten der Schichten fehlen');
  });
});

describe('Die genannten Tastenkürzel gibt es wirklich', () => {
  test('1 bis 4 wählen die Schicht, wie beschrieben', async () => {
    const o = await oeffneAnleitung(await starteOberflaeche());
    assert.ok(text(o).includes('Früh, Normaldienst, Spät, Rufbereitschaft wählen'));
    taste(o, 'Escape');
    await ruhe();
    for (const [t, schicht] of [['1', 'frueh'], ['2', 'normal'], ['3', 'spaet'], ['4', 'rufbereitschaft']]) {
      taste(o, t);
      assert.equal(o.$('.pill.active').dataset.shift, schicht, `Taste ${t}`);
    }
  });

  test('T springt in den laufenden Monat, / in die Suche', async () => {
    const o = await starteOberflaeche();
    const monat = () => o.$('#nav-month').value;
    const start = monat();
    taste(o, 'ArrowRight');
    await ruhe();
    assert.notEqual(monat(), start, 'der Pfeil blättert');
    taste(o, 't');
    await ruhe();
    assert.equal(monat(), start, 'T kommt zurück');
    taste(o, '/');
    assert.equal(o.dokument.activeElement.id, 'sb-search');
  });
});
