package web

import (
	"context"
	"fmt"
	"html"
	"io"
	"strings"

	"github.com/a-h/templ"
)

func ListPage(user CurrentUser, title, currentPath, endpoint string) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		clean := strings.ReplaceAll(currentPath, "/", "")
		clean = strings.ReplaceAll(clean, "-", "")
		if clean == "" {
			clean = "list"
		}

		content := fmt.Sprintf(`<section class="space-y-4">
  <div>
    <h1 class="text-3xl font-semibold">%s</h1>
    <p class="mt-1 text-slate-600">Operational data view.</p>
  </div>
  <section class="rounded-xl border border-slate-200 bg-white p-4 shadow-sm">
    <div id="%sState" class="mb-3 rounded border border-slate-200 bg-slate-50 px-3 py-2 text-sm text-slate-600">Loading...</div>
    <div class="overflow-x-auto">
      <table class="min-w-full text-left text-sm" id="%sTable">
        <thead id="%sHead" class="border-b border-slate-200 text-xs uppercase tracking-wider text-slate-500"></thead>
        <tbody id="%sBody" class="divide-y divide-slate-100"></tbody>
      </table>
    </div>
  </section>
</section>

<script>
(async function() {
  const state = document.getElementById('%sState')
  const head = document.getElementById('%sHead')
  const body = document.getElementById('%sBody')
  try {
    const res = await fetch('%s', { credentials: 'same-origin' })
    const payload = await res.json()
    if (!res.ok) {
      state.textContent = payload.message || 'Failed to load data'
      state.className = 'mb-3 rounded border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700'
      return
    }

    const rows = Array.isArray(payload) ? payload : (Array.isArray(payload.items) ? payload.items : [])
    if (rows.length === 0) {
      state.textContent = 'No records found.'
      return
    }

    const columns = Object.keys(rows[0])
    head.innerHTML = '<tr>' + columns.map((c) => '<th class="px-3 py-2 font-semibold">' + c.replaceAll('_', ' ') + '</th>').join('') + '</tr>'
    body.innerHTML = rows.map((row) => {
      const cells = columns.map((c) => {
        const raw = row[c]
        const val = Array.isArray(raw) ? raw.join(', ') : (raw === null || raw === undefined ? '' : String(raw))
        return '<td class="px-3 py-2 text-slate-700">' + val + '</td>'
      }).join('')
      return '<tr>' + cells + '</tr>'
    }).join('')
    state.textContent = 'Loaded ' + rows.length + ' records.'
  } catch (_err) {
    state.textContent = 'Unable to load data'
    state.className = 'mb-3 rounded border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700'
  }
})()
</script>`,
			html.EscapeString(title),
			html.EscapeString(clean), html.EscapeString(clean), html.EscapeString(clean), html.EscapeString(clean),
			html.EscapeString(clean), html.EscapeString(clean), html.EscapeString(clean),
			html.EscapeString(endpoint),
		)
		return AppLayout(user, title, currentPath, content).Render(ctx, w)
	})
}
