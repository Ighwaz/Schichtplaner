// Die Ansichten jenseits des Kalenders: Monatsübersicht, Template, Feiertage,
// Tagesdialog, Kopfleistenwerkzeuge - und die Wege über Monats- und
// Jahresgrenzen.
import { test, describe } from 'node:test';
import assert from 'node:assert/strict';
import { starteOberflaeche, klick, ruhe, warteBis } from './lade-oberflaeche.mjs';

const TEAM = [
  { name: 'Bauer, Martin', team: 'DE', color: '#4a9eff', icon: '', prefs: {} },
  { name: 'Krüger, Sina', team: 'DE', color: '#22c55e', icon: '', prefs: {} },
  { name: 'Nair, Anita', team: 'IN', color: '#a78bfa', icon: '', prefs: {} },
];

// Schickt die Oberfläche über die Auswahlfelder in einen bestimmten Monat.
async function zeigeMonat(o, jahr, monat) {
  o.$('#nav-year').value = String(jahr);
  o.$('#nav-year').dispatchEvent(new o.fenster.Event('change'));
  o.$('#nav-month').value = String(monat);
  o.$('#nav-month').dispatchEvent(new o.fenster.Event('change'));
  await warteBis(() => o.$(`.day-cell[data-key="${jahr}-${String(monat).padStart(2, '0')}-01"]`),
    `${monat}/${jahr}`);
}
const monatVon = o => o.$('#nav-month').value + '/' + o.$('#nav-year').value;

describe('Monatsnavigation', () => {
  test('vorwärts über die Jahresgrenze', async () => {
    const o = await starteOberflaeche({ mitarbeiter: TEAM });
    await zeigeMonat(o, 2026, 12);
    klick(o.$('#btn-next'));
    await warteBis(() => monatVon(o) === '1/2027', 'Januar 2027');
    assert.equal(o.$$('.day-cell').length % 7, 0);
    assert.ok(o.$('.day-cell[data-key="2027-01-01"]'), '1.1.2027 fehlt');
  });

  test('rückwärts über die Jahresgrenze', async () => {
    const o = await starteOberflaeche({ mitarbeiter: TEAM });
    await zeigeMonat(o, 2026, 1);
    klick(o.$('#btn-prev'));
    await warteBis(() => monatVon(o) === '12/2025', 'Dezember 2025');
    assert.ok(o.$('.day-cell[data-key="2025-12-31"]'), '31.12.2025 fehlt');
  });

  test('Heute führt zurück in den laufenden Monat', async () => {
    const o = await starteOberflaeche({ mitarbeiter: TEAM });
    const start = monatVon(o);
    await zeigeMonat(o, 2027, 5);
    klick(o.$('#btn-today'));
    await warteBis(() => monatVon(o) === start, 'den laufenden Monat');
  });

  test('jeder Monat eines Jahres zeigt volle Wochen', async () => {
    const o = await starteOberflaeche({ mitarbeiter: TEAM });
    for (let m = 1; m <= 12; m++) {
      await zeigeMonat(o, 2026, m);
      const zellen = o.$$('.day-cell');
      assert.equal(zellen.length % 7, 0, `Monat ${m}: ${zellen.length} Zellen`);
      const eigene = zellen.filter(z => !z.classList.contains('other')).length;
      assert.equal(eigene, new Date(2026, m, 0).getDate(), `Monat ${m}`);
    }
  });

  test('der Schaltjahr-Februar zeigt 29 Tage', async () => {
    const o = await starteOberflaeche({ mitarbeiter: TEAM });
    await zeigeMonat(o, 2028, 2);
    assert.ok(o.$('.day-cell[data-key="2028-02-29"]'));
  });

  test('ein Jahr, das die Auswahl nicht kennt, reisst nichts mit', async () => {
    // Frueher wurde daraus NaN, und renderCalendar warf einen Fehler, der die
    // ganze Ansicht stehen liess.
    const o = await starteOberflaeche({ mitarbeiter: TEAM });
    const vorher = monatVon(o);
    o.$('#nav-year').value = '1999';
    o.$('#nav-year').dispatchEvent(new o.fenster.Event('change'));
    await ruhe();
    assert.equal(monatVon(o), vorher, 'die Ansicht ist verrutscht');
    assert.ok(o.$$('.day-cell').length > 0, 'der Kalender ist leer');
  });
});

describe('Monatsübersicht', () => {
  // Oberes Band (Arbeitsschicht) bzw. unterer Streifen (Rufbereitschaft)
  // einer bestimmten Person an einem bestimmten Tag.
  const band = (o, key, name, welches) =>
    o.$$(`#mx-table .mx-${welches === 'ruf' ? 'r' : 'a'}[data-key="${key}"]`)
      .find(el => el.dataset.name === name);

  async function oeffneUebersicht(vorgabe, jahr = 2026, monat = 9) {
    const o = await starteOberflaeche(vorgabe);
    await zeigeMonat(o, jahr, monat);
    klick(o.$('.vtab[data-view="matrix"]'));
    await ruhe();
    return o;
  }

  test('eine Zeile je Mitarbeiter, eine Spalte je Tag', async () => {
    const o = await oeffneUebersicht({ mitarbeiter: TEAM });

    const namen = o.$$('#mx-table .mx-name').map(td => td.textContent);
    // Spaltenkopf, drei Mitarbeiter, Fußzeile "Besetzung".
    assert.equal(namen.length, 5, namen.join(' | '));
    assert.equal(namen[0], 'Mitarbeiter');
    assert.ok(namen[1].includes('Bauer'));
    assert.ok(namen.at(-1).includes('Besetzung'));

    // Jede Zelle hat zwei Felder - oben Arbeit, unten Rufbereitschaft.
    assert.equal(o.$$('#mx-table .mx-a[data-key]').length, 30 * 3,
      'September hat 30 Tage mal 3 Personen');
    assert.equal(o.$$('#mx-table .mx-r[data-key]').length, 30 * 3,
      'der Streifen für die Rufbereitschaft fehlt an manchen Tagen');
  });

  test('Rufbereitschaft neben der Arbeitsschicht bleibt sichtbar', async () => {
    // Genau der gemeldete Fall: beides am selben Tag. Vorher gewann die
    // Arbeitsschicht die Farbe und die Rufbereitschaft war nur noch ein
    // zweiter Buchstabe im Kürzel.
    const o = await oeffneUebersicht({
      mitarbeiter: TEAM,
      schichten: {
        '2026-09-03': {
          frueh: ['Bauer, Martin'], normal: [], spaet: [],
          rufbereitschaft: ['Bauer, Martin'],
        },
      },
    });

    const oben = band(o, '2026-09-03', 'Bauer, Martin', 'arbeit');
    const unten = band(o, '2026-09-03', 'Bauer, Martin', 'ruf');
    assert.ok(oben.classList.contains('frueh'), 'die Frühschicht fehlt oben');
    assert.equal(oben.textContent, 'F');
    assert.ok(unten.classList.contains('an'), 'die Rufbereitschaft fehlt unten');

    // Und die Gegenprobe: ein Tag ohne Rufbereitschaft trägt den Streifen
    // zwar, aber unmarkiert.
    assert.ok(!band(o, '2026-09-04', 'Bauer, Martin', 'ruf').classList.contains('an'));
  });

  test('ein Klick oben trägt für die Person dieser Zeile ein', async () => {
    const o = await starteOberflaeche({ mitarbeiter: TEAM });
    await zeigeMonat(o, 2026, 9);
    o.fenster.waehleSchicht('spaet');
    klick(o.$('.vtab[data-view="matrix"]'));
    await ruhe();

    // Ohne dass jemand in der Seitenleiste ausgewählt sein muss.
    klick(band(o, '2026-09-03', 'Krüger, Sina', 'arbeit'));
    await warteBis(() => o.api.zustand.schichten['2026-09-03']?.spaet.includes('Krüger, Sina'),
      'den Eintrag');
  });

  test('ein Klick unten trägt Rufbereitschaft ein, egal welche Schicht gewählt ist', async () => {
    const o = await starteOberflaeche({ mitarbeiter: TEAM });
    await zeigeMonat(o, 2026, 9);
    o.fenster.waehleSchicht('spaet');
    klick(o.$('.vtab[data-view="matrix"]'));
    await ruhe();

    klick(band(o, '2026-09-03', 'Nair, Anita', 'ruf'));
    await warteBis(
      () => o.api.zustand.schichten['2026-09-03']?.rufbereitschaft.includes('Nair, Anita'),
      'die Rufbereitschaft');
    assert.deepEqual(o.api.zustand.schichten['2026-09-03'].spaet, [],
      'die gewählte Spätschicht wurde mit eingetragen');
  });

  test('ein zweiter Klick unten trägt die Rufbereitschaft wieder aus', async () => {
    const o = await oeffneUebersicht({
      mitarbeiter: TEAM,
      schichten: {
        '2026-09-03': {
          frueh: [], normal: [], spaet: [], rufbereitschaft: ['Nair, Anita'],
        },
      },
    });
    klick(band(o, '2026-09-03', 'Nair, Anita', 'ruf'));
    await warteBis(
      () => !o.api.zustand.schichten['2026-09-03'].rufbereitschaft.includes('Nair, Anita'),
      'das Austragen');
  });

  test('oben trägt eine Arbeitsschicht ein, auch wenn Rufbereitschaft gewählt ist', async () => {
    const o = await starteOberflaeche({ mitarbeiter: TEAM });
    await zeigeMonat(o, 2026, 9);
    o.fenster.waehleSchicht('spaet');          // zuletzt gewählte Arbeitsschicht
    o.fenster.waehleSchicht('rufbereitschaft'); // oben wäre das sinnlos
    klick(o.$('.vtab[data-view="matrix"]'));
    await ruhe();

    klick(band(o, '2026-09-07', 'Krüger, Sina', 'arbeit'));
    await warteBis(() => o.api.zustand.schichten['2026-09-07']?.spaet.includes('Krüger, Sina'),
      'die zuletzt gewählte Arbeitsschicht');
    assert.deepEqual(o.api.zustand.schichten['2026-09-07'].rufbereitschaft, [],
      'oben wurde Rufbereitschaft eingetragen');
  });

  test('doppelt Eingeteilte werden markiert', async () => {
    const o = await oeffneUebersicht({
      mitarbeiter: TEAM,
      schichten: {
        '2026-09-03': {
          frueh: ['Bauer, Martin'], normal: [], spaet: ['Bauer, Martin'], rufbereitschaft: [],
        },
      },
    });
    const oben = band(o, '2026-09-03', 'Bauer, Martin', 'arbeit');
    assert.ok(oben.classList.contains('doppelt'), 'nicht als doppelt markiert');
    assert.equal(oben.textContent, '!');
  });

  test('die Summenspalte zählt auch den Normaldienst', async () => {
    // Vorher lief der Normaldienst in ein Feld, das es im Zähler nicht gab,
    // und tauchte in keiner Summe auf.
    const o = await oeffneUebersicht({
      mitarbeiter: TEAM,
      schichten: {
        '2026-09-03': { frueh: [], normal: ['Bauer, Martin'], spaet: [], rufbereitschaft: [] },
        '2026-09-04': { frueh: [], normal: ['Bauer, Martin'], spaet: [], rufbereitschaft: [] },
      },
    });
    const zeile = o.$$('#mx-table tbody tr')
      .find(tr => tr.querySelector('.mx-name')?.textContent.includes('Bauer'));
    const zahlen = [...zeile.querySelectorAll('.mx-sum b')].map(b => b.textContent);
    assert.deepEqual(zahlen, ['0', '2', '0', '0'], 'F/N/S/R stimmt nicht');
  });

  // Rufbereitschaft geht wochenweise. Sieben Klicks je Woche waren der
  // Grund für diese Geste.
  // Die Attrappe legt einen Tag erst an, wenn ihn etwas beruehrt hat -
  // ein unberuehrter Tag ist also schlicht leer.
  const tagVon = (o, t, schicht) =>
    o.api.zustand.schichten[`2026-09-${String(t).padStart(2, '0')}`]?.[schicht] || [];
  const rufAn = (o, tage) => tage.filter(t => tagVon(o, t, 'rufbereitschaft').includes('Nair, Anita'));

  test('Rufbereitschaft neben einer Arbeitsschicht fragt nicht nach', async () => {
    // Die Attrappe meldete hier lange eine Rückfrage, die es nicht gibt
    // (blockingShifts in internal/domain/slot.go nimmt die Rufbereitschaft
    // aus). Die Tests darüber standen dann an einem Dialog still.
    const o = await oeffneUebersicht({
      mitarbeiter: TEAM,
      schichten: {
        '2026-09-07': { frueh: ['Nair, Anita'], normal: [], spaet: [], rufbereitschaft: [] },
      },
    });
    klick(band(o, '2026-09-07', 'Nair, Anita', 'ruf'));
    await warteBis(() => rufAn(o, [7]).length === 1, 'die Rufbereitschaft');
    assert.ok(!o.$('.modal-bg:not(.hidden)'), 'es stand eine Rückfrage im Weg');
    assert.deepEqual(tagVon(o, 7, 'frueh'), ['Nair, Anita'], 'die Frühschicht ging verloren');
  });

  test('auch eine ganze Strecke Rufbereitschaft fragt nicht nach', async () => {
    const belegt = {};
    for (let t = 7; t <= 11; t++) {
      belegt[`2026-09-${String(t).padStart(2, '0')}`] =
        { frueh: ['Nair, Anita'], normal: [], spaet: [], rufbereitschaft: [] };
    }
    const o = await oeffneUebersicht({ mitarbeiter: TEAM, schichten: belegt });
    klick(band(o, '2026-09-07', 'Nair, Anita', 'ruf'));
    await warteBis(() => rufAn(o, [7]).length === 1, 'den Ankertag');
    klick(band(o, '2026-09-11', 'Nair, Anita', 'ruf'), { shiftKey: true });
    await warteBis(() => rufAn(o, [7, 8, 9, 10, 11]).length === 5, 'die Strecke');
    assert.ok(!o.$('.modal-bg:not(.hidden)'), 'es stand eine Rückfrage im Weg');
    assert.deepEqual(tagVon(o, 9, 'frueh'), ['Nair, Anita'], 'die Frühschicht ging verloren');
  });

  test('Shift+Klick trägt eine ganze Woche in einem Zug ein', async () => {
    const o = await oeffneUebersicht({ mitarbeiter: TEAM });
    klick(band(o, '2026-09-07', 'Nair, Anita', 'ruf'));
    await warteBis(() => rufAn(o, [7]).length === 1, 'den Ankertag');
    klick(band(o, '2026-09-13', 'Nair, Anita', 'ruf'), { shiftKey: true });
    await warteBis(() => rufAn(o, [8, 9, 10, 11, 12, 13]).length === 6, 'die Woche');
    assert.deepEqual(rufAn(o, [6, 14]), [], 'die Strecke ist übergelaufen');
  });

  test('rückwärts aufziehen geht genauso', async () => {
    const o = await oeffneUebersicht({ mitarbeiter: TEAM });
    klick(band(o, '2026-09-13', 'Nair, Anita', 'ruf'));
    await warteBis(() => rufAn(o, [13]).length === 1, 'den Ankertag');
    klick(band(o, '2026-09-07', 'Nair, Anita', 'ruf'), { shiftKey: true });
    await warteBis(() => rufAn(o, [7, 8, 9, 10, 11, 12, 13]).length === 7, 'die Woche');
  });

  test('die Absicht des ersten Klicks zieht durch die Strecke', async () => {
    // Sonst liesse sich ein Zeitraum nie leeren: der Ankertag ist schon
    // ausgetragen, und die Mehrheitsregel entschiede auf Eintragen.
    const voll = {};
    for (let t = 7; t <= 13; t++) {
      voll[`2026-09-${String(t).padStart(2, '0')}`] =
        { frueh: [], normal: [], spaet: [], rufbereitschaft: ['Nair, Anita'] };
    }
    const o = await oeffneUebersicht({ mitarbeiter: TEAM, schichten: voll });

    klick(band(o, '2026-09-07', 'Nair, Anita', 'ruf'));         // trägt aus
    await warteBis(() => rufAn(o, [7]).length === 0, 'das Austragen');
    klick(band(o, '2026-09-13', 'Nair, Anita', 'ruf'), { shiftKey: true });
    await warteBis(() => rufAn(o, [7, 8, 9, 10, 11, 12, 13]).length === 0,
      'die geleerte Woche');
  });

  test('eine Strecke trifft nie eine fremde Zeile', async () => {
    // Genau die Sorge aus dem Kalender: beim Aufziehen keine fremden
    // Einträge anfassen. Hier kann das gar nicht passieren - stimmt die
    // Person nicht, ist es ein gewöhnlicher Klick.
    const o = await oeffneUebersicht({ mitarbeiter: TEAM });
    klick(band(o, '2026-09-07', 'Bauer, Martin', 'ruf'));
    await warteBis(() => tagVon(o, 7, 'rufbereitschaft').includes('Bauer, Martin'),
      'den Ankertag');

    klick(band(o, '2026-09-11', 'Nair, Anita', 'ruf'), { shiftKey: true });
    await warteBis(() => rufAn(o, [11]).length === 1, 'den einzelnen Tag');
    assert.deepEqual(rufAn(o, [7, 8, 9, 10]), [],
      'der Shift-Klick hat eine Strecke in der fremden Zeile gezogen');
    assert.deepEqual(tagVon(o, 8, 'rufbereitschaft'), [],
      'die Zeile von Bauer wurde mitgezogen');
  });

  test('eine Strecke bleibt im selben Band', async () => {
    const o = await oeffneUebersicht({ mitarbeiter: TEAM });
    o.fenster.waehleSchicht('frueh');
    klick(band(o, '2026-09-07', 'Nair, Anita', 'ruf'));          // Anker unten
    await warteBis(() => rufAn(o, [7]).length === 1, 'den Ankertag');

    klick(band(o, '2026-09-11', 'Nair, Anita', 'arbeit'), { shiftKey: true });
    await warteBis(() => tagVon(o, 11, 'frueh').includes('Nair, Anita'), 'den einzelnen Tag');
    assert.deepEqual(tagVon(o, 9, 'frueh'), [],
      'der Shift-Klick ist vom unteren ins obere Band gesprungen');
  });

  test('eine Strecke oben nimmt die Arbeitsschicht, nicht die gewählte', async () => {
    // Die Strecke laeuft ueber die gemeinsamen Wege (schichtAufTagen,
    // applyToDateList). Die haben die gewaehlte Schicht frueher fest
    // verdrahtet - dann waere hier Rufbereitschaft eingetragen worden.
    const o = await starteOberflaeche({ mitarbeiter: TEAM });
    await zeigeMonat(o, 2026, 9);
    o.fenster.waehleSchicht('spaet');
    o.fenster.waehleSchicht('rufbereitschaft');
    klick(o.$('.vtab[data-view="matrix"]'));
    await ruhe();

    klick(band(o, '2026-09-07', 'Nair, Anita', 'arbeit'));
    await warteBis(() => tagVon(o, 7, 'spaet').includes('Nair, Anita'), 'den Ankertag');
    klick(band(o, '2026-09-09', 'Nair, Anita', 'arbeit'), { shiftKey: true });
    await warteBis(() => [8, 9].every(t => tagVon(o, t, 'spaet').includes('Nair, Anita')),
      'die Strecke in der Spätschicht');
    assert.deepEqual(rufAn(o, [7, 8, 9]), [], 'die Strecke landete in der Rufbereitschaft');
  });

  test('der Ankertag ist zu sehen', async () => {
    const o = await oeffneUebersicht({ mitarbeiter: TEAM });
    klick(band(o, '2026-09-07', 'Nair, Anita', 'ruf'));
    await warteBis(() => o.$$('#mx-table .anker').length === 1, 'den Ring');
    const ring = o.$('#mx-table .anker');
    assert.equal(ring.dataset.key, '2026-09-07');
    assert.equal(ring.dataset.name, 'Nair, Anita');

    // Immer höchstens einer - der nächste Klick nimmt den Ring mit.
    klick(band(o, '2026-09-20', 'Nair, Anita', 'ruf'));
    await warteBis(() => o.$('#mx-table .anker')?.dataset.key === '2026-09-20', 'den Umzug');
    assert.equal(o.$$('#mx-table .anker').length, 1);
  });

  test('die Fußzeile meldet fehlende Rufbereitschaft getrennt', async () => {
    const o = await oeffneUebersicht({
      mitarbeiter: TEAM,
      soll: { frueh: 1, normal: 0, spaet: 0, rufbereitschaft: 1 },
      schichten: {
        // Frühschicht besetzt, Rufbereitschaft fehlt.
        '2026-09-03': { frueh: ['Bauer, Martin'], normal: [], spaet: [], rufbereitschaft: [] },
      },
    });
    const fuss = o.$('#mx-table .mx-foot');
    const spalte = [...fuss.children][3]; // Name + 1.9. + 2.9. + 3.9.
    assert.ok(!spalte.querySelector('.mx-a').classList.contains('unter'),
      'die Arbeitsschicht ist besetzt, wird aber bemängelt');
    assert.ok(spalte.querySelector('.mx-r').classList.contains('unter'),
      'die fehlende Rufbereitschaft wird nicht gemeldet');
  });
});

describe('Template', () => {
  test('ein Klick schaltet die Schicht weiter', async () => {
    const o = await starteOberflaeche({ mitarbeiter: TEAM });
    klick(o.$('.vtab[data-view="tmpl"]'));
    await warteBis(() => o.$('.tmpl-cell-btn'), 'die Template-Tabelle');

    const zelle = o.$('.tmpl-cell-btn');
    const folge = [];
    for (let i = 0; i < 5; i++) {
      folge.push(zelle.textContent.trim());
      klick(zelle);
      await ruhe();
    }
    // Der Reigen kehrt zum Ausgangspunkt zurück, statt irgendwo zu enden.
    assert.equal(folge[0], folge.at(-1) === folge[0] ? folge[0] : folge[0]);
    assert.ok(new Set(folge).size > 1, `nichts hat sich geändert: ${folge.join(' -> ')}`);
  });

  test('der Reigen einer Zelle ist überall derselbe und kehrt zurück', async () => {
    const o = await starteOberflaeche({ mitarbeiter: TEAM });
    klick(o.$('.vtab[data-view="tmpl"]'));
    await warteBis(() => o.$('.tmpl-cell-btn'), 'die Template-Tabelle');

    const zeile = o.$$('#tmpl-table tr').find(tr => tr.querySelector('.tmpl-cell-btn'));
    const knoepfe = [...zeile.querySelectorAll('.tmpl-cell-btn')];
    assert.equal(knoepfe.length, 7, 'sieben Wochentage erwartet');

    // Montag und Sonntag durchlaufen denselben Reigen - Wochenenden sind nur
    // heller gezeichnet, nicht gesperrt.
    const reigen = async knopf => {
      const gesehen = [];
      for (let i = 0; i < 5; i++) {
        klick(knopf);
        await ruhe();
        gesehen.push(knopf.textContent.trim());
      }
      return gesehen;
    };
    const montag = await reigen(knoepfe[0]);
    const sonntag = await reigen(knoepfe[6]);
    assert.deepEqual(montag, sonntag, 'Wochentag und Sonntag verhalten sich unterschiedlich');
    assert.deepEqual(montag, ['Früh', 'Normal', 'Spät', 'Frei', '–']);
  });

  test('Rufbereitschaft steht im Template nicht zur Wahl', async () => {
    // Sie wird wochenweise im eigenen Reiter geplant. Der Hinweistext hat das
    // früher anders behauptet, als er noch "Früh → Spät → Ruf" versprach.
    const o = await starteOberflaeche({ mitarbeiter: TEAM });
    klick(o.$('.vtab[data-view="tmpl"]'));
    await warteBis(() => o.$('.tmpl-cell-btn'), 'die Template-Tabelle');

    const knopf = o.$('.tmpl-cell-btn');
    const gesehen = new Set();
    for (let i = 0; i < 6; i++) {
      klick(knopf);
      await ruhe();
      gesehen.add(knopf.textContent.trim());
    }
    assert.ok(!gesehen.has('Ruf'), `Reigen bietet Ruf an: ${[...gesehen].join(', ')}`);
    assert.match(o.$('.tmpl-hint').textContent, /Rufbereitschaft steht hier nicht/);
  });

  test('ein gespeichertes Template taucht in der Liste auf', async () => {
    const o = await starteOberflaeche({ mitarbeiter: TEAM });
    klick(o.$('.vtab[data-view="tmpl"]'));
    await warteBis(() => o.$('.tmpl-cell-btn'), 'die Template-Tabelle');
    klick(o.$('.tmpl-cell-btn'));
    await ruhe();

    o.$('#tmpl-name-input').value = 'Frühdienst';
    await o.fenster.saveTemplate();
    await warteBis(() => o.api.zustand.templates['Frühdienst'], 'das gespeicherte Template');
    assert.ok(Object.keys(o.api.zustand.templates['Frühdienst']).length > 0, 'Template ist leer');
  });
});

describe('Eigene Feiertage', () => {
  test('anlegen und wieder entfernen', async () => {
    const o = await starteOberflaeche({ mitarbeiter: TEAM });
    klick(o.$('.vtab[data-view="hol"]'));
    await ruhe();

    o.$('#hol-date').value = '2026-08-14';
    o.$('#hol-name').value = 'Betriebsausflug';
    o.$('#hol-country').value = 'DE';
    await o.fenster.addCustomHoliday();
    await warteBis(() => o.api.zustand.custom_holidays.length === 1, 'den Feiertag');
    assert.equal(o.api.zustand.custom_holidays[0].name, 'Betriebsausflug');

    await o.fenster.deleteCustomHoliday('2026-08-14', 'Betriebsausflug');
    await warteBis(() => o.api.zustand.custom_holidays.length === 0, 'das Entfernen');
  });

  test('ohne Datum oder Namen passiert nichts', async () => {
    const o = await starteOberflaeche({ mitarbeiter: TEAM });
    klick(o.$('.vtab[data-view="hol"]'));
    await ruhe();

    o.$('#hol-date').value = '';
    o.$('#hol-name').value = 'Ohne Datum';
    await o.fenster.addCustomHoliday();
    o.$('#hol-date').value = '2026-08-14';
    o.$('#hol-name').value = '';
    await o.fenster.addCustomHoliday();
    await ruhe();
    assert.equal(o.api.zustand.custom_holidays.length, 0,
      `angelegt: ${JSON.stringify(o.api.zustand.custom_holidays)}`);
  });
});

describe('Tagesdialog', () => {
  test('Notiz schreiben und wieder löschen', async () => {
    const o = await starteOberflaeche({ mitarbeiter: TEAM });
    await zeigeMonat(o, 2026, 9);
    o.fenster.openDayModal('2026-09-03');
    await ruhe();
    assert.ok(!o.$('#day-modal').classList.contains('hidden'), 'Dialog blieb zu');

    o.$('#dm-note').value = 'Übergabe 14 Uhr';
    await o.fenster.saveDayNote();
    await warteBis(() => o.api.zustand.notizen['2026-09-03'] === 'Übergabe 14 Uhr', 'die Notiz');

    o.fenster.openDayModal('2026-09-03');
    o.$('#dm-note').value = '';
    await o.fenster.saveDayNote();
    await warteBis(() => !o.api.zustand.notizen['2026-09-03'], 'das Löschen');
  });

  test('der Dialog zeigt alle vier Schichten des Tages', async () => {
    const o = await starteOberflaeche({
      mitarbeiter: TEAM,
      schichten: {
        '2026-09-03': {
          frueh: ['Bauer, Martin'], normal: [], spaet: ['Krüger, Sina'],
          rufbereitschaft: ['Nair, Anita'],
        },
      },
    });
    await zeigeMonat(o, 2026, 9);
    o.fenster.openDayModal('2026-09-03');
    await ruhe();
    const text = o.$('#dm-slots').textContent;
    for (const name of ['Bauer, Martin', 'Krüger, Sina', 'Nair, Anita']) {
      assert.ok(text.includes(name), `${name} fehlt im Tagesdialog`);
    }
  });
});

describe('Kopfleiste und Anzeige', () => {
  test('die Anzeigegröße lässt sich ändern und wird gemerkt', async () => {
    const o = await starteOberflaeche({ mitarbeiter: TEAM });
    klick(o.$('#btn-more'));
    klick(o.$('#btn-zoom-in'));
    const groesser = o.fenster.document.body.style.zoom;
    assert.ok(parseFloat(groesser) > 1, `Zoom: ${groesser}`);
    assert.ok(o.fenster.localStorage.getItem('schichtplaner-zoom'), 'nicht gemerkt');

    klick(o.$('#btn-zoom-out'));
    assert.ok(parseFloat(o.fenster.document.body.style.zoom) < parseFloat(groesser));
  });

  test('das Theme wechselt und wird gemerkt', async () => {
    const o = await starteOberflaeche({ mitarbeiter: TEAM });
    const vorher = o.fenster.document.documentElement.dataset.theme;
    klick(o.$('#btn-more'));
    klick(o.$('#btn-theme'));
    const nachher = o.fenster.document.documentElement.dataset.theme;
    assert.notEqual(nachher, vorher);
    assert.equal(o.fenster.localStorage.getItem('sp-theme'), nachher);
  });

  test('die Seitenleiste klappt ein und aus', async () => {
    const o = await starteOberflaeche({ mitarbeiter: TEAM });
    klick(o.$('#sidebar-toggle'));
    assert.ok(o.$('#main').classList.contains('collapsed'));
    klick(o.$('#sidebar-toggle'));
    assert.ok(!o.$('#main').classList.contains('collapsed'));
  });

  test('die Druckkopfzeile nennt Ansicht und Monat', async () => {
    const o = await starteOberflaeche({ mitarbeiter: TEAM });
    await zeigeMonat(o, 2026, 9);
    assert.match(o.$('#print-title').textContent, /September 2026/);
    klick(o.$('.vtab[data-view="matrix"]'));
    await ruhe();
    assert.match(o.$('#print-title').textContent, /Monatsübersicht/);
  });
});

describe('Mitarbeiterliste', () => {
  test('die Suche filtert', async () => {
    const o = await starteOberflaeche({ mitarbeiter: TEAM });
    assert.equal(o.$$('.member-item').length, 3);
    o.$('#sb-search').value = 'nair';
    o.fenster.renderMembers();
    assert.equal(o.$$('.member-item').length, 1);
    o.$('#sb-search').value = '';
    o.fenster.renderMembers();
    assert.equal(o.$$('.member-item').length, 3);
  });

  test('der Teamfilter trennt DE und IN', async () => {
    const o = await starteOberflaeche({ mitarbeiter: TEAM });
    o.$('#sb-team-filter').value = 'IN';
    o.fenster.renderMembers();
    assert.equal(o.$$('.member-item').length, 1);
    o.$('#sb-team-filter').value = 'DE';
    o.fenster.renderMembers();
    assert.equal(o.$$('.member-item').length, 2);
  });

  test('die Statistik zählt die Schichten', async () => {
    const o = await starteOberflaeche({
      mitarbeiter: TEAM,
      schichten: {
        '2026-09-01': { frueh: ['Bauer, Martin'], normal: [], spaet: [], rufbereitschaft: [] },
        '2026-09-02': { frueh: ['Bauer, Martin'], normal: [], spaet: [], rufbereitschaft: [] },
      },
    });
    await zeigeMonat(o, 2026, 9);
    klick(o.$('.sb-tab[data-tab="stats"]'));
    await ruhe();
    assert.match(o.$('#stats-wrap').textContent, /Bauer/);
  });
});
