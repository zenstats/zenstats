const uglify = require("uglify-js");
const fs = require('fs')
const path = require('path')
const Handlebars = require("handlebars");
const g = require('generatorics');
const { tracker_script_version } = require("./package.json");

if (process.env.NODE_ENV === 'dev') {
  console.info('COMPILATION SKIPPED: No changes detected in tracker dependencies')
  process.exit(0)
}

Handlebars.registerHelper('any', function (...args) {
  return args.slice(0, -1).some(Boolean)
})

function relPath(segment) {
  return path.join(__dirname, segment)
}

// ---------------------------------------------------------------------------
// Feature flags definition
// Each entry is a user-facing feature that can be toggled via checkbox.
// The `id` is used for file naming, and the `flag` is the Handlebars / UglifyJS
// compile-time global used in zenstats.js for dead-code elimination.
// ---------------------------------------------------------------------------
const FEATURES = [
  { id: 'ex', flag: 'COMPILE_EXCLUSIONS' },
  { id: 'fd', flag: 'COMPILE_FILE_DOWNLOADS' },
  { id: 'ha', flag: 'COMPILE_HASH' },
  { id: 'ma', flag: 'COMPILE_MANUAL' },
  { id: 'ol', flag: 'COMPILE_OUTBOUND_LINKS' },
  { id: 'te', flag: 'COMPILE_TAGGED_EVENTS' },
]

// These flags are always enabled for every variant.
const BASELINE_FLAGS = {
  COMPILE_PAGEVIEW_PROPS: true,
}

// ---------------------------------------------------------------------------
// Canonical file naming   script.{feature-id}sorted-alphabetically}.js
// ---------------------------------------------------------------------------
function canonicalName(featureIds) {
  if (featureIds.length === 0) return 'script.bare'
  const sorted = [...featureIds].sort()
  return `script.${sorted.join('.')}`
}

// ---------------------------------------------------------------------------
// Backward-compatible name overrides
// Certain feature combinations MUST keep their original file names so that
// existing embedded snippets continue to work.
// ---------------------------------------------------------------------------
const BACKWARD_COMPAT_MAP = {
  // script.js  = all base (everything except hash)
  'script.ex.fd.ma.ol.te': 'script',
  // script.hash.js = all features including hash
  'script.ex.fd.ha.ma.ol.te': 'script.hash',
}

// ---------------------------------------------------------------------------
// Generate all variants via power set of FEATURES.
// If a canonical name collides with a backward-compat target, disambiguate by
// appending '.bare' so the alias can overwrite it later.
// Returns an array of { name, fileName, features, flags }.
// ---------------------------------------------------------------------------
function generateVariants() {
  const featureIds = FEATURES.map(f => f.id)
  const backwardCompatTargets = new Set(Object.values(BACKWARD_COMPAT_MAP))
  const variants = []

  const allSubsets = [...g.clone.powerSet(featureIds)]

  for (const subset of allSubsets) {
    const sorted = [...subset].sort()
    let name = canonicalName(sorted)

    // Disambiguate: if the canonical name is a backward-compat TARGET but this
    // variant is not the backward-compat SOURCE, rename to *.bare.js
    if (backwardCompatTargets.has(name) && !BACKWARD_COMPAT_MAP[name]) {
      name = name + '.bare'
    }

    const flags = { ...BASELINE_FLAGS }
    for (const id of sorted) {
      const feat = FEATURES.find(f => f.id === id)
      flags[feat.flag] = true
    }

    variants.push({
      name,
      fileName: name + '.js',
      features: sorted,
      flags,
    })
  }

  return variants
}

// ---------------------------------------------------------------------------
// Compile a single variant: Handlebars render → UglifyJS minify → write.
// Returns the compiled code string for potential reuse.
// ---------------------------------------------------------------------------
function compileFile(input, output, templateVars = {}) {
  const code = fs.readFileSync(input).toString()
  const template = Handlebars.compile(code)
  const rendered = template({ ...templateVars, TRACKER_SCRIPT_VERSION: tracker_script_version })
  const result = uglify.minify(rendered, {
    compress: {
      global_defs: templateVars,
      passes: 3,
    },
  })
  if (result.code) {
    fs.writeFileSync(output, result.code)
    const sizeKB = (result.code.length / 1024).toFixed(1)
    console.log(`  ✓ ${path.basename(output)} (${sizeKB}KB)`)
    return result.code
  } else if (result.error) {
    throw new Error(`Failed to compile ${path.basename(output)}.\n${result.error}\n`)
  }
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------
const input = relPath('src/zenstats.js')
const outputDir = relPath('dist')
if (!fs.existsSync(outputDir)) {
  fs.mkdirSync(outputDir, { recursive: true })
}

const variants = generateVariants()

// 1. Write all canonically-named files
console.log(`Building ${variants.length} tracker variants:\n`)
const compiled = {}
variants.forEach(function (v) {
  const output = path.join(outputDir, v.fileName)
  const code = compileFile(input, output, v.flags)
  compiled[v.name] = { code, sizeKB: (code.length / 1024).toFixed(1) }
})

// 2. Write backward-compatible aliases (copies)
console.log('')
Object.entries(BACKWARD_COMPAT_MAP).forEach(([canonical, alias]) => {
  const src = path.join(outputDir, canonical + '.js')
  const dest = path.join(outputDir, alias + '.js')
  if (fs.existsSync(src)) {
    fs.copyFileSync(src, dest)
    const stat = fs.statSync(dest)
    console.log(`  ⇢ ${alias}.js ← ${canonical}.js (${(stat.size / 1024).toFixed(1)}KB)`)
  } else {
    console.warn(`  ⚠ ${canonical}.js not found, skipping alias ${alias}.js`)
  }
})

// 3. Also output the default full script to project root for backward compat
const fullSrc = path.join(outputDir, 'script.ex.fd.ma.ol.te.js')
if (fs.existsSync(fullSrc)) {
  fs.copyFileSync(fullSrc, relPath('../zenstats.js'))
  console.log(`\n  ⇢ zenstats.js ← ${path.basename(fullSrc)}`)
}

console.log('\nDone.')
