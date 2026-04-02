package web

import (
	"context"
	"io"

	"github.com/a-h/templ"
)

func DashboardPage(user CurrentUser) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		_ = ctx
		content := `<section class="space-y-6">
  <div>
    <h1 class="text-3xl font-semibold">Dashboard</h1>
    <p class="mt-1 text-slate-600">Current zone capacity and operational snapshot.</p>
  </div>

  <section class="grid gap-4 md:grid-cols-2 xl:grid-cols-4" id="capacityCards">
    <article class="rounded-xl border border-slate-200 bg-white p-4 shadow-sm">
      <p class="text-sm text-slate-500">Loading capacity cards...</p>
    </article>
  </section>

  <section class="grid gap-4 md:grid-cols-2">
    <article class="rounded-xl border border-slate-200 bg-white p-5 shadow-sm">
      <h2 class="text-lg font-semibold">Quick stats</h2>
      <dl class="mt-3 space-y-3 text-sm">
        <div class="flex items-center justify-between">
          <dt class="text-slate-600">Total zones</dt>
          <dd id="statZones" class="font-semibold text-slate-900">-</dd>
        </div>
        <div class="flex items-center justify-between">
          <dt class="text-slate-600">Reservations today</dt>
          <dd id="statReservations" class="font-semibold text-slate-900">-</dd>
        </div>
      </dl>
    </article>

    <article class="rounded-xl border border-slate-200 bg-white p-5 shadow-sm">
      <h2 class="text-lg font-semibold">Activity feed</h2>
      <p class="mt-3 text-sm text-slate-600">Recent activity snapshot from polling the live APIs.</p>
      <p class="mt-2 text-xs text-slate-500" id="activityStamp">Waiting for first poll...</p>
    </article>
  </section>

  <section class="grid gap-4 md:grid-cols-2">
    <article class="rounded-xl border border-slate-200 bg-white p-5 shadow-sm">
      <h2 class="text-lg font-semibold">Upcoming hold expirations</h2>
      <ul id="holdExpirations" class="mt-3 space-y-2 text-sm text-slate-700">
        <li class="text-slate-500">Loading...</li>
      </ul>
    </article>

    <article class="rounded-xl border border-slate-200 bg-white p-5 shadow-sm">
      <h2 class="text-lg font-semibold">Open exceptions</h2>
      <ul id="openExceptions" class="mt-3 space-y-2 text-sm text-slate-700">
        <li class="text-slate-500">Loading...</li>
      </ul>
    </article>
  </section>

  <section id="overCapacityWarnings" class="hidden rounded-xl border border-amber-300 bg-amber-50 p-4 text-sm text-amber-900"></section>
</section>

<script>
async function refreshDashboard() {
  const cards = document.getElementById('capacityCards')
  const statZones = document.getElementById('statZones')
  const statReservations = document.getElementById('statReservations')
  const warnings = document.getElementById('overCapacityWarnings')
  const holds = document.getElementById('holdExpirations')
  const exceptions = document.getElementById('openExceptions')
  const activityStamp = document.getElementById('activityStamp')
  try {
    const [capRes, statsRes, holdRes, excRes] = await Promise.all([
      fetch('/api/capacity/dashboard', { credentials: 'same-origin' }),
      fetch('/api/reservations/stats/today', { credentials: 'same-origin' }),
      fetch('/api/reservations?status=hold&limit=20', { credentials: 'same-origin' }),
      fetch('/api/exceptions', { credentials: 'same-origin' }),
    ])

    activityStamp.textContent = 'Last polled at ' + new Date().toLocaleTimeString()

    if (!capRes.ok) {
      throw new Error('failed to load capacity dashboard')
    }
    const cap = await capRes.json()
    const zones = Array.isArray(cap.zones) ? cap.zones : []
    statZones.textContent = String(zones.length)

    if (statsRes.ok) {
      const stats = await statsRes.json()
      statReservations.textContent = String(stats.total_reservations_today || 0)
    } else {
      statReservations.textContent = '0'
    }

    if (zones.length === 0) {
      cards.innerHTML = '<article class="rounded-xl border border-slate-200 bg-white p-4 shadow-sm"><p class="text-sm text-slate-500">No zones configured.</p></article>'
      return
    }

    cards.innerHTML = zones.map((zone) => {
      return '<article class="rounded-xl border border-slate-200 bg-white p-4 shadow-sm">'
        + '<p class="text-sm text-slate-500">' + zone.zone_name + '</p>'
        + '<p class="mt-1 text-2xl font-semibold text-slate-900">' + zone.available_stalls + '</p>'
        + '<p class="text-xs text-slate-500">available of ' + zone.total_stalls + '</p>'
        + '</article>'
    }).join('')

    const zeroZones = zones.filter((z) => Number(z.available_stalls) === 0)
    if (zeroZones.length > 0) {
      warnings.classList.remove('hidden')
      warnings.textContent = 'Over-capacity warning: ' + zeroZones.map((z) => z.zone_name).join(', ') + ' has no available stalls.'
    } else {
      warnings.classList.add('hidden')
      warnings.textContent = ''
    }

    if (holdRes.ok) {
      const holdPayload = await holdRes.json()
      const holdItems = Array.isArray(holdPayload.items) ? holdPayload.items : []
      if (holdItems.length === 0) {
        holds.innerHTML = '<li class="text-slate-500">No active holds.</li>'
      } else {
        holds.innerHTML = holdItems.slice(0, 6).map((h) => {
          const at = h.hold_expires_at || 'unknown'
          return '<li class="rounded border border-slate-200 bg-slate-50 px-3 py-2">Reservation ' + h.id + ' expires at ' + at + '</li>'
        }).join('')
      }
    }

    if (excRes.ok) {
      const excPayload = await excRes.json()
      const excItems = Array.isArray(excPayload.items) ? excPayload.items : []
      if (excItems.length === 0) {
        exceptions.innerHTML = '<li class="text-slate-500">No open exceptions.</li>'
      } else {
        exceptions.innerHTML = excItems.map((ex) => {
          return '<li class="rounded border border-slate-200 bg-slate-50 px-3 py-2">' + (ex.exception_type || 'exception') + '</li>'
        }).join('')
      }
    }
  } catch (_err) {
    cards.innerHTML = '<article class="rounded-xl border border-rose-200 bg-rose-50 p-4 text-sm text-rose-700">Unable to load dashboard metrics.</article>'
    holds.innerHTML = '<li class="text-rose-700">Unable to load holds.</li>'
    exceptions.innerHTML = '<li class="text-rose-700">Unable to load exceptions.</li>'
  }
}

refreshDashboard()
setInterval(refreshDashboard, 10000)
</script>`

		return AppLayout(user, "Dashboard", "/dashboard", content).Render(ctx, w)
	})
}
