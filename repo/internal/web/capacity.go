package web

import (
	"context"
	"io"

	"github.com/a-h/templ"
)

func CapacityPage(user CurrentUser) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		content := `<section class="space-y-6">
  <div>
    <h1 class="text-3xl font-semibold">Capacity</h1>
    <p class="mt-1 text-slate-600">Zone-level capacity overview.</p>
  </div>
  <section class="rounded-xl border border-slate-200 bg-white p-5 shadow-sm">
    <div id="capacityState" class="mb-3 rounded border border-slate-200 bg-slate-50 px-3 py-2 text-sm text-slate-600">Loading...</div>
    <div class="overflow-x-auto">
      <table class="min-w-full text-left text-sm">
        <thead class="border-b border-slate-200 text-xs uppercase tracking-wider text-slate-500">
          <tr>
            <th class="px-3 py-2">Zone</th>
            <th class="px-3 py-2">Total</th>
            <th class="px-3 py-2">Held</th>
            <th class="px-3 py-2">Confirmed</th>
            <th class="px-3 py-2">Available</th>
          </tr>
        </thead>
        <tbody id="capacityBody" class="divide-y divide-slate-100"></tbody>
      </table>
    </div>
  </section>
</section>

<script>
(async function() {
  const state = document.getElementById('capacityState')
  const body = document.getElementById('capacityBody')
  try {
    const res = await fetch('/api/capacity/dashboard', { credentials: 'same-origin' })
    const payload = await res.json()
    if (!res.ok) {
      state.textContent = payload.message || 'Failed to load capacity data'
      state.className = 'mb-3 rounded border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700'
      return
    }
    const zones = Array.isArray(payload.zones) ? payload.zones : []
    if (zones.length === 0) {
      state.textContent = 'No zones configured.'
      return
    }
    body.innerHTML = zones.map((z) => '<tr>'
      + '<td class="px-3 py-2">' + z.zone_name + '</td>'
      + '<td class="px-3 py-2">' + z.total_stalls + '</td>'
      + '<td class="px-3 py-2">' + z.held_stalls + '</td>'
      + '<td class="px-3 py-2">' + z.confirmed_stalls + '</td>'
      + '<td class="px-3 py-2 font-semibold">' + z.available_stalls + '</td>'
      + '</tr>').join('')
    state.textContent = 'Loaded ' + zones.length + ' zones.'
  } catch (_err) {
    state.textContent = 'Unable to load capacity data'
    state.className = 'mb-3 rounded border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700'
  }
})()
</script>`
		return AppLayout(user, "Capacity", "/capacity", content).Render(ctx, w)
	})
}
