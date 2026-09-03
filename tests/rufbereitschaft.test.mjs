// Der Weg vom Rufbereitschaftsreiter in den Kalender.
//
// Gemeldeter Fehler: "überträgt noch nicht zuverlässig die eingetragenen Daten
// an den Kalender". Ursache war, dass die Tabelle ihre Änderungen nur im
// Speicher hielt, der Server beim Übertragen aber den GESPEICHERTEN Plan
// liest - wer nicht vorher auf Speichern klickte, übertrug den alten Stand.
import { test, describe } from 'node:test';
import assert from 'node:assert/strict';
import { starteOberflaeche, klick, ruhe, warteBis, isoWochenSchluessel } from './lade-oberflaeche.mjs';

const TEAM = [
  { name: 'Bauer, Martin', team: 'DE', color: '#4a9eff', icon: '', prefs: {} },
  { name: 'Nair, Anita', team: 'IN', color: '#a78bfa', icon: '', prefs: {} },
];

// Oeffnet den Reiter und liefert die Zeile einer Kalenderwoche.
async function imReiter(o) {
  klick(o.$('.vtab[data-view="ruf"]'));
  await warteBis(() => o.$('.rufkw-person-cell'), 'die KW-Tabelle');
  return {
    zelle: kwKey => o.$(`.rufkw-person-cell[data-kwkey="${kwKey}"]`),
    // Person ueber den Picker zuweisen - derselbe Weg wie mit der Maus.
    async weise(kwKey, name) {
      const zelle = o.$(`.rufkw-person-cell[data-kwkey="${kwKey}"]`);
      assert.ok(zelle, `Zeile ${kwKey} fehlt in der Tabelle`);
      klick(zelle.querySelector('.rufkw-add'));
      await ruhe();
      const eintrag = [...o.$('#rufkw-picker').querySelectorAll('[data-pname]')]
        .find(el => el.dataset.pname === name);
      assert.ok(eintrag, `${name} steht nicht im Picker`);
      klick(eintrag);
      await ruhe();
    },
    async uebertrage(jahr) {
      klick([...o.$$('#ruf-wrap button')].find(b => b.textContent.includes('In den Kalender')));
      await ruhe();
      o.$('#rufkw-apply-year').value = String(jahr);
      klick([...o.$$('#rufkw-apply-modal button')].find(b => b.textContent.includes('Anwenden')));
      await warteBis(() => o.api.aufrufe.some(a => a.pfad === '/api/ruf_kw/apply'), 'die Uebertragung');
      await ruhe();
    },
  };
}

// Eine Woche mitten im Jahr - keine Randfaelle an der Jahresgrenze.
const JAHR = new Date().getFullYear();
const KW = `${JAHR}-W10`;

describe('Rufbereitschaft in den Kalender übertragen', () => {
  test('eine zugewiesene Woche wird sofort gespeichert', async () => {
    const o = await starteOberflaeche({ mitarbeiter: TEAM });
    const reiter = await imReiter(o);
    await reiter.weise(KW, 'Nair, Anita');

    // Ohne Speichern kennt der Server den Eintrag nicht - und genau daran
    // scheiterte das Übertragen.
    await warteBis(() => o.api.zustand.ruf_kw[KW]?.includes('Nair, Anita'),
      'den gespeicherten KW-Plan');
  });

  test('was in der Tabelle steht, landet auch im Kalender', async () => {
    const o = await starteOberflaeche({ mitarbeiter: TEAM });
    const reiter = await imReiter(o);
    await reiter.weise(KW, 'Nair, Anita');
    await reiter.uebertrage(JAHR);

    const antwort = o.api.aufrufe.filter(a => a.pfad === '/api/ruf_kw/apply');
    assert.equal(antwort.length, 1, 'genau eine Übertragung');
    // Sieben Tage der Woche, alle mit Nair besetzt.
    const tage = Object.entries(o.api.zustand.schichten)
      .filter(([, t]) => t.rufbereitschaft.includes('Nair, Anita'))
      .map(([k]) => k);
    assert.equal(tage.length, 7, `übertragene Tage: ${tage.join(', ')}`);
    for (const t of tage)
      assert.equal(isoWochenSchluessel(new Date(t + 'T00:00:00')), KW, `${t} gehört nicht zu ${KW}`);
  });

  test('eine entfernte Person verschwindet auch aus dem gespeicherten Plan', async () => {
    const o = await starteOberflaeche({ mitarbeiter: TEAM });
    const reiter = await imReiter(o);
    await reiter.weise(KW, 'Nair, Anita');
    await warteBis(() => o.api.zustand.ruf_kw[KW], 'den Eintrag');

    // Das ✕ am Chip nimmt die Person wieder heraus.
    klick(reiter.zelle(KW).querySelector('[data-rm-kw]'));
    await warteBis(() => !o.api.zustand.ruf_kw[KW], 'das Entfernen');
  });

  test('zwei Personen in einer Woche kommen beide an', async () => {
    const o = await starteOberflaeche({ mitarbeiter: TEAM });
    const reiter = await imReiter(o);
    await reiter.weise(KW, 'Nair, Anita');
    await reiter.weise(KW, 'Bauer, Martin');
    await warteBis(() => o.api.zustand.ruf_kw[KW]?.length === 2, 'beide Namen');
    await reiter.uebertrage(JAHR);

    const montag = Object.entries(o.api.zustand.schichten)
      .find(([k]) => isoWochenSchluessel(new Date(k + 'T00:00:00')) === KW);
    assert.deepEqual(montag[1].rufbereitschaft.sort(), ['Bauer, Martin', 'Nair, Anita']);
  });
});
