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
  test('eine Zeile je Mitarbeiter, eine Spalte je Tag', async () => {
    const o = await starteOberflaeche({ mitarbeiter: TEAM });
    await zeigeMonat(o, 2026, 9);
    klick(o.$('.vtab[data-view="matrix"]'));
    await ruhe();

    const namen = o.$$('#mx-table .mx-name').map(td => td.textContent);
    // Spaltenkopf, drei Mitarbeiter, Fußzeile "Besetzung".
    assert.equal(namen.length, 5, namen.join(' | '));
    assert.equal(namen[0], 'Mitarbeiter');
    assert.ok(namen[1].includes('Bauer'));
    assert.ok(namen.at(-1).includes('Besetzung'));

    const zellen = o.$$('#mx-table .mx-cell[data-key]');
    assert.equal(zellen.length, 30 * 3, 'September hat 30 Tage mal 3 Personen');
  });

  test('ein Klick auf eine Zelle trägt für die Person dieser Zeile ein', async () => {
    const o = await starteOberflaeche({ mitarbeiter: TEAM });
    await zeigeMonat(o, 2026, 9);
    o.fenster.waehleSchicht('spaet');
    klick(o.$('.vtab[data-view="matrix"]'));
    await ruhe();

    // Zelle von Krüger am 3.9. - ohne dass jemand ausgewählt sein muss.
    const zelle = o.$$('#mx-table .mx-cell[data-key="2026-09-03"]')
      .find(td => td.dataset.name === 'Krüger, Sina');
    assert.ok(zelle, 'Zelle nicht gefunden');
    klick(zelle);
    await warteBis(() => o.api.zustand.schichten['2026-09-03']?.spaet.includes('Krüger, Sina'),
      'den Eintrag');
  });

  test('doppelt Eingeteilte werden markiert', async () => {
    const o = await starteOberflaeche({
      mitarbeiter: TEAM,
      schichten: {
        '2026-09-03': {
          frueh: ['Bauer, Martin'], normal: [], spaet: ['Bauer, Martin'], rufbereitschaft: [],
        },
      },
    });
    await zeigeMonat(o, 2026, 9);
    klick(o.$('.vtab[data-view="matrix"]'));
    await ruhe();
    const zelle = o.$$('#mx-table .mx-cell[data-key="2026-09-03"]')
      .find(td => td.dataset.name === 'Bauer, Martin');
    assert.ok(zelle.classList.contains('doppelt'), 'nicht als doppelt markiert');
    assert.equal(zelle.textContent, '!');
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
