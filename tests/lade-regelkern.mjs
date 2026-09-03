// Schneidet den Regelkern aus frontend/index.html und macht ihn importierbar.
//
// Die Oberflaeche ist bewusst eine einzige Datei - eine EXE, keine
// Runtime-Abhaengigkeiten. Statt das aufzubrechen, holt der Testlauf den
// Block <script id="regelkern"> heraus und legt ihn als tests/.regelkern.js
// ab. Die fuehrenden Leerzeilen halten die Zeilennummern deckungsgleich mit
// index.html: was der Coverage-Bericht als "uncovered line" nennt, ist
// dieselbe Zeile in der Quelldatei.
import { readFileSync, writeFileSync } from 'node:fs';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { dirname, join } from 'node:path';

const hier = dirname(fileURLToPath(import.meta.url));
const quelle = join(hier, '..', 'frontend', 'index.html');
const ziel = join(hier, '.regelkern.js');

const html = readFileSync(quelle, 'utf8');
const treffer = html.match(/<script id="regelkern">\n([\s\S]*?)\n<\/script>/);
if (!treffer) {
  throw new Error(
    'In frontend/index.html fehlt der Block <script id="regelkern">. ' +
    'Wurde er umbenannt, muss diese Datei mitgezogen werden.'
  );
}

const zeileDesBlocks = html.slice(0, treffer.index).split('\n').length; // 1-basiert
writeFileSync(ziel, '\n'.repeat(zeileDesBlocks) + treffer[1] + '\n', 'utf8');

await import(pathToFileURL(ziel).href);

export const RK = globalThis.Regelkern;
if (!RK) throw new Error('Der Regelkern hat globalThis.Regelkern nicht gesetzt.');

// Fuehrt fn() unter einer festen Zeitzone aus und stellt die alte danach her.
export function inZeitzone(tz, fn) {
  const vorher = process.env.TZ;
  process.env.TZ = tz;
  try {
    return fn();
  } finally {
    if (vorher === undefined) delete process.env.TZ;
    else process.env.TZ = vorher;
  }
}
