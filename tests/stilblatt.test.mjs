// Fehler im Stilblatt, die kein DOM-Test sieht.
//
// Beide Fehler hier waren jahrelang da und fielen niemandem auf, weil sie
// nichts kaputtmachen: etwas sieht anders aus, oder man sieht es gar nicht.
// Ein Test, der nur Knoten zaehlt, geht daran vorbei - und ein Mensch merkt
// nicht, dass ein Tooltip fehlt, den er nie gesehen hat.
//
// jsdom rechnet die Kaskade mit, also lassen sich beide als Verhalten
// pruefen statt als Zeichenkette im Quelltext.
import { test, describe } from 'node:test';
import assert from 'node:assert/strict';
import { starteOberflaeche } from './lade-oberflaeche.mjs';

describe('Der Tooltip ist sichtbar, wenn er gezeigt wird', () => {
  // Frueher stand das Element zweimal im Stilblatt: .tooltip (Deckkraft 0,
  // .show macht 1) und weiter unten #tooltip mit display:none. Ein id wiegt
  // schwerer als zwei Klassen, also gewann display:none - showTooltip baute
  // den Inhalt, setzte .show, platzierte das Kaestchen, und zu sehen war
  // nichts.
  test('.show hebt das Verstecken wirklich auf', async () => {
    const o = await starteOberflaeche();
    const tt = o.$('#tooltip');
    const vorher = o.fenster.getComputedStyle(tt);
    assert.equal(vorher.opacity, '0', 'ohne .show soll er unsichtbar sein');

    tt.classList.add('show');
    const nachher = o.fenster.getComputedStyle(tt);
    assert.notEqual(nachher.display, 'none',
      'eine andere Regel haelt display:none fest - .show kann nur die Deckkraft');
    assert.equal(nachher.opacity, '1');
  });

  test('der Weg über showTooltip zeigt den Tag', async () => {
    const heute = new Date();
    const tag = `${heute.getFullYear()}-${String(heute.getMonth() + 1).padStart(2, '0')}-15`;
    const o = await starteOberflaeche({
      mitarbeiter: [{ name: 'Bauer, Martin', team: 'DE', color: '#4a9eff', icon: '', prefs: {} }],
      schichten: { [tag]: { frueh: ['Bauer, Martin'], normal: [], spaet: [], rufbereitschaft: [] } },
    });
    const zelle = o.$(`.day-cell[data-key="${tag}"]`);
    assert.ok(zelle, `die Zelle ${tag} fehlt`);

    o.fenster.showTooltip(zelle, tag);
    const tt = o.$('#tooltip');
    const stil = o.fenster.getComputedStyle(tt);
    assert.notEqual(stil.display, 'none', 'der Tooltip bleibt unsichtbar');
    assert.equal(stil.opacity, '1');
    assert.match(tt.textContent, /Bauer, Martin/, 'der Inhalt fehlt');

    o.fenster.hideTooltip();
    assert.equal(o.fenster.getComputedStyle(tt).opacity, '0', 'er geht nicht wieder weg');
  });
});

describe('Die Schriften werden überhaupt geladen', () => {
  // @import muss vor jeder anderen Regel stehen. Stand es dahinter - hier
  // hinter sechs @keyframes und einem @media-Block -, ist die Zeile
  // unzulaessig und der Browser verwirft sie stillschweigend. Gemessen war
  // die Textbreite in "Inter" dann auf das Zeichen gleich der in einer frei
  // erfundenen Schriftfamilie: es lief immer die Ersatzschrift.
  test('das @import steht vor allen anderen Regeln', async () => {
    const o = await starteOberflaeche();
    const blatt = o.dokument.styleSheets[0];
    const regeln = [...blatt.cssRules];
    const importe = regeln
      .map((r, i) => ({ r, i }))
      .filter(x => x.r.constructor.name === 'CSSImportRule');
    assert.equal(importe.length, 1, 'genau ein @import erwartet');
    assert.equal(importe[0].i, 0,
      `das @import steht an Stelle ${importe[0].i} - davor duerfen keine Regeln stehen, ` +
      'sonst verwirft der Browser es');
  });

  test('hell ist die Vorgabe, schon im ersten Bild', async () => {
    // Zwei Stellen entscheiden das: das Attribut im <html>-Tag malt das
    // erste Bild, bevor init() laeuft, und init() setzt es danach aus dem
    // gemerkten Wert. Standen die beiden auf verschiedenen Themen, blitzte
    // beim Start kurz das falsche auf.
    const o = await starteOberflaeche();
    assert.equal(o.dokument.documentElement.dataset.theme, 'light');
    assert.match(o.$('#btn-theme').textContent, /Theme/);
  });

  test('ohne Netz greift eine Oberflächenschrift, nicht die Vorgabe des Browsers', async () => {
    // Das @import holt die Schrift aus dem Netz. Ein Arbeitsplatz ohne
    // Verbindung bekommt sie also nicht - und fiel vorher direkt auf
    // sans-serif, was in WebView2 Arial heisst. Dazwischen gehoert die
    // Schrift des Systems, damit der unverbundene Start wie eine
    // Entscheidung aussieht und nicht wie ein Unfall.
    const o = await starteOberflaeche();
    const schrift = o.fenster.getComputedStyle(o.dokument.body).fontFamily;
    assert.match(schrift, /Inter/, 'Inter fehlt');
    assert.match(schrift, /system-ui|Segoe UI/,
      'zwischen Inter und sans-serif fehlt eine Schrift, die es auf dem Rechner gibt');
    assert.match(schrift, /sans-serif/, 'die allgemeine Ersatzschrift fehlt als letzte Stufe');
  });
});
