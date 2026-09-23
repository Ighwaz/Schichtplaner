// Typpruefung fuer frontend/index.html.
//
// Die Oberflaeche ist eine einzige Datei ohne Build-Schritt, und dabei
// bleibt es: die EXE bettet genau diese Datei ein. tsc kann aber kein
// JavaScript pruefen, das in HTML steht. Dieses Skript schneidet deshalb die
// beiden <script>-Bloecke heraus, legt sie unter tests/.typen/ ab und laesst
// tsc darueber laufen - ohne etwas zu erzeugen (noEmit).
//
// Fuehrende Leerzeilen halten die Zeilennummern deckungsgleich mit
// index.html, und die Pfade in den Meldungen werden zurueckgeschrieben:
// was tsc meldet, steht an genau dieser Stelle in frontend/index.html.
//
// Zwei Durchgaenge, zwei Strenggrade:
//   tsconfig.regelkern.json  der Regelkern allein, streng (strict)
//   tsconfig.json            beide Bloecke zusammen, die Oberflaeche lockerer
// Der zweite braucht den ersten mit: die Oberflaeche ruft den Regelkern auf,
// und dessen Typen sollen bis in jeden Aufruf durchgreifen.
//
// Ausfuehren:  npm run typen      (laeuft auch vor jedem npm test)
import { readFileSync, writeFileSync, mkdirSync } from 'node:fs';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { dirname, join, relative } from 'node:path';

const hier   = dirname(fileURLToPath(import.meta.url));
const wurzel = join(hier, '..');
const quelle = join(wurzel, 'frontend', 'index.html');
const ablage = join(hier, '.typen');

const html = readFileSync(quelle, 'utf8').replace(/\r\n/g, '\n');

// Die beiden Bloecke, an ihrem oeffnenden Tag erkannt. \n statt \r?\n, weil
// oben schon auf LF normalisiert wurde - eine Datei mit CRLF soll hier nicht
// stillschweigend "keinen Regelkern" melden.
const BLOECKE = [
  { datei: 'regelkern.js',   muster: /<script id="regelkern">\n([\s\S]*?)\n<\/script>/ },
  { datei: 'oberflaeche.js', muster: /<script>\n('use strict';[\s\S]*?)\n<\/script>/ },
];

mkdirSync(ablage, { recursive: true });
for (const { datei, muster } of BLOECKE) {
  const treffer = html.match(muster);
  if (!treffer) {
    console.error(`typpruefung: in frontend/index.html fehlt der Block fuer ${datei}. ` +
      'Wurde ein <script>-Tag geaendert, muss das Muster hier mitgezogen werden.');
    process.exit(2);
  }
  // Zeile des ersten Codezeichens, 1-basiert: alles davor wird zu Leerzeilen.
  const vorlauf = html.slice(0, treffer.index + treffer[0].indexOf(treffer[1])).split('\n').length - 1;
  writeFileSync(join(ablage, datei), '\n'.repeat(vorlauf) + treffer[1] + '\n', 'utf8');
}

const tsc = join(wurzel, 'node_modules', 'typescript', 'bin', 'tsc');
const zurueck = new RegExp(
  String.raw`tests[\\/]\.typen[\\/](regelkern|oberflaeche)\.js`, 'g');

let fehlgeschlagen = false;
for (const projekt of ['tsconfig.regelkern.json', 'tsconfig.json']) {
  const lauf = spawnSync(process.execPath, [tsc, '-p', join(wurzel, projekt), '--pretty', 'false'],
    { cwd: wurzel, encoding: 'utf8' });
  if (lauf.error) {
    console.error(`typpruefung: tsc liess sich nicht starten (${lauf.error.message}). ` +
      'Einmal "npm install" ausfuehren.');
    process.exit(2);
  }
  const ausgabe = (lauf.stdout + lauf.stderr).replace(zurueck, 'frontend/index.html').trim();
  if (lauf.status !== 0) {
    fehlgeschlagen = true;
    console.error(`\n✖ ${projekt}\n${ausgabe}`);
  } else {
    console.log(`✔ ${projekt}`);
  }
}
if (fehlgeschlagen) process.exit(1);
console.log(`Typen in ${relative(wurzel, quelle)} geprueft.`);
