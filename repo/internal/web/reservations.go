package web

import (
	"context"
	"io"

	"github.com/a-h/templ"
)

func ReservationsPage(user CurrentUser) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		content := `<section class="space-y-6">
    <header>
      <h1 class="text-3xl font-semibold">Reservation Calendar</h1>
      <p class="text-slate-600 mt-2">Check zone availability before confirmation to avoid conflicts.</p>
    </header>

    <section class="rounded-xl bg-white p-6 shadow-sm space-y-4">
      <h2 class="text-xl font-semibold">Conflict Warning Check</h2>
      <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
        <label class="text-sm">
          <span class="mb-1 block text-slate-700">Zone ID</span>
          <input id="zoneId" class="w-full rounded border border-slate-300 px-3 py-2" placeholder="zone uuid" />
        </label>
        <label class="text-sm">
          <span class="mb-1 block text-slate-700">Requested Stalls</span>
          <input id="stallCount" type="number" min="1" value="1" class="w-full rounded border border-slate-300 px-3 py-2" />
        </label>
        <label class="text-sm">
          <span class="mb-1 block text-slate-700">Start (RFC3339)</span>
          <input id="startAt" class="w-full rounded border border-slate-300 px-3 py-2" placeholder="2026-01-01T10:00:00Z" />
        </label>
        <label class="text-sm">
          <span class="mb-1 block text-slate-700">End (RFC3339)</span>
          <input id="endAt" class="w-full rounded border border-slate-300 px-3 py-2" placeholder="2026-01-01T12:00:00Z" />
        </label>
      </div>
      <button id="checkBtn" class="rounded bg-slate-900 px-4 py-2 text-white">Check Availability</button>
      <div id="result" class="rounded border border-slate-200 bg-slate-50 p-3 text-sm text-slate-700">No availability check run yet.</div>
      <div id="warning" class="hidden rounded border border-amber-300 bg-amber-50 p-3 text-sm text-amber-800">
        Conflict warning: this request may exceed available stalls. Review before confirmation.
      </div>
    </section>

  <script>
    const checkBtn = document.getElementById('checkBtn')
    const warning = document.getElementById('warning')
    const result = document.getElementById('result')

    checkBtn.addEventListener('click', async () => {
      warning.classList.add('hidden')
      const zoneId = document.getElementById('zoneId').value.trim()
      const startAt = document.getElementById('startAt').value.trim()
      const endAt = document.getElementById('endAt').value.trim()
      const requested = parseInt(document.getElementById('stallCount').value || '1', 10)

      const url = '/api/availability?zone_id=' + encodeURIComponent(zoneId)
        + '&time_window_start=' + encodeURIComponent(startAt)
        + '&time_window_end=' + encodeURIComponent(endAt)

      const res = await fetch(url, { credentials: 'same-origin' })
      const payload = await res.json()
      if (!res.ok) {
        result.textContent = payload.message || 'Failed to check availability.'
        return
      }

      result.textContent = 'Available stalls: ' + payload.available_stalls + ' / ' + payload.total_stalls
      if (Number(payload.available_stalls) < requested) {
        warning.classList.remove('hidden')
      }
    })
  </script>
</section>`
		return AppLayout(user, "Reservations", "/reservations", content).Render(ctx, w)
	})
}
