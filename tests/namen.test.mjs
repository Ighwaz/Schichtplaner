// Namen mit Apostroph und Anführungszeichen.
//
// Die Oberfläche baut ihre Listen als HTML-Text zusammen. Stand ein Name
// unmaskiert in einem Attribut, endete das Attribut beim ersten " im Namen -
// der Rest wurde zu Unsinn, und Knöpfe taten nichts oder das Falsche. Bei
// onclick="fn('...')" brach zusätzlich schon ein Apostroph den Aufruf auf.
//
// D'Souza ist in einem Team mit Kolleginnen in Indien kein erfundener Fall.
import { test, describe } from 'node:test';
import assert from 'node:assert/strict';
import { starteOberflaeche, klick, ruhe, warteBis } from './lade-oberflaeche.mjs';

const BOESE = 'D\'Souza, Ana "Ani"';
const TEAM = [
  { name: BOESE, team: 'IN', color: '#a78bfa', icon: '', prefs: {} },
  { name: 'Bauer, Martin', team: 'DE', color: '#4a9eff', icon: '', prefs: {} },
];

describe('Namen mit Sonderzeichen', () => {
  test('die Seitenleiste trägt den ganzen Namen im data-Attribut', async () => {
    const o = await starteOberflaeche({ mitarbeiter: TEAM });
    const zeile = o.$(`.member-item`);
    assert.equal(zeile.dataset.member, BOESE, 'der Name ist im Attribut abgeschnitten');
    assert.equal(o.$('.member-cb').dataset.cb, BOESE);
  });

  test('der KW-Dialog trägt die Person ein und zeigt es', async () => {
    // Vorher schrieb JSON.stringify(weekDates) doppelte Anführungszeichen in
    // das onclick-Attribut; es endete beim ersten davon, und kein Klick im
    // Dialog bewirkte etwas - unabhängig vom Namen.
    const o = await starteOberflaeche({ mitarbeiter: TEAM });
    o.fenster.showKwRufDialog(38, ['2026-09-14', '2026-09-15']);
    await ruhe();

    const zeile = o.$('#_cdlg .kw-ruf-person');
    assert.ok(zeile, 'der Dialog zeigt keine Personen');
    assert.equal(zeile.dataset.name, BOESE);

    klick(zeile);
    await warteBis(
      () => Object.values(o.api.zustand.ruf_kw).flat().includes(BOESE),
      'die Zuweisung im KW-Plan');
    assert.match(o.$('#_cdlg .kw-ruf-person').textContent, /✓/,
      'der Dialog zeigt nach dem Klick nicht, dass die Person nun zugewiesen ist');
  });

  test('das ✕ am Chip im KW-Plan trägt den ganzen Namen', async () => {
    const o = await starteOberflaeche({ mitarbeiter: TEAM });
    await o.fenster.setzeKwPersonen('2026-W38', [BOESE]);
    klick(o.$('.vtab[data-view="ruf"]'));
    await ruhe();

    const weg = o.$$('[data-rm-name]').find(b => b.dataset.rmKw === '2026-W38');
    assert.ok(weg, 'kein Entfernen-Knopf gefunden');
    assert.equal(weg.dataset.rmName, BOESE, 'der Name ist im Attribut abgeschnitten');
  });

  test('ein Template mit Anführungszeichen lässt sich löschen', async () => {
    const name = 'Team "A"';
    const o = await starteOberflaeche({ mitarbeiter: TEAM });
    o.api.zustand.templates[name] = { mo: { [BOESE]: 'frueh' } };
    await o.fenster.loadTemplates();

    const weg = o.$('[data-tmpl-del]');
    assert.ok(weg, 'kein Löschknopf gefunden');
    assert.equal(weg.dataset.tmplDel, name, 'der Name ist im Attribut abgeschnitten');

    klick(weg);
    await warteBis(() => !(name in o.api.zustand.templates), 'das Löschen');
  });

  test('ein eigener Feiertag mit Anführungszeichen lässt sich löschen', async () => {
    const name = 'Tag der "Einheit"';
    const o = await starteOberflaeche({ mitarbeiter: TEAM });
    klick(o.$('.vtab[data-view="hol"]'));
    await ruhe();

    o.$('#hol-date').value = '2026-10-03';
    o.$('#hol-name').value = name;
    o.$('#hol-country').value = 'DE';
    await o.fenster.addCustomHoliday();
    await warteBis(() => o.api.zustand.custom_holidays.length === 1, 'den Feiertag');

    const weg = o.$('.hol-del-btn');
    assert.ok(weg, 'kein Löschknopf gefunden');
    assert.equal(weg.dataset.holName, name, 'der Name ist im Attribut abgeschnitten');

    klick(weg);
    await warteBis(() => o.api.zustand.custom_holidays.length === 0, 'das Entfernen');
  });

  test('der Tooltip zeigt einen Feiertagsnamen als Text, nicht als HTML', async () => {
    const o = await starteOberflaeche({
      mitarbeiter: TEAM,
      feiertage: { '2026-09-03': { name: '<b>Fest</b>', country: 'DE', custom: true } },
    });
    o.fenster.showTooltip(o.$('.day-cell[data-key="2026-09-03"]'), '2026-09-03');
    const tt = o.$('#tooltip');
    assert.equal(tt.querySelector('b'), null, 'der Name wurde als HTML ausgeführt');
    assert.match(tt.textContent, /<b>Fest<\/b>/);
  });
});
