package web

import (
	"context"
	"io"

	"github.com/a-h/templ"
)

func NotificationPrefsPage(user CurrentUser) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		content := `<section class="space-y-6">
  <div>
    <h1 class="text-3xl font-semibold">Notification Preferences</h1>
    <p class="mt-1 text-slate-600">Manage topic subscriptions and Do-Not-Disturb settings.</p>
  </div>

  <!-- Topic Subscriptions -->
  <section class="rounded-xl border border-slate-200 bg-white p-5 shadow-sm">
    <h2 class="text-lg font-semibold mb-3">Topic Subscriptions</h2>
    <div id="topicState" class="text-sm text-slate-500">Loading...</div>
    <div id="topicList" class="space-y-2 mt-2"></div>
  </section>

  <!-- DND Settings -->
  <section class="rounded-xl border border-slate-200 bg-white p-5 shadow-sm">
    <h2 class="text-lg font-semibold mb-3">Do-Not-Disturb Window</h2>
    <form id="dndForm" class="space-y-3">
      <div class="flex items-center gap-3">
        <label class="text-sm font-medium text-slate-700">Enabled</label>
        <input type="checkbox" id="dndEnabled" class="h-4 w-4">
      </div>
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="block text-sm font-medium text-slate-700 mb-1">Start Time</label>
          <input type="time" id="dndStart" value="22:00" class="w-full rounded border border-slate-300 px-3 py-2 text-sm">
        </div>
        <div>
          <label class="block text-sm font-medium text-slate-700 mb-1">End Time</label>
          <input type="time" id="dndEnd" value="07:00" class="w-full rounded border border-slate-300 px-3 py-2 text-sm">
        </div>
      </div>
      <button type="submit" class="rounded bg-emerald-700 px-4 py-2 text-sm font-medium text-white hover:bg-emerald-800">Save DND Settings</button>
    </form>
    <div id="dndState" class="text-sm text-slate-500 mt-2"></div>
  </section>

  <!-- Frequency Cap Info -->
  <section class="rounded-xl border border-slate-200 bg-white p-5 shadow-sm">
    <h2 class="text-lg font-semibold mb-3">Frequency Cap</h2>
    <p class="text-sm text-slate-600">Notifications are capped at <strong>3 per booking per day</strong>. This is system-enforced and not configurable per-user.</p>
  </section>
</section>

<script>
(function(){
  async function loadTopics() {
    try {
      var res = await fetch('/api/notification-topics', {credentials:'same-origin'});
      if (!res.ok) { document.getElementById('topicState').textContent = 'Failed to load topics'; return; }
      var topics = await res.json();
      if (!Array.isArray(topics)) topics = topics.items || [];

      if (topics.length === 0) { document.getElementById('topicState').textContent = 'No topics available.'; return; }
      document.getElementById('topicState').textContent = '';
      document.getElementById('topicList').innerHTML = topics.map(function(t) {
        var checked = t.subscribed ? 'checked' : '';
        return '<label class="flex items-center gap-3 p-2 rounded hover:bg-slate-50 cursor-pointer">'
          + '<input type="checkbox" data-topic="'+t.id+'" '+checked+' class="h-4 w-4">'
          + '<span class="text-sm text-slate-700">'+esc(t.name)+'</span></label>';
      }).join('');

      document.querySelectorAll('[data-topic]').forEach(function(cb){
        cb.onchange = async function(){
          var topicId = cb.getAttribute('data-topic');
          if (cb.checked) {
            await fetch('/api/notification-topics/'+topicId+'/subscribe', {method:'POST', credentials:'same-origin'});
          } else {
            await fetch('/api/notification-topics/'+topicId+'/subscribe', {method:'DELETE', credentials:'same-origin'});
          }
          if(window.parkopsToast) window.parkopsToast('Subscription updated','success');
        };
      });
    } catch(e) { document.getElementById('topicState').textContent = 'Error loading topics'; }
  }

  async function loadDND() {
    try {
      var res = await fetch('/api/notification-settings/dnd', {credentials:'same-origin'});
      if (res.ok) {
        var d = await res.json();
        document.getElementById('dndEnabled').checked = !!d.enabled;
        if (d.start_time) document.getElementById('dndStart').value = d.start_time.substring(0,5);
        if (d.end_time) document.getElementById('dndEnd').value = d.end_time.substring(0,5);
      }
    } catch(e) {}
  }

  document.getElementById('dndForm').onsubmit = async function(e) {
    e.preventDefault();
    var body = {
      enabled: document.getElementById('dndEnabled').checked,
      start_time: document.getElementById('dndStart').value + ':00',
      end_time: document.getElementById('dndEnd').value + ':00'
    };
    var res = await fetch('/api/notification-settings/dnd', {method:'PATCH', headers:{'Content-Type':'application/json'}, body:JSON.stringify(body), credentials:'same-origin'});
    if (res.ok) {
      document.getElementById('dndState').textContent = 'Saved.';
      if(window.parkopsToast) window.parkopsToast('DND settings saved','success');
    } else {
      var d = await res.json();
      document.getElementById('dndState').textContent = d.message || 'Failed';
    }
  };

  function esc(s) { var d=document.createElement('div'); d.textContent=s; return d.innerHTML; }
  loadTopics();
  loadDND();
})();
</script>`
		return AppLayout(user, "Notification Preferences", "/notification-prefs", content).Render(ctx, w)
	})
}
