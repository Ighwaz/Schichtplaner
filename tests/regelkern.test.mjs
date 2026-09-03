// Tests des Regelkerns aus frontend/index.html.
// Ausfuehren:  node --test --experimental-test-coverage tests/
import { test, describe } from 'node:test';
import assert from 'node:assert/strict';
import { RK, inZeitzone } from './lade-regelkern.mjs';

// Ein voll besetzter Tag - Vorlage fuer die Soll-Tests.
const vollerTag = () => ({
  frueh: ['Bauer, Martin'],
  normal: [],
  spaet: ['Wolf, Tim'],
  rufbereitschaft: ['Nair, Anita'],
});
const SOLL = { frueh: 1, normal: 0, spaet: 1, rufbereitschaft: 1 };

describe('tagKey', () => {
  test('setzt Monat und Tag zweistellig', () => {
    assert.equal(RK.tagKey(2026, 9, 3), '2026-09-03');
    assert.equal(RK.tagKey(2026, 12, 31), '2026-12-31');
  });

  test('weist alles zurueck, was keine ganze Zahl ist', () => {
    for (const kaputt of [undefined, null, '2026', 1.5, NaN, Infinity, {}, []]) {
      assert.throws(() => RK.tagKey(kaputt, 9, 3), RangeError, `Jahr=${String(kaputt)}`);
      assert.throws(() => RK.tagKey(2026, kaputt, 3), RangeError, `Monat=${String(kaputt)}`);
      assert.throws(() => RK.tagKey(2026, 9, kaputt), RangeError, `Tag=${String(kaputt)}`);
    }
  });

  test('rechnet nicht um - der Schluessel ist Text, keine Zeitangabe', () => {
    // Kein Date im Spiel, also auch keine Zeitzone, die daran zoege.
    assert.equal(inZeitzone('Pacific/Kiritimati', () => RK.tagKey(2026, 1, 1)), '2026-01-01');
    assert.equal(inZeitzone('Pacific/Niue', () => RK.tagKey(2026, 1, 1)), '2026-01-01');
  });
});

describe('alsDatum', () => {
  test('nimmt Schluessel und Date', () => {
    assert.equal(RK.alsDatum('2026-09-03').getDate(), 3);
    const d = new Date(2026, 8, 3);
    assert.equal(RK.alsDatum(d), d);
  });

  test('weist ungueltiges Date zurueck', () => {
    assert.throws(() => RK.alsDatum(new Date('kein Datum')), RangeError);
  });

  test('weist Tage zurueck, die es nicht gibt', () => {
    // Der Browser parst '2026-02-31' still auf den 3. Maerz - das ist kein
    // Tag, den jemand gemeint hat.
    assert.throws(() => RK.alsDatum('2026-02-31'), /gibt es nicht/);
    assert.throws(() => RK.alsDatum('2026-13-01'), RangeError);
    assert.throws(() => RK.alsDatum('2025-02-29'), /gibt es nicht/);
  });

  test('nimmt den Schalttag im Schaltjahr an', () => {
    assert.equal(RK.alsDatum('2024-02-29').getMonth(), 1);
  });

  test('weist alles ausser Schluessel und Date zurueck', () => {
    for (const kaputt of [null, undefined, 0, 20260903, '3.9.2026', '2026-9-3', '999-01-01', {}, []])
      assert.throws(() => RK.alsDatum(kaputt), RangeError, `Eingabe=${String(kaputt)}`);
  });
});

describe('isoWoche', () => {
  test('zaehlt nach ISO 8601 - Woche 1 enthaelt den ersten Donnerstag', () => {
    assert.equal(RK.isoWoche('2026-01-01'), 1);   // Do -> KW1
    assert.equal(RK.isoWoche('2026-09-03'), 36);
    assert.equal(RK.isoWoche('2026-08-31'), 36);  // Montag derselben Woche
    assert.equal(RK.isoWoche('2026-08-30'), 35);  // Sonntag davor
  });

  test('schlaegt Jahresgrenzen richtig zu', () => {
    assert.equal(RK.isoWoche('2021-01-01'), 53);  // gehoert noch zu 2020
    assert.equal(RK.isoWoche('2024-12-30'), 1);   // gehoert schon zu 2025
    assert.equal(RK.isoWoche('2026-12-31'), 53);
  });

  test('nimmt auch ein Date', () => {
    assert.equal(RK.isoWoche(new Date(2026, 8, 3)), 36);
  });

  test('bleibt an der Sommerzeitumstellung stabil', () => {
    // In Berlin springt die Uhr am 29.3.2026 um 02:00 auf 03:00.
    inZeitzone('Europe/Berlin', () => {
      assert.equal(RK.isoWoche('2026-03-29'), 13);
      assert.equal(RK.isoWoche('2026-03-30'), 14);
    });
  });

  test('liefert in jeder Zeitzone dieselbe Woche', () => {
    for (const tz of ['Europe/Berlin', 'Asia/Kolkata', 'Pacific/Kiritimati', 'Pacific/Niue', 'UTC'])
      assert.equal(inZeitzone(tz, () => RK.isoWoche('2026-09-03')), 36, tz);
  });

  test('weist Unsinn zurueck', () => {
    assert.throws(() => RK.isoWoche('morgen'), RangeError);
  });
});

describe('tageImZeitraum', () => {
  test('nimmt beide Enden mit', () => {
    assert.deepEqual(RK.tageImZeitraum('2026-09-01', '2026-09-03'),
      ['2026-09-01', '2026-09-02', '2026-09-03']);
  });

  test('ist gegen die Reihenfolge der Argumente unempfindlich', () => {
    assert.deepEqual(RK.tageImZeitraum('2026-09-03', '2026-09-01'),
      RK.tageImZeitraum('2026-09-01', '2026-09-03'));
  });

  test('derselbe Tag ergibt genau einen Eintrag', () => {
    assert.deepEqual(RK.tageImZeitraum('2026-09-03', '2026-09-03'), ['2026-09-03']);
  });

  test('laeuft ueber Monats- und Jahresgrenzen', () => {
    assert.deepEqual(RK.tageImZeitraum('2026-01-30', '2026-02-02'),
      ['2026-01-30', '2026-01-31', '2026-02-01', '2026-02-02']);
    assert.deepEqual(RK.tageImZeitraum('2026-12-31', '2027-01-01'),
      ['2026-12-31', '2027-01-01']);
    assert.equal(RK.tageImZeitraum('2024-02-28', '2024-03-01').length, 3); // Schaltjahr
  });

  test('verliert an der Zeitumstellung keinen Tag', () => {
    // Sommerzeit beginnt (23-Stunden-Tag) und endet (25-Stunden-Tag).
    inZeitzone('Europe/Berlin', () => {
      const fruehjahr = RK.tageImZeitraum('2026-03-28', '2026-03-30');
      assert.deepEqual(fruehjahr, ['2026-03-28', '2026-03-29', '2026-03-30']);
      const herbst = RK.tageImZeitraum('2026-10-24', '2026-10-26');
      assert.deepEqual(herbst, ['2026-10-24', '2026-10-25', '2026-10-26']);
    });
  });

  test('deckelt einen offensichtlichen Vertipper, statt das Fenster einzufrieren', () => {
    assert.throws(() => RK.tageImZeitraum('1026-09-03', '2026-09-03'), RangeError);
    assert.throws(() => RK.tageImZeitraum('2026-01-01', '2040-01-01'), /mehr als die erlaubten/);
  });

  test('genau am Deckel geht noch', () => {
    const bis = new Date(2026, 0, 1);
    bis.setDate(bis.getDate() + RK.MAX_TAGE - 1);
    const key = RK.tagKey(bis.getFullYear(), bis.getMonth() + 1, bis.getDate());
    assert.equal(RK.tageImZeitraum('2026-01-01', key).length, RK.MAX_TAGE);
  });
});

describe('monatsRaster', () => {
  test('September 2026 beginnt an einem Dienstag', () => {
    const { zellen, wochen } = RK.monatsRaster(2026, 9);
    assert.equal(zellen[0].key, '2026-08-31');
    assert.equal(zellen[0].fremd, true);
    assert.equal(zellen[1].key, '2026-09-01');
    assert.equal(zellen[1].fremd, false);
    assert.equal(wochen[0].kw, 36);
    assert.equal(wochen.at(-1).kw, 40);
  });

  test('liefert immer volle Wochen', () => {
    for (let monat = 1; monat <= 12; monat++) {
      const { zellen, wochen } = RK.monatsRaster(2026, monat);
      assert.equal(zellen.length % 7, 0, `Monat ${monat}`);
      assert.equal(wochen.length, zellen.length / 7, `Monat ${monat}`);
      assert.equal(zellen.filter(z => !z.fremd).length,
        new Date(2026, monat, 0).getDate(), `Monat ${monat}`);
    }
  });

  test('ein Monat, der am Montag beginnt, braucht keine Vortage', () => {
    const { zellen } = RK.monatsRaster(2026, 6); // 1.6.2026 ist ein Montag
    assert.equal(zellen[0].key, '2026-06-01');
    assert.equal(zellen[0].fremd, false);
  });

  test('ein Monat, der am Sonntag beginnt, braucht sechs Vortage', () => {
    const { zellen } = RK.monatsRaster(2026, 2); // 1.2.2026 ist ein Sonntag
    assert.equal(zellen.filter(z => z.fremd && z.key < '2026-02-01').length, 6);
    assert.equal(zellen[6].key, '2026-02-01');
  });

  test('Februar im Schaltjahr hat 29 eigene Tage', () => {
    const { zellen } = RK.monatsRaster(2024, 2);
    assert.equal(zellen.filter(z => !z.fremd).length, 29);
    assert.ok(zellen.some(z => z.key === '2024-02-29'));
  });

  test('Januar und Dezember greifen ins Nachbarjahr', () => {
    assert.ok(RK.monatsRaster(2026, 1).zellen.some(z => z.key.startsWith('2025-12')));
    assert.ok(RK.monatsRaster(2026, 12).zellen.some(z => z.key.startsWith('2027-01')));
  });

  test('weist einen Monat ausserhalb 1-12 zurueck', () => {
    for (const monat of [0, 13, -1, 1.5, NaN, undefined, null])
      assert.throws(() => RK.monatsRaster(2026, monat), RangeError, `Monat=${String(monat)}`);
    assert.throws(() => RK.monatsRaster('2026', 9), RangeError);
  });
});

describe('sollAmTag', () => {
  test('ohne Eintrag gilt eine Person', () => {
    assert.equal(RK.sollAmTag({}, 'frueh', '2026-09-03'), 1);
    assert.equal(RK.sollAmTag(null, 'frueh', '2026-09-03'), 1);
    assert.equal(RK.sollAmTag(undefined, 'frueh', null), 1);
  });

  test('ein ausdrueckliches Soll von 0 bleibt 0', () => {
    // Der Normaldienst steht ueblicherweise auf 0 - vorher machte
    // `soll[shift] || 1` daraus wieder 1 und jeder Tag galt als unterbesetzt.
    assert.equal(RK.sollAmTag(SOLL, 'normal', '2026-09-03'), 0);
  });

  test('am Wochenende zaehlt nur die Rufbereitschaft', () => {
    for (const schicht of ['frueh', 'normal', 'spaet']) {
      assert.equal(RK.sollAmTag(SOLL, schicht, '2026-09-05'), 0, `Sa ${schicht}`);
      assert.equal(RK.sollAmTag(SOLL, schicht, '2026-09-06'), 0, `So ${schicht}`);
    }
    assert.equal(RK.sollAmTag(SOLL, 'rufbereitschaft', '2026-09-05'), 1);
  });

  test('ohne Tag greift die Wochenendregel nicht', () => {
    assert.equal(RK.sollAmTag(SOLL, 'frueh', null), 1);
    assert.equal(RK.sollAmTag(SOLL, 'frueh', undefined), 1);
  });

  test('unbekannte Schicht faellt auf 1 zurueck', () => {
    assert.equal(RK.sollAmTag(SOLL, 'nachtdienst', '2026-09-03'), 1);
  });
});

describe('fehlbesetzung / unterSoll', () => {
  test('ein voll besetzter Werktag meldet nichts', () => {
    assert.deepEqual(RK.fehlbesetzung(vollerTag(), '2026-09-03', SOLL), []);
    assert.equal(RK.unterSoll(vollerTag(), '2026-09-03', SOLL), false);
  });

  test('nennt die fehlende Schicht mit Ist und Soll', () => {
    const tag = vollerTag(); tag.spaet = [];
    assert.deepEqual(RK.fehlbesetzung(tag, '2026-09-03', SOLL),
      [{ schicht: 'spaet', ist: 0, soll: 1 }]);
  });

  test('am Wochenende fehlt nur die Rufbereitschaft', () => {
    const leer = { frueh: [], normal: [], spaet: [], rufbereitschaft: [] };
    assert.deepEqual(RK.fehlbesetzung(leer, '2026-09-05', SOLL).map(f => f.schicht),
      ['rufbereitschaft']);
  });

  test('ein fehlender Tag zaehlt wie ein leerer', () => {
    assert.equal(RK.fehlbesetzung(undefined, '2026-09-03', SOLL).length, 3); // F, S, R
    assert.equal(RK.fehlbesetzung(null, '2026-09-03', SOLL).length, 3);
  });

  test('unterSoll ohne Tag ist false, nicht undefined', () => {
    assert.equal(RK.unterSoll(null, '2026-09-03', SOLL), false);
    assert.equal(RK.unterSoll(undefined, '2026-09-03', SOLL), false);
  });

  test('Ueberbesetzung ist kein Mangel', () => {
    const tag = vollerTag(); tag.frueh = ['A', 'B', 'C'];
    assert.deepEqual(RK.fehlbesetzung(tag, '2026-09-03', SOLL), []);
  });
});

describe('tageUnterSoll', () => {
  test('ein durchgehend besetzter Monat meldet nichts', () => {
    const plan = {};
    for (const t of RK.tageImZeitraum('2026-09-01', '2026-09-30')) plan[t] = vollerTag();
    assert.deepEqual(RK.tageUnterSoll(2026, 9, plan, SOLL), []);
  });

  test('nennt genau die Tage mit Luecke', () => {
    const plan = {};
    for (const t of RK.tageImZeitraum('2026-09-01', '2026-09-30')) plan[t] = vollerTag();
    plan['2026-09-10'].spaet = [];             // Werktag ohne Spaetschicht
    plan['2026-09-12'].rufbereitschaft = [];   // Samstag ohne Rufbereitschaft
    plan['2026-09-13'].frueh = [];             // Sonntag ohne Frueh - kein Mangel
    const luecken = RK.tageUnterSoll(2026, 9, plan, SOLL);
    assert.deepEqual(luecken.map(l => l.key), ['2026-09-10', '2026-09-12']);
    assert.deepEqual(luecken[0].missing, [{ schicht: 'spaet', ist: 0, soll: 1 }]);
    assert.deepEqual(luecken[1].missing, [{ schicht: 'rufbereitschaft', ist: 0, soll: 1 }]);
  });

  test('ein leerer Monat meldet jeden Tag', () => {
    assert.equal(RK.tageUnterSoll(2026, 9, {}, SOLL).length, 30);
    assert.equal(RK.tageUnterSoll(2026, 9, null, SOLL).length, 30);
  });

  test('nimmt die richtige Monatslaenge', () => {
    assert.equal(RK.tageUnterSoll(2024, 2, {}, SOLL).length, 29);
    assert.equal(RK.tageUnterSoll(2026, 2, {}, SOLL).length, 28);
  });

  test('am Wochenende fehlt hoechstens die Rufbereitschaft', () => {
    const luecken = RK.tageUnterSoll(2026, 9, {}, SOLL);
    const samstag = luecken.find(l => l.key === '2026-09-05');
    assert.deepEqual(samstag.missing.map(m => m.schicht), ['rufbereitschaft']);
  });
});

describe('konflikte', () => {
  const team = { 'Bauer, Martin': 'DE', 'Nair, Anita': 'IN' };
  const teamVon = n => team[n];

  test('ein sauberer Tag hat keinen Konflikt', () => {
    const k = RK.konflikte(vollerTag(), '2026-09-03', null, teamVon);
    assert.deepEqual(k, { doppelt: [], amFeiertag: [], hatKonflikt: false });
  });

  test('findet, wer zugleich in Frueh und Spaet steht', () => {
    const tag = { frueh: ['Bauer, Martin'], spaet: ['Bauer, Martin'], rufbereitschaft: [] };
    const k = RK.konflikte(tag, '2026-09-03', null, teamVon);
    assert.deepEqual(k.doppelt, ['Bauer, Martin']);
    assert.equal(k.hatKonflikt, true);
  });

  test('Rufbereitschaft laeuft daneben und ist nie doppelt', () => {
    const tag = { frueh: ['Bauer, Martin'], rufbereitschaft: ['Bauer, Martin'] };
    assert.deepEqual(RK.konflikte(tag, '2026-09-03', null, teamVon).doppelt, []);
  });

  test('meldet, wer am Feiertag des eigenen Teams eingeteilt ist', () => {
    const tag = { frueh: ['Bauer, Martin'], spaet: ['Nair, Anita'] };
    const k = RK.konflikte(tag, '2026-10-03', { country: 'DE' }, teamVon);
    assert.deepEqual(k.amFeiertag, ['Bauer, Martin']);   // Nair hat in IN frei? nein: IN arbeitet
    assert.equal(k.hatKonflikt, true);
  });

  test('ein Feiertag des anderen Teams stoert nicht', () => {
    const tag = { frueh: ['Bauer, Martin'] };
    assert.deepEqual(RK.konflikte(tag, '2026-10-02', { country: 'IN' }, teamVon).amFeiertag, []);
  });

  test('ein gemeinsamer Feiertag (DE+IN) wird nicht geprueft', () => {
    const tag = { frueh: ['Bauer, Martin'], spaet: ['Nair, Anita'] };
    assert.deepEqual(RK.konflikte(tag, '2026-01-01', { country: 'DE+IN' }, teamVon).amFeiertag, []);
  });

  test('nennt niemanden zweimal', () => {
    const tag = { frueh: ['Bauer, Martin'], spaet: ['Bauer, Martin'], rufbereitschaft: ['Bauer, Martin'] };
    assert.deepEqual(RK.konflikte(tag, '2026-10-03', { country: 'DE' }, teamVon).amFeiertag,
      ['Bauer, Martin']);
  });

  test('Normaldienst bleibt am Feiertag ungeprueft (Verhalten der Vorgaengerfassung)', () => {
    // Bewusst so uebernommen: WORK_SHIFTS liess den Normaldienst schon immer
    // aus. Faellt das jemandem auf die Fuesse, ist ARBEIT_FUER_FEIERTAG die
    // eine Stelle, an der es sich aendern laesst.
    const tag = { normal: ['Bauer, Martin'] };
    assert.deepEqual(RK.konflikte(tag, '2026-10-03', { country: 'DE' }, teamVon).amFeiertag, []);
  });

  test('haelt fehlende Angaben aus', () => {
    assert.equal(RK.konflikte(null, null, null, null).hatKonflikt, false);
    assert.equal(RK.konflikte({}, '2026-10-03', { country: 'DE' }, undefined).hatKonflikt, false);
    assert.equal(RK.konflikte({ frueh: null }, '2026-09-03', null, teamVon).hatKonflikt, false);
    assert.equal(RK.konflikte({ frueh: ['X'] }, '2026-10-03', {}, teamVon).hatKonflikt, false);
  });

  test('unbekannte Person hat kein Team und damit keinen Feiertagskonflikt', () => {
    const k = RK.konflikte({ frueh: ['Wer auch immer'] }, '2026-10-03', { country: 'DE' }, teamVon);
    assert.deepEqual(k.amFeiertag, []);
  });
});

describe('naechsteAktion', () => {
  const plan = {
    '2026-09-01': { frueh: ['A'], spaet: [] },
    '2026-09-02': { frueh: ['A', 'B'], spaet: [] },
    '2026-09-03': { frueh: [], spaet: [] },
  };

  test('stehen alle schon drin, traegt der Klick aus', () => {
    assert.equal(RK.naechsteAktion(['2026-09-01', '2026-09-02'], ['A'], plan, 'frueh'), 'remove');
  });

  test('fehlt einer irgendwo, traegt der Klick ein', () => {
    assert.equal(RK.naechsteAktion(['2026-09-01', '2026-09-03'], ['A'], plan, 'frueh'), 'add');
    assert.equal(RK.naechsteAktion(['2026-09-01'], ['A', 'B'], plan, 'frueh'), 'add');
  });

  test('ein unbekannter Tag zaehlt als leer', () => {
    assert.equal(RK.naechsteAktion(['2026-12-24'], ['A'], plan, 'frueh'), 'add');
  });

  test('andere Schicht, andere Antwort', () => {
    assert.equal(RK.naechsteAktion(['2026-09-01'], ['A'], plan, 'spaet'), 'add');
  });

  test('ohne Tage oder ohne Namen ist nichts zu tun', () => {
    assert.equal(RK.naechsteAktion([], ['A'], plan, 'frueh'), null);
    assert.equal(RK.naechsteAktion(['2026-09-01'], [], plan, 'frueh'), null);
    assert.equal(RK.naechsteAktion(null, ['A'], plan, 'frueh'), null);
    assert.equal(RK.naechsteAktion(['2026-09-01'], null, plan, 'frueh'), null);
    assert.equal(RK.naechsteAktion('2026-09-01', ['A'], plan, 'frueh'), null);
  });

  test('kommt ohne Plan aus', () => {
    assert.equal(RK.naechsteAktion(['2026-09-01'], ['A'], null, 'frueh'), 'add');
    assert.equal(RK.naechsteAktion(['2026-09-01'], ['A'], {}, 'frueh'), 'add');
  });

  test('haelt einen grossen Zeitraum aus', () => {
    const tage = RK.tageImZeitraum('2026-01-01', '2026-12-31');
    assert.equal(RK.naechsteAktion(tage, ['A'], plan, 'frueh'), 'add');
  });
});

describe('zeitraumKlasse', () => {
  test('ohne Anfang keine Markierung', () => {
    assert.equal(RK.zeitraumKlasse('2026-09-03', null, null), '');
  });

  test('ein einzelner Tag ist Anfang und Ende zugleich', () => {
    assert.equal(RK.zeitraumKlasse('2026-09-03', '2026-09-03', null), 'range-start');
    assert.equal(RK.zeitraumKlasse('2026-09-03', '2026-09-03', '2026-09-03'), 'range-start');
  });

  test('markiert Anfang, Mitte und Ende', () => {
    const k = key => RK.zeitraumKlasse(key, '2026-09-01', '2026-09-03');
    assert.equal(k('2026-09-01'), 'in-range range-start');
    assert.equal(k('2026-09-02'), 'in-range');
    assert.equal(k('2026-09-03'), 'in-range range-end');
    assert.equal(k('2026-09-04'), '');
    assert.equal(k('2026-08-31'), '');
  });

  test('rueckwaerts aufgespannt gilt dasselbe', () => {
    const k = key => RK.zeitraumKlasse(key, '2026-09-03', '2026-09-01');
    assert.equal(k('2026-09-01'), 'in-range range-start');
    assert.equal(k('2026-09-03'), 'in-range range-end');
  });
});

describe('kurzName', () => {
  test('nimmt das erste Wort ohne haengendes Komma', () => {
    assert.equal(RK.kurzName('Bauer, Martin'), 'Bauer');
    assert.equal(RK.kurzName('Nair, Anita'), 'Nair');
    assert.equal(RK.kurzName('Tim'), 'Tim');
    assert.equal(RK.kurzName('van der Berg, Jan'), 'van');
  });

  test('vertraegt Leerraum und leere Eingaben', () => {
    assert.equal(RK.kurzName('   Wolf   Tim '), 'Wolf');
    assert.equal(RK.kurzName(''), '');
    assert.equal(RK.kurzName('   '), '');
    assert.equal(RK.kurzName(null), '');
    assert.equal(RK.kurzName(undefined), '');
  });

  test('macht aus allem eine Zeichenkette', () => {
    assert.equal(RK.kurzName(42), '42');
    assert.equal(RK.kurzName('Müller-Lüdenscheidt;'), 'Müller-Lüdenscheidt');
  });
});

// ── Gegenproben ────────────────────────────────────────────────
// Eingaben, die den Kern zum Absturz bringen sollen.
describe('Gegenproben', () => {
  test('ein Tag mit null-Listen kippt nichts um', () => {
    const kaputt = { frueh: null, normal: undefined, spaet: [], rufbereitschaft: null };
    assert.doesNotThrow(() => RK.fehlbesetzung(kaputt, '2026-09-03', SOLL));
    assert.doesNotThrow(() => RK.konflikte(kaputt, '2026-09-03', { country: 'DE' }, () => 'DE'));
    assert.equal(RK.unterSoll(kaputt, '2026-09-03', SOLL), true);
  });

  test('ein Plan mit null-Tagen kippt nichts um', () => {
    const plan = { '2026-09-01': null, '2026-09-02': undefined };
    assert.doesNotThrow(() => RK.tageUnterSoll(2026, 9, plan, SOLL));
    assert.equal(RK.naechsteAktion(['2026-09-01'], ['A'], plan, 'frueh'), 'add');
  });

  test('ein Soll mit Unsinn faellt auf den Vorgabewert zurueck', () => {
    assert.equal(RK.sollAmTag({ frueh: null }, 'frueh', '2026-09-03'), 1);
    assert.equal(RK.sollAmTag({ frueh: undefined }, 'frueh', '2026-09-03'), 1);
    assert.equal(RK.sollAmTag({ frueh: 0 }, 'frueh', '2026-09-03'), 0);
  });

  test('Namen aus fremden Schriften bleiben unangetastet', () => {
    const tag = { frueh: ['अनिता नायर'], spaet: ['अनिता नायर'] };
    assert.deepEqual(RK.konflikte(tag, null, null, () => 'IN').doppelt, ['अनिता नायर']);
    assert.equal(RK.kurzName('अनिता नायर'), 'अनिता');
  });

  test('ein Name wie eine Object-Eigenschaft verwirrt den Zaehler nicht', () => {
    const tag = { frueh: ['constructor', '__proto__'], spaet: ['constructor', '__proto__'] };
    const k = RK.konflikte(tag, null, null, () => undefined);
    assert.deepEqual(k.doppelt.sort(), ['__proto__', 'constructor']);
    assert.equal({}.polluted, undefined);
  });

  test('ein Plan mit einem __proto__-Schluessel vergiftet nichts', () => {
    const plan = JSON.parse('{"__proto__":{"frueh":["A"]},"2026-09-01":{"frueh":[]}}');
    assert.equal(RK.naechsteAktion(['2026-09-01'], ['A'], plan, 'frueh'), 'add');
    assert.equal({}.frueh, undefined);
  });

  test('das Raster bleibt auch weit weg von heute vollstaendig', () => {
    for (const [jahr, monat] of [[1970, 1], [2000, 2], [2100, 12], [2038, 1]]) {
      const { zellen } = RK.monatsRaster(jahr, monat);
      assert.equal(zellen.length % 7, 0, `${jahr}-${monat}`);
      assert.equal(zellen.filter(z => !z.fremd).length, new Date(jahr, monat, 0).getDate());
    }
  });

  test('das Raster stimmt in jeder Zeitzone', () => {
    for (const tz of ['Europe/Berlin', 'Asia/Kolkata', 'Pacific/Kiritimati', 'Pacific/Niue']) {
      const zellen = inZeitzone(tz, () => RK.monatsRaster(2026, 9).zellen);
      assert.equal(zellen[1].key, '2026-09-01', tz);
      assert.equal(zellen.filter(z => !z.fremd).length, 30, tz);
    }
  });
});
