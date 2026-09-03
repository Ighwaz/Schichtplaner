// Startet die komplette Oberflaeche aus frontend/index.html in jsdom.
//
// Die Anwendung selbst bleibt eine Datei ohne Abhaengigkeiten; jsdom ist reine
// Entwicklungsausstattung (siehe package.json). Die Seite laeuft hier so an
// wie im Fenster - jsdom fuehrt die eigenen Skripte der Datei aus, samt dem
// abschliessenden init(). Nur die API dahinter ist erfunden: beforeParse
// haengt die Attrappe ein, bevor die erste Zeile laeuft.
//
// Jeder Aufruf von starteOberflaeche() liefert ein frisches Fenster mit
// eigenem Zustand; die Tests koennen sich also nicht gegenseitig stoeren.
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import { JSDOM } from 'jsdom';

const hier = dirname(fileURLToPath(import.meta.url));
const html = readFileSync(join(hier, '..', 'frontend', 'index.html'), 'utf8');

// Ein leerer Tag - dieselbe Form, die der Go-Teil liefert.
const leererTag = () => ({ frueh: [], normal: [], spaet: [], rufbereitschaft: [] });

/**
 * Erfundene API. Haelt Mitarbeiter, Schichten, Soll und Notizen im Speicher
 * und beantwortet genau die Wege, die die Oberflaeche aufruft. Jeder Aufruf
 * wird in .aufrufe mitgeschrieben, damit Tests pruefen koennen, was die
 * Oberflaeche geschickt haette.
 */
export function baueAPI({ mitarbeiter = [], schichten = {}, soll = {}, feiertage = {} } = {}) {
  const zustand = {
    mitarbeiter: [...mitarbeiter],
    schichten: JSON.parse(JSON.stringify(schichten)),
    soll: { frueh: 1, normal: 0, spaet: 1, rufbereitschaft: 1, ...soll },
    notizen: {},
    templates: {},
    custom_holidays: [],
    ruf_kw: {},
  };
  const aufrufe = [];

  const tag = k => (zustand.schichten[k] ||= leererTag());

  function antwort(pfad, opts) {
    const koerper = opts && opts.body ? JSON.parse(opts.body) : null;
    aufrufe.push({ pfad, methode: (opts && opts.method) || 'GET', koerper });

    if (pfad === '/api/data') return { ...zustand };
    if (pfad === '/api/datadir') return { folder: '/testordner', file: '/testordner/schichtplan.db' };
    if (pfad === '/api/holiday_coverage') return { in_movable_from: 2020, in_movable_to: 2030 };
    if (pfad.startsWith('/api/holidays/')) return feiertage;
    if (pfad === '/api/templates') return zustand.templates;
    if (pfad === '/api/ruf_kw') return zustand.ruf_kw;
    if (pfad === '/api/custom_holidays') return zustand.custom_holidays;
    if (pfad.startsWith('/api/history')) return [];

    if (pfad === '/api/schicht') {
      const { dates, schicht, name, action } = koerper;
      const results = {};
      for (const d of dates) {
        const t = tag(d);
        if (action === 'add' && !t[schicht].includes(name)) t[schicht].push(name);
        if (action === 'remove') t[schicht] = t[schicht].filter(n => n !== name);
        results[d] = JSON.parse(JSON.stringify(t));
      }
      return { results, hol_warnings: {} };
    }
    if (pfad === '/api/paste') {
      for (const d of koerper.dates) {
        if (koerper.mode === 'replace') zustand.schichten[d] = { ...leererTag(), ...koerper.slot };
        else for (const [s, namen] of Object.entries(koerper.slot || {}))
          for (const n of namen || []) if (!tag(d)[s].includes(n)) tag(d)[s].push(n);
      }
      return { ok: true };
    }
    if (pfad === '/api/soll') { Object.assign(zustand.soll, koerper); return { ok: true }; }
    if (pfad === '/api/notiz') { zustand.notizen[koerper.date] = koerper.text; return { ok: true }; }
    if (pfad === '/api/snapshot') { zustand.schichten = koerper.schichten || {}; return { ok: true }; }
    return { ok: true };
  }

  return { zustand, aufrufe, antwort };
}

// Wartet, bis eine Bedingung zutrifft - oder bricht mit einer Meldung ab, die
// sagt, worauf gewartet wurde.
export async function warteBis(bedingung, was = 'Bedingung', versuche = 200) {
  for (let i = 0; i < versuche; i++) {
    if (bedingung()) return;
    await new Promise(r => setTimeout(r, 5));
  }
  throw new Error(`Zeit abgelaufen beim Warten auf: ${was}`);
}

/**
 * Baut ein Fenster mit geladener Oberflaeche.
 * Liefert { fenster, dokument, api, $, $$ }.
 */
export async function starteOberflaeche(vorgabe = {}) {
  const api = baueAPI(vorgabe);
  const dom = new JSDOM(html, {
    runScripts: 'dangerously',
    url: 'http://localhost/',
    pretendToBeVisual: true,
    beforeParse(fenster) {
      // Muss stehen, bevor die erste Zeile der Seite laeuft: init() greift
      // sofort zur API.
      fenster.fetch = async (pfad, opts) => ({ ok: true, json: async () => api.antwort(pfad, opts) });
      fenster.print = () => {};
      fenster.Element.prototype.scrollIntoView = function () {};
    },
  });
  const fenster = dom.window;

  // init() laeuft von selbst los; fertig ist es, wenn der Kalender steht.
  await warteBis(() => fenster.document.querySelector('.day-cell'), 'den ersten Kalendertag');
  await warteBis(() => api.aufrufe.some(a => a.pfad === '/api/datadir'), 'das Ende von init()');

  const $ = wahl => fenster.document.querySelector(wahl);
  const $$ = wahl => [...fenster.document.querySelectorAll(wahl)];
  return { fenster, dokument: fenster.document, api, $, $$ };
}

// Klick mit den ueblichen Zusatztasten.
export function klick(el, opts = {}) {
  el.dispatchEvent(new el.ownerDocument.defaultView.MouseEvent('click', { bubbles: true, ...opts }));
}

// Wartet, bis die angestossenen Zusagen abgearbeitet sind. Die Klickpfade
// sind async; ohne das prueft der Test den Stand von vorher.
export const ruhe = () => new Promise(r => setTimeout(r, 0));
