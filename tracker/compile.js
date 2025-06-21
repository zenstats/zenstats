const uglify = require("uglify-js");
const fs = require('fs')
const path = require('path')
const Handlebars = require("handlebars");
const g = require("generatorics");
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

function compilefile(input, output, templateVars = {}) {
  const code = fs.readFileSync(input).toString()
  const template = Handlebars.compile(code)
  const rendered = template({ ...templateVars, TRACKER_SCRIPT_VERSION: tracker_script_version })
  const result = uglify.minify(rendered)
  if (result.code) {
    fs.writeFileSync(output, result.code)
  } else {
    throw new Error(`Failed to compile ${output.split('/').pop()}.\n${result.error}\n`)
  }
}

compilefile(relPath('src/zenstats.js'), relPath('../zenstats.js'))
