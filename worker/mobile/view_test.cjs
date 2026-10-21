/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

// Verifies which view of the mobile application is visible.
//
// The script of the bundled web interface is executed with a small DOM stub. The
// behaviour of the views is checked for a fresh installation, for a missing
// connection, for a removed device and for a paired device.
//
// Usage: node view_test.cjs <path to index.html>

const fs = require('node:fs')
const vm = require('node:vm')

const path = process.argv[2]
if (!path) {
  console.error('usage: node view_test.cjs <index.html>')
  process.exit(2)
}

const html = fs.readFileSync(path, 'utf8')
const match = html.match(/<script>([\s\S]*?)<\/script>/)
if (!match) {
  console.error('the interface does not contain a script')
  process.exit(1)
}
const script = match[1]

function createElement() {
  const classes = new Set()
  return {
    innerText: '',
    innerHTML: '',
    style: {},
    classList: {
      add: (name) => classes.add(name),
      remove: (name) => classes.delete(name),
      toggle: (name, force) => (force ? classes.add(name) : classes.delete(name)),
      contains: (name) => classes.has(name),
    },
    hasClass: (name) => classes.has(name),
    disabled: false,
    appendChild() {},
    click() {},
    focus() {},
  }
}

const elements = new Map()
const document = {
  getElementById: (id) => {
    if (!elements.has(id)) elements.set(id, createElement())
    return elements.get(id)
  },
  createElement: () => createElement(),
  body: { appendChild() {}, removeChild() {} },
  addEventListener() {},
}

const context = {
  document,
  window: {},
  console,
  setInterval: () => 0,
  setTimeout: () => 0,
  clearInterval: () => {},
  clearTimeout: () => {},
  navigator: { clipboard: { writeText() {} } },
  localStorage: { getItem: () => null, setItem() {} },
  location: { origin: 'http://127.0.0.1' },
  URL,
}
context.window = context
context.AndroidBridge = {}
vm.createContext(context)
vm.runInContext(script, context)

const visible = (id) => (elements.get(id).hasClass('hidden') ? 'hidden' : 'visible')
const permissionsView = () => visible('permissions-view')
const pairingView = () => visible('unpaired-view')
const dashboardView = () => visible('paired-view')

let failures = 0
function check(label, actual, expected) {
  const ok = actual === expected
  if (!ok) failures++
  console.log(`${ok ? 'PASS' : 'FAIL'}  ${label}: ${actual} (expected ${expected})`)
}

function setPermissions(granted) {
  context.AndroidBridge.getPermissionStatus = () =>
    JSON.stringify({
      notifications: granted,
      battery: granted,
      bgactivity: granted,
      autostart: granted,
    })
  context.refreshPermissions()
}

console.log('=== 1) fresh installation, no permission granted ===')
setPermissions(false)
context.updateState(JSON.stringify({ is_paired: false }))
check('permission view', permissionsView(), 'visible')
check('pairing view', pairingView(), 'hidden')
check('dashboard view', dashboardView(), 'hidden')

console.log('\n=== 2) live check, no connection, no permission granted ===')
context.AndroidBridge.getWorkerStatus = () =>
  JSON.stringify({ is_paired: false, service_running: false, device_removed: false })
context.checkWorkerStatus()
check('permission view', permissionsView(), 'visible')
check('pairing view', pairingView(), 'hidden')
check('connection notice', visible('connection-notice'), 'visible')

console.log('\n=== 3) permissions granted, no connection ===')
setPermissions(true)
context.checkWorkerStatus()
check('permission view', permissionsView(), 'hidden')
check('pairing view', pairingView(), 'visible')

console.log('\n=== 4) the device was removed, no permission granted ===')
setPermissions(false)
context.AndroidBridge.getWorkerStatus = () =>
  JSON.stringify({
    is_paired: false,
    service_running: false,
    device_removed: true,
    device_removed_message: 'Cihaz kaldırıldı',
  })
context.checkWorkerStatus()
check('permission view', permissionsView(), 'visible')
check('pairing view', pairingView(), 'hidden')
check('connection notice', visible('connection-notice'), 'visible')

console.log('\n=== 5) paired device ===')
context.updateState(JSON.stringify({ is_paired: true, model_status: 'ready', current_tps: 12.5 }))
check('dashboard view', dashboardView(), 'visible')
check('permission view', permissionsView(), 'hidden')
check('pairing view', pairingView(), 'hidden')

console.log(failures === 0 ? '\nall view tests passed' : `\n${failures} view tests failed`)
process.exit(failures === 0 ? 0 : 1)
