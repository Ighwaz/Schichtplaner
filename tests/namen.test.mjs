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
    // Die App oeffnet im laufenden Monat (index.html:2275), also muss der
    // Tag aus dem laufenden Monat kommen. Vorher stand hier fest 2026-09-03;
    // das lief, solange September 2026 der laufende Monat war, und schlug ab
    // Oktober fehl, weil die Zelle dann gar nicht mehr gezeichnet wird.
    const heute = new Date();
    const tag = `${heute.getFullYear()}-${String(heute.getMonth() + 1).padStart(2, '0')}-15`;
    const o = await starteOberflaeche({
      mitarbeiter: TEAM,
      feiertage: { [tag]: { name: '<b>Fest</b>', country: 'DE', custom: true } },
    });
    const zelle = o.$(`.day-cell[data-key="${tag}"]`);
    assert.ok(zelle, `die Zelle ${tag} fehlt im laufenden Monat`);
    o.fenster.showTooltip(zelle, tag);
    const tt = o.$('#tooltip');
    assert.equal(tt.querySelector('b'), null, 'der Name wurde als HTML ausgeführt');
    assert.match(tt.textContent, /<b>Fest<\/b>/);
  });
});

// Der Name steckt nicht nur im Namensfeld, sondern auch in den zwei
// Buchstaben des Avatars. Die kamen unmaskiert ins Markup: aus "<b Mueller"
// macht initials() die Zeichenfolge "<M", und daraus baute der Browser ein
// angefangenes Tag <m<, in das die Nachbarelemente rutschten. Der Avatar war
// danach leer, die Zeile zerschossen.
describe('Namen, die als Initialen ein Tag öffnen', () => {
  const SPITZ = '<b Mueller';
  const SPITZES_TEAM = [
    { name: SPITZ, team: 'DE', color: '#4a9eff', icon: '', prefs: {} },
    { name: 'Bauer, Martin', team: 'DE', color: '#22c55e', icon: '', prefs: {} },
  ];

  test('die Seitenleiste zeigt die Initialen als Text', async () => {
    const o = await starteOberflaeche({ mitarbeiter: SPITZES_TEAM });
    const zeile = o.$(`.member-item[data-member="${SPITZ}"]`);
    assert.ok(zeile, 'die Zeile fehlt ganz');
    const avatar = zeile.querySelector('.avatar');
    assert.ok(avatar, 'der Avatar fehlt');
    assert.equal(avatar.textContent, '<M', 'die Initialen wurden als Markup gelesen');
    assert.equal(zeile.querySelector('.member-name').textContent, SPITZ);
  });

  test('die Monatsübersicht trägt die Zeile unversehrt', async () => {
    const o = await starteOberflaeche({ mitarbeiter: SPITZES_TEAM });
    klick(o.$('.vtab[data-view="matrix"]'));
    await warteBis(() => o.$('.mx-table tbody .mx-name'), 'die Monatsübersicht');
    // thead und die Fusszeile tragen dieselbe Klasse - hier zaehlen nur die
    // Personenzeilen.
    const zeilen = o.$$('.mx-table tbody tr:not(.mx-foot) .mx-name');
    assert.equal(zeilen.length, 2, 'eine Personenzeile ist verschwunden');
    const spitz = o.$(`.mx-table tbody .mx-name[title="${SPITZ}"]`);
    assert.ok(spitz, 'die Zeile mit der spitzen Klammer fehlt');
    assert.ok(spitz.textContent.includes(SPITZ), 'der Name steht nicht in der Zeile');
    assert.equal(spitz.querySelector('.avatar').textContent, '<M');
  });

  test('die Statistik zeigt den Avatar neben dem Namen', async () => {
    const o = await starteOberflaeche({ mitarbeiter: SPITZES_TEAM });
    klick(o.$('.sb-tab[data-tab="stats"]'));
    await warteBis(() => o.$('.stat-row'), 'die Statistik');
    const reihe = o.$('.stat-row');
    assert.equal(reihe.querySelector('.avatar').textContent, '<M');
    assert.ok(reihe.querySelector('.stat-name').textContent.includes(SPITZ));
  });
});
