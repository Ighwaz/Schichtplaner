// Tests der Oberflaeche: was gerendert wird und was ein Klick ausloest.
// Laeuft in jsdom gegen die echte frontend/index.html mit erfundener API.
import { test, describe } from 'node:test';
import assert from 'node:assert/strict';
import { starteOberflaeche, klick, ruhe, warteBis } from './lade-oberflaeche.mjs';

const TEAM = [
  { name: 'Bauer, Martin', team: 'DE', color: '#4a9eff', icon: '', prefs: {} },
  { name: 'Krüger, Sina', team: 'DE', color: '#22c55e', icon: '', prefs: {} },
  { name: 'Nair, Anita', team: 'IN', color: '#a78bfa', icon: '', prefs: {} },
];
const tag = (f = [], n = [], s = [], r = []) =>
  ({ frueh: f, normal: n, spaet: s, rufbereitschaft: r });

// Die Oberflaeche startet im laufenden Monat. Fuer feste Datumsangaben wird
// sie ueber die Auswahlfelder der Kopfleiste dorthin geschickt.
async function zeigeMonat(o, jahr, monat) {
  o.$('#nav-year').value = String(jahr);
  o.$('#nav-year').dispatchEvent(new o.fenster.Event('change'));
  o.$('#nav-month').value = String(monat);
  o.$('#nav-month').dispatchEvent(new o.fenster.Event('change'));
  await warteBis(() => o.$(`.day-cell[data-key="${jahr}-${String(monat).padStart(2, '0')}-01"]`),
    `den ${monat}/${jahr}`);
}

const zelle = (o, key) => o.$(`.day-cell[data-key="${key}"]`);
const zeile = (el, schicht) => el.querySelector(`.chips-row[data-shift="${schicht}"]`);
const chips = (el, schicht) => [...zeile(el, schicht).querySelectorAll('.chip')].map(c => c.textContent.trim());
const taste = (o, key, opts = {}) =>
  o.dokument.body.dispatchEvent(new o.fenster.KeyboardEvent('keydown', { key, bubbles: true, ...opts }));

// September 2026: 1.9. ist ein Dienstag, 5./6.9. Wochenende, KW 36-40.
const SEP = { jahr: 2026, monat: 9 };

describe('Aufbau der Seite', () => {
  test('eine Leiste mit Ansichten, Monat und Werkzeugen', async () => {
    const o = await starteOberflaeche();
    assert.equal(o.$$('header').length, 1, 'es darf nur eine Kopfleiste geben');
    const kopf = o.$('header');
    assert.equal(kopf.querySelectorAll('.vtab').length, 5);
    assert.ok(kopf.querySelector('#dnav'), 'Datumssteuerung sitzt in der Kopfleiste');
    assert.ok(kopf.querySelector('#btn-check'), 'Status-Chip sitzt in der Kopfleiste');
    assert.ok(kopf.querySelector('#btn-more'), 'Mehr-Menue sitzt in der Kopfleiste');
    assert.equal(o.$$('.context-bar').length, 0, 'die zweite Leiste ist weg');
    assert.equal(o.$$('#btn-range').length, 0, 'der Zeitraum-Knopf ist weg');
  });

  test('Werkzeug und Soll stehen in der Seitenleiste', async () => {
    const o = await starteOberflaeche();
    const leiste = o.$('.sidebar');
    assert.ok(leiste.querySelector('.tool-box #modebar'), 'Schichtwahl gehoert zum Werkzeug');
    assert.equal(leiste.querySelectorAll('#modebar .pill').length, 4);
    assert.ok(leiste.querySelector('.soll-foot'), 'Soll sitzt am Fuss');
    // Reihenfolge: Werkzeug oben, Soll unten.
    const kinder = [...leiste.children].map(k => k.className.split(' ')[0]);
    assert.equal(kinder[0], 'tool-box');
    assert.equal(kinder.at(-1), 'soll-foot');
  });

  test('die Fussleiste traegt die einzige Legende', async () => {
    const o = await starteOberflaeche();
    const legende = o.$$('.statusbar .lg').map(l => l.textContent.trim());
    assert.equal(legende.length, 4);
    assert.ok(legende[0].startsWith('Früh'));
    assert.ok(legende[3].startsWith('Rufbereitschaft'));
    assert.equal(o.$$('.print-legend').length, 0, 'keine zweite Legende fuer den Druck');
  });

  test('der Datenordner steht in der Fussleiste', async () => {
    const o = await starteOberflaeche();
    assert.match(o.$('#folder-label').textContent, /testordner/);
  });
});

describe('Kalenderraster', () => {
  test('fuenf volle Wochen mit Kalenderwochen 36 bis 40', async () => {
    const o = await starteOberflaeche();
    await zeigeMonat(o, SEP.jahr, SEP.monat);
    assert.equal(o.$$('.day-cell').length, 35);
    const kws = o.$$('#cal-grid > div').filter(d => d.title?.startsWith('KW')).map(d => d.textContent);
    assert.deepEqual(kws, ['36', '37', '38', '39', '40']);
  });

  test('Tage der Nachbarmonate sind gedimmt und nicht anklickbar', async () => {
    const o = await starteOberflaeche();
    await zeigeMonat(o, SEP.jahr, SEP.monat);
    assert.ok(zelle(o, '2026-08-31').classList.contains('other'));
    assert.ok(zelle(o, '2026-10-04').classList.contains('other'));
    assert.ok(!zelle(o, '2026-09-01').classList.contains('other'));
  });

  test('heute ist markiert', async () => {
    const o = await starteOberflaeche();
    const h = new Date();
    const key = `${h.getFullYear()}-${String(h.getMonth() + 1).padStart(2, '0')}-${String(h.getDate()).padStart(2, '0')}`;
    assert.ok(zelle(o, key).classList.contains('today'));
  });

  test('jede Zelle zeigt alle vier Schichten in fester Reihenfolge', async () => {
    const o = await starteOberflaeche();
    await zeigeMonat(o, SEP.jahr, SEP.monat);
    const reihen = [...zelle(o, '2026-09-01').querySelectorAll('.chips-row')]
      .map(r => r.dataset.shift);
    assert.deepEqual(reihen, ['frueh', 'normal', 'spaet', 'rufbereitschaft']);
  });

  test('ein Knopf je Zelle statt vier', async () => {
    const o = await starteOberflaeche();
    const z = o.$('.day-cell:not(.other)');
    const knoepfe = z.querySelectorAll('[data-action]');
    assert.equal(knoepfe.length, 1);
    assert.equal(knoepfe[0].dataset.action, 'menu');
  });
});

describe('Besetzung und Markierungen', () => {
  test('Zaehler und Chips stehen an der Schicht', async () => {
    const o = await starteOberflaeche({
      mitarbeiter: TEAM,
      schichten: { '2026-09-01': tag(['Bauer, Martin'], [], ['Krüger, Sina'], ['Nair, Anita']) },
    });
    await zeigeMonat(o, SEP.jahr, SEP.monat);
    const z = zelle(o, '2026-09-01');
    assert.deepEqual(chips(z, 'frueh'), ['Bauer']);       // ohne haengendes Komma
    assert.deepEqual(chips(z, 'spaet'), ['Krüger']);
    assert.deepEqual(chips(z, 'rufbereitschaft'), ['Nair']);
    assert.match(zeile(z, 'frueh').querySelector('.chips-label').textContent, /1\/1/);
  });

  test('der Chip traegt Schichtfarbe und Personenfarbe', async () => {
    const o = await starteOberflaeche({
      mitarbeiter: TEAM,
      schichten: { '2026-09-01': tag(['Bauer, Martin']) },
    });
    await zeigeMonat(o, SEP.jahr, SEP.monat);
    const chip = zeile(zelle(o, '2026-09-01'), 'frueh').querySelector('.chip');
    assert.ok(chip.classList.contains('frueh'), 'Schicht steckt in der Klasse');
    // jsdom rechnet die Hex-Angabe in rgb() um - #4a9eff ist rgb(74,158,255).
    assert.equal(chip.style.borderLeftColor, 'rgb(74, 158, 255)', 'Person steckt im Balken links');
  });

  test('was fehlt, faerbt nur die betroffene Zeile', async () => {
    const o = await starteOberflaeche({
      mitarbeiter: TEAM,
      schichten: { '2026-09-01': tag(['Bauer, Martin'], [], [], ['Nair, Anita']) },
    });
    await zeigeMonat(o, SEP.jahr, SEP.monat);
    const z = zelle(o, '2026-09-01');
    assert.ok(zeile(z, 'spaet').querySelector('.chips-label').classList.contains('unter'));
    assert.ok(!zeile(z, 'frueh').querySelector('.chips-label').classList.contains('unter'));
    assert.ok(!z.classList.contains('understaffed'), 'die ganze Zelle bleibt ruhig');
  });

  test('am Wochenende wird nur die Rufbereitschaft angemahnt', async () => {
    const o = await starteOberflaeche({ mitarbeiter: TEAM });
    await zeigeMonat(o, SEP.jahr, SEP.monat);
    const sa = zelle(o, '2026-09-05');
    assert.ok(sa.classList.contains('weekend'));
    assert.ok(!zeile(sa, 'frueh').querySelector('.chips-label').classList.contains('unter'));
    assert.ok(zeile(sa, 'rufbereitschaft').querySelector('.chips-label').classList.contains('unter'));
  });

  test('doppelte Einteilung wird als Konflikt markiert', async () => {
    const o = await starteOberflaeche({
      mitarbeiter: TEAM,
      schichten: { '2026-09-01': tag(['Bauer, Martin'], [], ['Bauer, Martin'], ['Nair, Anita']) },
    });
    await zeigeMonat(o, SEP.jahr, SEP.monat);
    const z = zelle(o, '2026-09-01');
    assert.ok(z.classList.contains('conflict'));
    assert.match(z.querySelector('.conflict-badge').textContent, /Bauer, Martin/);
  });

  test('Feiertage bekommen Marke und Einfaerbung', async () => {
    const o = await starteOberflaeche({
      mitarbeiter: TEAM,
      feiertage: { '2026-09-01': { name: 'Testfeiertag', country: 'DE' } },
    });
    await zeigeMonat(o, SEP.jahr, SEP.monat);
    const z = zelle(o, '2026-09-01');
    assert.ok(z.classList.contains('hol-de'));
    assert.match(z.querySelector('.hol-tag').textContent, /Testfeiertag/);
  });

  test('der Status-Chip zaehlt die Luecken und verschwindet, wenn keine da sind', async () => {
    const leer = await starteOberflaeche({ mitarbeiter: TEAM });
    await zeigeMonat(leer, SEP.jahr, SEP.monat);
    assert.equal(leer.$('#check-text').textContent, '30 Tage unter Soll');
    assert.ok(!leer.$('#btn-check').classList.contains('hidden'));

    const plan = {};
    for (let d = 1; d <= 30; d++) {
      const key = `2026-09-${String(d).padStart(2, '0')}`;
      plan[key] = tag(['Bauer, Martin'], [], ['Krüger, Sina'], ['Nair, Anita']);
    }
    const voll = await starteOberflaeche({ mitarbeiter: TEAM, schichten: plan });
    await zeigeMonat(voll, SEP.jahr, SEP.monat);
    assert.ok(voll.$('#btn-check').classList.contains('hidden'), 'kein Chip ohne Luecke');
  });
});

describe('Eintragen und Austragen', () => {
  async function mitWerkzeug(vorgabe = {}) {
    const o = await starteOberflaeche({ mitarbeiter: TEAM, ...vorgabe });
    await zeigeMonat(o, SEP.jahr, SEP.monat);
    klick(o.$$('.member-item')[0]);         // Bauer, Martin
    await ruhe();
    return o;
  }

  test('Person waehlen fuellt den Werkzeug-Block', async () => {
    const o = await mitWerkzeug();
    assert.match(o.$('#sel-info').textContent, /Bauer, Martin/);
    assert.match(o.$('#tool-hint').textContent, /Shift/);
  });

  test('ein Klick traegt ein, der naechste traegt aus', async () => {
    const o = await mitWerkzeug();
    klick(zelle(o, '2026-09-10'));
    await warteBis(() => o.api.zustand.schichten['2026-09-10']?.frueh.length === 1, 'den Eintrag');
    assert.deepEqual(chips(zelle(o, '2026-09-10'), 'frueh'), ['Bauer']);

    klick(zelle(o, '2026-09-10'));
    await warteBis(() => o.api.zustand.schichten['2026-09-10'].frueh.length === 0, 'das Austragen');
  });

  test('Shift+Klick zieht die Absicht ueber den Zeitraum', async () => {
    const o = await mitWerkzeug();
    klick(zelle(o, '2026-09-07'));
    await ruhe();
    klick(zelle(o, '2026-09-11'), { shiftKey: true });
    await warteBis(() => o.api.zustand.schichten['2026-09-11']?.frueh.length === 1, 'den Zeitraum');
    for (const d of ['07', '08', '09', '10', '11'])
      assert.equal(o.api.zustand.schichten[`2026-09-${d}`].frueh.length, 1, `am ${d}.`);

    // Anker austragen, dann Shift - der ganze Zeitraum geht wieder raus.
    klick(zelle(o, '2026-09-07'));
    await ruhe();
    klick(zelle(o, '2026-09-11'), { shiftKey: true });
    await warteBis(() => o.api.zustand.schichten['2026-09-11'].frueh.length === 0, 'das Leeren');
    for (const d of ['07', '08', '09', '10', '11'])
      assert.equal(o.api.zustand.schichten[`2026-09-${d}`].frueh.length, 0, `am ${d}.`);
  });

  test('Strg sammelt Tage, der naechste Klick traegt sie ein', async () => {
    const o = await mitWerkzeug();
    klick(zelle(o, '2026-09-14'), { ctrlKey: true });
    klick(zelle(o, '2026-09-15'), { ctrlKey: true });
    await ruhe();
    assert.match(o.$('#tool-hint').textContent, /2 Tage/);
    assert.ok(zelle(o, '2026-09-14').classList.contains('multi-day'));

    klick(zelle(o, '2026-09-16'));
    await warteBis(() => o.api.zustand.schichten['2026-09-15']?.frueh.length === 1, 'die Sammelbuchung');
    assert.equal(o.api.zustand.schichten['2026-09-14'].frueh.length, 1);
  });

  test('die Schichtwahl bestimmt, wohin der Klick geht', async () => {
    const o = await mitWerkzeug();
    klick(o.$('.pill[data-shift="rufbereitschaft"]'));
    klick(zelle(o, '2026-09-10'));
    await warteBis(() => o.api.zustand.schichten['2026-09-10']?.rufbereitschaft.length === 1, 'die Ruf-Buchung');
    assert.equal(o.api.zustand.schichten['2026-09-10'].frueh.length, 0);
  });

  test('ein Klick auf den Chip traegt genau diesen Eintrag aus', async () => {
    const o = await mitWerkzeug({
      schichten: { '2026-09-01': tag(['Bauer, Martin'], [], ['Krüger, Sina']) },
    });
    klick(zeile(zelle(o, '2026-09-01'), 'frueh').querySelector('.chip'));
    await warteBis(() => o.api.zustand.schichten['2026-09-01'].frueh.length === 0, 'das Austragen');
    assert.equal(o.api.zustand.schichten['2026-09-01'].spaet.length, 1, 'Spaet bleibt stehen');
  });

  test('ohne Werkzeug schreibt ein Klick nichts', async () => {
    const o = await starteOberflaeche({ mitarbeiter: TEAM });
    await zeigeMonat(o, SEP.jahr, SEP.monat);
    const vorher = o.api.aufrufe.filter(a => a.pfad === '/api/schicht').length;
    klick(zelle(o, '2026-09-10'));
    await ruhe();
    assert.equal(o.api.aufrufe.filter(a => a.pfad === '/api/schicht').length, vorher);
    assert.match(o.$('#toast').textContent, /Mitarbeiter/);
  });

  test('die Vorschau zeigt, was der Klick eintragen wuerde', async () => {
    const o = await mitWerkzeug();
    const z = zelle(o, '2026-09-10');
    z.dispatchEvent(new o.fenster.MouseEvent('mouseenter'));
    const vorschau = z.querySelector('.chip.vorschau');
    assert.ok(vorschau, 'gestrichelter Chip fehlt');
    assert.match(vorschau.textContent, /Bauer/);
    assert.equal(vorschau.closest('.chips-row').dataset.shift, 'frueh');
    z.dispatchEvent(new o.fenster.MouseEvent('mouseleave'));
    assert.equal(z.querySelector('.chip.vorschau'), null, 'Vorschau bleibt kleben');
  });
});

describe('Tagesmenue', () => {
  test('der Knopf oeffnet vier Eintraege, Einfuegen erst nach dem Kopieren', async () => {
    const o = await starteOberflaeche({
      mitarbeiter: TEAM,
      schichten: { '2026-09-01': tag(['Bauer, Martin']) },
    });
    await zeigeMonat(o, SEP.jahr, SEP.monat);
    klick(zelle(o, '2026-09-01').querySelector('[data-action="menu"]'));
    let menue = o.$('.daymenu');
    assert.ok(menue, 'Menue fehlt');
    const eintraege = [...menue.querySelectorAll('.mm-item')];
    assert.equal(eintraege.length, 4);
    assert.ok(eintraege[2].disabled, 'Einfuegen ohne Kopie muss aus sein');

    klick(eintraege[1]);                   // Tag kopieren
    await ruhe();
    klick(zelle(o, '2026-09-02').querySelector('[data-action="menu"]'));
    menue = o.$('.daymenu');
    assert.ok(!menue.querySelectorAll('.mm-item')[2].disabled, 'Einfuegen bleibt aus');
  });

  test('der Rechtsklick oeffnet dasselbe Menue', async () => {
    const o = await starteOberflaeche({ mitarbeiter: TEAM });
    await zeigeMonat(o, SEP.jahr, SEP.monat);
    zelle(o, '2026-09-01').dispatchEvent(
      new o.fenster.MouseEvent('contextmenu', { bubbles: true, clientX: 50, clientY: 50 }));
    assert.ok(o.$('.daymenu'));
  });

  test('Leeren raeumt den Tag', async () => {
    const o = await starteOberflaeche({
      mitarbeiter: TEAM,
      schichten: { '2026-09-01': tag(['Bauer, Martin'], [], ['Krüger, Sina'], ['Nair, Anita']) },
    });
    await zeigeMonat(o, SEP.jahr, SEP.monat);
    klick(zelle(o, '2026-09-01').querySelector('[data-action="menu"]'));
    klick([...o.$('.daymenu').querySelectorAll('.mm-item')][3]);   // Tag leeren
    await warteBis(() => o.api.zustand.schichten['2026-09-01'].frueh.length === 0, 'das Leeren');
    assert.equal(o.api.zustand.schichten['2026-09-01'].rufbereitschaft.length, 0);
  });
});

describe('Kopfleiste und Tastatur', () => {
  test('das Mehr-Menue geht auf und wieder zu', async () => {
    const o = await starteOberflaeche();
    assert.ok(o.$('#more-menu').classList.contains('hidden'));
    klick(o.$('#btn-more'));
    assert.ok(!o.$('#more-menu').classList.contains('hidden'));
    klick(o.dokument.body);
    assert.ok(o.$('#more-menu').classList.contains('hidden'));
  });

  test('Tasten 1 bis 4 waehlen die Schicht', async () => {
    const o = await starteOberflaeche();
    for (const [t, schicht] of [['1', 'frueh'], ['2', 'normal'], ['3', 'spaet'], ['4', 'rufbereitschaft']]) {
      taste(o, t);
      assert.equal(o.$('.pill.active').dataset.shift, schicht, `Taste ${t}`);
    }
  });

  test('im Suchfeld greifen die Kuerzel nicht', async () => {
    const o = await starteOberflaeche();
    taste(o, '3');
    const vorher = o.$('.pill.active').dataset.shift;
    o.$('#sb-search').dispatchEvent(new o.fenster.KeyboardEvent('keydown', { key: '1', bubbles: true }));
    assert.equal(o.$('.pill.active').dataset.shift, vorher);
  });

  test('Pfeile blaettern den Monat, T springt zurueck', async () => {
    const o = await starteOberflaeche();
    const monat = () => o.$('#nav-month').value;
    const start = monat();
    taste(o, 'ArrowRight');
    await ruhe();
    assert.notEqual(monat(), start);
    taste(o, 't');
    await ruhe();
    assert.equal(monat(), start);
  });

  test('der Status-Chip fuehrt zur Liste der Luecken', async () => {
    const o = await starteOberflaeche({ mitarbeiter: TEAM });
    klick(o.$('#btn-check'));
    await ruhe();
    assert.ok(!o.$('#under-modal').classList.contains('hidden'));
    assert.match(o.$('#under-content').textContent, /Unterbesetzung/);
  });

  test('die Datumssteuerung graut aus, wo kein Monat gilt', async () => {
    const o = await starteOberflaeche();
    klick(o.$('.vtab[data-view="tmpl"]'));
    assert.ok(o.$('#dnav').classList.contains('off'));
    assert.ok(o.$('#btn-check').classList.contains('hidden'));
    klick(o.$('.vtab[data-view="cal"]'));
    assert.ok(!o.$('#dnav').classList.contains('off'));
  });

  test('die Ansichten schalten die Bereiche um', async () => {
    const o = await starteOberflaeche({ mitarbeiter: TEAM });
    for (const sicht of ['matrix', 'ruf', 'tmpl', 'hol', 'cal']) {
      klick(o.$(`.vtab[data-view="${sicht}"]`));
      await ruhe();
      assert.equal(o.$$('.view-panel.active').length, 1, sicht);
      assert.equal(o.$('.view-panel.active').dataset.panel, sicht);
    }
  });

  test('Escape raeumt Menues und Auswahl weg', async () => {
    const o = await starteOberflaeche({ mitarbeiter: TEAM });
    await zeigeMonat(o, SEP.jahr, SEP.monat);
    klick(o.$$('.member-item')[0]);
    await ruhe();
    klick(zelle(o, '2026-09-14'), { ctrlKey: true });
    await ruhe();
    klick(o.$('#btn-more'));
    taste(o, 'Escape');
    await ruhe();
    assert.ok(o.$('#more-menu').classList.contains('hidden'));
    assert.equal(o.$$('.day-cell.multi-day').length, 0);
  });
});
