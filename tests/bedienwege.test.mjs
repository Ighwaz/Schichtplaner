// Bedienwege, die über einen einzelnen Klick hinausgehen: Rückgängig,
// Chips ziehen, die Liste der Unterbesetzung und das Verteilen der
// Rufbereitschaft reihum.
import { test, describe } from 'node:test';
import assert from 'node:assert/strict';
import { starteOberflaeche, klick, ruhe, warteBis } from './lade-oberflaeche.mjs';

const TEAM = [
  { name: 'Bauer, Martin', team: 'DE', color: '#4a9eff', icon: '', prefs: {} },
  { name: 'Krüger, Sina', team: 'DE', color: '#22c55e', icon: '', prefs: {} },
  { name: 'Nair, Anita', team: 'IN', color: '#a78bfa', icon: '', prefs: {} },
];

async function zeigeMonat(o, jahr, monat) {
  o.$('#nav-year').value = String(jahr);
  o.$('#nav-year').dispatchEvent(new o.fenster.Event('change'));
  o.$('#nav-month').value = String(monat);
  o.$('#nav-month').dispatchEvent(new o.fenster.Event('change'));
  await warteBis(() => o.$(`.day-cell[data-key="${jahr}-${String(monat).padStart(2, '0')}-01"]`),
    `${monat}/${jahr}`);
}
const ruf = (o, key) => (o.api.zustand.schichten[key] || {}).rufbereitschaft || [];
const frueh = (o, key) => (o.api.zustand.schichten[key] || {}).frueh || [];

describe('Rückgängig und Wiederholen', () => {
  test('ein Eintrag lässt sich zurücknehmen und wiederholen', async () => {
    const o = await starteOberflaeche({ mitarbeiter: TEAM });
    await zeigeMonat(o, 2026, 9);
    o.fenster.selectPerson('Bauer, Martin');
    o.fenster.waehleSchicht('frueh');

    assert.ok(o.$('#btn-undo').disabled, 'am Anfang gibt es nichts zurückzunehmen');
    klick(o.$('.day-cell[data-key="2026-09-03"]'));
    await warteBis(() => frueh(o, '2026-09-03').length === 1, 'den Eintrag');
    assert.ok(!o.$('#btn-undo').disabled, 'Rückgängig bleibt aus');

    await o.fenster.doUndo();
    await warteBis(() => frueh(o, '2026-09-03').length === 0, 'das Zurücknehmen');
    assert.ok(!o.$('#btn-redo').disabled, 'Wiederholen bleibt aus');

    await o.fenster.doRedo();
    await warteBis(() => frueh(o, '2026-09-03').length === 1, 'das Wiederholen');
  });

  test('ein ganzer Zeitraum ist ein einziger Schritt', async () => {
    const o = await starteOberflaeche({ mitarbeiter: TEAM });
    await zeigeMonat(o, 2026, 9);
    o.fenster.selectPerson('Bauer, Martin');
    o.fenster.waehleSchicht('frueh');

    klick(o.$('.day-cell[data-key="2026-09-07"]'));
    await ruhe();
    klick(o.$('.day-cell[data-key="2026-09-11"]'), { shiftKey: true });
    await warteBis(() => frueh(o, '2026-09-11').length === 1, 'den Zeitraum');

    // Ein Rückgängig nimmt die ganze Strecke zurück, nicht Tag für Tag.
    await o.fenster.doUndo();
    await warteBis(() => frueh(o, '2026-09-11').length === 0, 'das Zurücknehmen');
    for (const d of ['08', '09', '10']) {
      assert.equal(frueh(o, `2026-09-${d}`).length, 0, `am ${d}. steht noch etwas`);
    }
  });
});

describe('Chips ziehen', () => {
  // jsdom bringt kein DataTransfer mit - die Handler brauchen nur ein Objekt,
  // das effectAllowed annimmt.
  function ziehe(o, chip, zielZelle) {
    const start = new o.fenster.Event('dragstart', { bubbles: true });
    start.dataTransfer = { effectAllowed: '' };
    chip.dispatchEvent(start);

    const drop = new o.fenster.Event('drop', { bubbles: true });
    drop.dataTransfer = { effectAllowed: '' };
    zielZelle.dispatchEvent(drop);
  }

  test('ein Chip wandert auf einen anderen Tag', async () => {
    const o = await starteOberflaeche({
      mitarbeiter: TEAM,
      schichten: {
        '2026-09-03': { frueh: ['Bauer, Martin'], normal: [], spaet: [], rufbereitschaft: [] },
      },
    });
    await zeigeMonat(o, 2026, 9);

    const chip = o.$('.day-cell[data-key="2026-09-03"] .chip');
    assert.ok(chip, 'kein Chip zum Ziehen');
    ziehe(o, chip, o.$('.day-cell[data-key="2026-09-10"]'));

    await warteBis(() => frueh(o, '2026-09-10').includes('Bauer, Martin'), 'den Zieltag');
    await warteBis(() => frueh(o, '2026-09-03').length === 0, 'den Ausgangstag');
  });

  test('auf denselben Tag gezogen passiert nichts', async () => {
    const o = await starteOberflaeche({
      mitarbeiter: TEAM,
      schichten: {
        '2026-09-03': { frueh: ['Bauer, Martin'], normal: [], spaet: [], rufbereitschaft: [] },
      },
    });
    await zeigeMonat(o, 2026, 9);
    const vorher = o.api.aufrufe.filter(a => a.pfad === '/api/schicht').length;

    const chip = o.$('.day-cell[data-key="2026-09-03"] .chip');
    ziehe(o, chip, o.$('.day-cell[data-key="2026-09-03"]'));
    await ruhe();

    assert.equal(o.api.aufrufe.filter(a => a.pfad === '/api/schicht').length, vorher,
      'es wurde trotzdem geschrieben');
    assert.equal(frueh(o, '2026-09-03').length, 1);
  });
});

describe('Liste der Unterbesetzung', () => {
  test('nennt die Tage und springt zum angeklickten', async () => {
    const plan = {};
    for (let d = 1; d <= 30; d++) {
      const key = `2026-09-${String(d).padStart(2, '0')}`;
      plan[key] = {
        frueh: ['Bauer, Martin'], normal: [], spaet: ['Krüger, Sina'],
        rufbereitschaft: ['Nair, Anita'],
      };
    }
    delete plan['2026-09-17'].spaet;
    plan['2026-09-17'].spaet = [];

    const o = await starteOberflaeche({ mitarbeiter: TEAM, schichten: plan });
    await zeigeMonat(o, 2026, 9);

    assert.equal(o.$('#check-text').textContent, '1 Tag unter Soll');
    klick(o.$('#btn-check'));
    await ruhe();

    const eintraege = o.$$('.under-item');
    assert.equal(eintraege.length, 1, 'genau ein Tag fehlt');
    assert.match(eintraege[0].textContent, /17/);

    klick(eintraege[0]);
    await ruhe();
    assert.ok(o.$('#under-modal').classList.contains('hidden'), 'der Dialog blieb offen');
  });

  test('ein voll besetzter Monat meldet nichts', async () => {
    const plan = {};
    for (let d = 1; d <= 30; d++) {
      plan[`2026-09-${String(d).padStart(2, '0')}`] = {
        frueh: ['Bauer, Martin'], normal: [], spaet: ['Krüger, Sina'],
        rufbereitschaft: ['Nair, Anita'],
      };
    }
    const o = await starteOberflaeche({ mitarbeiter: TEAM, schichten: plan });
    await zeigeMonat(o, 2026, 9);
    assert.ok(o.$('#btn-check').classList.contains('hidden'));

    klick(o.$('#btn-check'));
    await ruhe();
    assert.match(o.$('#under-content').textContent, /vollständig besetzt/);
  });
});

describe('Rufbereitschaft reihum', () => {
  async function imReihumDialog(o) {
    klick(o.$('.vtab[data-view="ruf"]'));
    await warteBis(() => o.$('.rufkw-person-cell'), 'die KW-Tabelle');
    o.fenster.openRufReihum();
    await warteBis(() => o.$$('#reihum-liste input[type=checkbox]').length > 0, 'die Namensliste');
  }

  test('verteilt die Wochen der Reihe nach', async () => {
    const o = await starteOberflaeche({ mitarbeiter: TEAM });
    await imReihumDialog(o);

    const jahr = o.$('#rufkw-year').value;
    o.$('#reihum-von').value = `${jahr}-W01`;
    o.$('#reihum-bis').value = `${jahr}-W06`;
    await o.fenster.rufReihumFuellen();
    await warteBis(() => Object.keys(o.api.zustand.ruf_kw).length === 6, 'sechs verteilte Wochen');

    // Drei Personen, sechs Wochen: jede kommt zweimal dran, in fester Folge.
    const namen = [1, 2, 3, 4, 5, 6]
      .map(n => o.api.zustand.ruf_kw[`${jahr}-W0${n}`][0]);
    assert.equal(new Set(namen).size, 3, `nicht alle drei dran: ${namen}`);
    assert.equal(namen[0], namen[3], 'der Reigen wiederholt sich nicht');
    assert.equal(namen[1], namen[4]);
  });

  test('lässt belegte Wochen in Ruhe, solange man nicht überschreiben will', async () => {
    const o = await starteOberflaeche({ mitarbeiter: TEAM });
    await imReihumDialog(o);
    const jahr = o.$('#rufkw-year').value;

    // Eine Woche von Hand belegen ...
    await o.fenster.setzeKwPersonen(`${jahr}-W02`, ['Nair, Anita']);
    await warteBis(() => o.api.zustand.ruf_kw[`${jahr}-W02`], 'die belegte Woche');

    o.fenster.openRufReihum();
    await ruhe();
    o.$('#reihum-von').value = `${jahr}-W01`;
    o.$('#reihum-bis').value = `${jahr}-W03`;
    o.$('#reihum-ueber').checked = false;
    await o.fenster.rufReihumFuellen();
    await warteBis(() => o.api.zustand.ruf_kw[`${jahr}-W03`], 'die dritte Woche');

    assert.deepEqual(o.api.zustand.ruf_kw[`${jahr}-W02`], ['Nair, Anita'],
      'die belegte Woche wurde überschrieben');
  });

  test('wer abgewählt ist, kommt nicht dran', async () => {
    const o = await starteOberflaeche({ mitarbeiter: TEAM });
    await imReihumDialog(o);
    const jahr = o.$('#rufkw-year').value;

    // Den ersten Namen abwählen.
    const kaesten = o.$$('#reihum-liste input[type=checkbox]');
    klick(kaesten[0]);
    await ruhe();

    o.$('#reihum-von').value = `${jahr}-W01`;
    o.$('#reihum-bis').value = `${jahr}-W04`;
    await o.fenster.rufReihumFuellen();
    await warteBis(() => Object.keys(o.api.zustand.ruf_kw).length === 4, 'vier Wochen');

    const verteilt = new Set(Object.values(o.api.zustand.ruf_kw).flat());
    assert.equal(verteilt.size, 2, `es sind ${verteilt.size} Personen dran: ${[...verteilt]}`);
  });
});

describe('Rufbereitschaft aus dem Kalender lesen', () => {
  test('übernimmt eingetragene Wochen in den Plan', async () => {
    // Eine ganze Woche Rufbereitschaft im Kalender: Mo 7.9. bis So 13.9.2026.
    const plan = {};
    for (let d = 7; d <= 13; d++) {
      plan[`2026-09-${String(d).padStart(2, '0')}`] = {
        frueh: [], normal: [], spaet: [], rufbereitschaft: ['Nair, Anita'],
      };
    }
    const o = await starteOberflaeche({ mitarbeiter: TEAM, schichten: plan });
    await zeigeMonat(o, 2026, 9);
    klick(o.$('.vtab[data-view="ruf"]'));
    await warteBis(() => o.$('.rufkw-person-cell'), 'die KW-Tabelle');

    o.fenster.syncRufKWFromCalendar();
    await warteBis(() => o.api.zustand.ruf_kw['2026-W37'], 'die übernommene Woche');
    assert.deepEqual(o.api.zustand.ruf_kw['2026-W37'], ['Nair, Anita']);
  });

  test('einzelne Tage machen noch keine Woche', async () => {
    const o = await starteOberflaeche({
      mitarbeiter: TEAM,
      schichten: {
        '2026-09-08': { frueh: [], normal: [], spaet: [], rufbereitschaft: ['Nair, Anita'] },
      },
    });
    await zeigeMonat(o, 2026, 9);
    klick(o.$('.vtab[data-view="ruf"]'));
    await warteBis(() => o.$('.rufkw-person-cell'), 'die KW-Tabelle');

    o.fenster.syncRufKWFromCalendar();
    await ruhe();
    // Ein einzelner Tag reicht nicht für "diese Woche gehört ihr".
    assert.equal(o.api.zustand.ruf_kw['2026-W37'], undefined,
      `ein Tag wurde zur ganzen Woche: ${JSON.stringify(o.api.zustand.ruf_kw)}`);
  });
});

describe('Austragen im Kalender', () => {
  const mitEintraegen = () => starteOberflaeche({
    mitarbeiter: TEAM,
    schichten: {
      '2026-09-03': {
        frueh: ['Bauer, Martin'], normal: [], spaet: ['Krüger, Sina'],
        rufbereitschaft: ['Nair, Anita'],
      },
      '2026-09-09': {
        frueh: ['Krüger, Sina'], normal: [], spaet: [], rufbereitschaft: [],
      },
    },
  });

  test('jeder Chip trägt ein eigenes Kreuz zum Austragen', async () => {
    const o = await mitEintraegen();
    await zeigeMonat(o, 2026, 9);
    const chip = o.$('.day-cell[data-key="2026-09-03"] .chip');
    const kreuz = chip.querySelector('[data-weg]');
    assert.ok(kreuz, 'kein Kreuz am Chip');
    assert.match(kreuz.title, /austragen/i);

    klick(kreuz);
    await warteBis(() => frueh(o, '2026-09-03').length === 0, 'das Austragen');
    assert.deepEqual(o.api.zustand.schichten['2026-09-03'].spaet, ['Krüger, Sina'],
      'die anderen Schichten wurden mitgenommen');
  });

  test('ein Klick auf den Chip selbst trägt nicht mehr aus', async () => {
    // Früher entfernte jeder Klick auf einen Chip den Eintrag - unsichtbar für
    // den, der es nicht wusste, und ein Stolperstein beim Aufziehen.
    const o = await mitEintraegen();
    await zeigeMonat(o, 2026, 9);
    const chip = o.$('.day-cell[data-key="2026-09-03"] .chip .chip-name');
    klick(chip);
    await ruhe();
    assert.equal(frueh(o, '2026-09-03').length, 1, 'der Eintrag ist weg');
  });

  test('ein Zeitraum über fremde Chips löscht nichts', async () => {
    const o = await mitEintraegen();
    await zeigeMonat(o, 2026, 9);
    o.fenster.selectPerson('Bauer, Martin');
    o.fenster.waehleSchicht('frueh');

    klick(o.$('.day-cell[data-key="2026-09-07"]'));
    await ruhe();
    // Der zweite Klick landet auf dem Kreuz eines fremden Chips - mit Shift
    // zählt er trotzdem als Klick auf den Tag.
    const fremdesKreuz = o.$('.day-cell[data-key="2026-09-09"] .chip [data-weg]');
    assert.ok(fremdesKreuz, 'kein fremder Chip zum Danebenklicken');
    klick(fremdesKreuz, { shiftKey: true });
    await warteBis(() => frueh(o, '2026-09-08').includes('Bauer, Martin'), 'den Zeitraum');

    assert.ok(frueh(o, '2026-09-09').includes('Krüger, Sina'),
      'der fremde Eintrag wurde beim Aufziehen gelöscht');
    assert.ok(frueh(o, '2026-09-09').includes('Bauer, Martin'),
      'der Zeitraum endet nicht am gewählten Tag');
  });

  test('auch Strg+Klick auf ein Kreuz löscht nicht', async () => {
    const o = await mitEintraegen();
    await zeigeMonat(o, 2026, 9);
    o.fenster.selectPerson('Bauer, Martin');
    o.fenster.waehleSchicht('frueh');

    const kreuz = o.$('.day-cell[data-key="2026-09-09"] .chip [data-weg]');
    klick(kreuz, { ctrlKey: true });
    await ruhe();
    assert.ok(frueh(o, '2026-09-09').includes('Krüger, Sina'), 'Eintrag gelöscht');
    assert.equal(o.$$('.day-cell.multi-day').length, 1, 'der Tag wurde nicht gesammelt');
  });
});

describe('Ziehen auf einen belegten Tag', () => {
  function ziehe(o, chip, zielZelle) {
    const start = new o.fenster.Event('dragstart', { bubbles: true });
    start.dataTransfer = { effectAllowed: '' };
    chip.dispatchEvent(start);
    const drop = new o.fenster.Event('drop', { bubbles: true });
    drop.dataTransfer = { effectAllowed: '' };
    zielZelle.dispatchEvent(drop);
  }

  test('ein abgebrochener Zug lässt Wiederholen stehen', async () => {
    // pushUndo() leert den Wiederholen-Stapel. Wurde die Rückfrage dann
    // abgebrochen, nahm der Code nur den Rückgängig-Schritt zurück - und
    // Wiederholen war weg, obwohl sich nichts geändert hatte.
    const o = await starteOberflaeche({
      mitarbeiter: TEAM,
      schichten: {
        '2026-09-03': { frueh: ['Bauer, Martin'], normal: [], spaet: [], rufbereitschaft: [] },
        '2026-09-10': { frueh: [], normal: [], spaet: ['Bauer, Martin'], rufbereitschaft: [] },
      },
    });
    await zeigeMonat(o, 2026, 9);

    o.fenster.waehleSchicht('frueh');
    await o.fenster.schichtAufTagen(['2026-09-21'], ['Krüger, Sina']);
    await o.fenster.doUndo();
    await warteBis(() => !o.$('#btn-redo').disabled, 'einen Schritt zum Wiederholen');

    ziehe(o, o.$('.day-cell[data-key="2026-09-03"] .chip'), o.$('.day-cell[data-key="2026-09-10"]'));
    await warteBis(() => o.$('#_cdlg'), 'die Rückfrage');
    klick(o.$('#_cdlg-no'));
    await ruhe(); await ruhe();

    assert.ok(!o.$('#btn-redo').disabled, 'Wiederholen ist nach dem Abbruch verloren');
  });

  test('die Rückfrage nennt die gezogene Schicht, nicht die gewählte', async () => {
    // Gefunden bei der Typisierung: handleDrop rief die Rückfrage ohne
    // Schicht auf, und die fiel auf die aus der Leiste zurück.
    const o = await starteOberflaeche({
      mitarbeiter: TEAM,
      schichten: {
        '2026-09-03': { frueh: ['Bauer, Martin'], normal: [], spaet: [], rufbereitschaft: [] },
        '2026-09-10': { frueh: [], normal: [], spaet: ['Bauer, Martin'], rufbereitschaft: [] },
      },
    });
    await zeigeMonat(o, 2026, 9);
    o.fenster.waehleSchicht('normal');   // etwas anderes als die gezogene Frühschicht

    ziehe(o, o.$('.day-cell[data-key="2026-09-03"] .chip'), o.$('.day-cell[data-key="2026-09-10"]'));
    await warteBis(() => o.$('#_cdlg'), 'die Rückfrage');
    const text = o.$('#_cdlg').textContent;
    assert.match(text, /Durch Frühschicht ersetzen/, text);
    assert.doesNotMatch(text, /Normaldienst/, 'die Rückfrage nennt die Schicht aus der Leiste');
  });

  test('ein Chip geht beim Ziehen in einen Konflikt nicht verloren', async () => {
    // Bauer hat am 3.9. Früh und am 10.9. bereits Spät. Wird die Frühschicht
    // auf den 10. gezogen, stünde er in zwei Arbeitsschichten - dieselbe
    // Rückfrage wie beim Klick. Was nicht passieren darf: der Eintrag
    // verschwindet vom 3. und taucht am 10. nie auf.
    const o = await starteOberflaeche({
      mitarbeiter: TEAM,
      schichten: {
        '2026-09-03': { frueh: ['Bauer, Martin'], normal: [], spaet: [], rufbereitschaft: [] },
        '2026-09-10': { frueh: [], normal: [], spaet: ['Bauer, Martin'], rufbereitschaft: [] },
      },
    });
    await zeigeMonat(o, 2026, 9);

    ziehe(o, o.$('.day-cell[data-key="2026-09-03"] .chip'), o.$('.day-cell[data-key="2026-09-10"]'));
    await ruhe();
    await ruhe();

    // Ohne Bestätigung bleibt alles, wie es war.
    assert.deepEqual(frueh(o, '2026-09-03'), ['Bauer, Martin'],
      'der Eintrag ist vom Ausgangstag verschwunden, ohne am Zieltag anzukommen');
    assert.equal(frueh(o, '2026-09-10').length, 0, 'am Zieltag wurde ohne Rückfrage eingetragen');
    assert.ok(o.$('#_cdlg-yes'), 'es kam keine Rückfrage');
  });

  test('nach dem Bestätigen liegt der Eintrag am Zieltag', async () => {
    const o = await starteOberflaeche({
      mitarbeiter: TEAM,
      schichten: {
        '2026-09-03': { frueh: ['Bauer, Martin'], normal: [], spaet: [], rufbereitschaft: [] },
        '2026-09-10': { frueh: [], normal: [], spaet: ['Bauer, Martin'], rufbereitschaft: [] },
      },
    });
    await zeigeMonat(o, 2026, 9);
    ziehe(o, o.$('.day-cell[data-key="2026-09-03"] .chip'), o.$('.day-cell[data-key="2026-09-10"]'));
    await warteBis(() => o.$('#_cdlg-yes'), 'die Rückfrage');

    klick(o.$('#_cdlg-yes'));
    await warteBis(() => frueh(o, '2026-09-10').includes('Bauer, Martin'), 'den Zieltag');
    await warteBis(() => frueh(o, '2026-09-03').length === 0, 'den Ausgangstag');
    // Die abgegebene Spätschicht ist die, die ersetzt wurde.
    assert.equal((o.api.zustand.schichten['2026-09-10'].spaet || []).length, 0);
  });
});

describe('Zeitraum leeren', () => {
  // Ohne Werkzeug spannt Klick + Shift-Klick einen Zeitraum zum Löschen auf.
  async function spanneAuf(o) {
    klick(o.$('.day-cell[data-key="2026-09-07"]'));
    await ruhe();
    klick(o.$('.day-cell[data-key="2026-09-11"]'), { shiftKey: true });
    await ruhe();
  }

  test('ein Abbruch lässt die Strecke stehen', async () => {
    // clearRangeAll setzte frozenRangeEnd vor der Rückfrage auf null. Bei
    // einem Abbruch blieb der Löschblock zwar stehen, der Zustand dahinter
    // war aber weg - der nächste Klick darin traf ins Leere.
    const o = await starteOberflaeche({
      mitarbeiter: TEAM,
      schichten: {
        '2026-09-09': { frueh: ['Bauer, Martin'], normal: [], spaet: [], rufbereitschaft: [] },
      },
    });
    await zeigeMonat(o, 2026, 9);
    await spanneAuf(o);
    assert.match(o.$('#sel-info').textContent, /5 Tage/, o.$('#sel-info').textContent);

    o.fenster.clearRangeAll();
    await warteBis(() => o.$('#_cdlg'), 'die Rückfrage');
    klick(o.$('#_cdlg-no'));
    await ruhe(); await ruhe();

    // Nichts gelöscht ...
    assert.deepEqual(o.api.zustand.schichten['2026-09-09'].frueh, ['Bauer, Martin']);
    // ... und die Strecke steht noch, samt Löschblock.
    o.fenster.updateSelInfo();
    assert.match(o.$('#sel-info').textContent, /5 Tage/,
      'die Strecke ist nach dem Abbruch verschwunden');
  });
});

describe('Auswahlfelder der Rufbereitschaft', () => {
  test('es ist immer nur eines offen', async () => {
    const o = await starteOberflaeche({ mitarbeiter: TEAM });
    klick(o.$('.vtab[data-view="ruf"]'));
    await ruhe();
    const anker = o.$('.vtab[data-view="ruf"]');

    o.fenster.openRufTagPicker('2026-09-14', anker);
    assert.ok(!o.$('#ruftag-picker').classList.contains('hidden'), 'das Tagesfeld ging nicht auf');

    o.fenster.openRufKWPicker('2026-W38', anker);
    assert.ok(o.$('#ruftag-picker').classList.contains('hidden'),
      'das Tagesfeld blieb neben dem Wochenfeld offen');
    assert.ok(!o.$('#rufkw-picker').classList.contains('hidden'));

    o.fenster.openRufTagPicker('2026-09-15', anker);
    assert.ok(o.$('#rufkw-picker').classList.contains('hidden'),
      'das Wochenfeld blieb neben dem Tagesfeld offen');
  });
});

describe('Wenn der Server einen Fehler meldet', () => {
  test('ein Klick stürzt nicht ab und lässt keinen Rückgängig-Schritt zurück', async () => {
    // /api/schicht antwortet bei einem unbekannten Namen oder einem
    // unmöglichen Datum mit {error} und ohne "results". applyToDateList lief
    // darauf in Object.entries(undefined) - mitten im Klick, nach bereits
    // gesetztem Rückgängig-Punkt.
    const o = await starteOberflaeche({
      mitarbeiter: TEAM,
      schichtFehler: () => 'unbekannter Mitarbeiter',
    });
    await zeigeMonat(o, 2026, 9);
    o.fenster.waehleSchicht('frueh');

    await o.fenster.schichtAufTagen(['2026-09-07', '2026-09-08'], ['Bauer, Martin']);

    assert.match(o.$('#toast').textContent, /unbekannter Mitarbeiter/,
      'der Fehler wird nicht gemeldet');
    assert.ok(o.$('#btn-undo').disabled,
      'es blieb ein Rückgängig-Schritt stehen, obwohl nichts geschehen ist');
  });
});
