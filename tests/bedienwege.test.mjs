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
